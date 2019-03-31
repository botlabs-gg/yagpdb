package botrest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
	"github.com/mediocregopher/radix"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	ErrServerError    = errors.New("botrest server is having issues")
	ErrCantFindServer = errors.New("can't find botrest server for provided shard")
)

func GetServerAddrForGuild(guildID int64) string {
	shard := bot.GuildShardID(guildID)
	return GetServerAddrForShard(shard)
}

func GetServerAddrForShard(shard int) string {
	resp := ""
	err := common.RedisPool.Do(radix.Cmd(&resp, "GET", RedisKeyShardAddressMapping(shard)))
	if err != nil {
		logrus.WithError(err).Error("[botrest] failed retrieving shard server addr")
	}

	return resp
}

func Get(shard int, url string, dest interface{}) error {
	serverAddr := GetServerAddrForShard(shard)
	if serverAddr == "" {
		return ErrCantFindServer
	}

	return GetWithAddress(serverAddr, url, dest)
}

func GetWithAddress(addr string, url string, dest interface{}) error {
	resp, err := http.Get("http://" + addr + "/" + url)
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

func Post(shard int, url string, bodyData interface{}, dest interface{}) error {
	serverAddr := GetServerAddrForShard(shard)
	if serverAddr == "" {
		return ErrCantFindServer
	}

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
	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/guild", &g)
	return
}

func GetBotMember(guildID int64) (m *discordgo.Member, err error) {
	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/botmember", &m)
	return
}

func GetOnlineCount(guildID int64) (c int64, err error) {
	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/onlinecount", &c)
	return
}

func GetMembers(guildID int64, members ...int64) (m []*discordgo.Member, err error) {
	stringed := make([]string, 0, len(members))
	for _, v := range members {
		stringed = append(stringed, strconv.FormatInt(v, 10))
	}

	query := url.Values{"users": stringed}
	encoded := query.Encode()

	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/members?"+encoded, &m)
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

	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/membercolors?"+encoded, &m)
	return
}

func GetMemberMultiGuild(userID int64, guilds ...int64) (members []*discordgo.Member, err error) {

	members = make([]*discordgo.Member, 0, len(guilds))

	for _, v := range guilds {
		m, err := GetMembers(v, userID)
		if err == nil && len(m) > 0 {
			members = append(members, m[0])
		}
	}

	return
}

func GetChannelPermissions(guildID, channelID int64) (perms int64, err error) {
	err = Get(bot.GuildShardID(guildID), discordgo.StrID(guildID)+"/channelperms/"+discordgo.StrID(channelID), &perms)
	return
}

type NodeStatus struct {
	ID     string
	Shards []*ShardStatus `json:"shards"`
}

func GetNodeStatuses() (st []*NodeStatus, err error) {
	// retrieve a list of nodes

	// Special handling if were in clustered mode
	var clustered bool
	err = common.RedisPool.Do(radix.Cmd(&clustered, "EXISTS", "dshardorchestrator_nodes_z"))
	if err != nil {
		return nil, err
	}

	if clustered {
		return getNodeStatusesClustered()
	}

	var status []*ShardStatus
	err = Get(0, "gw_status", &status)
	if err != nil {
		return nil, err
	}

	return []*NodeStatus{&NodeStatus{
		ID:     "N/A",
		Shards: status,
	}}, nil
}

func getNodeStatusesClustered() (st []*NodeStatus, err error) {
	nodeIDs, err := common.GetActiveNodes()
	if err != nil {
		return nil, err
	}

	for _, n := range nodeIDs {
		// retrieve the REST address for this node
		var addr string
		err = common.RedisPool.Do(radix.Cmd(&addr, "GET", RedisKeyNodeAddressMapping(n)))
		if err != nil {
			logrus.WithError(err).Error("failed retrieving rest address for bot for node id: ", n)
			continue
		}

		var status []*ShardStatus
		err = GetWithAddress(addr, "gw_status", &status)
		if err != nil {
			logrus.WithError(err).Error("failed retrieving shard status for node ", n)
			continue
		}

		st = append(st, &NodeStatus{
			ID:     n,
			Shards: status,
		})
	}

	return
}

func SendReconnectShard(shardID int, reidentify bool) (err error) {
	queryParams := ""
	if reidentify {
		queryParams = "?reidentify=1"
	}

	err = Post(shardID, fmt.Sprintf("shard/%d/reconnect"+queryParams, shardID), nil, nil)
	return
}

func SendReconnectAll(reidentify bool) (err error) {
	queryParams := ""
	if reidentify {
		queryParams = "?reidentify=1"
	}

	err = Post(0, "shard/*/reconnect"+queryParams, nil, nil)
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
		err := Get(0, "ping", &dest)
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
