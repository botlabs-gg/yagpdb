package reminders

import (
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/redis"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"strconv"
	"strings"
	"time"
)

type Plugin struct{}

func RegisterPlugin() {
	common.RegisterScheduledEventHandler("reminders_check_user", checkUserEvtHandler)
	err := common.SQL.AutoMigrate(&Reminder{}).Error
	if err != nil {
		panic(err)
	}

	p := &Plugin{}
	bot.RegisterPlugin(p)
}

func (p *Plugin) Name() string {
	return "Reminders"
}

type Reminder struct {
	gorm.Model
	UserID    string
	ChannelID string
	Message   string
	When      int64
}

func (r *Reminder) Trigger() error {
	logrus.WithFields(logrus.Fields{"channel": r.ChannelID, "user": r.UserID, "message": r.Message}).Info("Triggered reminder")

	_, err := common.BotSession.ChannelMessageSend(r.ChannelID, common.EscapeEveryoneMention("**Reminder** <@"+r.UserID+">: "+r.Message))
	if err != nil {
		if _, ok := err.(*discordgo.RESTError); !ok {
			// Reschedule if discord didnt respond with an error (i.e they being down or something)
			return err
		} else {
			// Don't reschedule the event incase it was sent in a channel with no bot perms, or channel was deleted
			logrus.WithError(err).WithField("channel", r.ChannelID).WithField("user", r.UserID).Warn("Discord wouldnt let us send a message in this channel to remind")
			return nil
		}
	}

	// remove the actual reminder
	common.SQL.Delete(r)
	return nil
}

func GetUserReminders(userID string) (results []*Reminder, err error) {
	err = common.SQL.Where(&Reminder{UserID: userID}).Find(&results).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func GetChannelReminders(channel string) (results []*Reminder, err error) {
	err = common.SQL.Where(&Reminder{ChannelID: channel}).Find(&results).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func NewReminder(client *redis.Client, userID string, channelID string, message string, when time.Time) (*Reminder, error) {
	whenUnix := when.Unix()
	reminder := &Reminder{
		UserID:    userID,
		ChannelID: channelID,
		Message:   message,
		When:      whenUnix,
	}

	err := common.SQL.Create(reminder).Error
	if err != nil {
		return nil, err
	}

	err = common.ScheduleEvent(client, "reminders_check_user:"+strconv.FormatInt(whenUnix, 10), userID, when)
	return reminder, err
}

func checkUserEvtHandler(evt string) error {
	split := strings.Split(evt, ":")
	if len(split) < 2 {
		logrus.Error("Handled invalid check user scheduled event: ", evt)
		return nil
	}

	reminders, err := GetUserReminders(split[1])
	if err != nil {
		return err
	}

	now := time.Now()
	nowUnix := now.Unix()
	for _, v := range reminders {
		if v.When <= nowUnix {
			err := v.Trigger()
			if err != nil {
				// Try again
				return err
			}
		}
	}

	return nil
}
