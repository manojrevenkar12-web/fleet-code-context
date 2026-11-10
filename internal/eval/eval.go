// Package eval scores context sources on tickets whose correct change is known: does the context contain every
// file the change must touch or read, the team that must review it, and the conventions it must follow, within
// a token budget, and does it pass on stale instructions as if they were true?
//
// This is the only package that reads eval/truth. The server, the ranker and the packer never do.
package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/baseline"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
)

// Task is one ticket with the change it needs.
type Task struct {
	ID            string   `json:"id"`
	StartRepo     string   `json:"start_repo"`
	Ticket        string   `json:"ticket"`
	RequiredFiles []string `json:"required_files"`
	Conventions   []string `json:"conventions"`
	Owners        []string `json:"owners"`
}

// Outcome is one context source on one ticket at one budget.
type Outcome struct {
	Task        string   `json:"task"`
	Source      string   `json:"source"`
	Budget      int      `json:"budget"`
	Tokens      int      `json:"tokens"`
	FilesFound  int      `json:"files_found"`
	FilesNeeded int      `json:"files_needed"`
	Missing     []string `json:"missing,omitempty"`
	OwnersFound int      `json:"owners_found"`
	OwnersNeed  int      `json:"owners_needed"`
	ConvFound   int      `json:"conventions_found"`
	ConvNeed    int      `json:"conventions_needed"`
	StaleAsFact int      `json:"stale_as_fact"`
	Millis      float64  `json:"ms"`
}

// Summary aggregates outcomes for one source and budget on one split.
type Summary struct {
	Source        string  `json:"source"`
	Budget        int     `json:"budget"`
	Tasks         int     `json:"tasks"`
	FileRecall    float64 `json:"file_recall"`
	AllFiles      int     `json:"tasks_with_every_file"`
	OwnerRecall   float64 `json:"owner_recall"`
	ConvRecall    float64 `json:"convention_recall"`
	StaleAsFact   int     `json:"stale_as_fact"`
	MeanTokens    float64 `json:"mean_tokens"`
	CrossRepoNeed int     `json:"cross_repo_files_needed"`
	CrossRepoGot  int     `json:"cross_repo_files_found"`
}

// Sources are the context sources compared.
var Sources = []string{"agents-md", "grep-repo", "grep-fleet", "fleet-code-context"}

// Budgets are the token budgets each source is held to.
var Budgets = []int{2000, 4000, 8000}

// LoadTasks reads a split.
func LoadTasks(path string) ([]Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Tasks []Task `json:"tasks"`
	}
	return doc.Tasks, json.Unmarshal(b, &doc)
}

// Truth is the evaluation's own knowledge of the fleet.
type Truth struct {
	Conventions map[string]string `json:"conventions"`
	Stale       []struct {
		Repo     string `json:"repo"`
		Contains string `json:"contains"`
		Kind     string `json:"kind"`
	} `json:"stale"`
}

// LoadTruth reads eval/truth.
func LoadTruth(dir string) (Truth, error) {
	var t Truth
	for _, f := range []string{"conventions.json", "drift.json"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return t, err
		}
		if err := json.Unmarshal(b, &t); err != nil {
			return t, err
		}
	}
	return t, nil
}

// Run scores every source at every budget on a split.
func Run(ix *index.Index, r *rank.Ranker, claims []drift.Claim, tasks []Task, truth Truth, opt pack.Options) []Outcome {
	var out []Outcome
	for _, t := range tasks {
		for _, budget := range Budgets {
			for _, src := range Sources {
				start := time.Now()
				var text string
				var files []string
				var tokens int
				var owners []string
				switch src {
				case "agents-md":
					c := baseline.AgentsOnly(ix, t.StartRepo)
					text, tokens = c.Text, c.Tokens
				case "grep-repo", "grep-fleet":
					c := baseline.Grep(ix, t.Ticket, t.StartRepo, strings.TrimPrefix(src, "grep-"), budget)
					text, files, tokens = c.Text, c.Files, c.Tokens
				case "fleet-code-context":
					o := opt
					o.Budget = budget
					p := pack.Build(ix, r.Rank(t.Ticket, t.StartRepo), claims, t.StartRepo, o)
					text, files, tokens, owners = p.Text, p.Files, p.Tokens, p.Owners
				}
				ms := float64(time.Since(start).Microseconds()) / 1000
				out = append(out, score(t, src, budget, text, files, owners, tokens, truth, ms))
			}
		}
	}
	return out
}

var reTeam = regexp.MustCompile(`team:?\s+([\w-]+)`)

