package common

import (
	"database/sql"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common/fixedpool"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
	// "github.com/mediocregopher/radix.v2/pool"
	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
	stdlog "log"
	"os"
)

const (
	VERSIONMAJOR = 0
	VERSIONMINOR = 29
	VERSIONPATCH = 8
	Testing      = false // Disables stuff like command cooldowns
)

var (
	VERSIONNUMBER = fmt.Sprintf("%d.%d.%d", VERSIONMAJOR, VERSIONMINOR, VERSIONPATCH)
	VERSION       = VERSIONNUMBER + " Reluctant"

	GORM *gorm.DB
	PQ   *sql.DB

	// RedisPool   *pool.Pool
	RedisPool redisPool

	BotSession *discordgo.Session
	BotUser    *discordgo.User
	Conf       *CoreConfig

	RedisPoolSize = 25
)

// Initalizes all database connections, config loading and so on
func Init() error {

	stdlog.SetOutput(&STDLogProxy{})
	stdlog.SetFlags(0)

	if Testing {
		logrus.SetLevel(logrus.DebugLevel)
	}

	config, err := LoadConfig()
	if err != nil {
		return err
	}
	Conf = config

	BotSession, err = discordgo.New(config.BotToken)
	if err != nil {
		return err
	}
	BotSession.MaxRestRetries = 3

	err = connectRedis(config.Redis)
	if err != nil {
		return err
	}

	err = connectDB(config.PQHost, config.PQUsername, config.PQPassword, "yagpdb")
	if err != nil {
		panic(err)
	}

	BotUser, err = BotSession.UserMe()
	if err != nil {
		panic(err)
	}

	return err
}

func InitTest() {
	testDB := os.Getenv("YAGPDB_TEST_DB")
	if testDB == "" {
		return
	}

	err := connectDB("localhost", "postgres", "123", testDB)
	if err != nil {
		panic(err)
	}
}

func connectRedis(addr string) (err error) {
	// RedisPool, err = pool.NewCustom("tcp", addr, 25, redis.)
	if os.Getenv("YAGPDB_LEGACY_REDIS_POOL") != "" {
		logrus.Info("Using legacy redis pool")
		RedisPool, err = pool.NewCustom("tcp", addr, RedisPoolSize, RedisDialFunc)
	} else {
		logrus.Info("Using new redis pool, set YAGPDB_LEGACY_REDIS_POOL=yes if it's broken")
		RedisPool, err = fixedpool.NewCustom("tcp", addr, RedisPoolSize, redis.Dial)
	}

	if err != nil {
		logrus.WithError(err).Fatal("Failed initilizing redis pool")
	}

	return
}

func connectDB(host, user, pass, dbName string) error {
	if host == "" {
		host = "localhost"
	}

	db, err := gorm.Open("postgres", fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password='%s'", host, user, dbName, pass))
	GORM = db
	PQ = db.DB()
	boil.SetDB(PQ)
	if err == nil {
		PQ.SetMaxOpenConns(5)
	}

	return err
}

type redisPool interface {
	Cmd(cmd string, args ...interface{}) *redis.Resp
	Put(conn *redis.Client)
	Get() (*redis.Client, error)
}
