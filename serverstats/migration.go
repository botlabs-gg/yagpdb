package serverstats

import (
	"time"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/premium"
)

func StartMigrationToV2Format() error {
	existing, err := GetMigrationV2Progress()
	if err != nil {
		return errors.WithStackIf(err)
	}

	if existing.MsgPeriods.IsRunning() || existing.MemberPeriods.IsRunning() {
		return errors.New("Migration already running")
	}

	msgPeriodsExists, err := common.TableExists("server_stats_periods")
	if err != nil {
		return errors.WithStackIf(err)
	}

	memberPeriodsExist, err := common.TableExists("server_stats_member_periods")
	if err != nil {
		return errors.WithStackIf(err)
	}

	if !msgPeriodsExists || !memberPeriodsExist {
		logger.Info("Attempted migration without the old v1 tables existing")
		return nil
	}

	premiumGuilds, err := premium.AllGuildsOncePremium()
	if err != nil {
		return errors.WithStackIf(err)
	}

	go runV2Migration(premiumGuilds, existing)

	return nil
}

func runV2Migration(premiumGuilds map[int64]time.Time, lastProgress *MigrationProgress) {
	err := migrateV2Chunk("serverstats_migration_v2_progress_msgs", lastProgress.MsgPeriods.LastID, migrateChunkV2Messages)
	// err := runMsgMigrationV2(lastProgress.MsgPeriods.LastID)
	if err != nil {
		logger.WithError(err).Error("Failed running message v1 -> v2 table migration")
		return
	}

	err = migrateV2Chunk("serverstats_migration_v2_progress_members", lastProgress.MemberPeriods.LastID, migrateChunkV2Members)
	// err = runMemberMigrationV2(premiumGuilds, lastProgress.MsgPeriods.LastID)
	if err != nil {
		logger.WithError(err).Error("Failed running member v1 -> v2 table migration")
		return
	}
}

func migrateV2Chunk(name string, lastID int64, f func(lastID int64, premiumGuilds map[int64]time.Time) (newLastID int64, err error)) error {
	defer func() {
		updateSubProgress(name, lastID, false)
	}()

	for {
		premiumGuilds, err := premium.AllGuildsOncePremium()
		if err != nil {
			return errors.WithStackIf(err)
		}

		started := time.Now()

		newLastID, err := f(lastID, premiumGuilds)
		if err != nil {
			return errors.WithStackIf(err)
		}

		logger.Infof("Took %s to migrate chunk %s:%d to v2 format", time.Since(started), name, newLastID)

		if newLastID == lastID {
			break
		}

		lastID = newLastID

		time.Sleep(time.Second)
	}

	return nil
}

