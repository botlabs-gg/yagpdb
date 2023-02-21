package reminders

import (
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/mqueue"
	"github.com/botlabs-gg/yagpdb/v2/common/scheduledevents2"
	"github.com/botlabs-gg/yagpdb/v2/lib/discordgo"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
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

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Reminders",
		SysName:  "reminders",
		Category: common.PluginCategoryMisc,
	}
}

type Reminder struct {
	gorm.Model
	UserID    string
	ChannelID string
	GuildID   int64
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
		logger.Info("Tried to execute multiple reminders at once")
	}

	logger.WithFields(logrus.Fields{"channel": r.ChannelID, "user": r.UserID, "message": r.Message, "id": r.ID}).Info("Triggered reminder")
	embed := &discordgo.MessageEmbed{
		Title:       "Reminder from YAGPDB",
		Description: common.ReplaceServerInvites(r.Message, r.GuildID, "(removed-invite)"),
	}

	mqueue.QueueMessage(&mqueue.QueuedElement{
		Source:       "reminder",
		SourceItemID: "",

		GuildID:      r.GuildID,
		ChannelID:    r.ChannelIDInt(),
		MessageEmbed: embed,
		MessageStr:   "**Reminder** for <@" + r.UserID + ">",
		AllowedMentions: discordgo.AllowedMentions{
			Users: []int64{r.UserIDInt()},
		},
		Priority: 10, // above all feeds
	})
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

func NewReminder(userID int64, guildID int64, channelID int64, message string, when time.Time) (*Reminder, error) {
	whenUnix := when.Unix()
	reminder := &Reminder{
		UserID:    discordgo.StrID(userID),
		ChannelID: discordgo.StrID(channelID),
		Message:   message,
		When:      whenUnix,
		GuildID:   guildID,
	}

	err := common.GORM.Create(reminder).Error
	if err != nil {
		return nil, err
	}

	err = scheduledevents2.ScheduleEvent("reminders_check_user", guildID, when, userID)
	// err = scheduledevents.ScheduleEvent("reminders_check_user:"+strconv.FormatInt(whenUnix, 10), discordgo.StrID(userID), when)
	return reminder, err
}

func checkUserEvtHandlerLegacy(evt string) error {
	split := strings.Split(evt, ":")
	if len(split) < 2 {
		logger.Error("Handled invalid check user scheduled event: ", evt)
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
