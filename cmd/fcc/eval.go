package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/eval"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/synth"
)

// Neighbours is how many generated repositories surround the fixture fleet in the crowded evaluation.
const Neighbours = 300

// NeighbourSeed fixes the generated fleet.
const NeighbourSeed = 1

type fleetResult struct {
	Name      string          `json:"name"`
	Repos     int             `json:"repos"`
	Files     int             `json:"files"`
	Entities  int             `json:"entities"`
	CrossRefs int             `json:"cross_repo_refs"`
	Summary   []eval.Summary  `json:"summary"`
	Outcomes  []eval.Outcome  `json:"outcomes"`
	Drift     eval.DriftScore `json:"drift"`
}

type results struct {
	Generated  string        `json:"generated"`
	Neighbours int           `json:"neighbours"`
	Dev        []fleetResult `json:"dev"`
	Heldout    []fleetResult `json:"heldout,omitempty"`
}

func evalFleet(name, dir string, split string, tasks []eval.Task, truth eval.Truth) (fleetResult, error) {
	ix, st, err := index.Build(dir, nil)
	if err != nil {
		return fleetResult{}, err
	}
	s, o, err := settings()
	if err != nil {
		return fleetResult{}, err
	}
	claims := drift.Check(ix)
	outs := eval.Run(ix, rank.New(ix, s), claims, tasks, truth, o)
	return fleetResult{Name: name, Repos: st.Repos, Files: st.Files, Entities: st.Entities, CrossRefs: st.CrossRefs,
		Summary: eval.Summarize(outs, tasks), Outcomes: outs, Drift: eval.ScoreDrift(claims, truth)}, nil
}

func crowded() (string, func(), error) {
	dir, err := os.MkdirTemp("", "fcc-fleet-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	if err := synth.CopyTree("fleet", dir); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := synth.Generate(dir, Neighbours, NeighbourSeed); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

func cmdEval(args []string) error {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	heldout := fs.Bool("heldout", false, "also run the locked held-out tickets (their first run is kept, never rewritten)")
	fs.Parse(args)
	truth, err := eval.LoadTruth("eval/truth")
	if err != nil {
		return err
	}
	dir, cleanup, err := crowded()
	if err != nil {
		return err
	}
	defer cleanup()
	res := results{Generated: time.Now().UTC().Format("2006-01-02"), Neighbours: Neighbours}
	run := func(split string) ([]fleetResult, error) {
		tasks, err := eval.LoadTasks("eval/tasks/" + split + ".json")
		if err != nil {
			return nil, err
		}
		a, err := evalFleet("fixture", "fleet", split, tasks, truth)
		if err != nil {
			return nil, err
		}
		b, err := evalFleet("crowded", dir, split, tasks, truth)
		return []fleetResult{a, b}, err
	}
	if res.Dev, err = run("dev"); err != nil {
		return err
	}
	if *heldout {
		if err := eval.CheckLock("eval/tasks/heldout.json", "config/heldout.lock"); err != nil {
			return err
		}
		first := "reports/heldout/first_run.json"
		if _, err := os.Stat(first); err == nil {
			fmt.Fprintln(os.Stderr, "fcc: the held-out first run exists and is not rewritten; reading it")
			b, err := os.ReadFile(first)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &res.Heldout); err != nil {
				return err
			}
		} else {
			if res.Heldout, err = run("heldout"); err != nil {
				return err
			}
			if err := writeJSON(first, res.Heldout); err != nil {
				return err
			}
		}
	} else if b, err := os.ReadFile("reports/heldout/first_run.json"); err == nil {
		_ = json.Unmarshal(b, &res.Heldout)
	}
	for _, f := range res.Dev {
		for _, s := range f.Summary {
			if s.Budget == pack.DefaultOptions.Budget {
				fmt.Printf("dev %-8s %-19s file recall %.3f  every file %2d/%d  conventions %.2f  owners %.2f  stale as fact %d  tokens %.0f\n",
					f.Name, s.Source, s.FileRecall, s.AllFiles, s.Tasks, s.ConvRecall, s.OwnerRecall, s.StaleAsFact, s.MeanTokens)
			}
		}
	}
	return writeJSON("reports/results.json", res)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
