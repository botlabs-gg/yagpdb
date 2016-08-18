package commands

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/common"
	"log"
	"strings"
	"time"
)

var (
	RKeyCommandCooldown = func(uID, cmd string) string { return "cmd_cd:" + uID + ":" + cmd }
)

// Slight extension to the simplecommand, it will check if the command is enabled in the HandleCommand func
// And invoke a custom handlerfunc with provided redis client
type CustomCommand struct {
	*commandsystem.SimpleCommand
	Key      string // GuildId is appended to the key, e.g if key is "test:", it will check for "test:<guildid>"
	Default  bool   // The default state of this command
	Cooldown int    // Cooldown in seconds before user can use it again
	RunFunc  func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error)
}

func (cs *CustomCommand) HandleCommand(raw string, source commandsystem.CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) error {
	if source == commandsystem.CommandSourceDM {
		return errors.New("Cannot run this command in direct messages")
	}

	client, err := common.RedisPool.Get()
	if err != nil {
		log.Println("Failed retrieving redis client:", err)
		return errors.New("Failed retrieving redis client")
	}
	defer common.RedisPool.Put(client)

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	guild, err := s.State.Guild(channel.GuildID)
	if err != nil {
		return err
	}

	// Check wether it's enabled or not
	enabled, autodel, err := cs.Enabled(client, channel.ID, guild)
	if err != nil {
		s.ChannelMessageSend(channel.ID, "Bot is having issues... contact the junas D:")
		return err
	}

	if !enabled {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The %q command is currently disabled on this server. Server admins of the server can enabled it through the control panel <%s>.", cs.Name, common.Conf.Host))
		return nil
	}

	cdLeft, err := cs.CooldownLeft(client, m.Author.ID)
	if err != nil {
		// Just pretend the cooldown is off...
		log.Println("Failed checking command cooldown", err)
	}

	if cdLeft > 0 {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("**%q:** You need to wait %d seconds before you can use the %q command again", m.Author.Username, cdLeft, cs.Name))
		return nil
	}

	parsed, err := cs.ParseCommand(raw, m, s)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Failed parsing command: "+CensorError(err))
		return nil
	}

	parsed.Source = source
	parsed.Channel = channel
	parsed.Guild = guild

	if cs.RunFunc != nil {
		resp, err := cs.RunFunc(parsed, client, m)
		if resp != nil {
			out := ""

			if err, ok := resp.(error); ok {
				out = "Error: " + err.Error()
			} else if str, ok := resp.(string); ok {
				out = str
			}

			if out != "" {
				msgs, err := dutil.SplitSendMessage(s, m.ChannelID, out)
				// Autodelete response if enabled
				if autodel && err == nil {
					go cs.deleteResponse(msgs)
				}
			}
		}
		if err == nil {
			err = cs.SetCooldown(client, m.Author.ID)
			if err != nil {
				log.Println("Failed setting cooldown", err)
			}
		}
		return err
	}

	return nil
}

func (cs *CustomCommand) deleteResponse(msgs []*discordgo.Message) {
	ids := make([]string, len(msgs))
	for k, msg := range msgs {
		ids[k] = msg.ID
	}

	if len(ids) < 1 {
		return // ...
	}

	time.Sleep(time.Second * 10)

	// Either do a bulk delete or single delete depending on how big the response was
	if len(ids) > 1 {
		common.BotSession.ChannelMessagesBulkDelete(msgs[0].ChannelID, ids)
	} else {
		common.BotSession.ChannelMessageDelete(msgs[0].ChannelID, msgs[0].ID)
	}
}

// customEnabled returns wether the command is enabled by it's custom key or not
func (cs *CustomCommand) customEnabled(client *redis.Client, guildID string) (bool, error) {
	// No special key so it's automatically enabled
	if cs.Key == "" {
		return true, nil
	}

	// Check redis for settings
	reply := client.Cmd("GET", cs.Key+guildID)
	if reply.Err != nil {
		return false, reply.Err
	}

	enabled, _ := reply.Bool()

	if cs.Default {
		enabled = !enabled
	}

	if !enabled {
		return false, nil
	}

	return enabled, nil
}

// Enabled returns wether the command is enabled or not
func (cs *CustomCommand) Enabled(client *redis.Client, channel string, guild *discordgo.Guild) (enabled bool, autodel bool, err error) {
	ce, err := cs.customEnabled(client, guild.ID)
	if err != nil {
		return false, false, err
	}
	if !ce {
		return false, false, nil
	}

	config := GetConfig(client, guild.ID, guild.Channels)

	// Check overrides first to see if one was enabled, and if so determine if the command is available
	for _, override := range config.ChannelOverrides {
		if override.Channel == channel {
			if override.OverrideEnabled {
				// Find settings for this command
				for _, cmd := range override.Settings {
					if cmd.Cmd == cs.Name {
						return cmd.CommandEnabled, cmd.AutoDelete, nil
					}
				}

			}
			break
		}
	}

	// Return from global settings then
	for _, cmd := range config.Global {
		if cmd.Cmd == cs.Name {
			return cmd.CommandEnabled, cmd.AutoDelete, nil
		}
	}

	return false, false, nil
}

// CooldownLeft returns the number of seconds before a command can be used again
func (cs *CustomCommand) CooldownLeft(client *redis.Client, userID string) (int, error) {
	if cs.Cooldown < 1 {
		return 0, nil
	}

	ttl, err := client.Cmd("TTL", RKeyCommandCooldown(userID, cs.Name)).Int64()
	if ttl < 1 {
		return 0, nil
	}

	return int(ttl), err
}

// SetCooldown sets the cooldown of the command as it's defined in the struct
func (cs *CustomCommand) SetCooldown(client *redis.Client, userID string) error {
	if cs.Cooldown < 1 {
		return nil
	}
	now := time.Now().Unix()
	client.Append("SET", RKeyCommandCooldown(userID, cs.Name), now)
	client.Append("EXPIRE", RKeyCommandCooldown(userID, cs.Name), cs.Cooldown)
	_, err := common.GetRedisReplies(client, 2)
	return err
}

// Keys and other sensitive information shouldnt be sent in error messages, but just in case it is
func CensorError(err error) string {
	toCensor := []string{
		common.BotSession.Token,
		common.Conf.ClientSecret,
		common.Conf.PastebinDevKey,
	}

	out := err.Error()
	for _, c := range toCensor {
		out = strings.Replace(out, c, "", -1)
	}

	return out
}
