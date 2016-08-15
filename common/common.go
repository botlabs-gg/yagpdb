package common

import (
	"github.com/Syfaro/haste-client"
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
)

const (
	VERSION = "0.10 Freezing ALPHA"
)

var (
	RedisPool  *pool.Pool
	BotSession *discordgo.Session
	Conf       *Config
	Hastebin   = haste.NewHaste("http://hastebin.com")
)
