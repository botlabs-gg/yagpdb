package common

import (
	"database/sql"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/vattle/sqlboiler/boil"
	stdlog "log"
	"os"
)

const (
	VERSIONMAJOR = 0
	VERSIONMINOR = 24
	VERSIONPATCH = 0

	Testing = false // Disables stuff like command cooldowns
)

var (
	VERSIONNUMBER = fmt.Sprintf("%d.%d.%d", VERSIONMAJOR, VERSIONMINOR, VERSIONPATCH)
	VERSION       = VERSIONNUMBER + " RC-1"

	GORM        *gorm.DB
	PQ          *sql.DB
	RedisPool   *pool.Pool
	DSQLStateDB *sql.DB

	BotSession *discordgo.Session
	BotUser    *discordgo.User
	Conf       *CoreConfig
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

	if os.Getenv("YAGPDB_NO_CUSTOM_TRANSPORT") == "" {
		BotSession.Client.Transport = NewCustomDiscordHTTPTransport()
	}

	err = connectRedis(config.Redis)
	if err != nil {
		return err
	}

	err = connectDB(config.PQUsername, config.PQPassword, "yagpdb")

	BotUser, err = BotSession.User("@me")
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

	err := connectDB("postgres", "123", testDB)
	if err != nil {
		panic(err)
	}
}

func connectRedis(addr string) (err error) {
	// RedisPool, err = pool.NewCustom("tcp", addr, 25, redis.)
	RedisPool, err = pool.NewCustom("tcp", addr, 25, RedisDialFunc)
	if err != nil {
		logrus.WithError(err).Fatal("Failed initilizing redis pool")
	}
	return
}

func connectDB(user, pass, dbName string) error {
	db, err := gorm.Open("postgres", fmt.Sprintf("host=localhost user=%s dbname=%s sslmode=disable password=%s", user, dbName, pass))
	GORM = db
	PQ = db.DB()
	boil.SetDB(PQ)
	if err == nil {
		PQ.SetMaxOpenConns(5)
	}

	if os.Getenv("YAGPDB_SQLSTATE_ADDR") != "" {
		logrus.Info("Using special sql state db")
		addr := os.Getenv("YAGPDB_SQLSTATE_ADDR")
		user := os.Getenv("YAGPDB_SQLSTATE_USER")
		pass := os.Getenv("YAGPDB_SQLSTATE_PW")
		dbName := os.Getenv("YAGPDB_SQLSTATE_DB")

		db, err := sql.Open("postgres", fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password=%s", addr, user, dbName, pass))
		if err != nil {
			DSQLStateDB = PQ
			return err
		}

		DSQLStateDB = db

	} else {
		DSQLStateDB = PQ
	}

	return err
}
