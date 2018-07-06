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

func (s *SlaveImpl) StartShard() {

}

func (s *SlaveImpl) StopShard() {

}
