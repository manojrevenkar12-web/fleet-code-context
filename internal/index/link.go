package index

import (
	"path"
	"sort"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// link turns the parsed files into entities and the references between them. It returns the number of
// references that cross a repository boundary, the edges grep cannot see from inside one repository.
func link(ix *Index) int {
	ent := ix.Entities
	add := func(e *Entity) *Entity {
		if old, ok := ent[e.Key]; ok {
			return old
		}
		ent[e.Key] = e
		return e
	}
	type protoPkg struct {
		file    *File
		rpcs    map[string][]*Entity // rpc name -> rpc entities
		types   map[string]*Entity   // Go type name -> message or enum
		fields  map[string][]*Entity // Go field name -> fields
		topics  map[string]*Entity   // TopicX -> event
		reqType map[string]bool      // request message names
	}
	protos := map[string]*protoPkg{} // Go import path -> package

	paths := sortedFiles(ix)
	for _, p := range paths {
		f := ix.Files[p]
		if f.Proto == nil {
			continue
		}
		pf := f.Proto
		pp := protos[pf.GoPackage]
		if pp == nil {
			pp = &protoPkg{file: f, rpcs: map[string][]*Entity{}, types: map[string]*Entity{}, fields: map[string][]*Entity{},
				topics: map[string]*Entity{}, reqType: map[string]bool{}}
			protos[pf.GoPackage] = pp
		}
		isEvents := strings.HasPrefix(pf.Package, "events.")
		for _, s := range pf.Services {
			sk := "service:" + pf.Package + "." + s.Name
			add(&Entity{Key: sk, Kind: "service", Name: s.Name, Owner: f.Repo, Words: words.Split(s.Name), Doc: s.Doc,
				Defs: []Loc{{p, s.Span}}})
			for _, r := range s.RPCs {
				e := add(&Entity{Key: "rpc:" + pf.Package + "." + s.Name + "/" + r.Name, Kind: "rpc", Name: r.Name, Owner: f.Repo,
					Words: words.Split(r.Name), Parent: sk, Defs: []Loc{{p, Span{r.Line, r.Line}}}})
				pp.rpcs[r.Name] = append(pp.rpcs[r.Name], e)
				pp.reqType[r.Request] = true
			}
		}
		for _, m := range pf.Messages {
			kind := "message"
			if isEvents {
				kind = "event"
			}
			mk := kind + ":" + pf.Package + "." + m.Name
			me := add(&Entity{Key: mk, Kind: kind, Name: m.Name, Owner: f.Repo, Words: words.Split(m.Name), Doc: m.Doc,
				Defs: []Loc{{p, m.Span}}, GoName: m.Name})
			pp.types[m.Name] = me
			if isEvents {
				pp.topics["Topic"+m.Name] = me
			}
			for _, fd := range m.Fields {
				fe := add(&Entity{Key: "field:" + pf.Package + "." + m.Name + "." + fd.Name, Kind: "field", Name: fd.Name,
					Owner: f.Repo, Words: words.Split(fd.Name), Parent: mk, Defs: []Loc{{p, Span{fd.Line, fd.Line}}},
					GoName: GoFieldName(fd.Name)})
				pp.fields[fe.GoName] = append(pp.fields[fe.GoName], fe)
			}
		}
		for _, en := range pf.Enums {
			w := words.Split(en.Name)
			for _, v := range en.Values {
				w = append(w, words.Split(v)...)
			}
			e := add(&Entity{Key: "enum:" + pf.Package + "." + en.Name, Kind: "enum", Name: en.Name, Owner: f.Repo,
				Words: dedupe(w), Defs: []Loc{{p, en.Span}}, GoName: en.Name})
			pp.types[en.Name] = e
		}
	}

	// the generated .pb.go next to a .proto defines the same names; record it as a definition too
	for _, p := range paths {
		f := ix.Files[p]
		if f.Kind != "go" || f.Test {
			continue
		}
		importPath := goImportPath(ix, f)
		if pp := protos[importPath]; pp != nil {
			for _, d := range f.Decls {
				if e := pp.types[d.Name]; e != nil {
					e.Defs = append(e.Defs, Loc{p, d.Span})
				}
				if e := pp.topics[d.Name]; e != nil {
					e.Defs = append(e.Defs, Loc{p, d.Span})
				}
				if strings.HasSuffix(d.Name, "Client") || strings.HasSuffix(d.Name, "Server") {
					svc := strings.TrimSuffix(strings.TrimSuffix(d.Name, "Client"), "Server")
					if e := ent["service:"+pp.file.Proto.Package+"."+svc]; e != nil {
						e.Defs = append(e.Defs, Loc{p, d.Span})
					}
				}
			}
			continue
		}
		// exported declarations of shared Go packages become entities: money.Format, bus.Bus, log.Info
		if f.Package == "main" {
			continue
		}
		for _, d := range f.Decls {
			if d.Name == "" || !isExported(d.Name) || d.Kind == "method" {
				continue
			}
			k := "go:" + importPath + "." + d.Name
			e := add(&Entity{Key: k, Kind: map[bool]string{true: "type", false: "func"}[d.Kind == "type"], Name: d.Name, Owner: f.Repo,
				Words: words.Split(d.Name), Doc: d.Doc, Parent: "pkg:" + importPath})
			e.Defs = append(e.Defs, Loc{p, d.Span})
		}
	}

	for _, r := range ix.Repos {
		w := append(words.Split(r.Name), words.Split(r.Team)...)
		add(&Entity{Key: "repo:" + r.Name, Kind: "repo", Name: r.Name, Owner: r.Name, Words: dedupe(w),
			Defs: []Loc{{r.Name + "/service.yaml", Span{1, 1}}}})
	}

	cross := 0
	ref := func(e *Entity, f *File, line int, rel string) {
		e.Refs = append(e.Refs, Ref{File: f.Path, Line: line, Rel: rel})
		if e.Owner != f.Repo {
			cross++
		}
	}
	for _, p := range paths {
		f := ix.Files[p]
		if f.Kind != "go" {
			continue
		}
		if protos[goImportPath(ix, f)] != nil {
			continue // generated code refers to itself
		}
		imported := map[string]*protoPkg{}
		for _, imp := range f.Imports {
			if pp := protos[imp]; pp != nil {
				imported[imp] = pp
			}
		}
		usedTypes := map[int][]string{} // line -> proto types used there, to recognise handlers
		for _, u := range f.Uses {
			if pp := imported[u.Pkg]; pp != nil {
				if e := pp.types[u.Name]; e != nil {
					ref(e, f, u.Line, "uses")
					usedTypes[u.Line] = append(usedTypes[u.Line], u.Name)
				} else if e := pp.topics[u.Name]; e != nil {
					_ = e // recorded with its publish or subscribe call below
				}
				continue
			}
			if e := ent["go:"+u.Pkg+"."+u.Name]; e != nil {
				ref(e, f, u.Line, "uses")
			}
		}
		for _, ps := range f.PubSub {
			if pp := imported[ps.Pkg]; pp != nil {
				if e := pp.topics[ps.Const]; e != nil {
					ref(e, f, ps.Line, ps.Op)
				}
			}
		}
		for _, c := range f.Calls {
			for _, pp := range imported {
				for _, e := range pp.rpcs[c.Name] {
					ref(e, f, c.Line, "calls")
				}
			}
		}
		for _, d := range f.Decls {
			if d.Kind != "method" {
				continue
			}
			for _, pp := range imported {
				for _, e := range pp.rpcs[d.Name] {
					// a handler takes the RPC's request type in its signature
					if takesRequest(usedTypes, d.Span.Start, pp.reqType) {
						ref(e, f, d.Span.Start, "implements")
						if svc := ent[e.Parent]; svc != nil && svc.Owner == pp.file.Repo {
							svc.Owner = f.Repo // the service belongs to the team that runs it, not to the contracts repo
						}
					}
				}
			}
		}
		for _, s := range f.Selectors {
			for _, pp := range imported {
				for _, e := range pp.fields[s.Name] {
					ref(e, f, s.Line, "field")
				}
			}
		}
	}
	// an RPC is owned by whoever implements its service
	for _, e := range ent {
		if e.Kind == "rpc" {
			if svc := ent[e.Parent]; svc != nil {
				e.Owner = svc.Owner
			}
		}
	}
	return cross
}

func takesRequest(used map[int][]string, line int, req map[string]bool) bool {
	for l := line; l <= line+2; l++ { // the signature may wrap
		for _, t := range used[l] {
			if req[t] {
				return true
			}
		}
	}
	return false
}

// goImportPath is the import path of the package a Go file belongs to.
func goImportPath(ix *Index, f *File) string {
	r := ix.Repos[f.Repo]
	if r == nil || r.Module == "" {
		return ""
	}
	dir := path.Dir(strings.TrimPrefix(f.Path, f.Repo+"/"))
	if dir == "." {
		return r.Module
	}
	return r.Module + "/" + dir
}

func sortedFiles(ix *Index) []string {
	out := make([]string, 0, len(ix.Files))
	for p := range ix.Files {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func isExported(n string) bool { return n[0] >= 'A' && n[0] <= 'Z' }

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
