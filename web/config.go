package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	BotToken     string `json:"bot_token"`
	Redis        string `json:"redis"`
}

func LoadConfig(path string) (c *Config, err error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	err = json.Unmarshal(file, &c)
	return
}