func score(t Task, src string, budget int, text string, files, owners []string, tokens int, truth Truth, ms float64) Outcome {
	o := Outcome{Task: t.ID, Source: src, Budget: budget, Tokens: tokens, FilesNeeded: len(t.RequiredFiles),
		OwnersNeed: len(t.Owners), ConvNeed: len(t.Conventions), Millis: ms}
	got := map[string]bool{}
	for _, f := range files {
		got[f] = true
	}
	for _, f := range t.RequiredFiles {
		if got[f] {
			o.FilesFound++
		} else {
			o.Missing = append(o.Missing, f)
		}
	}
	named := map[string]bool{}
	for _, w := range owners {
		named[w] = true
	}
	for _, m := range reTeam.FindAllStringSubmatch(text, -1) {
		named[m[1]] = true
	}
	for _, w := range t.Owners {
		if named[w] {
			o.OwnersFound++
		}
	}
	// what the agent is told to do: everything except the section that marks instructions as out of date
	told := text
	if i := strings.Index(told, "## Instructions that are out of date"); i >= 0 {
		rest := told[i+3:]
		j := strings.Index(rest, "\n## ")
		if j < 0 {
			told = told[:i]
		} else {
			told = told[:i] + rest[j+1:]
		}
	}
	for _, c := range t.Conventions {
		if s := truth.Conventions[c]; s != "" && strings.Contains(told, s) {
			o.ConvFound++
		}
	}
	for _, s := range truth.Stale {
		if strings.Contains(told, s.Contains) {
			o.StaleAsFact++
		}
	}
	return o
}

// Summarize aggregates outcomes per source and budget.
func Summarize(outs []Outcome, tasks []Task) []Summary {
	byTask := map[string]Task{}
	for _, t := range tasks {
		byTask[t.ID] = t
	}
	type key struct {
		src    string
		budget int
	}
	agg := map[key]*Summary{}
	var keys []key
	for _, o := range outs {
		k := key{o.Source, o.Budget}
		s := agg[k]
		if s == nil {
			s = &Summary{Source: o.Source, Budget: o.Budget}
			agg[k] = s
			keys = append(keys, k)
		}
		s.Tasks++
		s.FileRecall += frac(o.FilesFound, o.FilesNeeded)
		if o.FilesFound == o.FilesNeeded {
			s.AllFiles++
		}
		s.OwnerRecall += frac(o.OwnersFound, o.OwnersNeed)
		s.ConvRecall += frac(o.ConvFound, o.ConvNeed)
		s.StaleAsFact += o.StaleAsFact
		s.MeanTokens += float64(o.Tokens)
		t := byTask[o.Task]
		missing := map[string]bool{}
		for _, m := range o.Missing {
			missing[m] = true
		}
		for _, f := range t.RequiredFiles {
			if !strings.HasPrefix(f, t.StartRepo+"/") {
				s.CrossRepoNeed++
				if !missing[f] {
					s.CrossRepoGot++
				}
			}
		}
	}
	var out []Summary
	for _, k := range keys {
		s := agg[k]
		n := float64(s.Tasks)
		s.FileRecall, s.OwnerRecall, s.ConvRecall, s.MeanTokens = round(s.FileRecall/n), round(s.OwnerRecall/n), round(s.ConvRecall/n), round(s.MeanTokens/n)
		out = append(out, *s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Budget != out[j].Budget {
			return out[i].Budget < out[j].Budget
		}
		return indexOf(Sources, out[i].Source) < indexOf(Sources, out[j].Source)
	})
	return out
}

// DriftScore compares flagged claims with the planted stale instructions.
type DriftScore struct {
	Claims    int      `json:"claims"`
	Checked   int      `json:"checked"`
	Flagged   int      `json:"flagged"`
	Planted   int      `json:"planted"`
	Caught    int      `json:"caught"`
	FalseFlag []string `json:"false_flags,omitempty"`
	Missed    []string `json:"missed,omitempty"`
}

// ScoreDrift scores the drift checker against the truth.
func ScoreDrift(claims []drift.Claim, truth Truth) DriftScore {
	d := DriftScore{Claims: len(claims), Planted: len(truth.Stale)}
	matched := map[int]bool{}
	for _, c := range claims {
		if c.Status != "unchecked" {
			d.Checked++
		}
		if c.Status != "stale" {
			continue
		}
		d.Flagged++
		hit := false
		for i, s := range truth.Stale {
			if s.Repo == c.Repo && strings.Contains(c.Text, s.Contains) {
				hit, matched[i] = true, true
			}
		}
		if !hit {
			d.FalseFlag = append(d.FalseFlag, c.Repo+": "+c.Text)
		}
	}
	for i, s := range truth.Stale {
		if matched[i] {
			d.Caught++
		} else {
			d.Missed = append(d.Missed, s.Repo+": "+s.Contains)
		}
	}
	return d
}

// CheckLock verifies the held-out split is the one that was locked.
func CheckLock(tasksPath, lockPath string) error {
	b, err := os.ReadFile(tasksPath)
	if err != nil {
		return err
	}
	lock, err := os.ReadFile(lockPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	for _, line := range strings.Split(string(lock), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			if line != got {
				return fmt.Errorf("held-out tickets changed since they were locked: %s, lock says %s", got, line)
			}
			return nil
		}
	}
	return fmt.Errorf("no hash in %s", lockPath)
}

func frac(a, b int) float64 {
	if b == 0 {
		return 1
	}
	return float64(a) / float64(b)
}

func round(x float64) float64 { return float64(int(x*1000+0.5)) / 1000 }

func indexOf(xs []string, x string) int {
	for i, y := range xs {
		if y == x {
			return i
		}
	}
	return len(xs)
}
