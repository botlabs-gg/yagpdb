package common

import (
	"github.com/Syfaro/haste-client"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/discordgo"
)

const (
	VERSION = "0.14 Dillweed ALPHA"
)

var (
	RedisPool  *pool.Pool
	BotSession *discordgo.Session
	Conf       *Config
	Hastebin   = haste.NewHaste("http://hastebin.com")
)
