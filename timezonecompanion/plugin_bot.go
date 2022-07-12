package timezonecompanion

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/bot/eventsystem"
	"github.com/botlabs-gg/yagpdb/v2/bot/paginatedmessages"
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/timezonecompanion/models"
	"github.com/volatiletech/sqlboiler/boil"
)

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) BotInit() {
	eventsystem.AddHandlerAsyncLastLegacy(p, p.handleMessageCreate, eventsystem.EventMessageCreate)
}

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, &commands.YAGCommand{
		CmdCategory: commands.CategoryTool,
		Name:        "settimezone",
		Aliases:     []string{"setz", "tzset"},
		Description: "Sets your timezone, used for various purposes such as auto conversion. Give it your country.",
		Arguments: []*dcmd.ArgDef{
			{Name: "Timezone", Type: dcmd.String},
		},
		ArgSwitches: []*dcmd.ArgDef{
			{Name: "u", Help: "Display current"},
			{Name: "d", Help: "Delete TZ record"},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {

			localTZ := time.Now().Location()
			userZone, userOffset := time.Now().In(localTZ).Zone()
			getUserTZ := GetUserTimezone(parsed.Author.ID)
			tzState := "server's"

			if getUserTZ != nil {
				userZone, userOffset = time.Now().In(getUserTZ).Zone()
				localTZ = getUserTZ
				tzState = "registered to"
			}

			humanizeOffset := fmt.Sprintf("%+d", userOffset/3600)
			if (userOffset % 3600 / 60) != 0 {
				humanizeOffset += fmt.Sprintf(":%d", int(math.Abs(float64(userOffset%3600/60))))
			}

			userTZ := fmt.Sprintf("Your current time zone is %s: `%s` %s (UTC%s)", tzState, localTZ, userZone, humanizeOffset)

			if parsed.Switches["u"].Value != nil && parsed.Switches["u"].Value.(bool) {
				return userTZ, nil
			}

			if parsed.Switches["d"].Value != nil && parsed.Switches["d"].Value.(bool) {
				if getUserTZ != nil {

					m := &models.UserTimezone{
						UserID:       parsed.Author.ID,
						TimezoneName: localTZ.String(),
					}
					_, err := m.DeleteG(parsed.Context())
					if err != nil {
						return nil, err
					}
					return "Deleted", nil
				} else {
					return "You don't have a registered time zone", nil
				}
			}

			zones := FindZone(parsed.Args[0].Str())
			// No zones matching user input
			if len(zones) < 1 {
				return fmt.Sprintf("Unknown timezone, enter a country or timezone (not abbreviation like CET). there's a timezone picker here: <https://kevinnovak.github.io/Time-Zone-Picker/> you can use, enter the `Area/City` result\n\n%s", userTZ), nil
			}
			// Multiple zones matching user input
			note := ""
			zone := ""
			if len(zones) > 1 {
				if len(zones) > 10 {
					if parsed.Context().Value(paginatedmessages.CtxKeyNoPagination) != nil {
						return paginatedTimezones(zones)(nil, 1)
					}
					_, err := paginatedmessages.CreatePaginatedMessage(
						parsed.GuildData.GS.ID, parsed.ChannelID, 1, int(math.Ceil(float64(len(zones))/10)), paginatedTimezones(zones))
					return nil, err
				}

				matches := ""
				for _, v := range zones {
					if s := StrZone(v); s != "" {
						matches += s + "\n"
					}
				}
				// "matches" now contains all zones, as newline-separated list

				// Check whether the requested zone has an exact match in zones
				found := false
				for n, candidate := range zones {
					if strings.ToLower(candidate) == strings.ToLower(parsed.Args[0].Str()) {
						found = true
						// Select matching zone
						zone = zones[n]
						// Set a note for the user
						note = "Other matching timezones were found, you can reuse the command with any of them:\n" + matches
					}
				}
				if !found {
					out := "More than 1 result, reuse the command with one of the following:\n" + matches + "\n" + userTZ
					return out, nil
				}
			} else {
				zone = zones[0]
			}

			// Here, either `zones` is of length 1, or one zone of several is an exact match
			// Either way, `zone` is already set to the proper value
			loc, err := time.LoadLocation(zone)
			if err != nil {
				return fmt.Sprintf("Unknown timezone `%s`", zone), nil
			}

			name, _ := time.Now().In(loc).Zone()

			m := &models.UserTimezone{
				UserID:       parsed.Author.ID,
				TimezoneName: zone,
			}

			err = m.UpsertG(parsed.Context(), true, []string{"user_id"}, boil.Infer(), boil.Infer())
			if err != nil {
				return nil, err
			}

			return fmt.Sprintf("Set your timezone to `%s`: %s\n%s", zone, name, note), nil
			// Note that an empty "note" variable will be invisible, since Discord trims trailing message whitespace
		},
	}, &commands.YAGCommand{
		CmdCategory:         commands.CategoryTool,
		Name:                "ToggleTimeConversion",
		Aliases:             []string{"toggletconv", "ttc"},
		Description:         "Toggles automatic time conversion for people with registered timezones (setz) in this channel, it's on by default, toggle all channels by giving it `all`",
		RequireDiscordPerms: []int64{discordgo.PermissionManageMessages, discordgo.PermissionManageServer},
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "flags", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			allStr := parsed.Args[0].Str()
			all := false
			if strings.EqualFold(allStr, "all") || strings.EqualFold(allStr, "*") {
				all = true
			}

			insert := false
			conf, err := models.FindTimezoneGuildConfigG(parsed.Context(), parsed.GuildData.GS.ID)
			if err != nil {
				if err == sql.ErrNoRows {
					conf = &models.TimezoneGuildConfig{
						GuildID: parsed.GuildData.GS.ID,
					}
					insert = true
				} else {
					return nil, err
				}
			}

			resp := ""
			if all {
				if conf.NewChannelsDisabled {
					conf.NewChannelsDisabled = false
					conf.DisabledInChannels = []int64{}
					resp = "Enabled time conversion in all channels."
				} else {
					conf.NewChannelsDisabled = true
					conf.EnabledInChannels = []int64{}
					resp = "Disabled time conversion in all channels, including newly created channels."
				}
			} else {
				status := "off"

				found := false
				for i, v := range conf.DisabledInChannels {
					if v == parsed.ChannelID {
						found = true
						conf.DisabledInChannels = append(conf.DisabledInChannels[:i], conf.DisabledInChannels[i+1:]...)
						status = "on"

						if conf.NewChannelsDisabled {
							conf.EnabledInChannels = append(conf.EnabledInChannels, parsed.ChannelID)
						}

						break
					}
				}

				if !found {
					conf.DisabledInChannels = append(conf.DisabledInChannels, parsed.ChannelID)

					for i, v := range conf.EnabledInChannels {
						if v == parsed.ChannelID {
							conf.EnabledInChannels = append(conf.EnabledInChannels[:i], conf.EnabledInChannels[i+1:]...)
						}
					}
				}

				resp = fmt.Sprintf("Automatic time conversion in this channel toggled `%s`", status)
			}

			if insert {
				err = conf.InsertG(parsed.Context(), boil.Infer())
			} else {
				_, err = conf.UpdateG(parsed.Context(), boil.Infer())
			}

			if err != nil {
				return nil, err
			}

			return resp, nil
		},
	})
}

