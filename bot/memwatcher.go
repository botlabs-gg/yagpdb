package bot

import (
	"github.com/shirou/gopsutil/mem"
	"os"
	"runtime/debug"
	"time"
)

const MemFreeThreshold = 90

type MemWatcher struct {
	lastTimeFreed time.Time
}

func watchMemusage() {
	if os.Getenv("YAGPDB_ENABLE_MEM_MONITOR") == "" || os.Getenv("YAGPDB_ENABLE_MEM_MONITOR") == "no" || os.Getenv("YAGPDB_ENABLE_MEM_MONITOR") == "false" {
		// not enabled
		return
	}

	watcher := &MemWatcher{}
	go watcher.Run()
}

func (mw *MemWatcher) Run() {
	logger.Info("[mem_monitor] launching memory monitor")
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
		logger.WithError(err).Error("[mem_monitor] failed retrieving os memory stats")
		return
	}

	if sysMem.UsedPercent > MemFreeThreshold {
		logger.Info("[mem_monitor] LOW SYSTEM MEMORY, ATTEMPTING TO FREE SOME")
		debug.FreeOSMemory()
		mw.lastTimeFreed = time.Now()
	}
}