func migrateChunkV2Messages(lastID int64, premiumGuilds map[int64]time.Time) (newLastID int64, err error) {

	const qGetOld = `SELECT id, started, guild_id, count FROM server_stats_periods
	WHERE id > $1 ORDER BY ID ASC LIMIT 5000;`

	const qSetNew = `INSERT INTO server_stats_periods_compressed 
	(guild_id, t, premium, num_messages, num_members, max_online, joins, leaves, max_voice) 
	VALUES ($1, $2, $3,      $4,              $5,         $6,        $7,      $8,   $9)
	ON CONFLICT (guild_id, t) DO UPDATE
	SET num_messages = server_stats_periods_compressed.num_messages + $4`

	// get a chunk of old rows
	rows, err := common.PQ.Query(qGetOld, lastID)
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	// sum them up in this map
	guildSums := make(map[time.Time]map[int64]int)

	for rows.Next() {
		var t time.Time
		var guildID int64
		var count int

		err = rows.Scan(&lastID, &t, &guildID, &count)
		if err != nil {
			rows.Close()
			return lastID, errors.WithStackIf(err)
		}

		t = t.Truncate(time.Hour * 24)
		if tBucket, ok := guildSums[t]; ok {
			tBucket[guildID] = tBucket[guildID] + count
		} else {
			guildSums[t] = make(map[int64]int)
			guildSums[t][guildID] = count
		}
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	// convert them to the new format
	for t, guilds := range guildSums {
		for g, count := range guilds {
			_, isPremium := premiumGuilds[g]

			_, err = tx.Exec(qSetNew, g, t, isPremium, count, 0, 0, 0, 0, 0)
			if err != nil {
				tx.Rollback()
				return lastID, errors.WithStackIf(err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	if common.RedisPool != nil {
		err = updateSubProgress("serverstats_migration_v2_progress_msgs", lastID, true)
	}

	return lastID, err
}

func migrateChunkV2Members(lastID int64, premiumGuilds map[int64]time.Time) (newLastID int64, err error) {
	const qGetOld = `SELECT id, guild_id, created_at, num_members, joins, leaves, max_online
	FROM server_stats_member_periods
	WHERE id > $1 ORDER BY ID ASC LIMIT 5000;`

	const qSetNew = `INSERT INTO server_stats_periods_compressed 
	(guild_id, t, premium, num_messages, num_members, max_online, joins, leaves, max_voice) 
	VALUES ($1, $2, $3,      $4,              $5,         $6,        $7,      $8,   $9)
	ON CONFLICT (guild_id, t) DO UPDATE
	SET num_members = GREATEST(server_stats_periods_compressed.num_members, $5),
	max_online = GREATEST(server_stats_periods_compressed.max_online, $6),
	max_voice = GREATEST(server_stats_periods_compressed.max_voice, $9),
	joins = server_stats_periods_compressed.joins + $7,
	leaves = server_stats_periods_compressed.leaves + $8
	`

	// get a chunk of old rows
	rows, err := common.PQ.Query(qGetOld, lastID)
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	// sum them up in this map
	guildSums := make(map[time.Time]map[int64]*migratedv2Row)

	for rows.Next() {
		var t time.Time
		var guildID int64
		var numMembers, joins, leaves, maxOnline int

		err = rows.Scan(&lastID, &guildID, &t, &numMembers, &joins, &leaves, &maxOnline)
		if err != nil {
			rows.Close()
			return lastID, errors.WithStackIf(err)
		}

		t = t.Truncate(time.Hour * 24)

		if tBucket, ok := guildSums[t]; ok {
			if row, ok := tBucket[guildID]; ok {
				// merge with existing
				row.Joins += joins
				row.Leaves += leaves

				if maxOnline > row.MaxOnline {
					row.MaxOnline = maxOnline
				}
				if numMembers > row.NumMembers {
					row.NumMembers = numMembers
				}
			} else {
				// new row
				tBucket[guildID] = &migratedv2Row{
					GuildID: guildID,

					Joins:      joins,
					Leaves:     leaves,
					NumMembers: numMembers,
					MaxOnline:  maxOnline,
				}
			}

		} else {
			// new day and row
			guildSums[t] = make(map[int64]*migratedv2Row)
			guildSums[t][guildID] = &migratedv2Row{
				GuildID: guildID,

				Joins:      joins,
				Leaves:     leaves,
				NumMembers: numMembers,
				MaxOnline:  maxOnline,
			}
		}
	}

	tx, err := common.PQ.Begin()
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	// convert them to the new format
	for t, guilds := range guildSums {
		for g, row := range guilds {
			_, isPremium := premiumGuilds[g]

			_, err = tx.Exec(qSetNew, g, t, isPremium, 0, row.NumMembers, row.MaxOnline, row.Joins, row.Leaves, 0)
			if err != nil {
				tx.Rollback()
				return lastID, errors.WithStackIf(err)
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return lastID, errors.WithStackIf(err)
	}

	if common.RedisPool != nil {
		err = updateSubProgress("serverstats_migration_v2_progress_members", lastID, true)
	}

	return lastID, err
}

type migratedv2Row struct {
	GuildID int64

	Joins      int
	Leaves     int
	NumMembers int
	MaxOnline  int
}

func updateSubProgress(key string, lastID int64, running bool) error {
	prog := &MigrationSubProgress{
		LastUpdated: time.Now(),
		LastID:      lastID,
		Running:     running,
	}

	return common.SetRedisJson(key, prog)
}

type MigrationSubProgress struct {
	LastUpdated time.Time
	LastID      int64
	Running     bool
}

func (m *MigrationSubProgress) IsRunning() bool {
	if time.Since(m.LastUpdated) > time.Minute*1 {
		return false
	}

	return m.Running
}

type MigrationProgress struct {
	MsgPeriods    *MigrationSubProgress
	MemberPeriods *MigrationSubProgress
}

func GetMigrationV2Progress() (*MigrationProgress, error) {
	var msgsProgress *MigrationSubProgress
	err := common.GetRedisJson("serverstats_migration_v2_progress_msgs", &msgsProgress)
	if err != nil {
		return nil, err
	}

	if msgsProgress == nil {
		msgsProgress = &MigrationSubProgress{
			LastID: -1,
		}
		logger.Infof("starting migration to v2 format for messages from sctatch")
	}

	var memberProgress *MigrationSubProgress
	err = common.GetRedisJson("serverstats_migration_v2_progress_members", &memberProgress)
	if err != nil {
		return nil, err
	}

	if memberProgress == nil {
		memberProgress = &MigrationSubProgress{
			LastID: -1,
		}
		logger.Infof("starting migration to v2 format for members from sctatch")
	}

	return &MigrationProgress{
		MsgPeriods:    msgsProgress,
		MemberPeriods: memberProgress,
	}, nil
}
