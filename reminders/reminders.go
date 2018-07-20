package reminders

import (
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/mqueue"
	"github.com/jonas747/yagpdb/common/scheduledevents"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type Plugin struct{}

func RegisterPlugin() {
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

func (r *Reminder) UserIDInt() (i int64) {
	i, _ = strconv.ParseInt(r.UserID, 10, 64)
	return
}

func (r *Reminder) ChannelIDInt() (i int64) {
	i, _ = strconv.ParseInt(r.ChannelID, 10, 64)
	return
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

func GetUserReminders(userID int64) (results []*Reminder, err error) {
	err = common.GORM.Where(&Reminder{UserID: discordgo.StrID(userID)}).Find(&results).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func GetChannelReminders(channel int64) (results []*Reminder, err error) {
	err = common.GORM.Where(&Reminder{ChannelID: discordgo.StrID(channel)}).Find(&results).Error
	if err == gorm.ErrRecordNotFound {
		err = nil
	}
	return
}

func NewReminder(userID int64, channelID int64, message string, when time.Time) (*Reminder, error) {
	whenUnix := when.Unix()
	reminder := &Reminder{
		UserID:    discordgo.StrID(userID),
		ChannelID: discordgo.StrID(channelID),
		Message:   message,
		When:      whenUnix,
	}

	err := common.GORM.Create(reminder).Error
	if err != nil {
		return nil, err
	}

	err = scheduledevents.ScheduleEvent("reminders_check_user:"+strconv.FormatInt(whenUnix, 10), discordgo.StrID(userID), when)
	return reminder, err
}

func checkUserEvtHandler(evt string) error {
	split := strings.Split(evt, ":")
	if len(split) < 2 {
		logrus.Error("Handled invalid check user scheduled event: ", evt)
		return nil
	}

	parsed, _ := strconv.ParseInt(split[1], 10, 64)
	reminders, err := GetUserReminders(parsed)
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
