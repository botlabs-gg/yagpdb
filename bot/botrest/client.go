package botrest

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jonas747/discordgo"
	"github.com/jonas747/yagpdb/common"
	"github.com/jonas747/yagpdb/common/internalapi"
	"github.com/mediocregopher/radix/v3"
)

var clientLogger = common.GetFixedPrefixLogger("botrest_client")

func GetGuild(guildID int64) (g *discordgo.Guild, err error) {
	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/guild", &g)
	return
}

func GetBotMember(guildID int64) (m *discordgo.Member, err error) {
	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/botmember", &m)
	return
}

func GetOnlineCount(guildID int64) (c int64, err error) {
	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/onlinecount", &c)
	return
}

func GetMembers(guildID int64, members ...int64) (m []*discordgo.Member, err error) {
	stringed := make([]string, 0, len(members))
	for _, v := range members {
		stringed = append(stringed, strconv.FormatInt(v, 10))
	}

	query := url.Values{"users": stringed}
	encoded := query.Encode()

	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/members?"+encoded, &m)
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

	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/membercolors?"+encoded, &m)
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
	err = internalapi.GetWithGuild(guildID, discordgo.StrID(guildID)+"/channelperms/"+discordgo.StrID(channelID), &perms)
	return
}

func GetSessionInfo(addr string) (st []*shardSessionInfo, err error) {
	err = internalapi.GetWithAddress(addr, "/shard_sessions", &st)
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
	err = common.RedisPool.Do(radix.Cmd(&clustered, "EXISTS", "dshardorchestrator_nodes_z"))
	if err != nil {
		return nil, err
	}

	if clustered {
		return getNodeStatusesClustered()
	}

	var status *NodeStatus
	err = internalapi.GetWithShard(0, "node_status", &status)
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

	totalShards, _ := common.ServicePoller.GetShardCount()
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
	addr, err := common.ServicePoller.GetNodeAddress(nodeID)
	if err != nil {
		clientLogger.WithError(err).Error("failed retrieving rest address for bot for node id: ", nodeID)
		retCh <- err
		return
	}

	var status *NodeStatus
	err = internalapi.GetWithAddress(addr, "node_status", &status)
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

	err = internalapi.PostWithShard(shardID, fmt.Sprintf("shard/%d/reconnect"+queryParams, shardID), nil, nil)
	return
}

func SendReconnectAll(reidentify bool) (err error) {
	queryParams := ""
	if reidentify {
		queryParams = "?reidentify=1"
	}

	err = internalapi.PostWithShard(0, "shard/*/reconnect"+queryParams, nil, nil)
	return
}
