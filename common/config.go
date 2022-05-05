package common

import (
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
)

var (
	confOwner  = config.RegisterOption("yagpdb.owner", "ID of the owner of the bot", 0)
	confOwners = config.RegisterOption("yagpdb.owners", "Comma seperated IDs of the owners of the bot", "")

	ConfClientID     = config.RegisterOption("yagpdb.clientid", "Client ID of the discord application", nil)
	ConfClientSecret = config.RegisterOption("yagpdb.clientsecret", "Client Secret of the discord application", nil)
	ConfBotToken     = config.RegisterOption("yagpdb.bottoken", "Token of the bot user", nil)
	ConfHost         = config.RegisterOption("yagpdb.host", "Host without the protocol, example: example.com, used by the webserver", nil)
	ConfEmail        = config.RegisterOption("yagpdb.email", "Email used when fetching lets encrypt certificate", "")

	ConfPQHost     = config.RegisterOption("yagpdb.pqhost", "Postgres host", "localhost")
	ConfPQUsername = config.RegisterOption("yagpdb.pqusername", "Postgres user", "postgres")
	ConfPQPassword = config.RegisterOption("yagpdb.pqpassword", "Postgres passoword", "")
	ConfPQDB       = config.RegisterOption("yagpdb.pqdb", "Postgres database", "yagpdb")

	ConfMaxCCR            = config.RegisterOption("yagpdb.max_ccr", "Maximum number of concurrent outgoing requests to discord", 25)
	ConfDisableKeepalives = config.RegisterOption("yagpdb.disable_keepalives", "Disables keepalive connections for outgoing requests to discord, this shouldn't be needed but i had networking issues once so i had to", false)

	confNoSchemaInit = config.RegisterOption("yagpdb.no_schema_init", "Disable schema intiialization", false)

	confMaxSQLConns = config.RegisterOption("yagdb.pq_max_conns", "Max connections to postgres", 3)

	ConfTotalShards             = config.RegisterOption("yagpdb.sharding.total_shards", "Total number shards", 0)
	ConfActiveShards            = config.RegisterOption("yagpdb.sharding.active_shards", "Shards active on this hoste, ex: '1-10,25'", "")
	ConfLargeBotShardingEnabled = config.RegisterOption("yagpdb.large_bot_sharding", "Set to enable large bot sharding (for 200k+ guilds)", false)
	ConfBucketsPerNode          = config.RegisterOption("yagpdb.shard.buckets_per_node", "Number of buckets per node", 8)
	ConfShardBucketSize         = config.RegisterOption("yagpdb.shard.shard_bucket_size", "Shards per bucket", 2)

	BotOwners []int64
)

var configLoaded = false

func LoadConfig() (err error) {
	if configLoaded {
		return nil
	}

	configLoaded = true

	config.AddSource(&config.EnvSource{})
	config.AddSource(&config.RedisConfigStore{Pool: RedisPool})
	config.Load()

	requiredConf := []*config.ConfigOption{
		ConfClientID,
		ConfClientSecret,
		ConfBotToken,
		ConfHost,
	}

	for _, v := range requiredConf {
		if v.LoadedValue == nil {
			envFormat := strings.ToUpper(strings.Replace(v.Name, ".", "_", -1))
			return errors.Errorf("Did not set required config option: %q (%s as env var)", v.Name, envFormat)
		}
	}

	if int64(confOwner.GetInt()) != 0 {
		BotOwners = append(BotOwners, int64(confOwner.GetInt()))
	}

	ownersStr := confOwners.GetString()
	split := strings.Split(ownersStr, ",")
	for _, o := range split {
		parsed, _ := strconv.ParseInt(o, 10, 64)
		if parsed != 0 {
			BotOwners = append(BotOwners, parsed)
		}
	}

	return nil
}
