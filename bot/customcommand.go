package bot

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/common"
	"log"
)

// Slight extension to the simplecommand, it will check if the command is enabled in the CheckMatch func
type CustomCommand struct {
	*commandsystem.SimpleCommand
	Key     string // GuildId is appended to the key, e.g if key is "test:", it will check for "test:<guildid>"
	Default bool
}

func (c *CustomCommand) CheckMatch(raw string, source commandsystem.CommandSource, m *discordgo.MessageCreate, s *discordgo.Session) bool {
	if !c.SimpleCommand.CheckMatch(raw, source, m, s) {
		return false
	}

	if source == commandsystem.CommandSourceDM {
		return false
	}

	client, err := common.RedisPool.Get()
	if err != nil {
		log.Println("Failed checking cmd mathc:", err)
		return false
	}
	defer common.RedisPool.Put(client)

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		log.Println("Failed retrieving channel from state", err)
		return false
	}

	enabled, err := client.Cmd("GET", c.Key+channel.GuildID).Bool()
	if err != nil {
		log.Println("Failed checking command enabled in redis", err)
		return false
	}

	if c.Default {
		enabled = !enabled
	}

	return enabled
}
