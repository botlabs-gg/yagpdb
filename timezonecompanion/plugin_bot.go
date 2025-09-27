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
	"github.com/botlabs-gg/yagpdb/v2/commands"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/lib/dcmd"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/timezonecompanion/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
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
		Description: "Sets your timezone, used for various purposes such as auto conversion. Give it a TZ identifier as [listed on Wikipedia](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).",
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

			zone := parsed.Args[0].Str()
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

			return fmt.Sprintf("Set your timezone to `%s`: %s\n", zone, name), nil
		},
	}, &commands.YAGCommand{
		CmdCategory:         commands.CategoryTool,
		Name:                "ToggleTimeConversion",
		Aliases:             []string{"toggletconv", "ttc"},
		Description:         "Toggles automatic time conversion for people with registered timezones (setz) in this channel, it's on by default, toggle all channels by giving it `all`",
		RequireDiscordPerms: []int64{discordgo.PermissionManageMessages, discordgo.PermissionManageGuild},
		Arguments: []*dcmd.ArgDef{
			{Name: "flags", Type: dcmd.String},
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
