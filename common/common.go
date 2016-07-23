package common

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
)

var (
	RedisPool  *pool.Pool
	BotSession *discordgo.Session
	Conf       *Config
)
