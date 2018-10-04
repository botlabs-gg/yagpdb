package soundboard

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"strings"
)

func (p *Plugin) BotInit() {
	go transcoderLoop()
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(&commands.YAGCommand{
		CmdCategory: commands.CategoryFun,
		Name:        "Soundboard",
		Aliases:     []string{"sb"},
		Description: "Play, or list soundboard sounds",
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Name", Type: dcmd.String},
		},
		RunFunc: func(data *dcmd.Data) (interface{}, error) {
			config := &SoundboardConfig{}
			err := configstore.Cached.GetGuildConfig(context.Background(), data.GS.ID, config)
			if err != nil {
				return nil, errors.WithMessage(err, "GetGuildConfig")
			}

			// Get member from api or state
			member, err := bot.GetMember(data.GS.ID, data.Msg.Author.ID)
			if err != nil {
				return nil, errors.WithMessage(err, "GetMember")
			}

			if data.Args[0].Str() == "" {
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

			data.GS.RLock()
			defer data.GS.RUnlock()

			var voiceChannel int64
			vs := data.GS.VoiceState(false, data.Msg.Author.ID)
			if vs != nil {
				voiceChannel = vs.ChannelID
			}

			if voiceChannel == 0 {
				return "You're not in a voice channel", nil
			}

			if RequestPlaySound(data.GS.ID, voiceChannel, data.Msg.ChannelID, sound.ID) {
				return "Queued up", nil
			}

			return "Playing it now", nil
		},
	})
}

func ListSounds(config *SoundboardConfig, ms *dstate.MemberState) string {
	canPlay := ""
	restricted := ""

	for _, sound := range config.Sounds {
		if sound.CanPlay(ms.Roles) {
			canPlay += "`" + sound.Name + "`, "
		} else {
			restricted += "`" + sound.Name + "`, "
		}
	}
	out := "Soundboard sounds:\n"

	if canPlay != "" {
		out += "Can Play: " + canPlay[:len(canPlay)-2] + "\n"
	}
	if restricted != "" {
		out += "No access: " + restricted[:len(restricted)-2] + "\n"
	}

	out += "\nPlay a sound with `sb <soundname>`"
	return out
}
