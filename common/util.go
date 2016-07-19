package common

import (
	"github.com/fzzy/radix/redis"
)

func GetRedisReplies(client *redis.Client, n int) []*redis.Reply {
	out := make([]*redis.Reply, n)
	for i := 0; i < n; i++ {
		out[i] = client.GetReply()
	}
	return out
}

type RedisCmd struct {
	Name string
	Args []interface{}
}

// Will do the following commands and stop if an error occurs
func SafeRedisCommands(client *redis.Client, cmds []*RedisCmd) ([]*redis.Reply, error) {
	out := make([]*redis.Reply, 0)
	for _, cmd := range cmds {
		reply := client.Cmd(cmd.Name, cmd.Args...)
		out = append(out, reply)
		if reply.Err != nil {
			return out, reply.Err
		}
	}
	return out, nil
}
