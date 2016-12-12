package soundboard

import (
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"golang.org/x/net/context"
	"strings"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		Category: commands.CategoryFun,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Soundboard",
			Aliases:     []string{"sb"},
			Description: "Play, or list soundboard sounds",
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Name", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			config := &SoundboardConfig{}
			err := configstore.Cached.GetGuildConfig(context.Background(), parsed.Guild.ID, config)
			if err != nil {
				return "Something bad is happenings..", err
			}

			member, err := common.GetGuildMember(common.BotSession, parsed.Guild.ID, m.Author.ID)
			if err != nil {
				return "Uh oh i could't find you /shrug", err
			}

			if parsed.Args[0] == nil || parsed.Args[0].Str() == "" {
				return ListSounds(config, member), nil
			}

			var sound *SoundboardSound
			for _, v := range config.Sounds {
				if strings.ToLower(v.Name) == strings.ToLower(parsed.Args[0].Str()) {
					sound = v
					break
				}
			}
			if sound == nil {
				return "Sound not found, " + ListSounds(config, member), nil
			} else if !sound.CanPlay(member.Roles) {
				return "You can't play that sound, " + ListSounds(config, member), nil
			}

			voiceChannel := ""
			for _, v := range parsed.Guild.VoiceStates {
				if v.UserID == m.Author.ID {
					voiceChannel = v.ChannelID
					break
				}
			}
			if voiceChannel == "" {
				return "You're not in a voice channel stopid.", nil
			}

			if RequestPlaySound(parsed.Guild.ID, voiceChannel, sound.ID) {
				return "Sure why not", nil
			}
			return "Ayay", nil
		},
	})
}

func ListSounds(config *SoundboardConfig, member *discordgo.Member) string {
	canPlay := ""
	restricted := ""

	for _, sound := range config.Sounds {
		if sound.CanPlay(member.Roles) {
			canPlay += "`" + sound.Name + "`, "
		} else {
			restricted += "`" + sound.Name + "`, "
		}
	}
	out := "Sounboard sounds:\n"

	if canPlay != "" {
		out += "Can Play: " + canPlay[:len(canPlay)-2] + "\n"
	}
	if restricted != "" {
		out += "No access: " + restricted[:len(restricted)-2] + "\n"
	}

	out += "\nPlay a sound with `sb <soundname>`"
	return out
}
