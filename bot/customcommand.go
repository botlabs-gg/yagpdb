package bot

import (
	"errors"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/dutil"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/common"
	"log"
	"time"
)

// Slight extension to the simplecommand, it will check if the command is enabled in the HandleCommand func
// And invoke a custom handlerfunc with provided redis client
type CustomCommand struct {
	*commandsystem.SimpleCommand
	Key     string // GuildId is appended to the key, e.g if key is "test:", it will check for "test:<guildid>"
	Default bool   // The default state of this command
	RunFunc func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error)
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

	if cs.Key != "" {

		enabled, err := client.Cmd("GET", cs.Key+channel.GuildID).Bool()
		if err != nil {
			log.Println("Failed checking command enabled in redis", err)
			return errors.New("Bot is having issues... contact the junas (Failed checking redis for enabled command)")
		}

		if cs.Default {
			enabled = !enabled
		}

		if !enabled {
			go common.SendTempMessage(common.BotSession, time.Second*10, m.ChannelID, "This command is disabled, an admin of the discord server can enable it in the control panel")
			return nil
		}

	}

	parsed, err := cs.ParseCommand(raw, m, s)
	if err != nil {
		return err
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
				dutil.SplitSendMessage(s, m.ChannelID, out)
			}
		}
		return err
	}

	return nil
}
