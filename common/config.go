package common

import (
	"github.com/kelseyhightower/envconfig"
)

type CoreConfig struct {
	Owner string
	BotID string

	ClientID     string
	ClientSecret string
	BotToken     string
	Host         string
	Email        string // The letsencrypt cert will use this email

	PQUsername string
	PQPassword string

	Redis string

	// Third party api's other than discord
	// for the Alyien text analysys plugin api access

	// AylienAppID  string `json:"aylien_app_id"`
	// AylienAppKey string `json:"aylien_app_key"`
}

func LoadConfig() (c *CoreConfig, err error) {
	c = &CoreConfig{}
	err = envconfig.Process("YAGPDB", c)
	return
}
