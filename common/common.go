package common

import (
	"github.com/bwmarrin/discordgo"
	"github.com/fzzy/radix/extra/pool"
	"github.com/jonas747/yagpdb/common/pastebin"
)

const (
	VERSION = "0.9 Judgemental ALPHA"
)

var (
	RedisPool  *pool.Pool
	BotSession *discordgo.Session
	Conf       *Config
	Pastebin   *pastebin.Pastebin
)
