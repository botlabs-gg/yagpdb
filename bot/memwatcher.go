package bot

import (
	"runtime/debug"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"

	"github.com/shirou/gopsutil/mem"
)

const MemFreeThreshold = 90

type MemWatcher struct {
	lastTimeFreed time.Time
}

var confEnableMemMonitor = config.RegisterOption("yagpdb.mem_monitor.enabled", "Enable the memory monitor, will attempt to free resources when os is running low", true)
var memLogger = common.GetFixedPrefixLogger("[mem_monitor]")

func watchMemusage() {
	if !confEnableMemMonitor.GetBool() {
		// not enabled
		memLogger.Info("memory monitor disabled")
		return
	}

	watcher := &MemWatcher{}
	go watcher.Run()
}

func (mw *MemWatcher) Run() {
	memLogger.Info("launching memory monitor")
	ticker := time.NewTicker(time.Second * 10)
	for {
		<-ticker.C
		mw.Check()
	}
}

func (mw *MemWatcher) Check() {
	if time.Since(mw.lastTimeFreed) < time.Minute*10 {
		return
	}

	sysMem, err := mem.VirtualMemory()
	if err != nil {
		memLogger.WithError(err).Error("failed retrieving os memory stats")
		return
	}

	if sysMem.UsedPercent > MemFreeThreshold {
		memLogger.Info("LOW SYSTEM MEMORY, ATTEMPTING TO FREE SOME")
		debug.FreeOSMemory()
		mw.lastTimeFreed = time.Now()
	}
}
