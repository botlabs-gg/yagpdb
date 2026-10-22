package bot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/metrics"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/botlabs-gg/yagpdb/v2/common"
	"github.com/botlabs-gg/yagpdb/v2/common/config"
	"github.com/sirupsen/logrus"

	"github.com/shirou/gopsutil/mem"
)

const (
	MemFreeThreshold = 90

	// Forcing a full GC while the host is swapping pages the whole heap back in and stalls every node.
	SwapBusyThreshold = 25

	// Consecutive fresh checks the heap must stay over a dump threshold, so a single spike does not trigger one.
	// A check is fresh only if a GC finished since the previous one, as live heap is only measured by GC.
	HeapDumpSustainedChecks = 3
)

var (
	confEnableMemMonitor    = config.RegisterOption("yagpdb.mem_monitor.enabled", "Enable the memory monitor, will attempt to free resources when os is running low", true)
	confMemStatsInterval    = config.RegisterOption("yagpdb.mem_monitor.stats_interval_minutes", "How often to log this node's heap usage, 0 disables it", 5)
	confHeapDumpMultipliers = config.RegisterOption("yagpdb.mem_monitor.heap_dump_multipliers", "Comma separated multiples of the baseline live heap, a heap and goroutine profile is written once as each is crossed, empty disables it", "2,3")
	confHeapDumpWarmup      = config.RegisterOption("yagpdb.mem_monitor.heap_dump_warmup_minutes", "Minutes after start before the baseline live heap is tracked, covers the initial state load", 15)
	confHeapDumpDir         = config.RegisterOption("yagpdb.mem_monitor.heap_dump_dir", "Directory heap dumps are written to", "heap_dumps")
	confHeapDumpMaxFiles    = config.RegisterOption("yagpdb.mem_monitor.heap_dump_max_files", "Oldest files in the heap dump directory beyond this count are deleted", 20)
)

var memLogger = common.GetFixedPrefixLogger("[mem_monitor]")

type MemWatcher struct {
	lastTimeFreed   time.Time
	lastStatsLogged time.Time

	startedAt time.Time

	// Lowest live heap seen since warmup, follows the slow post-startup decline down so it never reads as growth.
	baselineLive uint64

	dumpMultipliers     []float64
	nextDump            int
	checksOverThreshold int
	lastGCCycles        uint64
}

func watchMemusage() {
	if !confEnableMemMonitor.GetBool() {
		// not enabled
		memLogger.Info("memory monitor disabled")
		return
	}

	watcher := &MemWatcher{
		startedAt:       time.Now(),
		dumpMultipliers: parseDumpMultipliers(confHeapDumpMultipliers.GetString()),
	}
	go watcher.Run()
}

