package reputation

//go:generate sqlboiler -w "reputation_settings,reputation_users,reputation_log" postgres

import (
	"errors"
	"github.com/fzzy/radix/redis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/web"
)

type Plugin struct{}

func RegisterPlugin() {
	plugin := &Plugin{}
	bot.RegisterPlugin(plugin)
	web.RegisterPlugin(plugin)
}

func (p *Plugin) Name() string {
	return "Reputation"
}

type Settings struct {
	Cooldown int
	Enabled  bool
}

func (s *Settings) Save(client *redis.Client, guildID string) error {
	client.Append("SET", "reputation_enabled:"+guildID, s.Enabled)
	client.Append("SET", "reputation_cooldown:"+guildID, s.Cooldown)

	_, err := common.GetRedisReplies(client, 2)
	return err
}

func GetFullSettings(client *redis.Client, guildID string) (setings *Settings, err error) {
	client.Append("GET", "reputation_enabled:"+guildID)
	client.Append("GET", "reputation_cooldown:"+guildID)

	replies, _ := common.GetRedisReplies(client, 2)

	for _, r := range replies {
		if r.Err != nil {
			if _, ok := r.Err.(*redis.CmdError); ok {
				return &Settings{Cooldown: 180, Enabled: false}, nil
			}
			return nil, r.Err
		}

		if r.Type == redis.NilReply {
			return &Settings{Cooldown: 180, Enabled: false}, nil
		}
	}

	enabled, err1 := replies[0].Bool()
	cooldown, err2 := replies[1].Int()

	if err1 != nil {
		return nil, err1
	}

	if err2 != nil {
		return nil, err2
	}

	return &Settings{
		Cooldown: cooldown,
		Enabled:  enabled,
	}, nil
}

var ErrUserNotFound = errors.New("User not found or has never been given rep")

func GetUserStats(client *redis.Client, guildID, user string) (score int64, rank int, err error) {
	reply := client.Cmd("ZSCORE", "reputation_users:"+guildID, user)
	if reply.Type == redis.NilReply {
		return 0, 0, ErrUserNotFound
	}

	score, err = reply.Int64()
	if err != nil {
		return
	}

	rank, err = client.Cmd("ZREVRANK", "reputation_users:"+guildID, user).Int()
	return
}
