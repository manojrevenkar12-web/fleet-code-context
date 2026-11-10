// Package baseline builds the context a coding agent gets without this service, for comparison:
//
//   - AgentsOnly: the AGENTS.md of the repository the agent starts in, which harnesses such as Claude Code load
//     on their own.
//   - Grep: that AGENTS.md, then whole files ranked by how many of the ticket's words they contain, the way an
//     agent greps and reads. Scope "repo" searches only the starting repository (the checkout the agent has);
//     scope "fleet" searches every repository, as if the whole fleet were checked out.
package baseline

import (
	"math"
	"sort"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// Context is what a baseline hands the agent.
type Context struct {
	Text   string   `json:"text"`
	Tokens int      `json:"tokens"`
	Files  []string `json:"files"`
}

// AgentsOnly returns the starting repository's AGENTS.md.
func AgentsOnly(ix *index.Index, startRepo string) Context {
	r := ix.Repos[startRepo]
	if r == nil {
		return Context{}
	}
	t := "# " + startRepo + "/AGENTS.md\n" + r.Agents
	return Context{Text: t, Tokens: words.Tokens(t)}
}

// Grep greps the ticket's words (case-insensitive, whole source text) and reads the best files whole until
// the budget is spent. Terms are weighted by rarity, which is kinder to grep than an agent's first guesses.
func Grep(ix *index.Index, ticket, startRepo, scope string, budget int) Context {
	c := AgentsOnly(ix, startRepo)
	terms := dedupe(words.Split(ticket))
	type cand struct {
		path  string
		score float64
		src   string
	}
	var cands []cand
	df := map[string]int{}
	srcs := map[string]string{}
	for p, f := range ix.Files {
		if f.Kind != "go" && f.Kind != "proto" {
			continue
		}
		if scope == "repo" && f.Repo != startRepo {
			continue
		}
		src, err := ix.Source(p)
		if err != nil {
			continue
		}
		low := strings.ToLower(src)
		srcs[p] = low
		for _, t := range terms {
			if strings.Contains(low, t) {
				df[t]++
			}
		}
	}
	n := float64(len(srcs))
	for p, low := range srcs {
		s := 0.0
		for _, t := range terms {
			if strings.Contains(low, t) {
				s += math.Log(1 + n/float64(df[t]))
			}
		}
		if s > 0 {
			cands = append(cands, cand{p, s, low})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].path < cands[j].path
	})
	var b strings.Builder
	b.WriteString(c.Text)
	for _, cd := range cands {
		src, _ := ix.Source(cd.path)
		chunk := "\n### " + cd.path + "\n```\n" + src + "```\n"
		if words.Tokens(b.String()+chunk) > budget {
			continue
		}
		b.WriteString(chunk)
		c.Files = append(c.Files, cd.path)
	}
	c.Text = b.String()
	c.Tokens = words.Tokens(c.Text)
	return c
}

func dedupe(ws []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range ws {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}
