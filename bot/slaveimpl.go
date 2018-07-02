package bot

import (
	"github.com/jonas747/yagpdb/master/slave"
	"github.com/sirupsen/logrus"
)

// Slave implementation
type SlaveImpl struct {
}

var _ slave.Bot = (*SlaveImpl)(nil)

func (s *SlaveImpl) FullStart() {
	logrus.Println("Starting full start")

	stateLock.Lock()
	if state != StateSoftStarting {
		go ShardManager.Start()
	}

	state = StateRunningWithMaster
	stateLock.Unlock()
}

func (s *SlaveImpl) SoftStart() {
	logrus.Println("Starting soft start")

	stateLock.Lock()
	state = StateSoftStarting
	stateLock.Unlock()

	go ShardManager.Start()
}

func (s *SlaveImpl) Shutdown() {

}

func (s *SlaveImpl) StartShardTransferFrom() int {
	stateLock.Lock()
	state = StateShardMigrationFrom
	stateLock.Unlock()

	return ShardManager.GetNumShards()
}

func (s *SlaveImpl) StartShardTransferTo(numShards int) {
	ShardManager.SetNumShards(numShards)
	err := ShardManager.Init()
	if err != nil {
		panic("Failed initializing shard manager: " + err.Error())
	}

	stateLock.Lock()
	state = StateShardMigrationTo
	stateLock.Unlock()
}

func (s *SlaveImpl) StopShard(shard int) (sessionID string, sequence int64) {
	err := ShardManager.Sessions[shard].Close()
	if err != nil {
		logrus.WithError(err).Error("Failed stopping shard: ", err)
	}
	sessionID, sequence = ShardManager.Sessions[shard].GatewayManager.GetSessionInfo()
	return
}

func (s *SlaveImpl) StartShard(shard int, sessionID string, sequence int64) {
}
