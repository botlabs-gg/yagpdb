package soundboard

import (
	"errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common/configstore"
	"golang.org/x/net/context"
	"strings"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(&commands.CustomCommand{
		Category: commands.CategoryFun,
		Command: &commandsystem.Command{
			Name:        "Soundboard",
			Aliases:     []string{"sb"},
			Description: "Play, or list soundboard sounds",
			Arguments: []*commandsystem.ArgDef{
				&commandsystem.ArgDef{Name: "Name", Type: commandsystem.ArgumentString},
			},
			Run: func(data *commandsystem.ExecData) (interface{}, error) {
				config := &SoundboardConfig{}
				err := configstore.Cached.GetGuildConfig(context.Background(), data.Guild.ID(), config)
				if err != nil {
					return "Something bad is happenings..", err
				}

				// Get member from api or state
				member, err := bot.GetMember(data.Guild.ID(), data.Message.Author.ID)
				if err != nil {
					return "Something went wrong, we couldn't find you?", errors.New("Failed finding guild member")
				}

				if data.Args[0] == nil || data.Args[0].Str() == "" {
					return ListSounds(config, member), nil
				}

				var sound *SoundboardSound
				for _, v := range config.Sounds {
					if strings.ToLower(v.Name) == strings.ToLower(data.Args[0].Str()) {
						sound = v
						break
					}
				}
				if sound == nil {
					return "Sound not found, " + ListSounds(config, member), nil
				} else if !sound.CanPlay(member.Roles) {
					return "You can't play that sound, " + ListSounds(config, member), nil
				}

				data.Guild.RLock()
				defer data.Guild.RUnlock()

				voiceChannel := ""
				vs := data.Guild.VoiceState(false, data.Message.Author.ID)
				if vs != nil {
					voiceChannel = vs.ChannelID
				}

				if voiceChannel == "" {
					return "You're not in a voice channel stopid.", nil
				}

				if RequestPlaySound(data.Guild.ID(), voiceChannel, sound.ID) {
					return "Sure why not", nil
				}

				return "Ayay", nil
			},
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
