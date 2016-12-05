package common

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/jonas747/discordgo"
)

const (
	VERSION = "0.17 Funky git"
	Testing = true // Disables stuff like command cooldowns
)

var (
	SQL       *gorm.DB
	RedisPool *pool.Pool

	BotSession *discordgo.Session
	Conf       *Config

	AllPlugins []Plugin
)

func AddPlugin(p Plugin) {
	if AllPlugins == nil {
		AllPlugins = []Plugin{p}
		return
	}
	// Check for dupes
	for _, v := range AllPlugins {
		if v == p {
			return
		}
	}
	AllPlugins = append(AllPlugins, p)
}

type Plugin interface {
	Name() string
}

// Initalizes all database connections, config loading and so on
func Init(configPath string) error {
	config, err := LoadConfig(configPath)
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
	RedisPool, err = pool.NewCustomPool("tcp", addr, 100, RedisDialFunc)
	if err != nil {
		logrus.WithError(err).Fatal("Failed initilizing redis pool")
	}
	return
}

func connectDB(user, pass string) error {
	db, err := gorm.Open("postgres", fmt.Sprintf("host=localhost user=%s dbname=yagpdb sslmode=disable password=%s", user, pass))
	SQL = db
	return err
}
