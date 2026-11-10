// Package drift checks what a repository's AGENTS.md tells coding agents against the fleet as it is now.
//
// Instruction files rot: a directory is renamed, a make target is dropped, a client package is replaced, a
// field changes unit. An agent cannot tell a stale sentence from a true one, so it follows both. Each bullet
// is a claim; every reference in it that can be checked (a path, a command, a Go symbol, an import path, a
// unit) is checked against the index. A claim is stale if any of its references is wrong.
package drift

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
)

// Claim is one bullet of an AGENTS.md.
type Claim struct {
	Repo    string   `json:"repo"`
	Line    int      `json:"line"`
	Section string   `json:"section"`
	Text    string   `json:"text"`
	Status  string   `json:"status"` // ok, stale, unchecked
	Checks  int      `json:"checks"` // references checked
	Reasons []string `json:"reasons,omitempty"`
}

var (
	reTick    = regexp.MustCompile("`([^`]+)`")
	reSymbol  = regexp.MustCompile(`^([a-z]\w*)\.([A-Z]\w*)(?:\.(\w+))?$`)
	reMake    = regexp.MustCompile(`^make\s+([\w-]+)$`)
	reGoCmd   = regexp.MustCompile(`^go\s+(?:test|run|build|vet)\s+(\./[\w./-]*)`)
	quantity  = []string{"eta", "total", "price", "fee", "amount", "distance"}
	unitWords = map[string]string{"minute": "minutes", "minutes": "minutes", "second": "seconds", "seconds": "seconds",
		"pound": "pounds", "pounds": "pounds", "pence": "pence", "penny": "pence", "metres": "metres", "meters": "metres",
		"km": "km", "kilometres": "km"}
)

// Check returns every claim of every repository's AGENTS.md, checked.
func Check(ix *index.Index) []Claim {
	var out []Claim
	names := make([]string, 0, len(ix.Repos))
	for n := range ix.Repos {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, CheckRepo(ix, n)...)
	}
	return out
}

// CheckRepo checks one repository's AGENTS.md.
func CheckRepo(ix *index.Index, repo string) []Claim {
	r := ix.Repos[repo]
	if r == nil || r.Agents == "" {
		return nil
	}
	var out []Claim
	section := ""
	for i, raw := range strings.Split(r.Agents, "\n") {
		t := strings.TrimSpace(raw)
		if strings.HasPrefix(t, "#") {
			section = strings.TrimSpace(strings.TrimLeft(t, "#"))
			continue
		}
		if !strings.HasPrefix(t, "- ") {
			continue
		}
		c := Claim{Repo: repo, Line: i + 1, Section: section, Text: strings.TrimPrefix(t, "- ")}
		checkClaim(ix, r, &c)
		switch {
		case len(c.Reasons) > 0:
			c.Status = "stale"
		case c.Checks > 0:
			c.Status = "ok"
		default:
			c.Status = "unchecked"
		}
		out = append(out, c)
	}
	return out
}

func checkClaim(ix *index.Index, r *index.Repo, c *Claim) {
	for _, m := range reTick.FindAllStringSubmatch(c.Text, -1) {
		ref := strings.TrimSpace(m[1])
		switch {
		case reMake.MatchString(ref):
			c.Checks++
			target := reMake.FindStringSubmatch(ref)[1]
			if !has(r.MakeTargets, target) {
				c.Reasons = append(c.Reasons, fmt.Sprintf("`%s`: the Makefile has no %q target (it has %s)", ref, target, strings.Join(r.MakeTargets, ", ")))
			}
		case reGoCmd.MatchString(ref):
			c.Checks++
			p := strings.TrimSuffix(strings.TrimSuffix(reGoCmd.FindStringSubmatch(ref)[1], "..."), "/")
			if !existsIn(ix, r.Name, p) {
				c.Reasons = append(c.Reasons, fmt.Sprintf("`%s`: %s does not exist in %s", ref, p, r.Name))
			}
		case strings.HasPrefix(ref, "fleet.example/") || (r.Module != "" && strings.HasPrefix(ref, strings.SplitN(r.Module, "/", 2)[0]+"/")):
			c.Checks++
			if !importExists(ix, ref) {
				c.Reasons = append(c.Reasons, fmt.Sprintf("`%s`: no such package in the fleet", ref))
			}
		case reSymbol.MatchString(ref) && stdlib[reSymbol.FindStringSubmatch(ref)[1]]:
			// standard library: nothing in the fleet to check it against
		case reSymbol.MatchString(ref):
			c.Checks++
			if why := symbolProblem(ix, ref); why != "" {
				c.Reasons = append(c.Reasons, why)
			}
		case looksLikePath(ref):
			c.Checks++
			if !existsIn(ix, r.Name, strings.TrimSuffix(ref, "/")) {
				c.Reasons = append(c.Reasons, fmt.Sprintf("`%s`: no such file or directory in %s", ref, r.Name))
			}
		}
	}
	if why := unitProblem(ix, c.Text); why != "" {
		c.Checks++
		c.Reasons = append(c.Reasons, why)
	} else if mentionsUnit(c.Text) {
		c.Checks++
	}
}

