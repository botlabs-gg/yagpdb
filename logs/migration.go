package logs

import (
	"context"
	"database/sql"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"strconv"
	"time"
)

var cmdMigrate = &commands.YAGCommand{
	CmdCategory:          commands.CategoryTool,
	Name:                 "MigrateLogs",
	Description:          "Migrates logs from the old format to the new format.",
	HideFromHelp:         true,
	RunInDM:              true,
	HideFromCommandsPage: true,
	Arguments: []*dcmd.ArgDef{
		{Name: "User", Type: dcmd.User},
	},
	RunFunc: util.RequireOwner(func(parsed *dcmd.Data) (interface{}, error) {

		last := -1
		more := true
		for more {
			started := time.Now()
			var err error
			last, more, err = migrateLogs(last, 10)
			if err != nil {
				return nil, err
			}

			s := time.Since(started)
			logger.Infof("Migrated %d logs in %s, last: %d, more: %t", 10, s.String(), last, more)
		}

		return "Doneso!", nil
	}),
}

func migrateLogs(after, count int) (last int, more bool, err error) {
	err = common.SqlTX(func(tx *sql.Tx) error {
		logs, err := models.MessageLogs(
			models.MessageLogWhere.ID.GT(after),
			qm.OrderBy("id asc"),
			qm.Limit(count),
			qm.Load("Messages", qm.OrderBy("id desc")),
		).All(context.Background(), tx)

		if err != nil {
			return errors.Wrap(err, "messagelogs")
		}

		for _, v := range logs {
			err := migrateLog(tx, v)
			if err != nil {
				return errors.Wrapf(err, "migratelog %d", v.ID)
			}
		}

		if len(logs) == count {
			more = true
			last = logs[len(logs)-1].ID
		}

		return nil
	})

	return
}

func migrateLog(tx *sql.Tx, l *models.MessageLog) error {
	guildID, err := strconv.ParseInt(l.GuildID.String, 10, 64)
	if err != nil {
		return errors.Wrap(err, "parse_guildid")
	}

	id, err := common.GenLocalIncrID(guildID, "message_logs")
	if err != nil {
		return errors.Wrap(err, "gen_id")
	}

	mIds := make([]int64, len(l.R.Messages))
	for i, v := range l.R.Messages {
		parsedMID, err := strconv.ParseInt(v.MessageID.String, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "parse msg id %d", v.ID)
		}

		mIds[i] = parsedMID
	}

	authorID, err := strconv.ParseInt(l.AuthorID.String, 10, 64)
	if err != nil {
		return errors.Wrap(err, "parse author")
	}

	channelID, err := strconv.ParseInt(l.ChannelID.String, 10, 64)
	if err != nil {
		return errors.Wrap(err, "parse channelid")
	}

	m := &models.MessageLogs2{
		ID:       int(id),
		GuildID:  guildID,
		LegacyID: l.ID,

		ChannelName:    l.ChannelName.String,
		ChannelID:      channelID,
		AuthorID:       authorID,
		AuthorUsername: l.Author.String,

		CreatedAt: l.CreatedAt.Time,
		UpdatedAt: l.CreatedAt.Time,

		Messages: mIds,
	}

	if !l.UpdatedAt.Valid {
		m.UpdatedAt = m.CreatedAt
	}

	err = m.Insert(context.Background(), tx, boil.Infer())
	if err != nil {
		return errors.Wrap(err, "insert log")
	}

	// migrate the individual messages
	for _, v := range l.R.Messages {
		err = migrateMessage(tx, guildID, v)
		if err != nil {
			return errors.Wrapf(err, "migrate message %d", v.ID)
		}
	}

	return nil
}

func migrateMessage(tx *sql.Tx, guildID int64, m *models.Message) error {

	mID, err := strconv.ParseInt(m.MessageID.String, 10, 64)
	if err != nil {
		return errors.Wrap(err, "parse messageid")
	}

	authorID, err := strconv.ParseInt(m.AuthorID.String, 10, 64)
	if err != nil {
		return errors.Wrap(err, "parse authorid")
	}

	parsedTS, err := discordgo.Timestamp(m.Timestamp.String).Parse()
	if err != nil {
		return errors.Wrap(err, "parse timestamp")
	}

	model := &models.Messages2{
		ID:             mID,
		CreatedAt:      parsedTS,
		UpdatedAt:      m.UpdatedAt.Time,
		Deleted:        m.Deleted.Bool,
		GuildID:        guildID,
		AuthorUsername: m.AuthorUsername.String + "#" + m.AuthorDiscrim.String,
		AuthorID:       authorID,
		Content:        m.Content.String,
	}

	updateCols := boil.Infer()
	if !m.Deleted.Bool {
		updateCols = boil.Blacklist("deleted")
	}

	err = model.Upsert(context.Background(), tx, true, []string{"id"}, updateCols, boil.Infer())
	if err != nil {
		return errors.Wrap(err, "insert")
	}

	return nil
}
