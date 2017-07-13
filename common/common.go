package common

import (
	"database/sql"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
	"github.com/vattle/sqlboiler/boil"
	stdlog "log"
	"os"
)

const (
	VERSIONMAJOR = 0
	VERSIONMINOR = 21
	VERSIONPATCH = 3

	Testing = false // Disables stuff like command cooldowns
	// Testing = true // Disables stuff like command cooldowns
)

var (
	VERSIONNUMBER = fmt.Sprintf("%d.%d.%d", VERSIONMAJOR, VERSIONMINOR, VERSIONPATCH)
	VERSION       = VERSIONNUMBER + " Jazzy"

	GORM        *gorm.DB
	PQ          *sql.DB
	RedisPool   *pool.Pool
	DSQLStateDB *sql.DB

	BotSession *discordgo.Session
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

	err = connectRedis(config.Redis)
	if err != nil {
		return err
	}

	err = connectDB(config.PQUsername, config.PQPassword)

	return err
}

func connectRedis(addr string) (err error) {
	RedisPool, err = pool.NewCustomPool("tcp", addr, 25, RedisDialFunc)
	if err != nil {
		logrus.WithError(err).Fatal("Failed initilizing redis pool")
	}
	return
}
func connectDB(user, pass string) error {
	db, err := gorm.Open("postgres", fmt.Sprintf("host=localhost user=%s dbname=yagpdb sslmode=disable password=%s", user, pass))
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
