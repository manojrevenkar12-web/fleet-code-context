// Package rank decides which entities a ticket is about and which files a change for it will touch.
//
// Two signals are combined. Text: the ticket's terms against each file's identifiers, comments and strings,
// weighted by how rare each term is. Graph: the ticket's terms against contract entities (RPCs, messages,
// events, fields, shared functions), then out along the edges the index found, so a ticket that names an
// event also reaches the service that publishes it and every service that consumes it, in other repositories.
package rank

import (
	"math"
	"sort"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// Settings tune the ranker. The defaults are in config/rank.json; they were set on the dev tickets only.
type Settings struct {
	SeedRatio       float64 `json:"seed_ratio"`        // keep entities scoring at least this share of the best
	MaxSeeds        int     `json:"max_seeds"`         // and at most this many
	TextWeight      float64 `json:"text_weight"`       // weight of the text score against the graph score
	ContentSeedFrom float64 `json:"content_seed_from"` // files this close to the best text match lend their edges
	ContentSeedTop  int     `json:"content_seed_top"`  // from at most this many files
	GeneratedWeight float64 `json:"generated_weight"`  // a .pb.go counts this much of its .proto
	TestWeight      float64 `json:"test_weight"`
	StartRepoBonus  float64 `json:"start_repo_bonus"` // scores in the starting repository are multiplied by 1 + this
	NeighbourBonus  float64 `json:"neighbour_bonus"`  // and in repositories one contract edge away by 1 + this
	ExactNameBoost  float64 `json:"exact_name_boost"` // the ticket spells the identifier itself
	ParentWeight    float64 `json:"parent_weight"`    // words of the parent (an RPC's service, a field's message)
	DelegateFrom    float64 `json:"delegate_from"`    // files this close to the best graph score lend their own-repo calls
	DelegateWeight  float64 `json:"delegate_weight"`  // and pass on this share of their score
	ExtraTieShare   float64 `json:"extra_tie_share"`  // share of a file's ties beyond its strongest that counts
	FanOutFree      float64 `json:"fan_out_free"`     // edges of one kind up to this many count in full
	DocWeight       float64 `json:"doc_weight"`
}

// Defaults are the settings used when config/rank.json is absent.
var Defaults = Settings{SeedRatio: 0.45, MaxSeeds: 8, TextWeight: 0.6, ContentSeedFrom: 0.6, ContentSeedTop: 3,
	GeneratedWeight: 0.4, TestWeight: 0.5, StartRepoBonus: 1.0, NeighbourBonus: 0.5, ExactNameBoost: 3, ParentWeight: 0.5, DocWeight: 0.3,
	DelegateFrom: 0.5, DelegateWeight: 0.4, ExtraTieShare: 0.3, FanOutFree: 3}

// Scored is a scored entity or file.
type Scored struct {
	Key   string  `json:"key"`
	Score float64 `json:"score"`
	Why   string  `json:"why,omitempty"`
}

// Result is a ranking for one ticket.
type Result struct {
	Terms []string `json:"terms"`
	Seeds []Scored `json:"seeds"`
	Files []Scored `json:"files"`
	Repos []Scored `json:"repos"`
}

// Ranker holds the term statistics of one index.
type Ranker struct {
	ix         *index.Index
	set        Settings
	entIDF     map[string]float64
	fileIDF    map[string]float64
	fileTerms  map[string]map[string]bool
	usesByFile map[string][]string  // file -> Go entities (go:...) it uses
	repoRefs   map[string][]repoRef // repo -> entities its files refer to, and how
}

type repoRef struct {
	key, rel string
}

// counterpart is the direction a dependency runs: a repository that subscribes to an event depends on the
// ones that publish it, one that calls an RPC on the ones that implement it, and the reverse.
var counterpart = map[string][]string{
	"subscribes": {"publishes"}, "publishes": {"subscribes"}, "calls": {"implements"}, "implements": {"calls"},
	"uses": {}, "field": {},
}

// Locality returns how close each repository is to the starting one: 0 for itself, 1 for repositories one
// contract edge away, and the repositories that define what it uses (contracts, shared libraries).
func (r *Ranker) Locality(start string) map[string]int {
	out := map[string]int{}
	if r.ix.Repos[start] == nil {
		return out
	}
	out[start] = 0
	for _, rr := range r.repoRefs[start] {
		e := r.ix.Entities[rr.key]
		for _, d := range e.Defs {
			if repo := r.ix.Files[d.File].Repo; repo != start {
				if _, ok := out[repo]; !ok {
					out[repo] = 1
				}
			}
		}
		want := counterpart[rr.rel]
		for _, ref := range e.Refs {
			for _, w := range want {
				if ref.Rel == w {
					if repo := r.ix.Files[ref.File].Repo; repo != start {
						if _, ok := out[repo]; !ok {
							out[repo] = 1
						}
					}
				}
			}
		}
	}
	return out
}

func (r *Ranker) localBonus(loc map[string]int, repo string) float64 {
	d, ok := loc[repo]
	switch {
	case !ok:
		return 1
	case d == 0:
		return 1 + r.set.StartRepoBonus
	default:
		return 1 + r.set.NeighbourBonus
	}
}

// New prepares a ranker for an index.
func New(ix *index.Index, s Settings) *Ranker {
	r := &Ranker{ix: ix, set: s, entIDF: map[string]float64{}, fileIDF: map[string]float64{}, fileTerms: map[string]map[string]bool{},
		usesByFile: map[string][]string{}}
	r.repoRefs = map[string][]repoRef{}
	for k, e := range ix.Entities {
		for _, ref := range e.Refs {
			if strings.HasPrefix(k, "go:") {
				r.usesByFile[ref.File] = append(r.usesByFile[ref.File], k)
			}
			if f := ix.Files[ref.File]; f != nil {
				r.repoRefs[f.Repo] = append(r.repoRefs[f.Repo], repoRef{k, ref.Rel})
			}
		}
	}
	df := map[string]int{}
	for _, e := range ix.Entities {
		for _, w := range e.Words {
			df[w]++
		}
	}
	for w, n := range df {
		r.entIDF[w] = math.Log(1 + float64(len(ix.Entities))/float64(n))
	}
	fdf := map[string]int{}
	nf := 0
	for p, f := range ix.Files {
		if !codeFile(f) {
			continue
		}
		nf++
		set := make(map[string]bool, len(f.Terms))
		for _, t := range f.Terms {
			set[t] = true
			fdf[t]++
		}
		r.fileTerms[p] = set
	}
	for w, n := range fdf {
		r.fileIDF[w] = math.Log(1 + float64(nf)/float64(n))
	}
	return r
}

func codeFile(f *index.File) bool { return f.Kind == "go" || f.Kind == "proto" }

// Rank scores entities, files and repositories for a ticket. startRepo may be empty.
func (r *Ranker) Rank(ticket, startRepo string) Result {
	q := dedupe(words.Split(ticket))
	qset := map[string]bool{}
	for _, t := range q {
		qset[t] = true
	}
	res := Result{Terms: q}

	// text score per file
	text := map[string]float64{}
	maxText := 0.0
	for p, terms := range r.fileTerms {
		s := 0.0
		for _, t := range q {
			if terms[t] {
				s += r.fileIDF[t]
			}
		}
		if s == 0 {
			continue
		}
		s /= 1 + math.Log(1+float64(len(terms))/200)
		text[p] = s
		maxText = math.Max(maxText, s)
	}

	// entity scores
	low := strings.ToLower(ticket)
	loc := r.Locality(startRepo)
	ents := map[string]float64{}
	why := map[string]string{}
	for k, e := range r.ix.Entities {
		s := r.entityScore(e, qset)
		if s == 0 {
			continue
		}
		if identifierLike(e.Name) && (strings.Contains(ticket, e.Name) || strings.Contains(low, strings.ToLower(e.Name))) {
			s *= r.set.ExactNameBoost
			why[k] = "named in the ticket"
		}
		owner := e.Owner
		if len(e.Defs) > 0 {
			if f := r.ix.Files[e.Defs[0].File]; f != nil {
				owner = f.Repo
			}
		}
		s *= math.Max(r.localBonus(loc, owner), r.localBonus(loc, e.Owner))
		ents[k] = s
	}
	// edges of the files that match the ticket's text best: "the push that says arriving in 0 minutes" finds
	// rider_assigned.go by its string, and rider_assigned.go subscribes to RiderAssigned
	tops := topN(text, r.set.ContentSeedTop)
	for _, t := range tops {
		if maxText == 0 || t.Score < r.set.ContentSeedFrom*maxText {
			continue
		}
		for k, e := range r.ix.Entities {
			for _, ref := range e.Refs {
				library := ref.Rel == "uses" && (e.Kind == "package" || e.Kind == "func") && e.Owner != r.ix.Files[t.Key].Repo
				if ref.File == t.Key && (ref.Rel == "publishes" || ref.Rel == "subscribes" || ref.Rel == "implements" || ref.Rel == "calls" || library) {
					v := 0.5 * t.Score / maxText * maxScore(ents)
					if v > ents[k] {
						ents[k] = v
						why[k] = ref.Rel + " in " + t.Key
					}
				}
			}
		}
	}
	best := maxScore(ents)
	for _, s := range topN(ents, len(ents)) {
		if len(res.Seeds) >= r.set.MaxSeeds || s.Score < r.set.SeedRatio*best {
			break
		}
		s.Why = why[s.Key]
		res.Seeds = append(res.Seeds, s)
	}

	// graph score per file, from the seeds: the strongest tie counts in full, further ties add a share, so a
	// central file touched by every seed does not drown a file only one seed reaches
	sum, top := map[string]float64{}, map[string]float64{}
	for _, s := range res.Seeds {
		w := s.Score / best
		e := r.ix.Entities[s.Key]
		one := map[string]float64{}
		r.spread(e, w, one)
		if e.Kind == "field" || e.Kind == "rpc" {
			if p := r.ix.Entities[e.Parent]; p != nil {
				for _, d := range p.Defs {
					one[d.File] += 0.5 * w * r.defWeight(d.File)
				}
			}
		}
		for f, v := range one {
			sum[f] += v
			top[f] = math.Max(top[f], v)
		}
	}
	graph := map[string]float64{}
	for f, v := range sum {
		graph[f] = top[f] + r.set.ExtraTieShare*(v-top[f])
	}
	maxGraph := 0.0
	for _, v := range graph {
		maxGraph = math.Max(maxGraph, v)
	}
	// one hop into the code a strongly tied file delegates to in its own repository: a handler that calls
	// menu.Store.For will need menu.go changed with it
	if maxGraph > 0 {
		add := map[string]float64{}
		for f, v := range graph {
			if v < r.set.DelegateFrom*maxGraph {
				continue
			}
			repo := r.ix.Files[f].Repo
			for _, k := range r.usesByFile[f] {
				e := r.ix.Entities[k]
				if e.Owner != repo {
					continue
				}
				for _, d := range e.Defs {
					if d.File != f {
						add[d.File] = math.Max(add[d.File], r.set.DelegateWeight*v)
					}
				}
			}
		}
		for f, v := range add {
			graph[f] += v
		}
	}
	final := map[string]float64{}
	for p := range r.fileTerms {
		s := 0.0
		if maxGraph > 0 {
			s += graph[p] / maxGraph
		}
		if maxText > 0 {
			s += r.set.TextWeight * text[p] / maxText
		}
		if s == 0 {
			continue
		}
		f := r.ix.Files[p]
		if f.Test {
			s *= r.set.TestWeight
		}
		final[p] = s * r.localBonus(loc, f.Repo)
	}
	res.Files = topN(final, len(final))
	repos := map[string]float64{}
	for _, f := range res.Files {
		repo := r.ix.Files[f.Key].Repo
		repos[repo] = math.Max(repos[repo], f.Score)
	}
	res.Repos = topN(repos, len(repos))
	return res
}

func (r *Ranker) spread(e *index.Entity, w float64, graph map[string]float64) {
	for _, d := range e.Defs {
		graph[d.File] = math.Max(graph[d.File], w*r.defWeight(d.File))
	}
	// an event with thirty consumers says less about any one of them than an event with two: damp each edge by
	// the fan-out of its kind, past a few
	fan := map[string]map[string]bool{}
	for _, ref := range e.Refs {
		if fan[ref.Rel] == nil {
			fan[ref.Rel] = map[string]bool{}
		}
		fan[ref.Rel][ref.File] = true
	}
	seen := map[string]float64{}
	for _, ref := range e.Refs {
		damp := math.Max(1, math.Sqrt(float64(len(fan[ref.Rel]))/r.set.FanOutFree))
		seen[ref.File] = math.Max(seen[ref.File], index.RelWeight[ref.Rel]/damp)
	}
	for f, v := range seen {
		graph[f] = math.Max(graph[f], w*v)
	}
}

func (r *Ranker) defWeight(file string) float64 {
	if strings.HasSuffix(file, ".pb.go") {
		return r.set.GeneratedWeight
	}
	return index.RelWeight["def"]
}

func (r *Ranker) entityScore(e *index.Entity, q map[string]bool) float64 {
	if len(e.Words) == 0 {
		return 0
	}
	s, matched := 0.0, 0
	for _, w := range e.Words {
		if q[w] {
			s += r.entIDF[w]
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	s *= math.Sqrt(float64(matched) / float64(len(e.Words)))
	if p := r.ix.Entities[e.Parent]; p != nil {
		for _, w := range p.Words {
			if q[w] {
				s += r.set.ParentWeight * r.entIDF[w]
			}
		}
	}
	for _, w := range words.Split(e.Doc) {
		if q[w] {
			s += r.set.DocWeight * r.entIDF[w]
		}
	}
	return s
}

// identifierLike is true for names a person would only write when they mean the identifier: GetEta,
// delivery_fee_pence. Plain words such as "location" are matched as words.
func identifierLike(name string) bool {
	if len(name) < 4 || strings.Contains(name, "_") {
		return len(name) >= 4
	}
	for _, r := range name[1:] {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func topN(m map[string]float64, n int) []Scored {
	out := make([]Scored, 0, len(m))
	for k, v := range m {
		out = append(out, Scored{Key: k, Score: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func maxScore(m map[string]float64) float64 {
	best := 0.0
	for _, v := range m {
		best = math.Max(best, v)
	}
	if best == 0 {
		return 1
	}
	return best
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
