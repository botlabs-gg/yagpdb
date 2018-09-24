package botrest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	ErrServerError = errors.New("reststate server is having issues")
)

func Get(url string, dest interface{}) error {
	resp, err := http.Get("http://" + serverAddr + "/" + url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errDest string
		err := json.NewDecoder(resp.Body).Decode(&errDest)
		if err != nil {
			return ErrServerError
		}

		return errors.New(errDest)
	}

	return errors.WithMessage(json.NewDecoder(resp.Body).Decode(dest), "json.Decode")
}

func Post(url string, bodyData interface{}, dest interface{}) error {
	var bodyBuf bytes.Buffer
	if bodyData != nil {
		encoder := json.NewEncoder(&bodyBuf)
		err := encoder.Encode(bodyData)
		if err != nil {
			return err
		}
	}
	resp, err := http.Post("http://"+serverAddr+"/"+url, "application/json", &bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errDest string
		err := json.NewDecoder(resp.Body).Decode(&errDest)
		if err != nil {
			return ErrServerError
		}

		return errors.New(errDest)
	}

	if dest == nil {
		return nil
	}

	return errors.WithMessage(json.NewDecoder(resp.Body).Decode(dest), "json.Decode")
}

func GetGuild(guildID int64) (g *discordgo.Guild, err error) {
	err = Get(discordgo.StrID(guildID)+"/guild", &g)
	return
}

func GetBotMember(guildID int64) (m *discordgo.Member, err error) {
	err = Get(discordgo.StrID(guildID)+"/botmember", &m)
	return
}

func GetOnlineCount(guildID int64) (c int64, err error) {
	err = Get(discordgo.StrID(guildID)+"/onlinecount", &c)
	return
}

func GetMembers(guildID int64, members ...int64) (m []*discordgo.Member, err error) {
	stringed := make([]string, 0, len(members))
	for _, v := range members {
		stringed = append(stringed, strconv.FormatInt(v, 10))
	}

	query := url.Values{"users": stringed}
	encoded := query.Encode()

	err = Get(discordgo.StrID(guildID)+"/members?"+encoded, &m)
	return
}

func GetMemberColors(guildID int64, members ...int64) (m map[string]int, err error) {
	m = make(map[string]int)

	stringed := make([]string, 0, len(members))
	for _, v := range members {
		stringed = append(stringed, strconv.FormatInt(v, 10))
	}

	query := url.Values{"users": stringed}
	encoded := query.Encode()

	err = Get(discordgo.StrID(guildID)+"/membercolors?"+encoded, &m)
	return
}

func GetChannelPermissions(guildID, channelID int64) (perms int64, err error) {
	err = Get(discordgo.StrID(guildID)+"/channelperms/"+discordgo.StrID(channelID), &perms)
	return
}

func GetShardStatuses() (st []*ShardStatus, err error) {
	err = Get("gw_status", &st)
	return
}

func SendReconnectShard(shardID int, reidentify bool) (err error) {
	queryParams := ""
	if reidentify {
		queryParams = "?reidentify=1"
	}

	err = Post(fmt.Sprintf("shard/%d/reconnect"+queryParams, shardID), nil, nil)
	return
}

func SendReconnectAll(reidentify bool) (err error) {
	queryParams := ""
	if reidentify {
		queryParams = "?reidentify=1"
	}

	err = Post("shard/*/reconnect"+queryParams, nil, nil)
	return
}

var (
	lastPing      time.Time
	lastPingMutex sync.RWMutex
)

func RunPinger() {
	lastFailed := false
	for {
		time.Sleep(time.Second)

		var dest string
		err := Get("ping", &dest)
		if err != nil {
			if !lastFailed {
				logrus.Warn("Ping to bot failed: ", err)
				lastFailed = true
			}
			continue
		} else if lastFailed {
			logrus.Info("Ping to bot succeeded again after failing!")
		}

		lastPingMutex.Lock()
		lastPing = time.Now()
		lastPingMutex.Unlock()
		lastFailed = false
	}
}

// Returns wether the bot is running or not, (time since last sucessfull ping was less than 5 seconds)
func BotIsRunning() bool {
	lastPingMutex.RLock()
	defer lastPingMutex.RUnlock()
	return time.Since(lastPing) < time.Second*5
}
