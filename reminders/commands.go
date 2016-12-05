package reminders

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/dustin/go-humanize"
	"github.com/fzzy/radix/redis"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(cmds...)
}

// Reminder management commands
var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Category: commands.CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Remindme",
			Description:  "Schedules a reminder, example: 'remindme 1h30min are you alive still?'",
			Aliases:      []string{"remind"},
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Time", Type: commandsystem.ArgumentTypeString},
				&commandsystem.ArgumentDef{Name: "Message", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			currentReminders, _ := GetUserReminders(m.Author.ID)
			if len(currentReminders) >= 25 {
				return "You can have a maximum of 25 active reminders, list your reminders with the `reminders` command", nil
			}

			when, err := parseReminderTime(parsed.Args[0].Str())
			if err != nil {
				return err, err
			}

			if when.After(time.Now().Add(time.Hour * 24 * 366)) {
				return "Can be max 265 days from now...", nil
			}

			_, err = NewReminder(client, m.Author.ID, m.ChannelID, parsed.Args[1].Str(), when)
			if err != nil {
				return err, err
			}

			timeFromNow := humanize.Time(when)
			tStr := when.Format(time.RFC822)

			return "Set a reminder for " + timeFromNow + " from now (" + tStr + ")\nView reminders with the reminders command", nil
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "Reminders",
			Description: "Lists your active reminders",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			currentReminders, err := GetUserReminders(m.Author.ID)
			if err != nil {
				return "Failed fetching your reminders, contact bot owner", err
			}

			out := "Your reminders:\n"
			out += stringReminders(common.BotSession.State, currentReminders, false)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:        "CReminders",
			Description: "Lists reminders in channel, only users with 'manage server' permissions can use this.",
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
			ok, err := common.AdminOrPerm(discordgo.PermissionManageChannels, m.Author.ID, m.ChannelID)
			if err != nil {
				return "An eror occured checkign for perms", err
			}
			if !ok {
				return "You do not have access to this command (requires manage channel permission)", nil
			}

			currentReminders, err := GetChannelReminders(m.ChannelID)
			if err != nil {
				return "Failed fetching reminders, contact bot owner", err
			}

			out := "Reminders in this channel:\n"
			out += stringReminders(common.BotSession.State, currentReminders, true)
			out += "\nRemove a reminder with `delreminder/rmreminder (id)` where id is the first number for each reminder above"
			return out, nil
		},
	},
	&commands.CustomCommand{
		Category: commands.CategoryTool,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "Delreminder",
			Aliases:      []string{"rmreminder"},
			Description:  "Deletes a reminder.",
			RequiredArgs: 1,
			Arguments: []*commandsystem.ArgumentDef{
				{Name: "ID", Type: commandsystem.ArgumentTypeNumber},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {

			var reminder *Reminder
			err := common.SQL.Where(parsed.Args[0].Int()).First(&reminder).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return "No reminder by that id found", nil
				}
				return "Error retrieving reminder", err
			}

			// Check perms
			if reminder.UserID != m.Author.ID {
				ok, err := common.AdminOrPerm(discordgo.PermissionManageChannels, m.Author.ID, reminder.ChannelID)
				if err != nil {
					return "An eror occured checkign for perms", err
				}
				if !ok {
					return "You need manage channel permission in the channel the reminder is in to delete reminders that are not your own", nil
				}
			}

			// Do the actual deletion
			err = common.SQL.Delete(reminder).Error

			// Check if we should remove the scheduled event
			currentReminders, err := GetUserReminders(reminder.UserID)
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

			// No other reminder for this user at this timestamp, remove the scheduled event
			common.RemoveScheduledEvent(client, fmt.Sprintf("reminders_check_user:%s", reminder.When), reminder.UserID)

			return delMsg, nil
		},
	},
}

func stringReminders(state *discordgo.State, reminders []*Reminder, displayUsernames bool) string {
	out := ""
	for _, v := range reminders {
		channel := common.MustGetChannel(v.ChannelID)

		t := time.Unix(v.When, 0)
		timeFromNow := humanize.Time(t)
		tStr := t.Format(time.RFC822)
		if !displayUsernames {
			out += fmt.Sprintf("**%d**: <#%s>: %q - %s from now (%s)\n", v.ID, channel.ID, v.Message, timeFromNow, tStr)
		} else {
			member, err := state.Member(channel.GuildID, v.UserID)
			if err != nil {
				panic(err)
			}
			out += fmt.Sprintf("**%d**: %s: %q - %s from now (%s)\n", v.ID, member.User.Username, v.Message, timeFromNow, tStr)
		}
	}
	return out
}

// Parses a time string like 1day3h
func parseReminderTime(str string) (time.Time, error) {
	logrus.Info(str)

	t := time.Now()

	currentNumBuf := ""
	currentModifierBuf := ""

	// Parse the time
	for _, v := range str {
		if unicode.Is(unicode.White_Space, v) {
			continue
		}

		if unicode.IsNumber(v) {
			if currentModifierBuf != "" {
				if currentNumBuf == "" {
					currentNumBuf = "1"
				}
				d, err := parseDuration(currentNumBuf, currentModifierBuf)
				if err != nil {
					return t, err
				}

				t = t.Add(d)

				currentNumBuf = ""
				currentModifierBuf = ""
			}

			currentNumBuf += string(v)

		} else {
			currentModifierBuf += string(v)
		}
	}

	if currentNumBuf != "" {
		d, err := parseDuration(currentNumBuf, currentModifierBuf)
		if err == nil {
			t = t.Add(d)
		}
	}

	return t, nil
}

func parseDuration(numStr, modifierStr string) (time.Duration, error) {
	parsedNum, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, err
	}

	parsedDur := time.Duration(parsedNum)

	if modifierStr == "" || strings.HasPrefix(modifierStr, "s") {
		parsedDur = parsedDur * time.Second
	} else if strings.HasPrefix(modifierStr, "m") && (len(modifierStr) < 2 || modifierStr[1] != 'o') {
		parsedDur = parsedDur * time.Minute
	} else if strings.HasPrefix(modifierStr, "h") {
		parsedDur = parsedDur * time.Hour
	} else if strings.HasPrefix(modifierStr, "d") {
		parsedDur = parsedDur * time.Hour * 24
	} else if strings.HasPrefix(modifierStr, "w") {
		parsedDur = parsedDur * time.Hour * 24 * 7
	} else if strings.HasPrefix(modifierStr, "mo") {
		parsedDur = parsedDur * time.Hour * 24 * 30
	} else if strings.HasPrefix(modifierStr, "y") {
		parsedDur = parsedDur * time.Hour * 24 * 365
	} else {
		return parsedDur, errors.New("Couldn't figure out what '" + numStr + modifierStr + "` was")
	}

	return parsedDur, nil

}
