package soundboard

import (
	"github.com/jonas747/dcmd"
	"github.com/jonas747/dstate"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/soundboard/models"
	"github.com/pkg/errors"
	"strings"
)

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
			sounds, err := GetSoundboardSounds(data.GS.ID, data.Context())
			if err != nil {
				return nil, errors.WithMessage(err, "GetSoundboardSounds")
			}

			// Get member from api or state
			member := commands.ContextMS(data.Context())

			if data.Args[0].Str() == "" {
				return ListSounds(sounds, member), nil
			}

			var sound *models.SoundboardSound
			for _, v := range sounds {
				if strings.EqualFold(v.Name, data.Args[0].Str()) {
					sound = v
					break
				}
			}

			if sound == nil {
				return "Sound not found, " + ListSounds(sounds, member), nil
			} else if !CanPlaySound(sound, member.Roles) {
				return "You can't play that sound, " + ListSounds(sounds, member), nil
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

func ListSounds(sounds []*models.SoundboardSound, ms *dstate.MemberState) string {
	canPlay := ""
	restricted := ""

	for _, sound := range sounds {
		if CanPlaySound(sound, ms.Roles) {
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
