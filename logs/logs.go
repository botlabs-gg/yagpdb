package logs

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"github.com/jonas747/yagpdb/web"
	"golang.org/x/net/context"
	"strconv"
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Logs"
}

func InitPlugin() {
	//p := &Plugin{}
	err := common.SQL.AutoMigrate(&MessageLog{}, &Message{}, &UsernameListing{}, &NicknameListing{}, GuildLoggingConfig{}).Error
	if err != nil {
		panic(err)
	}

	configstore.RegisterConfig(configstore.SQL, &GuildLoggingConfig{})

	p := &Plugin{}
	web.RegisterPlugin(p)
	bot.RegisterPlugin(p)

}

type GuildLoggingConfig struct {
	configstore.GuildConfigModel
	UsernameLoggingEnabled bool
	NicknameLoggingEnabled bool
}

func (g *GuildLoggingConfig) GetName() string {
	return "guild_logging_config"
}

// Returns either stored config, err or a default config
func GetConfig(guildID string) (*GuildLoggingConfig, error) {
	var general GuildLoggingConfig
	err := configstore.Cached.GetGuildConfig(context.Background(), guildID, &general)
	if err != nil {
		if err == configstore.ErrNotFound {
			return &GuildLoggingConfig{
				UsernameLoggingEnabled: true,
				NicknameLoggingEnabled: true,
			}, nil
		}
		return nil, err
	}

	return &general, nil
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
	common.SmallModel
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

	cs := bot.State.Channel(true, channelID)
	// Make a light copy of the channel
	channel := cs.Copy(true, false)

	msgs, err := bot.GetMessages(channel.ID, count, true)
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

		if len(v.Embeds) > 0 {
			body += fmt.Sprintf("(%d embeds is not shown)", len(v.Embeds))
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
	return nil, nil
}

func GetChannelLogs(id int64) (*MessageLog, error) {

	var result MessageLog
	err := common.SQL.Where(id).First(&result).Error

	if err != nil {
		return nil, err
	}

	err = common.SQL.Where("message_log_id = ?", result.ID).Order("id desc").Find(&result.Messages).Error
	// err = common.SQL.Model(&result).Related(&result.Messages, "MessageLogID").Error

	return &result, err
}

func GetGuilLogs(guildID string, before, after, limit int) ([]*MessageLog, error) {

	var result []*MessageLog
	var q *gorm.DB
	if before != 0 {
		q = common.SQL.Where("guild_id = ? AND id < ?", guildID, before)
	} else if after != 0 {
		q = common.SQL.Where("guild_id = ? AND id > ?", guildID, after)
	} else {
		q = common.SQL.Where("guild_id = ?", guildID)
	}

	err := q.Order("id desc").Limit(limit).Find(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []*MessageLog{}, nil
		}

		return nil, err
	}

	return result, err
}

func GetUsernames(userID string, limit int) ([]UsernameListing, error) {
	var listings []UsernameListing
	err := common.SQL.Where(&UsernameListing{UserID: MustParseID(userID)}).Order("id desc").Limit(limit).Find(&listings).Error
	return listings, err
}

func GetNicknames(userID, GuildID string, limit int) ([]NicknameListing, error) {
	var listings []NicknameListing
	err := common.SQL.Where(&NicknameListing{UserID: MustParseID(userID), GuildID: GuildID}).Order("id desc").Limit(limit).Find(&listings).Error
	return listings, err
}

func MustParseID(id string) int64 {
	v, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		panic("Failed parsing id: " + err.Error())
	}

	return v
}
