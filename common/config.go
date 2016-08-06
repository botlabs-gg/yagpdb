package common

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BotToken     string `json:"bot_token"`
	Host         string `json:"host"`

	Redis string `json:"redis"`

	// Third party api's other than discord
	PastebinDevKey string `json:"pastebin_dev_key"`
}

func LoadConfig(path string) (c *Config, err error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = json.Unmarshal(file, &c)
	return
}
