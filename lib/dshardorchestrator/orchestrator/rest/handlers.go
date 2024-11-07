package rest

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/botlabs-gg/yagpdb/v2/lib/dshardorchestrator/orchestrator"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type StatusResponse struct {
	Nodes []*orchestrator.NodeStatus
}

func (ra *RESTAPI) handleGETStatus(c *gin.Context) {
	status := ra.orchestrator.GetFullNodesStatus()
	c.JSON(http.StatusOK, &StatusResponse{
		Nodes: status,
	})
}

type BasicResponse struct {
	Message string
	Error   bool
}

func sendBasicResponse(c *gin.Context, err error, successMessage string) {
	status := http.StatusOK
	var resp interface{}

	if err != nil {
		resp = &BasicResponse{
			Error:   true,
			Message: err.Error(),
		}
		status = http.StatusInternalServerError
	} else {
		resp = &BasicResponse{
			Message: successMessage,
		}
	}

	c.JSON(status, resp)
}

func (ra *RESTAPI) handlePOSTStartNode(c *gin.Context) {
	id, err := ra.orchestrator.StartNewNode()
	sendBasicResponse(c, err, "started a new node successfully: "+id)
}

func (ra *RESTAPI) handlePOSTShutdownNode(c *gin.Context) {
	node, _ := c.GetPostForm("node_id")
	fmt.Println("REST: should shut down " + node)

	err := ra.orchestrator.ShutdownNode(node)
	sendBasicResponse(c, err, "stopped node successfully")
}

func (ra *RESTAPI) handlePOSTMigrateShard(c *gin.Context) {
	shardIDStr, _ := c.GetPostForm("shard")
	dstNodeID, _ := c.GetPostForm("destination_node")

	if shardIDStr == "" || dstNodeID == "" {
		sendBasicResponse(c, errors.New("destination_node or shard not provided"), "")
		return
	}

	parsedShardID, err := strconv.Atoi(shardIDStr)
	if err != nil {
		sendBasicResponse(c, errors.WithMessage(err, "parse-shardid"), "")
		return
	}

	err = ra.orchestrator.StartShardMigration(dstNodeID, parsedShardID)
	sendBasicResponse(c, err, "started shard migration process")
}

func (ra *RESTAPI) handlePOSTMigrateNode(c *gin.Context) {
	originNode, _ := c.GetPostForm("origin_node")
	dstNodeID, _ := c.GetPostForm("destination_node")
	shutdownOld := false
	if s, ok := c.GetPostForm("shutdown"); ok && s == "true" {
		shutdownOld = true
	}

	if dstNodeID == "" || originNode == "" {
		sendBasicResponse(c, errors.New("destination_node or origin_node not provided"), "")
		return
	}

	err := ra.orchestrator.MigrateFullNode(originNode, dstNodeID, shutdownOld)
	sendBasicResponse(c, err, "migrated node")
}

func (ra *RESTAPI) handlePOSTFullMigration(c *gin.Context) {
	err := ra.orchestrator.MigrateAllNodesToNewNodes(true)
	sendBasicResponse(c, err, "migrated all nodes to new nodes")
}

func (ra *RESTAPI) handlePOSTStopShard(c *gin.Context) {
	shardIDStr, _ := c.GetPostForm("shard")

	parsedShardID, err := strconv.Atoi(shardIDStr)
	if err != nil {
		sendBasicResponse(c, errors.WithMessage(err, "parse-shardid"), "")
		return
	}

	err = ra.orchestrator.StopShard(parsedShardID)
	sendBasicResponse(c, err, "sent stop shard action")
}

func (ra *RESTAPI) handlePOSTBlacklistNode(c *gin.Context) {
	node, _ := c.GetPostForm("node")

	if node == "" {
		sendBasicResponse(c, errors.New("node not provided"), "")
		return
	}

	ra.orchestrator.BlacklistNode(node)
	sendBasicResponse(c, nil, "denied node node")
}

func (ra *RESTAPI) handlePOSTPullVersion(c *gin.Context) {
	if ra.orchestrator.VersionUpdater == nil {
		sendBasicResponse(c, errors.New("no updater provided"), "")
		return
	}

	newVersion, err := ra.orchestrator.VersionUpdater.PullNewVersion()
	sendBasicResponse(c, err, newVersion)
}

func (ra *RESTAPI) handleGETDeployedVersion(c *gin.Context) {
	version, err := ra.orchestrator.NodeLauncher.LaunchVersion()
	sendBasicResponse(c, err, version)
}
