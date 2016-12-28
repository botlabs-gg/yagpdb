package commands

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/dutil/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"strings"
	"time"
)

type ContextKey int

const (
	CtxKeyRedisClient ContextKey = iota
)

type CommandCategory string

const (
	CategoryGeneral    CommandCategory = "General"
	CategoryTool       CommandCategory = "Tools"
	CategoryModeration CommandCategory = "Moderation"
	CategoryFun        CommandCategory = "Misc/Fun"
)

var (
	RKeyCommandCooldown = func(uID, cmd string) string { return "cmd_cd:" + uID + ":" + cmd }
)

// Slight extension to the simplecommand, it will check if the command is enabled in the HandleCommand func
// And invoke a custom handlerfunc with provided redis client
type CustomCommand struct {
	*commandsystem.Command
	HideFromCommandsPage bool   // Set to  hide this command from the commands page
	Key                  string // GuildId is appended to the key, e.g if key is "test:", it will check for "test:<guildid>"
	CustomEnabled        bool   // Set to true to handle the enable check itself
	Default              bool   // The default state of this command
	Cooldown             int    // Cooldown in seconds before user can use it again
	Category             CommandCategory
}

func (cs *CustomCommand) HandleCommand(raw string, trigger *commandsystem.TriggerData, ctx context.Context) ([]*discordgo.Message, error) {
	// Track how long execution of a command took
	started := time.Now()
	defer func() {
		cs.logExecutionTime(time.Since(started), raw, trigger.Message.Author.Username)
	}()

	// Need a redis client to check cooldowns and retrieve command settings
	client, err := common.RedisPool.Get()
	if err != nil {
		log.WithError(err).Error("Failed retrieving redis client")
		return nil, errors.New("Failed retrieving redis client")
	}
	defer common.RedisPool.Put(client)

	cState := bot.State.Channel(true, trigger.Message.ChannelID)
	if cState == nil {
		return nil, errors.New("Channel not found")
	}

	var guild *dstate.GuildState
	var autodel bool

	if trigger.Source != commandsystem.SourceDM {

		guild = cState.Guild
		if guild == nil {
			return nil, errors.New("Guild not found")
		}

		var enabled bool
		// Check wether it's enabled or not
		enabled, autodel, err = cs.Enabled(client, cState.ID(), guild)
		if err != nil {
			trigger.Session.ChannelMessageSend(cState.ID(), "Bot is having issues... contact the bot author D:")
			return nil, err
		}

		if !enabled {
			go common.SendTempMessage(trigger.Session, time.Second*10, trigger.Message.ChannelID, fmt.Sprintf("The %q command is currently disabled on this server or channel. *(Control panel to enable/disable <https://%s>)*", cs.Name, common.Conf.Host))
			return nil, nil
		}
	}

	cdLeft, err := cs.CooldownLeft(client, trigger.Message.Author.ID)
	if err != nil {
		// Just pretend the cooldown is off...
		log.WithError(err).Error("Failed checking command cooldown")
	}

	if cdLeft > 0 {
		trigger.Session.ChannelMessageSend(trigger.Message.ChannelID, fmt.Sprintf("**%q:** You need to wait %d seconds before you can use the %q command again", trigger.Message.Author.Username, cdLeft, cs.Name))
		return nil, nil
	}

	// parsed, err := cs.ParseCommand(raw, m, s)
	// if err != nil {
	// 	trigger.Session.ChannelMessageSend(m.ChannelID, "Failed parsing command: "+CensorError(err))
	// 	return nil, nil
	// }

	// parsed.Source = source
	// parsed.Channel = channel
	// parsed.Guild = guild

	replies, err := cs.Command.HandleCommand(raw, trigger, context.WithValue(ctx, CtxKeyRedisClient, client))

	if len(replies) > 0 && autodel {
		go cs.deleteResponse(append(replies, trigger.Message))
	}

	if err == nil {
		err = cs.SetCooldown(client, trigger.Message.Author.ID)
		if err != nil {
			log.WithError(err).Error("Failed setting cooldown")
		}
	}
	return replies, err
}

func (cs *CustomCommand) logExecutionTime(dur time.Duration, raw string, sender string) {
	log.Infof("Handled Command [%4dms] %s: %s", int(dur.Seconds()*1000), sender, raw)
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
	if cs.Key == "" || cs.CustomEnabled {
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
func (cs *CustomCommand) Enabled(client *redis.Client, channel string, gState *dstate.GuildState) (enabled bool, autodel bool, err error) {
	gState.RLock()
	defer gState.RUnlock()

	if cs.HideFromCommandsPage {
		return true, false, nil
	}

	ce, err := cs.customEnabled(client, gState.ID())
	if err != nil {
		return false, false, err
	}
	if !ce {
		return false, false, nil
	}

	channels := make([]*discordgo.Channel, len(gState.Channels))
	i := 0
	for _, v := range gState.Channels {
		channels[i] = v.Channel
		i++
	}

	config := GetConfig(client, gState.ID(), channels)

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

	if cs.Key != "" || cs.CustomEnabled {
		return true, false, nil
	}

	// Return from global settings then
	for _, cmd := range config.Global {
		if cmd.Cmd == cs.Name {
			if cs.Key != "" {
				return true, cmd.AutoDelete, nil
			}

			return cmd.CommandEnabled, cmd.AutoDelete, nil
		}
	}

	return false, false, nil
}

// CooldownLeft returns the number of seconds before a command can be used again
func (cs *CustomCommand) CooldownLeft(client *redis.Client, userID string) (int, error) {
	if cs.Cooldown < 1 || common.Testing {
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
	}

	out := err.Error()
	for _, c := range toCensor {
		out = strings.Replace(out, c, "", -1)
	}

	return out
}
