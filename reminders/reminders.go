package reminders

import (
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/mediocregopher/radix.v2/redis"
	"strconv"
	"strings"
	"time"
)

type Plugin struct{}

func RegisterPlugin() {
	scheduledevents.RegisterEventHandler("reminders_check_user", checkUserEvtHandler)
	err := common.GORM.AutoMigrate(&Reminder{}).Error
	if err != nil {
		panic(err)
	}

	p := &Plugin{}
	common.RegisterPlugin(p)
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
	// remove the actual reminder
	rows := common.GORM.Delete(r).RowsAffected
	if rows < 1 {
		logrus.Info("Tried to execute multiple reminders at once")
	}

	logrus.WithFields(logrus.Fields{"channel": r.ChannelID, "user": r.UserID, "message": r.Message}).Info("Triggered reminder")

	mqueue.QueueMessageString("reminder", "", r.ChannelID, common.EscapeSpecialMentions("**Reminder** <@"+r.UserID+">: "+r.Message))
	return nil
}

func GetUserReminders(userID string) (results []*Reminder, err error) {
	err = common.GORM.Where(&Reminder{UserID: userID}).Find(&results).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func GetChannelReminders(channel string) (results []*Reminder, err error) {
	err = common.GORM.Where(&Reminder{ChannelID: channel}).Find(&results).Error
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

	err := common.GORM.Create(reminder).Error
	if err != nil {
		return nil, err
	}

	err = scheduledevents.ScheduleEvent(client, "reminders_check_user:"+strconv.FormatInt(whenUnix, 10), userID, when)
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
