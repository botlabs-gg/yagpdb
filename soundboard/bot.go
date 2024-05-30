package soundboard

import (
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/analytics"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/dstate"
	"github.com/botlabs-gg/yagpdb/v2/soundboard/models"
)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p,
		&commands.YAGCommand{
			CmdCategory: commands.CategoryFun,
			Name:        "Soundboard",
			Aliases:     []string{"sb"},
			Description: "Play, or list soundboard sounds",
			Arguments: []*dcmd.ArgDef{
				{Name: "Name", Type: dcmd.String},
			},
			SlashCommandEnabled: true,
			DefaultEnabled:      true,
			RunFunc: func(data *dcmd.Data) (interface{}, error) {
				sounds, err := GetSoundboardSounds(data.GuildData.GS.ID, data.Context())
				if err != nil {
					return nil, errors.WithMessage(err, "GetSoundboardSounds")
				}

				// Get member from api or state
				member := data.GuildData.MS

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
				} else if !CanPlaySound(sound, member.Member.Roles) {
					return "You can't play that sound, either you have a blacklisted role or missing a required role for this sound", nil
				}

				status := TranscodingStatus(sound.Status)
				if status != TranscodingStatusReady {
					switch status {
					case TranscodingStatusQueued:
						return "This sound has yet to be transcoded, if it appear to be stuck in this state then contact support", nil
					case TranscodingStatusFailedLong:
						return "This sound is too long", nil
					case TranscodingStatusFailedOther:
						return "This sound failed transcoding, which means you linked or uploaded a invalid media file. You cannot link youtube videos or web pages, has to be direct links to a media file.", nil
					case TranscodingStatusTranscoding:
						return "This sound is in the process of being converted, please try again in a couple seconds...", nil
					}
				}

				var voiceChannel int64
				vs := data.GuildData.GS.GetVoiceState(data.Author.ID)
				if vs != nil {
					voiceChannel = vs.ChannelID
				}

				if voiceChannel == 0 {
					return "You're not in a voice channel", nil
				}

				go analytics.RecordActiveUnit(data.GuildData.GS.ID, p, "playing sound")

				if RequestPlaySound(data.GuildData.GS.ID, voiceChannel, data.ChannelID, sound.ID) {
					return "Queued up", nil
				}

				return "Playing it now", nil
			},
		},

		&commands.YAGCommand{
			CmdCategory:         commands.CategoryFun,
			Name:                "SoundboardReset",
			Aliases:             []string{"sbclose", "sbReset"},
			Description:         "Reset Soundboard Player",
			SlashCommandEnabled: true,
			DefaultEnabled:      true,
			RunFunc: func(data *dcmd.Data) (interface{}, error) {
				response := resetPlayerServer(data.GuildData.GS.ID)
				if response != "" {
					return response, nil
				}
				return "Reset Complete!", nil
			},
		})
}

func ListSounds(sounds []*models.SoundboardSound, ms *dstate.MemberState) string {
	canPlay := ""
	restricted := ""

	for _, sound := range sounds {
		if CanPlaySound(sound, ms.Member.Roles) {
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
