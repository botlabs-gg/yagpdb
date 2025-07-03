package reminders

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/botlabs-gg/yagpdb/v2/bot"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/botlabs-gg/yagpdb/v2/reminders/models"
	"github.com/sirupsen/logrus"
)

//go:generate sqlboiler --no-hooks --add-soft-deletes psql

type Plugin struct{}

func RegisterPlugin() {
	p := &Plugin{}
	common.RegisterPlugin(p)

	common.InitSchemas("reminders", DBSchemas...)
}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Reminders",
		SysName:  "reminders",
		Category: common.PluginCategoryMisc,
	}
}

func TriggerReminder(r *models.Reminder) error {
	r.DeleteG(context.Background(), false /* hardDelete */)

	logger.WithFields(logrus.Fields{"channel": r.ChannelID, "user": r.UserID, "message": r.Message, "id": r.ID}).Info("Triggered reminder")
	embed := &discordgo.MessageEmbed{
		Title:       "Reminder from YAGPDB",
		Description: common.ReplaceServerInvites(r.Message, r.GuildID, "(removed-invite)"),
	}

	channelID, _ := discordgo.ParseID(r.ChannelID)
	userID, _ := discordgo.ParseID(r.UserID)
	return mqueue.QueueMessage(&mqueue.QueuedElement{
		Source:       "reminder",
		SourceItemID: "",

		GuildID:      r.GuildID,
		ChannelID:    channelID,
		MessageEmbed: embed,
		MessageStr:   "**Reminder** for <@" + r.UserID + ">",
		AllowedMentions: discordgo.AllowedMentions{
			Users: []int64{userID},
		},
		Priority: 10, // above all feeds
	})
}

func NewReminder(userID int64, guildID int64, channelID int64, message string, when time.Time) (*models.Reminder, error) {
	reminder := &models.Reminder{
		UserID:    discordgo.StrID(userID),
		ChannelID: discordgo.StrID(channelID),
		Message:   message,
		When:      when.Unix(),
		GuildID:   guildID,
	}

	err := reminder.InsertG(context.Background(), boil.Infer())
	if err != nil {
		return nil, err
	}

	err = scheduledevents2.ScheduleEvent("reminders_check_user", guildID, when, userID)
	return reminder, err
}

type DisplayRemindersMode int

const (
	ModeDisplayChannelReminders DisplayRemindersMode = iota
	ModeDisplayUserReminders
)

func DisplayReminders(reminders models.ReminderSlice, mode DisplayRemindersMode) string {
	var out strings.Builder
	for _, r := range reminders {
		t := time.Unix(r.When, 0)
		timeFromNow := common.HumanizeTime(common.DurationPrecisionMinutes, t)

		switch mode {
		case ModeDisplayChannelReminders:
			// don't show the channel; do show the user
			uid, _ := discordgo.ParseID(r.UserID)
			member, _ := bot.GetMember(r.GuildID, uid)
			username := "Unknown user"
			if member != nil {
				username = member.User.Username
			}

			fmt.Fprintf(&out, "**%d**: %s: '%s' - %s from now (<t:%d:f>)\n", r.ID, username, CutReminderShort(r.Message), timeFromNow, t.Unix())

		case ModeDisplayUserReminders:
			// do show the channel; don't show the user
			channel := "<#" + r.ChannelID + ">"
			fmt.Fprintf(&out, "**%d**: %s: '%s' - %s from now (<t:%d:f>)\n", r.ID, channel, CutReminderShort(r.Message), timeFromNow, t.Unix())
		}
	}

	return out.String()
}

func CutReminderShort(msg string) string {
	return common.CutStringShort(msg, 50)
}
