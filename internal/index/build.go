package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// Stats says what a build did.
type Stats struct {
	Repos       int `json:"repos"`
	Files       int `json:"files"`
	Reparsed    int `json:"reparsed_repos"`
	Reused      int `json:"reused_repos"`
	Entities    int `json:"entities"`
	CrossRefs   int `json:"cross_repo_refs"`
	ParseErrors int `json:"parse_errors"`
}

// Build indexes every repository directly under root. A repository whose content hash matches prev is not
// parsed again; its files are taken from prev. Linking always covers the whole fleet, because a change in one
// repository can create or remove edges into every other.
func Build(root string, prev *Index) (*Index, Stats, error) {
	var st Stats
	ix := &Index{Root: root, Repos: map[string]*Repo{}, Files: map[string]*File{}, Entities: map[string]*Entity{}}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, st, err
	}
	type pending struct {
		repo  *Repo
		files map[string][]byte
	}
	var todo []pending
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if !exists(filepath.Join(dir, "go.mod")) && !exists(filepath.Join(dir, "service.yaml")) {
			continue
		}
		files, hash, err := readRepo(root, dir)
		if err != nil {
			return nil, st, err
		}
		if prev != nil {
			if pr := prev.Repos[e.Name()]; pr != nil && pr.Hash == hash {
				ix.Repos[e.Name()] = pr
				for p, f := range prev.Files {
					if f.Repo == e.Name() {
						ix.Files[p] = f
					}
				}
				st.Reused++
				continue
			}
		}
		r := readMeta(e.Name(), files)
		r.Hash = hash
		ix.Repos[r.Name] = r
		todo = append(todo, pending{r, files})
		st.Reparsed++
	}
	// protos first, so Go files can resolve unaliased imports of generated packages
	protoNames := map[string]string{}
	for _, f := range ix.Files {
		if f.Proto != nil {
			protoNames[f.Proto.GoPackage] = f.Proto.GoName
		}
	}
	for _, p := range todo {
		for path, src := range p.files {
			if strings.HasSuffix(path, ".proto") {
				pf := ParseProto(string(src))
				ix.Files[path] = &File{Repo: p.repo.Name, Path: path, Kind: "proto", Package: pf.Package, Proto: pf,
					Lines: strings.Count(string(src), "\n") + 1, Terms: keys(words.Set(string(src)))}
				protoNames[pf.GoPackage] = pf.GoName
			}
		}
	}
	for _, p := range todo {
		for path, src := range p.files {
			switch {
			case strings.HasSuffix(path, ".go"):
				f, err := ParseGo(path, src, protoNames)
				if err != nil {
					st.ParseErrors++
					f = &File{Kind: "go", Lines: strings.Count(string(src), "\n") + 1, Terms: keys(words.Set(string(src)))}
				}
				f.Repo, f.Path = p.repo.Name, path
				ix.Files[path] = f
			case strings.HasSuffix(path, ".proto"):
			default:
				ix.Files[path] = &File{Repo: p.repo.Name, Path: path, Kind: docKind(path), Lines: strings.Count(string(src), "\n") + 1,
					Terms: keys(words.Set(string(src)))}
			}
		}
	}
	st.CrossRefs = link(ix)
	st.Repos, st.Files, st.Entities = len(ix.Repos), len(ix.Files), len(ix.Entities)
	return ix, st, nil
}

// Source returns the text of an indexed file.
func (ix *Index) Source(path string) (string, error) {
	b, err := os.ReadFile(filepath.Join(ix.Root, path))
	return string(b), err
}

// Save writes the index as JSON.
func (ix *Index) Save(path string) error {
	b, err := json.Marshal(ix)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads an index written by Save.
func Load(path string) (*Index, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ix Index
	return &ix, json.Unmarshal(b, &ix)
}

var indexed = map[string]bool{".go": true, ".proto": true, ".md": true, ".yaml": true, ".yml": true, ".mod": true}

func readRepo(root, dir string) (map[string][]byte, string, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); p != dir && (strings.HasPrefix(n, ".") || n == "vendor" || n == "node_modules" || n == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !indexed[filepath.Ext(p)] && d.Name() != "Makefile" && d.Name() != "OWNERS" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	paths := keysB(files)
	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s\x00%d\x00", p, len(files[p]))
		h.Write(files[p])
	}
	return files, hex.EncodeToString(h.Sum(nil))[:16], err
}

var (
	reYAML      = regexp.MustCompile(`(?m)^(\w+):\s*"?([^"\n]*)"?\s*$`)
	reReviewers = regexp.MustCompile(`(?m)^reviewers:\s*\[([^\]]*)\]`)
	reTarget    = regexp.MustCompile(`(?m)^([A-Za-z][\w-]*):`)
	reModule    = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

func readMeta(name string, files map[string][]byte) *Repo {
	r := &Repo{Name: name, Kind: "service"}
	get := func(f string) string { return string(files[name+"/"+f]) }
	for _, m := range reYAML.FindAllStringSubmatch(get("service.yaml"), -1) {
		switch m[1] {
		case "kind":
			r.Kind = m[2]
		case "team":
			r.Team = m[2]
		case "tier":
			r.Tier, _ = strconv.Atoi(m[2])
		case "slack":
			r.Slack = m[2]
		}
	}
	owners := get("OWNERS")
	if m := reYAML.FindStringSubmatch(owners); m != nil && m[1] == "team" && r.Team == "" {
		r.Team = m[2]
	}
	if m := reReviewers.FindStringSubmatch(owners); m != nil {
		for _, p := range strings.Split(m[1], ",") {
			if p = strings.TrimSpace(p); p != "" {
				r.Reviewers = append(r.Reviewers, p)
			}
		}
	}
	if m := reModule.FindStringSubmatch(get("go.mod")); m != nil {
		r.Module = m[1]
	}
	r.Agents = get("AGENTS.md")
	for _, m := range reTarget.FindAllStringSubmatch(get("Makefile"), -1) {
		r.MakeTargets = append(r.MakeTargets, m[1])
	}
	return r
}

func docKind(p string) string {
	if strings.HasSuffix(p, ".md") {
		return "doc"
	}
	return "config"
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysB(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
