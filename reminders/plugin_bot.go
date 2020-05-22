package reminders

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents2"
	seventsmodels "github.com/jonas747/yagpdb/common/scheduledevents2/models"
)

var logger = common.GetPluginLogger(&Plugin{})

var _ bot.BotInitHandler = (*Plugin)(nil)
var _ commands.CommandProvider = (*Plugin)(nil)

func (p *Plugin) AddCommands() {
	commands.AddRootCommands(p, cmds...)
}

func (p *Plugin) BotInit() {
	// scheduledevents.RegisterEventHandler("reminders_check_user", checkUserEvtHandlerLegacy)
	scheduledevents2.RegisterHandler("reminders_check_user", int64(0), checkUserScheduledEvent)
	scheduledevents2.RegisterLegacyMigrater("reminders_check_user", migrateLegacyScheduledEvents)
}

// Reminder management commands
var cmds = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "Remindme",
		Description:  "Schedules a reminder, example: 'remindme 1h30min are you alive still?'",
		Aliases:      []string{"remind", "reminder"},
		RequiredArgs: 2,
		Arguments: []*dcmd.ArgDef{
			&dcmd.ArgDef{Name: "Time", Type: &commands.DurationArg{}},
			&dcmd.ArgDef{Name: "Message", Type: dcmd.String},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			currentReminders, _ := GetUserReminders(parsed.Msg.Author.ID)
			if len(currentReminders) >= 25 {
				return "You can have a maximum of 25 active reminders, list your reminders with the `reminders` command", nil
			}

			fromNow := parsed.Args[0].Value.(time.Duration)

			durString := common.HumanizeDuration(common.DurationPrecisionSeconds, fromNow)
			when := time.Now().Add(fromNow)
			tStr := when.UTC().Format(time.RFC822)

			if when.After(time.Now().Add(time.Hour * 24 * 366)) {
				return "Can be max 365 days from now...", nil
			}

			_, err := NewReminder(parsed.Msg.Author.ID, parsed.GS.ID, parsed.CS.ID, parsed.Args[1].Str(), when)
			if err != nil {
				return nil, err
			}

			return "Set a reminder in " + durString + " from now (" + tStr + ")\nView reminders with the reminders command", nil
		},
	},
	&commands.YAGCommand{
		CmdCategory: commands.CategoryTool,
		Name:        "Reminders",
		Description: "Lists your active reminders",
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			currentReminders, err := GetUserReminders(parsed.Msg.Author.ID)
			if err != nil {
				return nil, err
			}

			out := "Your reminders:\n"
			out += stringReminders(currentReminders, false)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	&commands.YAGCommand{
		CmdCategory: commands.CategoryTool,
		Name:        "CReminders",
		Description: "Lists reminders in channel, only users with 'manage server' permissions can use this.",
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {
			ok, err := bot.AdminOrPermMS(parsed.CS.ID, parsed.MS, discordgo.PermissionManageChannels)
			if err != nil {
				return nil, err
			}
			if !ok {
				return "You do not have access to this command (requires manage channel permission)", nil
			}

			currentReminders, err := GetChannelReminders(parsed.CS.ID)
			if err != nil {
				return nil, err
			}

			out := "Reminders in this channel:\n"
			out += stringReminders(currentReminders, true)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "DelReminder",
		Aliases:      []string{"rmreminder"},
		Description:  "Deletes a reminder.",
		RequiredArgs: 1,
		Arguments: []*dcmd.ArgDef{
			{Name: "ID", Type: dcmd.Int},
		},
		RunFunc: func(parsed *dcmd.Data) (interface{}, error) {

			var reminder Reminder
			err := common.GORM.Where(parsed.Args[0].Int()).First(&reminder).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return "No reminder by that id found", nil
				}
				return "Error retrieving reminder", err
			}

			if reminder.GuildID != parsed.GS.ID {
				return "That reminder is not from this server", nil
			}

			// Check perms
			if reminder.UserID != discordgo.StrID(parsed.Msg.Author.ID) {
				ok, err := bot.AdminOrPermMS(reminder.ChannelIDInt(), parsed.MS, discordgo.PermissionManageChannels)
				if err != nil {
					return nil, err
				}
				if !ok {
					return "You need manage channel permission in the channel the reminder is in to delete reminders that are not your own", nil
				}
			}

			// Do the actual deletion
			err = common.GORM.Delete(reminder).Error
			if err != nil {
				return nil, err
			}

			// Check if we should remove the scheduled event
			currentReminders, err := GetUserReminders(reminder.UserIDInt())
			if err != nil {
				return nil, err
			}

			delMsg := fmt.Sprintf("Deleted reminder **#%d**: %q", reminder.ID, reminder.Message)

			// If there is another reminder with the same timestamp, do not remove the scheduled event
			for _, v := range currentReminders {
				if v.When == reminder.When {
					return delMsg, nil
				}
			}

			return delMsg, nil
		},
	},
}

func stringReminders(reminders []*Reminder, displayUsernames bool) string {
	out := ""
	for _, v := range reminders {
		parsedCID, _ := strconv.ParseInt(v.ChannelID, 10, 64)

		t := time.Unix(v.When, 0)
		timeFromNow := common.HumanizeTime(common.DurationPrecisionMinutes, t)
		tStr := t.Format(time.RFC822)
		if !displayUsernames {
			channel := "<#" + discordgo.StrID(parsedCID) + ">"
			out += fmt.Sprintf("**%d**: %s: %q - %s from now (%s)\n", v.ID, channel, v.Message, timeFromNow, tStr)
		} else {
			member, _ := bot.GetMember(v.GuildID, v.UserIDInt())
			username := "Unknown user"
			if member != nil {
				username = member.Username
			}
			out += fmt.Sprintf("**%d**: %s: %q - %s from now (%s)\n", v.ID, username, v.Message, timeFromNow, tStr)
		}
	}
	return out
}

func checkUserScheduledEvent(evt *seventsmodels.ScheduledEvent, data interface{}) (retry bool, err error) {
	// !important! the evt.GuildID can be 1 in cases where it was migrated from the legacy scheduled event system

	userID := *data.(*int64)

	reminders, err := GetUserReminders(userID)
	if err != nil {
		return true, err
	}

	now := time.Now()
	nowUnix := now.Unix()
	for _, v := range reminders {
		if v.When <= nowUnix {
			err := v.Trigger()
			if err != nil {
				// possibly try again
				return scheduledevents2.CheckDiscordErrRetry(err), err
			}
		}
	}

	return false, nil
}

func migrateLegacyScheduledEvents(t time.Time, data string) error {
	split := strings.Split(data, ":")
	if len(split) < 2 {
		logger.Error("invalid check user scheduled event: ", data)
		return nil
	}

	parsed, _ := strconv.ParseInt(split[1], 10, 64)

	return scheduledevents2.ScheduleEvent("reminders_check_user", 1, t, parsed)
}
