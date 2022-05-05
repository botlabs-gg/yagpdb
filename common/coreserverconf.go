package common

import (
	"context"
	"database/sql"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common/models"
	"github.com/karlseguin/rcache"
	"github.com/volatiletech/sqlboiler/boil"
)

const CoreServerConfDBSchema = `
CREATE TABLE IF NOT EXISTS core_configs (
	guild_id BIGINT PRIMARY KEY,

	allowed_read_only_roles BIGINT[],
	allowed_write_roles BIGINT[],

	allow_all_members_read_only BOOLEAN NOT NULL,
	allow_non_members_read_only BOOLEAN NOT NULL
)

`

var CoreServerConfigCache = rcache.NewInt(coreServerConfigCacheFetcher, time.Minute)

func GetCoreServerConfCached(guildID int64) *models.CoreConfig {
	return CoreServerConfigCache.Get(int(guildID)).(*models.CoreConfig)
}

func coreServerConfigCacheFetcher(key int) interface{} {
	conf, err := models.FindCoreConfigG(context.Background(), int64(key))
	if err != nil && err != sql.ErrNoRows {
		logger.WithError(err).WithField("guild", key).Error("failed fetching core server config")
	}

	if conf == nil {
		conf = &models.CoreConfig{
			GuildID: int64(key),
		}
	}

	return conf
}

func ContextCoreConf(ctx context.Context) *models.CoreConfig {
	v := ctx.Value(ContextKeyCoreConfig)
	if v == nil {
		return nil
	}

	return v.(*models.CoreConfig)
}

func CoreConfigSave(ctx context.Context, m *models.CoreConfig) error {
	err := m.UpsertG(ctx, true, []string{"guild_id"}, boil.Infer(), boil.Infer())
	if err != nil {
		return err
	}

	CoreServerConfigCache.Delete(int(m.GuildID))

	return nil
}
