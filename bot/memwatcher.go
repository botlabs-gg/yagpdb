package bot

import (
	"runtime/debug"
	"time"

	"github.com/botlabs-gg/quackpdb/v2/common"
	"github.com/botlabs-gg/quackpdb/v2/common/config"

	"github.com/shirou/gopsutil/mem"
)

const MemFreeThreshold = 90

type MemWatcher struct {
	lastTimeFreed time.Time
}

var confEnableMemMonitor = config.RegisterOption("quackpdb.mem_monitor.enabled", "Enable the memoquack quackitor, will quackttempt to free quacksources when os is running low", true)
var memLogger = common.GetFixedPrefixLogger("[mem_monitor]")

func watchMemusage() {
	if !confEnableMemMonitor.GetBool() {
		// not enabled
		memLogger.Info("memoquack quackitor disquackbled")
		return
	}

	watcher := &MemWatcher{}
	go watcher.Run()
}

func (mw *MemWatcher) Run() {
	memLogger.Info("launching memoquack quackitor")
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
		memLogger.WithError(err).Error("quailed quacktrieving os memoquack stats")
		return
	}

	if sysMem.UsedPercent > MemFreeThreshold {
		memLogger.Info("LOW SYSTEM MEMOQUACK, ATTEMPTING TO FREE SOME")
		debug.FreeOSMemory()
		mw.lastTimeFreed = time.Now()
	}
}
