package botrest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/jonas747/discordgo"
	"github.com/jonas747/retryableredis"
	"github.com/jonas747/yagpdb/bot"
	"github.com/jonas747/yagpdb/common"
)

var (
	ErrServerError    = errors.New("botrest server is having issues")
	ErrCantFindServer = errors.New("can't find botrest server for provided shard")
)

var (
	clientLogger = common.GetFixedPrefixLogger("botrest_client")
)

func GetServerAddrForGuild(guildID int64) string {
	shard := bot.GuildShardID(guildID)
	return GetServerAddrForShard(shard)
}

func GetServerAddrForShard(shard int) string {
	resp := ""
	err := common.RedisPool.Do(retryableredis.Cmd(&resp, "GET", RedisKeyShardAddressMapping(shard)))
	if err != nil {
		clientLogger.WithError(err).Error("failed retrieving shard server addr")
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
	ID     string         `json:"id"`
	Shards []*ShardStatus `json:"shards"`
	Host   string         `json:"host"`
	Uptime time.Duration  `json:"uptime"`
}

type NodeStatusesResponse struct {
	Nodes         []*NodeStatus `json:"nodes"`
	MissingShards []int         `json:"missing_shards"`
	TotalShards   int           `json:"total_shards"`
}

func GetNodeStatuses() (st *NodeStatusesResponse, err error) {
	// retrieve a list of nodes

	// Special handling if were in clustered mode
	var clustered bool
	err = common.RedisPool.Do(retryableredis.Cmd(&clustered, "EXISTS", "dshardorchestrator_nodes_z"))
	if err != nil {
		return nil, err
	}

	if clustered {
		return getNodeStatusesClustered()
	}

	var status *NodeStatus
	err = Get(0, "node_status", &status)
	if err != nil {
		return nil, err
	}

	status.ID = "N/A"
	return &NodeStatusesResponse{
		Nodes:       []*NodeStatus{status},
		TotalShards: 1,
	}, nil
}

func getNodeStatusesClustered() (st *NodeStatusesResponse, err error) {
	nodeIDs, err := common.GetActiveNodes()
	if err != nil {
		return nil, err
	}

	totalShards := bot.GetTotalShards()
	st = &NodeStatusesResponse{
		TotalShards: int(totalShards),
	}

	// send requests
	resultCh := make(chan interface{}, len(nodeIDs))
	for _, n := range nodeIDs {
		go getNodeStatus(n, resultCh)
	}

	timeout := time.After(time.Second * 3)

	// fetch responses
	for index := 0; index < len(nodeIDs); index++ {
		select {
		case <-timeout:
			clientLogger.Errorf("Timed out waiting for %d nodes", len(nodeIDs)-index)
			break
		case result := <-resultCh:
			switch t := result.(type) {
			case error:
				continue
			case *NodeStatus:
				st.Nodes = append(st.Nodes, t)
			}
		}
	}

	// check for missing nodes/shards
OUTER:
	for i := 0; i < int(totalShards); i++ {
		for _, node := range st.Nodes {
			for _, shard := range node.Shards {
				if shard.ShardID == i {
					continue OUTER // shard found
				}
			}
		}

		// shard not found
		st.MissingShards = append(st.MissingShards, i)
	}

	return
}

func getNodeStatus(nodeID string, retCh chan interface{}) {
	// retrieve the REST address for this node
	var addr string
	err := common.RedisPool.Do(retryableredis.Cmd(&addr, "GET", RedisKeyNodeAddressMapping(nodeID)))
	if err != nil {
		clientLogger.WithError(err).Error("failed retrieving rest address for bot for node id: ", nodeID)
		retCh <- err
		return
	}

	var status *NodeStatus
	err = GetWithAddress(addr, "node_status", &status)
	if err != nil {
		clientLogger.WithError(err).Error("failed retrieving shard status for node ", nodeID)
		retCh <- err
		return
	}

	retCh <- status
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
				clientLogger.Warn("Ping to bot failed: ", err)
				lastFailed = true
			}
			continue
		} else if lastFailed {
			clientLogger.Info("Ping to bot succeeded again after failing!")
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
