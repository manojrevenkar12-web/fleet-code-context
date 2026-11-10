package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/eval"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/mcp"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/synth"
)

type benchRow struct {
	Repos          int     `json:"repos"`
	Files          int     `json:"files"`
	Entities       int     `json:"entities"`
	CrossRefs      int     `json:"cross_repo_refs"`
	BuildSeconds   float64 `json:"full_index_seconds"`
	RebuildSeconds float64 `json:"one_repo_changed_seconds"`
	HeapMB         float64 `json:"heap_mb"`
	SnapshotMB     float64 `json:"snapshot_mb"`
	PackP50        float64 `json:"context_pack_p50_ms"`
	PackP95        float64 `json:"context_pack_p95_ms"`
	MCPP50         float64 `json:"mcp_call_p50_ms"`
	MCPP95         float64 `json:"mcp_call_p95_ms"`
	DriftSeconds   float64 `json:"check_instructions_seconds"`
}

// BenchSizes are the neighbour counts the benchmark runs (0 is the fixture fleet alone).
var BenchSizes = []int{0, 300, 1000, 2000}

func cmdBench(args []string) error {
	s, o, err := settings()
	if err != nil {
		return err
	}
	var tasks []eval.Task
	for _, split := range []string{"dev", "heldout"} {
		t, err := eval.LoadTasks("eval/tasks/" + split + ".json")
		if err != nil {
			return err
		}
		tasks = append(tasks, t...)
	}
	var rows []benchRow
	for _, n := range BenchSizes {
		dir, err := os.MkdirTemp("", "fcc-bench-")
		if err != nil {
			return err
		}
		if err := synth.CopyTree("fleet", dir); err == nil {
			_, err = synth.Generate(dir, n, NeighbourSeed)
		}
		if err != nil {
			os.RemoveAll(dir)
			return err
		}
		row, err := benchOne(dir, tasks, s, o)
		os.RemoveAll(dir)
		if err != nil {
			return err
		}
		fmt.Printf("%5d repos %6d files: index %.2fs, one repo changed %.2fs, heap %.0f MB, pack p50 %.1f ms p95 %.1f ms, MCP p95 %.1f ms\n",
			row.Repos, row.Files, row.BuildSeconds, row.RebuildSeconds, row.HeapMB, row.PackP50, row.PackP95, row.MCPP95)
		rows = append(rows, row)
	}
	return writeJSON("reports/bench.json", map[string]any{"cpus": runtime.NumCPU(), "go": runtime.Version(), "rows": rows})
}

func benchOne(dir string, tasks []eval.Task, s rank.Settings, o pack.Options) (benchRow, error) {
	runtime.GC()
	t0 := time.Now()
	ix, st, err := index.Build(dir, nil)
	if err != nil {
		return benchRow{}, err
	}
	row := benchRow{Repos: st.Repos, Files: st.Files, Entities: st.Entities, CrossRefs: st.CrossRefs, BuildSeconds: since(t0)}
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	row.HeapMB = round1(float64(ms.HeapAlloc) / (1 << 20))
	snap := filepath.Join(dir, ".index.json")
	if err := ix.Save(snap); err != nil {
		return row, err
	}
	if fi, err := os.Stat(snap); err == nil {
		row.SnapshotMB = round1(float64(fi.Size()) / (1 << 20))
	}
	touched := filepath.Join(dir, "riders/internal/location/location.go")
	b, _ := os.ReadFile(touched)
	if err := os.WriteFile(touched, append(b, []byte("\n// touched\n")...), 0o644); err != nil {
		return row, err
	}
	t1 := time.Now()
	if _, _, err := index.Build(dir, ix); err != nil {
		return row, err
	}
	row.RebuildSeconds = since(t1)
	t2 := time.Now()
	claims := drift.Check(ix)
	row.DriftSeconds = since(t2)
	r := rank.New(ix, s)
	var packs, calls []float64
	srv := mcp.New(ix, s, o, nil)
	for rep := 0; rep < 3; rep++ {
		for _, t := range tasks {
			t3 := time.Now()
			pack.Build(ix, r.Rank(t.Ticket, t.StartRepo), claims, t.StartRepo, o)
			packs = append(packs, ms1(t3))
			args, _ := json.Marshal(map[string]any{"task": t.Ticket, "repo": t.StartRepo})
			line := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"context_pack","arguments":%s}}`, args)
			var out strings.Builder
			t4 := time.Now()
			if err := srv.Serve(strings.NewReader(line), &out); err != nil {
				return row, err
			}
			calls = append(calls, ms1(t4))
		}
	}
	row.PackP50, row.PackP95 = pct(packs, 50), pct(packs, 95)
	row.MCPP50, row.MCPP95 = pct(calls, 50), pct(calls, 95)
	return row, nil
}

func since(t time.Time) float64 { return round2(time.Since(t).Seconds()) }

func ms1(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }

func pct(xs []float64, p int) float64 {
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s) == 0 {
		return 0
	}
	i := (len(s) - 1) * p / 100
	return round1(s[i])
}

func round1(x float64) float64 { return float64(int(x*10+0.5)) / 10 }
func round2(x float64) float64 { return float64(int(x*100+0.5)) / 100 }
