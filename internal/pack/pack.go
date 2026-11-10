// Package pack turns a ranking into the context a coding agent receives: who owns what the change touches,
// the instructions that still hold, the ones that no longer do, the contract edges across repositories, and
// the code itself, cut to the declarations that matter and fitted into a token budget.
package pack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// Options shape a pack.
type Options struct {
	Budget       int     // tokens
	MaxRepos     int     // repositories whose cards and instructions are included
	FileFloor    float64 // files scoring below this share of the best are left out
	SnippetLines int     // a file with no matching declaration contributes its first lines
}

// DefaultOptions are used when the caller sets none.
var DefaultOptions = Options{Budget: 4000, MaxRepos: 4, FileFloor: 0.2, SnippetLines: 30}

// Pack is the context for one ticket.
type Pack struct {
	Text      string   `json:"text"`
	Tokens    int      `json:"tokens"`
	Files     []string `json:"files"`    // files whose code is in the pack
	Pointers  []string `json:"pointers"` // files named in the edges, whose code may not be
	Repos     []string `json:"repos"`
	Owners    []string `json:"owners"` // teams that own what the change touches
	Stale     int      `json:"stale_warnings"`
	Truncated bool     `json:"truncated"`
}

type section struct {
	title string
	items []string
}

// Build makes the pack for a ranking.
func Build(ix *index.Index, res rank.Result, claims []drift.Claim, startRepo string, opt Options) Pack {
	if opt.Budget <= 0 {
		opt = DefaultOptions
	}
	p := Pack{}
	repos := pickRepos(ix, res, startRepo, opt.MaxRepos)
	p.Repos = repos

	// 1. owners
	var cards []string
	for _, name := range repos {
		r := ix.Repos[name]
		cards = append(cards, fmt.Sprintf("- %s (%s, tier %d): team %s, reviewers %s, %s", r.Name, r.Kind, r.Tier, r.Team,
			strings.Join(r.Reviewers, ", "), r.Slack))
		p.Owners = append(p.Owners, r.Team)
	}

	// 2. instructions: fleet-wide conventions from libraries, then the involved repositories' own
	byRepo := map[string][]drift.Claim{}
	for _, c := range claims {
		byRepo[c.Repo] = append(byRepo[c.Repo], c)
	}
	var hold, stale []string
	inRepos := map[string]bool{}
	for _, r := range repos {
		inRepos[r] = true
	}
	var libs []string
	for name, r := range ix.Repos {
		if r.Kind == "library" || r.Kind == "contracts" {
			libs = append(libs, name)
		}
	}
	sort.Strings(libs)
	order := append(append([]string{}, repos...), libs...)
	seenRepo := map[string]bool{}
	for _, name := range order {
		if seenRepo[name] {
			continue
		}
		seenRepo[name] = true
		r := ix.Repos[name]
		for _, c := range byRepo[name] {
			fleetWide := r.Kind == "library" && strings.EqualFold(c.Section, "Conventions")
			contract := r.Kind == "contracts" && touchesContracts(ix, res)
			if !inRepos[name] && !fleetWide && !contract {
				continue
			}
			if strings.EqualFold(c.Section, "Layout") {
				continue // the code below shows the layout as it is
			}
			if c.Status == "stale" {
				stale = append(stale, fmt.Sprintf("- %s AGENTS.md:%d says %q. Do not follow it: %s.", name, c.Line, c.Text, strings.Join(c.Reasons, "; ")))
				continue
			}
			scope := name
			if fleetWide {
				scope = "fleet"
			}
			hold = append(hold, fmt.Sprintf("- [%s] %s", scope, c.Text))
		}
	}
	p.Stale = len(stale)

	// 3. edges across repositories, for the seeds
	var edges []string
	pointers := map[string]bool{}
	for _, s := range res.Seeds {
		e := ix.Entities[s.Key]
		if e == nil || e.Kind == "repo" || len(e.Refs) == 0 && len(e.Defs) == 0 {
			continue
		}
		line, ptrs := describe(ix, e)
		edges = append(edges, line)
		for _, f := range ptrs {
			pointers[f] = true
		}
	}

	// 4. code
	head := []section{{"Who owns what this touches", cards}, {"Instructions that are out of date", stale},
		{"Instructions that hold", hold}, {"How the contracts connect", edges}}
	var b strings.Builder
	b.WriteString("# Context for: " + oneLine(strings.Join(res.Terms, " ")) + "\n")
	for _, s := range head {
		if len(s.items) == 0 {
			continue
		}
		chunk := "\n## " + s.title + "\n" + strings.Join(s.items, "\n") + "\n"
		if words.Tokens(b.String()+chunk) > opt.Budget {
			// keep what fits, item by item
			b.WriteString("\n## " + s.title + "\n")
			for _, it := range s.items {
				if words.Tokens(b.String()+it+"\n") > opt.Budget {
					p.Truncated = true
					break
				}
				b.WriteString(it + "\n")
			}
			continue
		}
		b.WriteString(chunk)
	}
	b.WriteString("\n## Code\n")
	best := 0.0
	if len(res.Files) > 0 {
		best = res.Files[0].Score
	}
	for _, f := range res.Files {
		if f.Score < opt.FileFloor*best {
			break
		}
		snip := snippet(ix, f.Key, res, opt.SnippetLines)
		if snip == "" {
			continue
		}
		if words.Tokens(b.String()+snip) > opt.Budget {
			p.Truncated = true
			continue // a smaller file further down may still fit
		}
		b.WriteString(snip)
		p.Files = append(p.Files, f.Key)
	}
	p.Text = b.String()
	// everyone whose code the change touches reviews it, contracts and shared libraries included
	have := map[string]bool{}
	for _, o := range p.Owners {
		have[o] = true
	}
	var extra []string
	for _, f := range p.Files {
		r := ix.Repos[ix.Files[f].Repo]
		if r != nil && r.Team != "" && !have[r.Team] {
			have[r.Team] = true
			p.Owners = append(p.Owners, r.Team)
			extra = append(extra, fmt.Sprintf("- %s (%s): team %s, reviewers %s", r.Name, r.Kind, r.Team, strings.Join(r.Reviewers, ", ")))
		}
	}
	if len(extra) > 0 {
		add := "\n## Also review\n" + strings.Join(extra, "\n") + "\n"
		if words.Tokens(p.Text+add) <= opt.Budget {
			p.Text += add
		}
	}
	p.Tokens = words.Tokens(p.Text)
	for f := range pointers {
		p.Pointers = append(p.Pointers, f)
	}
	sort.Strings(p.Pointers)
	return p
}

