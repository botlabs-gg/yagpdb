package common

import (
	"github.com/kelseyhightower/envconfig"
)

type CoreConfig struct {
	Owner int64
	BotID int64

	ClientID     string
	ClientSecret string
	BotToken     string
	Host         string
	Email        string // The letsencrypt cert will use this email

	PQHost     string
	PQUsername string
	PQPassword string
	Redis      string

	DogStatsdAddress string
}

func LoadConfig() (c *CoreConfig, err error) {
	c = &CoreConfig{}
	err = envconfig.Process("YAGPDB", c)
	return
}