func StrZone(zone string) string {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return ""
	}

	name, _ := time.Now().In(loc).Zone()

	return fmt.Sprintf("`%s`: %s", zone, name)
}

func paginatedTimezones(timezones []string) func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
	return func(p *paginatedmessages.PaginatedMessage, page int) (*discordgo.MessageEmbed, error) {
		numSkip := (page - 1) * 10

		out := ""
		numAdded := 0
		for i := numSkip; i < len(timezones); i++ {
			if s := StrZone(timezones[i]); s != "" {
				out += s + "\n"
				numAdded++
				if numAdded >= 10 {
					break
				}
			}
		}

		return &discordgo.MessageEmbed{
			Description: "Please redo the command with one of the following:\n" + out,
		}, nil
	}
}

func GetUserTimezone(userID int64) *time.Location {
	m, err := models.FindUserTimezoneG(context.Background(), userID)
	if err != nil {
		return nil
	}

	loc, err := time.LoadLocation(m.TimezoneName)
	if err != nil {
		logger.WithError(err).Error("failed loading location")
		return nil
	}

	return loc
}

func FindZone(in string) []string {
	lowerIn := strings.ToLower(in)
	inSpaceReplaced := strings.ReplaceAll(lowerIn, " ", "_")

	ccs := make([]string, 0)
	for country, code := range CountryCodes {
		if strings.Contains(strings.ToLower(country), lowerIn) {
			ccs = appendIfNotExists(ccs, code)
		}
	}

	matchesZones := make([]string, 0)

	for code, zones := range CCToZones {
		// if common.ContainsString()

		// check if we specified the country
		if common.ContainsStringSlice(ccs, code) || strings.EqualFold(code, lowerIn) {
			for _, v := range zones {
				matchesZones = appendIfNotExists(matchesZones, v)
			}

			continue
		}

		for _, v := range zones {
			if strings.Contains(strings.ToLower(v), inSpaceReplaced) {
				matchesZones = appendIfNotExists(matchesZones, v)
			}
		}
	}

	return matchesZones
}

func appendIfNotExists(in []string, elem string) []string {
	if !common.ContainsStringSlice(in, elem) {
		return append(in, elem)
	}

	return in
}

func (p *Plugin) handleMessageCreate(evt *eventsystem.EventData) {
	m := evt.MessageCreate()
	if m.GuildID == 0 {
		return
	}

	//Additional check to ensure not reacting to own message
	if m.Author.ID == common.BotUser.ID {
		return
	}

	result, err := p.DateParser.Parse(m.Content, time.Now())
	if err != nil || result == nil {
		return
	}

	conf, err := models.FindTimezoneGuildConfigG(evt.Context(), m.GuildID)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.WithError(err).WithField("guild", m.GuildID).Error("failed fetching guild config")
			return
		}
	} else if common.ContainsInt64Slice(conf.DisabledInChannels, m.ChannelID) || (conf.NewChannelsDisabled && !common.ContainsInt64Slice(conf.EnabledInChannels, m.ChannelID)) {
		// disabled in this channel
		return
	}

	zone := GetUserTimezone(m.Author.ID)
	if zone == nil {
		return
	}

	// re-parse it with proper context
	result, err = p.DateParser.Parse(m.Content, time.Now().In(zone))
	if err != nil || result == nil {
		return
	}

	common.BotSession.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Timestamp: result.Time.Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Above time (" + result.Time.Format("15:04 MST") + ") in your local time",
		},
	})

	// common.BotSession.ChannelMessageSend(m.ChannelID, "Time: `"+result.Time.Format(time.RFC822)+"`")
}
