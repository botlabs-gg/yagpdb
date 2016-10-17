package common

import (
	"fmt"
	"github.com/fzzy/radix/redis"
	"strings"
	"time"
)

func RedisDialFunc(network, addr string) (client *redis.Client, err error) {
	for {
		client, err = redis.Dial(network, addr)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "socket: too many open files") ||
				strings.Contains(errStr, "cannot assign requested address") {
				// Sleep for 100 milliseconds and try again
				time.Sleep(time.Millisecond * 100)
				continue
			} else {
				return
			}
		} else {
			break
		}
	}

	return
}

func GenID(client *redis.Client, key string) string {
	idInt, err := client.Cmd("INCR", key).Int64()
	if err != nil {
		panic(err)
	}

	return fmt.Sprintf("r%d", idInt)
}