func (mw *MemWatcher) Run() {
	memLogger.Info("launching memory monitor")
	ticker := time.NewTicker(time.Second * 10)
	for {
		<-ticker.C
		stats := readHeapStats()
		mw.logStats(stats)
		mw.dumpIfHeapGrew(stats, time.Now())
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

	if sysMem.UsedPercent <= MemFreeThreshold {
		return
	}

	mw.lastTimeFreed = time.Now()

	swap, err := mem.SwapMemory()
	if err == nil && swap.UsedPercent > SwapBusyThreshold {
		memLogger.Infof("LOW SYSTEM MEMORY, host is swapping (%.0f%% swap used), skipping forced GC", swap.UsedPercent)
		return
	}

	memLogger.Info("LOW SYSTEM MEMORY, ATTEMPTING TO FREE SOME")
	debug.FreeOSMemory()
}

type heapStats struct {
	live       uint64
	objects    uint64
	total      uint64
	goroutines uint64
	gcCycles   uint64
}

var heapStatsMetricNames = []string{
	"/gc/heap/live:bytes",
	"/memory/classes/heap/objects:bytes",
	"/memory/classes/total:bytes",
	"/sched/goroutines:goroutines",
	"/gc/cycles/total:gc-cycles",
}

func readHeapStats() heapStats {
	samples := make([]metrics.Sample, len(heapStatsMetricNames))
	for i, name := range heapStatsMetricNames {
		samples[i].Name = name
	}
	metrics.Read(samples)

	value := func(i int) uint64 {
		if samples[i].Value.Kind() != metrics.KindUint64 {
			return 0
		}
		return samples[i].Value.Uint64()
	}

	return heapStats{
		live:       value(0),
		objects:    value(1),
		total:      value(2),
		goroutines: value(3),
		gcCycles:   value(4),
	}
}

func (mw *MemWatcher) logStats(stats heapStats) {
	interval := time.Duration(confMemStatsInterval.GetInt()) * time.Minute
	if interval <= 0 || time.Since(mw.lastStatsLogged) < interval {
		return
	}
	mw.lastStatsLogged = time.Now()

	memLogger.WithFields(logrus.Fields{
		"heap_live_gb":     gigabytes(stats.live),
		"heap_objects_gb":  gigabytes(stats.objects),
		"runtime_total_gb": gigabytes(stats.total),
		"goroutines":       stats.goroutines,
		"baseline_live_gb": gigabytes(mw.baselineLive),
	}).Info("heap stats")
}

func (mw *MemWatcher) dumpIfHeapGrew(stats heapStats, now time.Time) {
	warmup := time.Duration(confHeapDumpWarmup.GetInt()) * time.Minute
	if now.Sub(mw.startedAt) < warmup || stats.live == 0 {
		return
	}

	if mw.baselineLive == 0 || stats.live < mw.baselineLive {
		mw.baselineLive = stats.live
	}

	freshReading := stats.gcCycles != mw.lastGCCycles
	mw.lastGCCycles = stats.gcCycles

	if mw.nextDump >= len(mw.dumpMultipliers) || stats.live < mw.dumpThreshold(mw.nextDump) {
		mw.checksOverThreshold = 0
		return
	}

	// Without a GC since the last check this is the same measurement again, possibly of memory that
	// has since become garbage on an idle node, so it says nothing about the heap staying high.
	if !freshReading {
		return
	}

	mw.checksOverThreshold++
	if mw.checksOverThreshold < HeapDumpSustainedChecks {
		return
	}
	mw.checksOverThreshold = 0

	for mw.nextDump < len(mw.dumpMultipliers) && stats.live >= mw.dumpThreshold(mw.nextDump) {
		mw.nextDump++
	}

	writeMemDumps(stats, mw.baselineLive)
}

func (mw *MemWatcher) dumpThreshold(i int) uint64 {
	return uint64(float64(mw.baselineLive) * mw.dumpMultipliers[i])
}

func writeMemDumps(stats heapStats, baselineLive uint64) {
	dir := confHeapDumpDir.GetString()
	if err := os.MkdirAll(dir, 0755); err != nil {
		memLogger.WithError(err).Error("failed creating heap dump directory")
		return
	}

	growth := float64(stats.live) / float64(baselineLive)
	name := fmt.Sprintf("%s_%s_%.1fgb_%.1fx", NodeID(), time.Now().UTC().Format("2006-01-02T15-04-05"), gigabytes(stats.live), growth)
	prefix := filepath.Join(dir, name)

	memLogger.WithFields(logrus.Fields{
		"heap_live_gb":     gigabytes(stats.live),
		"baseline_live_gb": gigabytes(baselineLive),
	}).Infof("live heap grew %.1fx over baseline, writing %s.*", growth, prefix)
	writeProfile(prefix+".heap.pb.gz", "heap", 0)
	writeProfile(prefix+".goroutines.txt", "goroutine", 1)

	pruneOldestFiles(dir, confHeapDumpMaxFiles.GetInt())
}

func writeProfile(path, profile string, debugLevel int) {
	f, err := os.Create(path)
	if err != nil {
		memLogger.WithError(err).Errorf("failed creating %s", path)
		return
	}
	defer f.Close()

	if err := pprof.Lookup(profile).WriteTo(f, debugLevel); err != nil {
		memLogger.WithError(err).Errorf("failed writing %s profile", profile)
	}
}

func pruneOldestFiles(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil || maxFiles <= 0 || len(entries) <= maxFiles {
		return
	}

	type file struct {
		path    string
		modTime time.Time
	}

	files := make([]file, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, file{path: filepath.Join(dir, entry.Name()), modTime: info.ModTime()})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })

	for i := 0; i < len(files)-maxFiles; i++ {
		os.Remove(files[i].path)
	}
}

func parseDumpMultipliers(raw string) []float64 {
	var multipliers []float64
	for part := range strings.SplitSeq(raw, ",") {
		m, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || m <= 1 {
			continue
		}
		multipliers = append(multipliers, m)
	}

	sort.Float64s(multipliers)
	return multipliers
}

func gigabytes(bytes uint64) float64 {
	return float64(bytes) / (1 << 30)
}
