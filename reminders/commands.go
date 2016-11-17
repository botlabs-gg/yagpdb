package reminders

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/dutil/commandsystem"
	"github.com/jonas747/yagpdb/commands"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (p *Plugin) InitBot() {
	commands.CommandSystem.RegisterCommands(cmds...)
}

var cmds = []commandsystem.CommandHandler{
	&commands.CustomCommand{
		Category: commands.CategoryModeration,
		SimpleCommand: &commandsystem.SimpleCommand{
			Name:         "remindme",
			Description:  "Schedules a reminder",
			Aliases:      []string{"remind"},
			RequiredArgs: 2,
			Arguments: []*commandsystem.ArgumentDef{
				&commandsystem.ArgumentDef{Name: "Time from now", Type: commandsystem.ArgumentTypeString},
				&commandsystem.ArgumentDef{Name: "Message", Type: commandsystem.ArgumentTypeString},
			},
		},
		RunFunc: func(parsed *commandsystem.ParsedCommand, client *redis.Client, m *discordgo.MessageCreate) (interface{}, error) {
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

			until := when.Sub(time.Now()).String()

			return "Set a reminder for " + until + " from now", nil
		},
	},
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

	logrus.Info(currentNumBuf, currentModifierBuf)

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
