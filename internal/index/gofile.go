package index

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// ParseGo reads one Go file: its package, imports, declarations, the selectors it uses on imported packages,
// the methods it calls on values, and the topics it publishes or subscribes to. protoNames maps proto Go
// import paths to their package names, so an import without an alias still resolves.
func ParseGo(path string, src []byte, protoNames map[string]string) (*File, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	f := &File{Kind: "go", Package: af.Name.Name, Test: strings.HasSuffix(path, "_test.go")}
	f.Lines = fset.Position(af.End()).Line
	alias := map[string]string{} // local name -> import path
	for _, imp := range af.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		f.Imports = append(f.Imports, p)
		name := p[strings.LastIndex(p, "/")+1:]
		if n, ok := protoNames[p]; ok {
			name = n
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		alias[name] = p
	}
	line := func(n ast.Node) int { return fset.Position(n.Pos()).Line }
	span := func(n ast.Node) Span { return Span{fset.Position(n.Pos()).Line, fset.Position(n.End()).Line} }
	terms := map[string]bool{}
	addTerms := func(s string) {
		for _, w := range words.Split(s) {
			terms[w] = true
		}
	}

	for _, d := range af.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			decl := Decl{Name: d.Name.Name, Kind: "func", Span: span(d)}
			if d.Doc != nil {
				decl.Span.Start = line(d.Doc)
				decl.Doc = strings.TrimSpace(d.Doc.Text())
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				decl.Kind = "method"
				decl.Recv = recvName(d.Recv.List[0].Type)
			}
			f.Decls = append(f.Decls, decl)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				var name string
				var kind string
				switch s := s.(type) {
				case *ast.TypeSpec:
					name, kind = s.Name.Name, "type"
				case *ast.ValueSpec:
					if len(s.Names) == 0 {
						continue
					}
					name, kind = s.Names[0].Name, "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
				default:
					continue
				}
				decl := Decl{Name: name, Kind: kind, Span: span(s)}
				if len(d.Specs) == 1 {
					decl.Span = span(d)
				}
				if d.Doc != nil {
					decl.Doc = strings.TrimSpace(d.Doc.Text())
					if len(d.Specs) == 1 {
						decl.Span.Start = line(d.Doc)
					}
				}
				f.Decls = append(f.Decls, decl)
			}
		}
	}

	ast.Inspect(af, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			addTerms(n.Name)
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if s, err := strconv.Unquote(n.Value); err == nil {
					addTerms(s)
				}
			}
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && alias[x.Name] != "" {
				return true // a package function; recorded as a use below
			}
			f.Calls = append(f.Calls, NameLine{sel.Sel.Name, line(sel.Sel)})
			if sel.Sel.Name == "Publish" || sel.Sel.Name == "Subscribe" {
				op := map[string]string{"Publish": "publishes", "Subscribe": "subscribes"}[sel.Sel.Name]
				for _, a := range n.Args {
					if as, ok := a.(*ast.SelectorExpr); ok {
						if x, ok := as.X.(*ast.Ident); ok && alias[x.Name] != "" && strings.HasPrefix(as.Sel.Name, "Topic") {
							f.PubSub = append(f.PubSub, PubSub{Op: op, Pkg: alias[x.Name], Const: as.Sel.Name, Line: line(as)})
						}
					}
				}
			}
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && alias[x.Name] != "" {
				f.Uses = append(f.Uses, Use{Pkg: alias[x.Name], Name: n.Sel.Name, Line: line(n.Sel)})
			} else {
				f.Selectors = append(f.Selectors, NameLine{n.Sel.Name, line(n.Sel)})
			}
		case *ast.KeyValueExpr:
			if k, ok := n.Key.(*ast.Ident); ok {
				f.Selectors = append(f.Selectors, NameLine{k.Name, line(k)}) // composite literal fields
			}
		}
		return true
	})
	for _, c := range af.Comments {
		addTerms(c.Text())
	}
	for t := range terms {
		f.Terms = append(f.Terms, t)
	}
	return f, nil
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return recvName(t.X)
	}
	return ""
}
