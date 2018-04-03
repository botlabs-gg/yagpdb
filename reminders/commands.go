package reminders

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
	"time"
)

func (p *Plugin) InitBot() {
	commands.AddRootCommands(cmds...)
}

// Reminder management commands
var cmds = []*commands.YAGCommand{
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "Remindme",
		Description:  "Schedules a reminder, example: 'remindme 1h30min are you alive still?'",
		Aliases:      []string{"remind"},
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
			tStr := when.Format(time.RFC822)

			if when.After(time.Now().Add(time.Hour * 24 * 366)) {
				return "Can be max 365 days from now...", nil
			}

			client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
			_, err := NewReminder(client, parsed.Msg.Author.ID, parsed.CS.ID(), parsed.Args[1].Str(), when)
			if err != nil {
				return err, err
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
				return "Failed fetching your reminders, contact bot owner", err
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
			ok, err := bot.AdminOrPerm(discordgo.PermissionManageChannels, parsed.Msg.Author.ID, parsed.CS.ID())
			if err != nil {
				return "An eror occured checkign for perms", err
			}
			if !ok {
				return "You do not have access to this command (requires manage channel permission)", nil
			}

			currentReminders, err := GetChannelReminders(parsed.CS.ID())
			if err != nil {
				return "Failed fetching reminders, contact bot owner", err
			}

			out := "Reminders in this channel:\n"
			out += stringReminders(currentReminders, true)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	&commands.YAGCommand{
		CmdCategory:  commands.CategoryTool,
		Name:         "Delreminder",
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

			// Check perms
			if reminder.UserID != discordgo.StrID(parsed.Msg.Author.ID) {
				ok, err := bot.AdminOrPerm(discordgo.PermissionManageChannels, parsed.Msg.Author.ID, reminder.ChannelIDInt())
				if err != nil {
					return "An eror occured checkign for perms", err
				}
				if !ok {
					return "You need manage channel permission in the channel the reminder is in to delete reminders that are not your own", nil
				}
			}

			// Do the actual deletion
			err = common.GORM.Delete(reminder).Error
			if err != nil {
				return "Failed deleting reminder?", err
			}

			// Check if we should remove the scheduled event
			currentReminders, err := GetUserReminders(reminder.UserIDInt())
			if err != nil {
				return "Failed fetching reminders, contact bot owner", err
			}

			delMsg := fmt.Sprintf("Deleted reminder **#%d**: %q", reminder.ID, reminder.Message)

			// If there is another reminder with the same timestamp, do not remove the scheduled event
			for _, v := range currentReminders {
				if v.When == reminder.When {
					return delMsg, nil
				}
			}

			client := parsed.Context().Value(commands.CtxKeyRedisClient).(*redis.Client)
			// No other reminder for this user at this timestamp, remove the scheduled event
			scheduledevents.RemoveEvent(client, fmt.Sprintf("reminders_check_user:%s", reminder.When), reminder.UserID)

			return delMsg, nil
		},
	},
}

func stringReminders(reminders []*Reminder, displayUsernames bool) string {
	out := ""
	for _, v := range reminders {
		parsedCID, _ := strconv.ParseInt(v.ChannelID, 10, 64)

		cs := bot.State.Channel(true, parsedCID)

		t := time.Unix(v.When, 0)
		timeFromNow := common.HumanizeTime(common.DurationPrecisionMinutes, t)
		tStr := t.Format(time.RFC822)
		if !displayUsernames {
			channel := "Unknown channel"
			if cs != nil {
				channel = "<#" + discordgo.StrID(cs.ID()) + ">"
			}
			out += fmt.Sprintf("**%d**: %s: %q - %s from now (%s)\n", v.ID, channel, v.Message, timeFromNow, tStr)
		} else {
			member, _ := bot.GetMember(cs.Guild.ID(), v.UserIDInt())
			username := "Unknown user"
			if member != nil {
				username = member.User.Username
			}
			out += fmt.Sprintf("**%d**: %s: %q - %s from now (%s)\n", v.ID, username, v.Message, timeFromNow, tStr)
		}
	}
	return out
}
