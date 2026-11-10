// Package perf holds the sandbox-only profiling switch for the TUI.
// It is wired up by internal/cli (`mtm tui --profile FILE`) and by the
// MTM_PPROF environment variable (`MTM_PPROF=cpu:/path/to.pprof`).
//
// Profiling is a measurement mode, never a feature: Setup refuses to run
// unless the process is inside the throwaway sandbox (MTM_SANDBOX=1 and
// every mtm root under a temp dir — see internal/paths). When admission
// fails the TUI must not start; the caller returns the error instead.
//
// On stop it writes the CPU profile (if a cpu target was given) and a
// stats JSON next to it (<file>.stats.json) with the View() counters and
// runtime alloc totals, which contrib/perf.py consumes.
package perf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/loharrahulofficial-crypto/mtm/internal/paths"
)

// Counters bumped by the TUI render path. Cheap enough to leave in always;
// reads are atomics.
var (
	viewCount int64
	viewBytes int64
)

// CountView records one View() rebuild of n bytes. Called from the TUI
// hot path; a no-op cost of two atomic adds when disabled.
func CountView(n int) {
	atomic.AddInt64(&viewCount, 1)
	atomic.AddInt64(&viewBytes, int64(n))
}

// Views / ViewBytes report the counters (used by tests and the stop path).
func Views() int64     { return atomic.LoadInt64(&viewCount) }
func ViewBytes() int64 { return atomic.LoadInt64(&viewBytes) }

// ResetForTest zeroes the counters.
func ResetForTest() {
	atomic.StoreInt64(&viewCount, 0)
	atomic.StoreInt64(&viewBytes, 0)
}

// Stats is the JSON document written next to the CPU profile on stop.
type Stats struct {
	Views           int64  `json:"views"`
	ViewBytes       int64  `json:"view_bytes"`
	WallNanos       int64  `json:"wall_ns"`
	Mallocs         uint64 `json:"mallocs"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	NumGC           uint32 `json:"num_gc"`
	CPUProfile      string `json:"cpu_profile,omitempty"`
	ProfileKinds    string `json:"profile_kinds"`
}

// Setup parses the profiling request, admits the sandbox and starts what
// was asked for. profilePath is the --profile flag value ("" when unset);
// MTM_PPROF carries "kind:path" pairs separated by commas, e.g.
// MTM_PPROF=cpu:/tmp/mtm-perf/cpu.pprof — a bare path means cpu.
//
// It returns a stop func that must be called before the process exits.
// stop writes <file>.stats.json next to every file it wrote.
func Setup(profilePath string) (stop func(), err error) {
	spec := strings.TrimSpace(os.Getenv("MTM_PPROF"))
	if spec == "" && profilePath != "" {
		spec = "cpu:" + profilePath
	}
	if spec == "" {
		return func() {}, nil
	}
	kinds := map[string]string{} // kind → path
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kind, path, ok := strings.Cut(part, ":")
		if !ok {
			kind, path = "cpu", part
		}
		kind = strings.ToLower(strings.TrimSpace(kind))
		path = strings.TrimSpace(path)
		switch kind {
		case "cpu":
		default:
			return nil, fmt.Errorf("MTM_PPROF: unknown profile kind %q (want cpu:/path)", kind)
		}
		if path == "" {
			return nil, fmt.Errorf("MTM_PPROF: %s: missing path", part)
		}
		kinds[kind] = path
	}
	if len(kinds) == 0 {
		return func() {}, nil
	}
	// Refuse to profile outside the sandbox, and refuse to let the profile
	// itself land outside a temp dir — the file may contain heap contents.
	if err := paths.AdmitSandbox(); err != nil {
		return nil, fmt.Errorf("refusing to profile: %v", err)
	}
	for _, path := range kinds {
		if err := paths.AdmitSandbox(path); err != nil {
			return nil, fmt.Errorf("refusing to profile: %v", err)
		}
	}
	start := time.Now()
	var cpuFile *os.File
	if path, ok := kinds["cpu"]; ok {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return nil, err
		}
		cpuFile = f
	}
	stop = func() {
		if cpuFile != nil {
			pprof.StopCPUProfile()
			_ = cpuFile.Close()
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		stats := Stats{
			Views:           Views(),
			ViewBytes:       ViewBytes(),
			WallNanos:       time.Since(start).Nanoseconds(),
			Mallocs:         ms.Mallocs,
			TotalAllocBytes: ms.TotalAlloc,
			NumGC:           ms.NumGC,
			ProfileKinds:    strings.Join(sortedKinds(kinds), ","),
			CPUProfile:      kinds["cpu"],
		}
		for _, path := range kinds {
			_ = writeStats(path, stats)
		}
	}
	return stop, nil
}

func sortedKinds(kinds map[string]string) []string {
	out := make([]string, 0, len(kinds))
	for k := range kinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func writeStats(profilePath string, s Stats) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profilePath+".stats.json", data, 0o644)
}
