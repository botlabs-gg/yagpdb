package logs

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Logs"
}

func InitPlugin() {
	//p := &Plugin{}
	err := common.SQL.AutoMigrate(&MessageLog{}, &Message{}, &UsernameListing{}, &NicknameListing{}).Error
	if err != nil {
		panic(err)
	}

	p := &Plugin{}
	web.RegisterPlugin(p)
	bot.RegisterPlugin(p)
}

type MessageLog struct {
	gorm.Model
	Messages []Message

	ChannelName string
	ChannelID   string
	GuildID     string

	Author   string
	AuthorID string
}

func (m *MessageLog) Link() string {
	return fmt.Sprintf("https://%s/public/%s/logs/%d", common.Conf.Host, m.GuildID, m.ID)
}

type Message struct {
	gorm.Model
	MessageLogID uint `gorm:"index"` // Foreign key, belongs to MessageLog

	MessageID string
	Content   string `gorm:"size:2000"`
	Timestamp string

	AuthorUsername string
	AuthorDiscrim  string
	AuthorID       string
	Deleted        bool
}

func CreateChannelLog(channelID, author, authorID string, count int) (*MessageLog, error) {
	if count > 1000 {
		panic("count > 1000")
	}

	channel, err := common.BotSession.State.Channel(channelID)
	if err != nil {
		return nil, err
	}

	msgs, err := common.GetMessages(channel.ID, count)
	if err != nil {
		return nil, err
	}

	logMsgs := make([]Message, len(msgs))

	for k, v := range msgs {
		if v.Author == nil || v.Timestamp == "" {
			continue
		}

		body := v.Content
		for _, attachment := range v.Attachments {
			body += fmt.Sprintf(" (Attachment: %s)", attachment.URL)
		}

		logMsgs[k] = Message{
			MessageID:      v.ID,
			Content:        body,
			Timestamp:      string(v.Timestamp),
			AuthorUsername: v.Author.Username,
			AuthorDiscrim:  v.Author.Discriminator,
			AuthorID:       v.Author.ID,
		}
	}

	log := &MessageLog{
		Messages:    logMsgs,
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		Author:      author,
		AuthorID:    authorID,
		GuildID:     channel.GuildID,
	}

	err = common.SQL.Create(log).Error

	return log, err
}

func GetChannelLogs(id int64) (*MessageLog, error) {

	var result MessageLog
	err := common.SQL.Where(id).First(&result).Error

	if err != nil {
		return nil, err
	}

	err = common.SQL.Model(&result).Related(&result.Messages, "MessageLogID").Error

	return &result, err
}