func touchesContracts(ix *index.Index, res rank.Result) bool {
	for i, f := range res.Files {
		if i >= 6 {
			break
		}
		if ix.Files[f.Key].Kind == "proto" {
			return true
		}
	}
	return false
}

// pickRepos takes the start repository and the best-scoring others, skipping libraries and contracts (their
// rules arrive as fleet-wide conventions).
func pickRepos(ix *index.Index, res rank.Result, start string, max int) []string {
	var out []string
	seen := map[string]bool{}
	if ix.Repos[start] != nil {
		out = append(out, start)
		seen[start] = true
	}
	for _, r := range res.Repos {
		if len(out) >= max {
			break
		}
		repo := ix.Repos[r.Key]
		if seen[r.Key] || repo == nil || repo.Kind != "service" {
			continue
		}
		if len(res.Repos) > 0 && r.Score < 0.35*res.Repos[0].Score {
			break
		}
		out = append(out, r.Key)
		seen[r.Key] = true
	}
	return out
}

// describe writes one line per entity: where it is defined and who uses it how, grouped by repository.
func describe(ix *index.Index, e *index.Entity) (string, []string) {
	var ptrs []string
	def := ""
	for _, d := range e.Defs {
		if !strings.HasSuffix(d.File, ".pb.go") {
			def = fmt.Sprintf("%s:%d", d.File, d.Span.Start)
			ptrs = append(ptrs, d.File)
			break
		}
	}
	byRel := map[string]map[string]bool{}
	for _, r := range e.Refs {
		if byRel[r.Rel] == nil {
			byRel[r.Rel] = map[string]bool{}
		}
		byRel[r.Rel][r.File] = true
	}
	var parts []string
	for _, rel := range []string{"implements", "publishes", "subscribes", "calls", "uses", "field"} {
		if len(byRel[rel]) == 0 {
			continue
		}
		fs := make([]string, 0, len(byRel[rel]))
		for f := range byRel[rel] {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		label := map[string]string{"implements": "implemented in", "publishes": "published by", "subscribes": "consumed by",
			"calls": "called from", "uses": "used in", "field": "read or set in"}[rel]
		if len(fs) > 5 {
			parts = append(parts, fmt.Sprintf("%s %s and %d more", label, strings.Join(fs[:5], ", "), len(fs)-5))
			ptrs = append(ptrs, fs[:5]...)
		} else {
			parts = append(parts, label+" "+strings.Join(fs, ", "))
			ptrs = append(ptrs, fs...)
		}
	}
	name := e.Name
	if e.Kind == "field" || e.Kind == "rpc" {
		if p := ix.Entities[e.Parent]; p != nil {
			name = p.Name + "." + e.Name
		}
	}
	line := fmt.Sprintf("- %s %s (owner %s), defined at %s", e.Kind, name, e.Owner, def)
	if e.Topic != "" {
		line += ", topic " + e.Topic
	}
	if len(parts) > 0 {
		line += "; " + strings.Join(parts, "; ")
	}
	return line, ptrs
}

// snippet cuts a file down to the declarations the ranking touches; if none match, its first lines.
func snippet(ix *index.Index, path string, res rank.Result, maxLines int) string {
	src, err := ix.Source(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(src, "\n")
	f := ix.Files[path]
	want := map[int]bool{} // lines that reference a seed
	for _, s := range res.Seeds {
		e := ix.Entities[s.Key]
		for _, r := range e.Refs {
			if r.File == path {
				want[r.Line] = true
			}
		}
		for _, d := range e.Defs {
			if d.File == path {
				for l := d.Span.Start; l <= d.Span.End; l++ {
					want[l] = true
				}
			}
		}
	}
	q := map[string]bool{}
	for _, t := range res.Terms {
		q[t] = true
	}
	var spans []index.Span
	if f.Kind == "go" {
		for _, d := range f.Decls {
			hit := false
			for l := d.Span.Start; l <= d.Span.End && !hit; l++ {
				hit = want[l]
			}
			if !hit {
				for _, w := range words.Split(d.Name + " " + d.Doc) {
					if q[w] {
						hit = true
						break
					}
				}
			}
			if hit {
				spans = append(spans, d.Span)
			}
		}
	}
	if f.Kind == "proto" && f.Proto != nil {
		for _, s := range f.Proto.Services {
			if overlaps(want, s.Span) {
				spans = append(spans, s.Span)
			}
		}
		for _, m := range f.Proto.Messages {
			if overlaps(want, m.Span) {
				spans = append(spans, m.Span)
			}
		}
		for _, en := range f.Proto.Enums {
			if overlaps(want, en.Span) {
				spans = append(spans, en.Span)
			}
		}
	}
	if len(spans) == 0 {
		end := maxLines
		if end > len(lines) {
			end = len(lines)
		}
		spans = []index.Span{{Start: 1, End: end}}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
	var b strings.Builder
	fmt.Fprintf(&b, "\n### %s\n```%s\n", path, map[string]string{"go": "go", "proto": "protobuf"}[f.Kind])
	last := 0
	for _, s := range spans {
		if s.Start <= last {
			s.Start = last + 1
		}
		if s.Start > s.End {
			continue
		}
		if last > 0 && s.Start > last+1 {
			b.WriteString("// ...\n")
		}
		for l := s.Start; l <= s.End && l <= len(lines); l++ {
			fmt.Fprintf(&b, "%s\n", lines[l-1])
		}
		last = s.End
	}
	b.WriteString("```\n")
	return b.String()
}

func overlaps(want map[int]bool, s index.Span) bool {
	for l := s.Start; l <= s.End; l++ {
		if want[l] {
			return true
		}
	}
	return false
}

func oneLine(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
