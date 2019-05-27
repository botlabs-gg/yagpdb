package logs

//go:generate sqlboiler --no-hooks psql

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/jonas747/yagpdb/web"
	"github.com/pkg/errors"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"golang.org/x/net/context"
)

var (
	ErrChannelBlacklisted = errors.New("Channel blacklisted from creating message logs")

	logger = common.GetPluginLogger(&Plugin{})
)

type Plugin struct{}

func (p *Plugin) PluginInfo() *common.PluginInfo {
	return &common.PluginInfo{
		Name:     "Logging",
		SysName:  "logging",
		Category: common.PluginCategoryMisc,
	}
}

func RegisterPlugin() {
	common.InitSchema(DBSchema, "logs")

	p := &Plugin{}
	common.RegisterPlugin(p)
}

// Returns either stored config, err or a default config
func GetConfig(ctx context.Context, guildID int64) (*models.GuildLoggingConfig, error) {

	config, err := models.FindGuildLoggingConfigG(ctx, guildID)
	if err == sql.ErrNoRows {
		// return default config
		return &models.GuildLoggingConfig{
			UsernameLoggingEnabled: null.BoolFrom(true),
			NicknameLoggingEnabled: null.BoolFrom(true),
		}, nil
	}

	return config, err
}

func CreateLink(guildID int64, id int) string {
	return fmt.Sprintf("%s/public/%d/logs/%d", web.BaseURL(), guildID, id)
}

func CreateChannelLog(ctx context.Context, config *models.GuildLoggingConfig, guildID, channelID int64, author string, authorID int64, count int) (*models.MessageLog, error) {
	if config == nil {
		var err error
		config, err = GetConfig(ctx, guildID)
		if err != nil {
			return nil, err
		}
	}

	// note: since the blacklisted channels column is just a TEXT type with a comma seperator...
	// i was not a smart person back then
	strCID := strconv.FormatInt(channelID, 10)
	split := strings.Split(config.BlacklistedChannels.String, ",")
	if common.ContainsStringSlice(split, strCID) {
		return nil, ErrChannelBlacklisted
	}

	if count > 300 {
		count = 300
	}

	// Make a light copy of the channel
	channel := bot.State.ChannelCopy(true, channelID)
	if channel == nil {
		return nil, errors.New("Unknown channel")
	}

	msgs, err := bot.GetMessages(channel.ID, count, true)
	if err != nil {
		return nil, err
	}

	logMsgs := make([]*models.Message, 0, len(msgs))

	tx, err := common.PQ.Begin()
	if err != nil {
		return nil, errors.Wrap(err, "pq.begin")
	}

	log := &models.MessageLog{
		ChannelID:   null.StringFrom(discordgo.StrID(channel.ID)),
		ChannelName: null.StringFrom(channel.Name),
		Author:      null.StringFrom(author),
		AuthorID:    null.StringFrom(discordgo.StrID(authorID)),
		GuildID:     null.StringFrom(discordgo.StrID(channel.Guild.ID)),
	}

	err = log.Insert(ctx, tx, boil.Infer())
	if err != nil {
		tx.Rollback()
		return nil, errors.Wrap(err, "log.insert")
	}

	for _, v := range msgs {
		body := v.Content
		for _, attachment := range v.Attachments {
			body += fmt.Sprintf(" (Attachment: %s)", attachment.URL)
		}

		if len(v.Embeds) > 0 {
			body += fmt.Sprintf(" (%d embeds is not shown)", len(v.Embeds))
		}

		// Strip out nul characters since postgres dont like them and discord dont filter them out (like they do in a lot of other places)
		body = strings.Replace(body, string(0), "", -1)

		messageModel := &models.Message{
			MessageID:      null.StringFrom(discordgo.StrID(v.ID)),
			MessageLogID:   null.IntFrom(log.ID),
			Content:        null.StringFrom(body),
			Timestamp:      null.StringFrom(v.ParsedCreated.Format(time.RFC3339)),
			AuthorUsername: null.StringFrom(v.Author.Username),
			AuthorDiscrim:  null.StringFrom(v.Author.Discriminator),
			AuthorID:       null.StringFrom(discordgo.StrID(v.Author.ID)),
			Deleted:        null.BoolFrom(v.Deleted),
		}

		err = messageModel.Insert(ctx, tx, boil.Infer())
		if err != nil {
			tx.Rollback()
			return nil, errors.Wrap(err, "message.insert")
		}

		logMsgs = append(logMsgs, messageModel)
	}

	err = tx.Commit()
	if err != nil {
		return nil, errors.Wrap(err, "commit")
	}

	log.R = log.R.NewStruct()
	log.R.Messages = logMsgs

	return log, nil
}

func GetChannelLogs(ctx context.Context, id, guildID int64) (*models.MessageLog, error) {

	logs, err := models.MessageLogs(
		models.MessageLogWhere.ID.EQ(int(id)),
		models.MessageLogWhere.GuildID.EQ(null.StringFrom(discordgo.StrID(guildID))),
		models.MessageLogWhere.DeletedAt.IsNull(),
		qm.Load("Messages", qm.OrderBy("id desc"))).OneG(ctx)

	return logs, err
}

func GetGuilLogs(ctx context.Context, guildID int64, before, after, limit int) ([]*models.MessageLog, error) {

	qms := []qm.QueryMod{
		qm.OrderBy("id desc"),
		qm.Limit(limit),
		models.MessageLogWhere.DeletedAt.IsNull(),
		models.MessageLogWhere.GuildID.EQ(null.StringFrom(discordgo.StrID(guildID))),
	}

	if before != 0 {
		qms = append(qms, models.MessageLogWhere.ID.LT(before))
	} else if after != 0 {
		qms = append(qms, models.MessageLogWhere.ID.GT(after))
	}

	logs, err := models.MessageLogs(qms...).AllG(ctx)
	return logs, err
}

func GetUsernames(ctx context.Context, userID int64, limit int) ([]*models.UsernameListing, error) {
	result, err := models.UsernameListings(models.UsernameListingWhere.UserID.EQ(null.Int64From(userID)), qm.OrderBy("id desc"), qm.Limit(limit)).AllG(ctx)
	return result, err
}

func GetNicknames(ctx context.Context, userID, guildID int64, limit int) ([]*models.NicknameListing, error) {

	return models.NicknameListings(
		models.NicknameListingWhere.GuildID.EQ(null.StringFrom(discordgo.StrID(guildID))),
		models.NicknameListingWhere.UserID.EQ(null.Int64From(userID)),
		qm.OrderBy("id desc"), qm.Limit(limit)).AllG(ctx)
}
