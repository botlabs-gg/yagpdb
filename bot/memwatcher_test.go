package bot

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseDumpMultipliers(t *testing.T) {
	got := parseDumpMultipliers(" 3, 2,bad,,1,0.5,1.5")
	want := []float64{1.5, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got := parseDumpMultipliers(""); len(got) != 0 {
		t.Fatalf("expected no multipliers for empty config, got %v", got)
	}
}

func TestDumpIfHeapGrew(t *testing.T) {
	dir := t.TempDir()
	confHeapDumpDir.LoadedValue = dir
	confHeapDumpMaxFiles.LoadedValue = 20
	confHeapDumpWarmup.LoadedValue = 15

	start := time.Now()
	mw := &MemWatcher{startedAt: start, dumpMultipliers: []float64{2, 3}}
	var gcCycles uint64
	tick := func(at time.Duration, liveGB uint64, times int) {
		for i := 0; i < times; i++ {
			gcCycles++
			mw.dumpIfHeapGrew(heapStats{live: liveGB << 30, gcCycles: gcCycles}, start.Add(at))
		}
	}

	tick(5*time.Minute, 40, 5)
	assertFileCount(t, dir, 0)
	if mw.baselineLive != 0 {
		t.Fatalf("baseline must not be tracked during warmup, got %d", mw.baselineLive)
	}

	// Startup memory that takes hours to come back down must never read as growth.
	for gb := uint64(18); gb >= 9; gb-- {
		tick(time.Duration(30-int(gb))*time.Hour/4, gb, 3)
	}
	assertFileCount(t, dir, 0)
	if mw.baselineLive != 9<<30 {
		t.Fatalf("expected baseline to follow the decline down to 9gb, got %.1fgb", gigabytes(mw.baselineLive))
	}

	tick(6*time.Hour, 19, HeapDumpSustainedChecks-1)
	tick(6*time.Hour, 10, 1)
	tick(6*time.Hour, 19, HeapDumpSustainedChecks-1)
	assertFileCount(t, dir, 0)

	tick(7*time.Hour, 19, HeapDumpSustainedChecks)
	assertFileCount(t, dir, 2)

	tick(7*time.Hour, 25, HeapDumpSustainedChecks*2)
	assertFileCount(t, dir, 2)

	time.Sleep(time.Second)
	tick(8*time.Hour, 80, HeapDumpSustainedChecks)
	assertFileCount(t, dir, 4)

	tick(9*time.Hour, 90, HeapDumpSustainedChecks*2)
	assertFileCount(t, dir, 4)

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".heap.pb.gz") {
			continue
		}

		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		data, err := readGzip(f)
		f.Close()
		if err != nil || len(data) == 0 {
			t.Fatalf("%s is not a non-empty gzipped profile: %v", entry.Name(), err)
		}
	}
}

func TestDumpIfHeapGrewIgnoresReadingsWithoutANewGC(t *testing.T) {
	dir := t.TempDir()
	confHeapDumpDir.LoadedValue = dir
	confHeapDumpMaxFiles.LoadedValue = 20
	confHeapDumpWarmup.LoadedValue = 15

	start := time.Now()
	mw := &MemWatcher{startedAt: start, dumpMultipliers: []float64{2, 3}}
	at := start.Add(time.Hour)

	mw.dumpIfHeapGrew(heapStats{live: 15 << 20, gcCycles: 100}, at)

	// An idle node: a spike was measured by the last GC, then no GC ran while it became garbage.
	for i := 0; i < HeapDumpSustainedChecks*5; i++ {
		mw.dumpIfHeapGrew(heapStats{live: 510 << 20, gcCycles: 101}, at)
	}
	assertFileCount(t, dir, 0)

	// The next GC finds the spike gone, so nothing was sustained.
	mw.dumpIfHeapGrew(heapStats{live: 15 << 20, gcCycles: 102}, at)
	assertFileCount(t, dir, 0)

	// Memory that stays live across fresh GCs still dumps after the sustained checks.
	for i := uint64(0); i < HeapDumpSustainedChecks; i++ {
		mw.dumpIfHeapGrew(heapStats{live: 510 << 20, gcCycles: 103 + i}, at)
	}
	assertFileCount(t, dir, 2)
}

func TestPruneOldestFiles(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for i, name := range []string{"a", "b", "c", "d"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
		modTime := base.Add(time.Duration(i) * time.Minute)
		os.Chtimes(path, modTime, modTime)
	}

	pruneOldestFiles(dir, 2)

	entries, _ := os.ReadDir(dir)
	var remaining []string
	for _, entry := range entries {
		remaining = append(remaining, entry.Name())
	}
	if !reflect.DeepEqual(remaining, []string{"c", "d"}) {
		t.Fatalf("expected the two newest files to remain, got %v", remaining)
	}
}

func assertFileCount(t *testing.T, dir string, want int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != want {
		t.Fatalf("expected %d files in %s, got %d", want, dir, len(entries))
	}
}

func readGzip(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

func TestReadHeapStatsResolvesAllMetrics(t *testing.T) {
	runtime.GC()
	stats := readHeapStats()
	if stats.live == 0 || stats.objects == 0 || stats.total == 0 || stats.goroutines == 0 || stats.gcCycles == 0 {
		t.Fatalf("expected every runtime metric to resolve, got %+v", stats)
	}
}