func looksLikePath(s string) bool {
	if strings.ContainsAny(s, " ()<>") {
		return false
	}
	if strings.HasPrefix(s, ".") && !strings.Contains(s, "/") {
		return false // a file extension such as `.proto`, not a path
	}
	return strings.Contains(s, "/") || filepath.Ext(s) != ""
}

// existsIn checks a path relative to a repository, on disk (the index skips files it does not read).
func existsIn(ix *index.Index, repo, rel string) bool {
	rel = strings.TrimPrefix(rel, "./")
	_, err := os.Stat(filepath.Join(ix.Root, repo, rel))
	return err == nil
}

func importExists(ix *index.Index, imp string) bool {
	for _, r := range ix.Repos {
		if r.Module == "" || !strings.HasPrefix(imp+"/", r.Module+"/") {
			continue
		}
		return existsIn(ix, r.Name, strings.TrimPrefix(strings.TrimPrefix(imp, r.Module), "/"))
	}
	return false
}

// symbolProblem resolves pkg.Name or pkg.Type.Method against proto packages and shared Go packages.
func symbolProblem(ix *index.Index, ref string) string {
	m := reSymbol.FindStringSubmatch(ref)
	pkg, name, member := m[1], m[2], m[3]
	for _, f := range ix.Files {
		if f.Proto != nil && f.Proto.GoName == pkg {
			pf := f.Proto
			for _, s := range pf.Services {
				if name == s.Name+"Client" || name == s.Name+"Server" || name == s.Name {
					if member == "" {
						return ""
					}
					for _, r := range s.RPCs {
						if r.Name == member {
							return ""
						}
					}
					return fmt.Sprintf("`%s`: %s has no RPC %s", ref, s.Name, member)
				}
			}
			for _, msg := range pf.Messages {
				if msg.Name == name {
					return ""
				}
			}
			for _, en := range pf.Enums {
				if en.Name == name {
					return ""
				}
			}
			return fmt.Sprintf("`%s`: %s declares no %s", ref, pf.Package, name)
		}
	}
	found := false
	for k := range ix.Entities {
		if strings.HasPrefix(k, "go:") && strings.HasSuffix(k, "/"+pkg+"."+name) {
			return ""
		}
		if strings.HasPrefix(k, "go:") && strings.Contains(k, "/"+pkg+".") {
			found = true
		}
	}
	if found {
		return fmt.Sprintf("`%s`: package %s has no exported %s", ref, pkg, name)
	}
	return fmt.Sprintf("`%s`: there is no package %s in the fleet", ref, pkg)
}

func mentionsUnit(text string) bool {
	q, u := quantityAndUnit(text)
	return q != "" && u != ""
}

func quantityAndUnit(text string) (string, string) {
	low := strings.ToLower(text)
	fields := strings.FieldsFunc(low, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') })
	var q, u string
	for _, w := range fields {
		for _, qq := range quantity {
			if w == qq || w == qq+"s" {
				q = qq
			}
		}
		if unit, ok := unitWords[w]; ok && u == "" {
			u = unit
		}
	}
	return q, u
}

// unitProblem catches "ETAs are in minutes" when every contract field for ETAs is eta_seconds.
func unitProblem(ix *index.Index, text string) string {
	q, u := quantityAndUnit(text)
	if q == "" || u == "" {
		return ""
	}
	units := map[string][]string{}
	for _, e := range ix.Entities {
		if e.Kind != "field" {
			continue
		}
		parts := strings.Split(e.Name, "_")
		if len(parts) < 2 || !has(parts[:len(parts)-1], q) {
			continue
		}
		if fu, ok := unitWords[parts[len(parts)-1]]; ok {
			units[fu] = append(units[fu], e.Name)
		}
	}
	if len(units) == 0 || len(units[u]) > 0 {
		return ""
	}
	var seen []string
	for fu, fs := range units {
		sort.Strings(fs)
		seen = append(seen, fmt.Sprintf("%s (%s)", fu, fs[0]))
	}
	sort.Strings(seen)
	return fmt.Sprintf("says %s is in %s, but the contracts carry it in %s", q, u, strings.Join(seen, ", "))
}

func has(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
