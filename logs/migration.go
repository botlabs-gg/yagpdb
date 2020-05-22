package logs

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/dcmd"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/commands"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/logs/models"
	"github.com/jonas747/yagpdb/stdcommands/util"
	"github.com/mediocregopher/radix/v3"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

/*
If you accidentally run the migrate command several times you can get rid of duplicates using the following query:

DELETE FROM message_logs2 a USING (
      SELECT MIN(ctid) as ctid, legacy_id
        FROM message_logs2
        GROUP BY legacy_id HAVING COUNT(*) > 1
      ) b
      WHERE a.legacy_id = b.legacy_id
      AND a.ctid <> b.ctid
      AND a.legacy_id != 0;
*/

var cmdMigrate = &commands.YAGCommand{
	CmdCategory:          commands.CategoryTool,
	Name:                 "MigrateLogs",
	Description:          "Migrates logs from the old format to the new format. This dosen't delete anything and also dosen't deal with duplicates, you should not run it several times.",
	HideFromHelp:         true,
	RunInDM:              true,
	HideFromCommandsPage: true,
	RunFunc: util.RequireOwner(func(parsed *dcmd.Data) (interface{}, error) {
		resp := ""
		err := common.RedisPool.Do(radix.Cmd(&resp, "SET", "yagpdb_logs_migrated", "1", "NX"))
		if err != nil {
			return nil, err
		}

		if resp != "OK" {
			return "Already ran migration previously", nil
		}

		last := -1
		more := true
		for more {
			started := time.Now()
			var err error
			last, more, err = migrateLogs(last, 500)
			if err != nil {
				return nil, err
			}

			s := time.Since(started)
			logger.Infof("Migrated %d logs in %s, last: %d, more: %t", 100, s.String(), last, more)

			time.Sleep(time.Second)
		}

		return "Doneso!", nil
	}),
}

func migrateLogs(after, count int) (last int, more bool, err error) {
	err = common.SqlTX(func(tx *sql.Tx) error {
		logs, err := models.MessageLogs(
			models.MessageLogWhere.ID.GT(after),
			qm.Where("deleted_at IS NULL"),
			qm.OrderBy("id asc"),
			qm.Limit(count),
			qm.Load("Messages", qm.OrderBy("id desc")),
		).All(context.Background(), tx)

		if err != nil {
			return errors.WrapIf(err, "messagelogs")
		}

		for _, v := range logs {
			err := migrateLog(tx, v)
			if err != nil {
				return errors.WrapIff(err, "migratelog %d", v.ID)
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
		return errors.WrapIf(err, "parse_guildid")
	}

	id, err := common.GenLocalIncrID(guildID, "message_logs")
	if err != nil {
		return errors.WrapIf(err, "gen_id")
	}

	mIds := make([]int64, 0, len(l.R.Messages))
	for _, v := range l.R.Messages {
		parsedMID, err := strconv.ParseInt(v.MessageID.String, 10, 64)
		if err != nil {
			continue
		}

		mIds = append(mIds, parsedMID)
	}

	authorID, _ := strconv.ParseInt(l.AuthorID.String, 10, 64)
	channelID, _ := strconv.ParseInt(l.ChannelID.String, 10, 64)

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
		return errors.WrapIf(err, "insert log")
	}

	// migrate the individual messages
	for _, v := range l.R.Messages {
		err = migrateMessage(tx, guildID, v)
		if err != nil {
			return errors.WrapIff(err, "migrate message %d", v.ID)
		}
	}

	return nil
}

func migrateMessage(tx *sql.Tx, guildID int64, m *models.Message) error {

	mID, err := strconv.ParseInt(m.MessageID.String, 10, 64)
	if err != nil {
		return nil
	}

	authorID, _ := strconv.ParseInt(m.AuthorID.String, 10, 64)

	parsedTS, err := discordgo.Timestamp(m.Timestamp.String).Parse()
	if err != nil {
		return errors.WrapIf(err, "parse timestamp")
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
		return errors.WrapIf(err, "insert")
	}

	return nil
}
