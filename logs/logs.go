package logs

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/configstore"
	"golang.org/x/net/context"
	"strconv"
	"strings"
)

var (
	ErrChannelBlacklisted = errors.New("Channel blacklisted from creating message logs")
)

type Plugin struct{}

func (p *Plugin) Name() string {
	return "Logs"
}

func InitPlugin() {
	//p := &Plugin{}
	err := common.GORM.AutoMigrate(&MessageLog{}, &Message{}, &UsernameListing{}, &NicknameListing{}, GuildLoggingConfig{}).Error
	if err != nil {
		panic(err)
	}

	configstore.RegisterConfig(configstore.SQL, &GuildLoggingConfig{})

	p := &Plugin{}
	common.RegisterPlugin(p)

}

type GuildLoggingConfig struct {
	configstore.GuildConfigModel
	UsernameLoggingEnabled bool
	NicknameLoggingEnabled bool
	BlacklistedChannels    string

	ManageMessagesCanViewDeleted bool
	EveryoneCanViewDeleted       bool

	ParsedBlacklistedchannels []int64 `gorm:"-"`
}

func (g *GuildLoggingConfig) PostFetch() {
	split := strings.Split(g.BlacklistedChannels, ",")
	for _, v := range split {
		if v != "" && v != "0" {
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				g.ParsedBlacklistedchannels = append(g.ParsedBlacklistedchannels, parsed)
			}
		}
	}
}

func (g *GuildLoggingConfig) GetName() string {
	return "guild_logging_config"
}

// Returns either stored config, err or a default config
func GetConfig(guildID int64) (*GuildLoggingConfig, error) {
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

	MessageID string `gorm:"index"`
	Content   string `gorm:"size:2000"`
	Timestamp string

	AuthorUsername string
	AuthorDiscrim  string
	AuthorID       string
	Deleted        bool
}

func CreateChannelLog(config *GuildLoggingConfig, guildID, channelID int64, author string, authorID int64, count int) (*MessageLog, error) {
	if config == nil {
		var err error
		config, err = GetConfig(guildID)
		if err != nil {
			return nil, err
		}
	}

	if len(config.ParsedBlacklistedchannels) > 0 {
		for _, v := range config.ParsedBlacklistedchannels {
			if v == channelID {
				return nil, ErrChannelBlacklisted
			}
		}
	}

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

		// Strip out nul characters since postgres dont like them and discord dont filter them out (like they do in a lot of other places)
		body = strings.Replace(body, string(0), "", -1)

		logMsgs[k] = Message{
			MessageID:      discordgo.StrID(v.ID),
			Content:        body,
			Timestamp:      string(v.Timestamp),
			AuthorUsername: v.Author.Username,
			AuthorDiscrim:  v.Author.Discriminator,
			AuthorID:       discordgo.StrID(v.Author.ID),
			Deleted:        v.Deleted,
		}
	}

	log := &MessageLog{
		Messages:    logMsgs,
		ChannelID:   discordgo.StrID(channel.ID),
		ChannelName: channel.Name,
		Author:      author,
		AuthorID:    discordgo.StrID(authorID),
		GuildID:     discordgo.StrID(channel.Guild.ID),
	}

	err = common.GORM.Create(log).Error

	return log, err
	return nil, nil
}

func GetChannelLogs(id int64) (*MessageLog, error) {

	var result MessageLog
	err := common.GORM.Where(id).First(&result).Error

	if err != nil {
		return nil, err
	}

	err = common.GORM.Where("message_log_id = ?", result.ID).Order("id desc").Find(&result.Messages).Error
	// err = common.GORM.Model(&result).Related(&result.Messages, "MessageLogID").Error

	return &result, err
}

func GetGuilLogs(guildID int64, before, after, limit int) ([]*MessageLog, error) {

	var result []*MessageLog
	var q *gorm.DB
	if before != 0 {
		q = common.GORM.Where("guild_id = ? AND id < ?", guildID, before)
	} else if after != 0 {
		q = common.GORM.Where("guild_id = ? AND id > ?", guildID, after)
	} else {
		q = common.GORM.Where("guild_id = ?", guildID)
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

func GetUsernames(userID int64, limit int) ([]UsernameListing, error) {
	var listings []UsernameListing
	err := common.GORM.Where(&UsernameListing{UserID: userID}).Order("id desc").Limit(limit).Find(&listings).Error
	return listings, err
}

func GetNicknames(userID, guildID int64, limit int) ([]NicknameListing, error) {
	var listings []NicknameListing
	err := common.GORM.Where(&NicknameListing{UserID: userID, GuildID: discordgo.StrID(guildID)}).Order("id desc").Limit(limit).Find(&listings).Error
	return listings, err
}
