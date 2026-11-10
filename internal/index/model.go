// Package index reads a fleet of repositories into one graph: repositories and their owners, protobuf
// contracts, Go declarations, and the edges between them that cross repository boundaries (who implements an
// RPC, who calls it, who publishes and who consumes an event, who uses a shared library function).
package index

// Repo is one repository of the fleet.
type Repo struct {
	Name        string   `json:"name"`
	Module      string   `json:"module"`
	Kind        string   `json:"kind"` // service, library, contracts
	Team        string   `json:"team"`
	Tier        int      `json:"tier"`
	Slack       string   `json:"slack"`
	Reviewers   []string `json:"reviewers"`
	Agents      string   `json:"agents"` // AGENTS.md as written by the team
	MakeTargets []string `json:"make_targets"`
	Hash        string   `json:"hash"` // content hash of every indexed file, for incremental re-indexing
}

// Span is a range of lines in a file, 1-based and inclusive.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Decl is a declaration inside a file: a Go function, method, type or constant; a proto service, RPC,
// message, enum or field.
type Decl struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Recv string `json:"recv,omitempty"`
	Doc  string `json:"doc,omitempty"`
	Span Span   `json:"span"`
}

// File is one indexed source file.
type File struct {
	Repo    string   `json:"repo"`
	Path    string   `json:"path"` // relative to the fleet root, e.g. dispatch/internal/eta/eta.go
	Kind    string   `json:"kind"` // go, proto, doc, config
	Package string   `json:"package,omitempty"`
	Imports []string `json:"imports,omitempty"`
	Test    bool     `json:"test,omitempty"`
	Lines   int      `json:"lines"`
	Decls   []Decl   `json:"decls,omitempty"`
	Terms   []string `json:"terms,omitempty"` // identifier, comment and string-literal terms, for text search

	// raw facts about the file, kept so a changed repository can be re-linked without re-parsing the others
	Uses      []Use      `json:"uses,omitempty"`      // pkg.Name selectors on imported packages
	Calls     []NameLine `json:"calls,omitempty"`     // x.Method(...) calls on values
	Selectors []NameLine `json:"selectors,omitempty"` // x.Field selectors on values
	PubSub    []PubSub   `json:"pubsub,omitempty"`    // bus.Publish / bus.Subscribe with a topic constant
	Proto     *ProtoFile `json:"proto,omitempty"`
}

// Use is a selector on an imported package, e.g. eventsv1.RiderAssigned.
type Use struct {
	Pkg  string `json:"pkg"`
	Name string `json:"name"`
	Line int    `json:"line"`
}

// NameLine is a name at a line.
type NameLine struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

// PubSub is a publish or subscribe call with a topic constant from an imported package.
type PubSub struct {
	Op    string `json:"op"` // publishes or subscribes
	Pkg   string `json:"pkg"`
	Const string `json:"const"`
	Line  int    `json:"line"`
}

// ProtoFile is what a .proto file declares.
type ProtoFile struct {
	Package   string         `json:"package"`
	GoPackage string         `json:"go_package"` // import path
	GoName    string         `json:"go_name"`
	Services  []ProtoService `json:"services,omitempty"`
	Messages  []ProtoMessage `json:"messages,omitempty"`
	Enums     []ProtoEnum    `json:"enums,omitempty"`
}

type ProtoService struct {
	Name string     `json:"name"`
	Doc  string     `json:"doc,omitempty"`
	Span Span       `json:"span"`
	RPCs []ProtoRPC `json:"rpcs"`
}

type ProtoRPC struct {
	Name     string `json:"name"`
	Request  string `json:"request"`
	Response string `json:"response"`
	Line     int    `json:"line"`
}

type ProtoMessage struct {
	Name   string       `json:"name"`
	Doc    string       `json:"doc,omitempty"`
	Span   Span         `json:"span"`
	Fields []ProtoField `json:"fields"`
}

type ProtoField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Number   int    `json:"number"`
	Repeated bool   `json:"repeated,omitempty"`
	Line     int    `json:"line"`
}

type ProtoEnum struct {
	Name   string   `json:"name"`
	Span   Span     `json:"span"`
	Values []string `json:"values"`
}

// Entity is something a change can be about: a contract element, an event, a shared function, a repository.
type Entity struct {
	Key   string   `json:"key"`  // e.g. rpc:dispatch.v1.DispatchService/GetEta
	Kind  string   `json:"kind"` // service, rpc, message, enum, field, event, func, type, repo
	Name  string   `json:"name"`
	Owner string   `json:"owner"`           // repository that defines it
	Words []string `json:"words"`           // terms of the name and its parents, for matching a ticket
	Doc   string   `json:"doc,omitempty"`   // its comment
	Defs  []Loc    `json:"defs"`            // where it is defined
	Refs  []Ref    `json:"refs,omitempty"`  // where it is used, and how
	Topic  string  `json:"topic,omitempty"`  // for events: the topic string
	Parent string  `json:"parent,omitempty"` // e.g. the service of an RPC, the message of a field
	GoName string  `json:"go_name,omitempty"`
}

// Loc is a place in a file.
type Loc struct {
	File string `json:"file"`
	Span Span   `json:"span"`
}

// Ref is a use of an entity.
type Ref struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Rel  string `json:"rel"` // implements, calls, publishes, subscribes, uses, field
}

// Index is the whole fleet.
type Index struct {
	Root     string             `json:"root"`
	Repos    map[string]*Repo   `json:"repos"`
	Files    map[string]*File   `json:"files"`
	Entities map[string]*Entity `json:"entities"`
}

// Rel weights: how strongly a reference ties a file to the entity it refers to. A file that implements an RPC
// will almost surely change with it; one that only mentions a type in passing less so.
var RelWeight = map[string]float64{
	"def":        1.0,
	"implements": 0.9,
	"publishes":  0.85,
	"subscribes": 0.85,
	"calls":      0.85,
	"uses":       0.6,
	"field":      0.5,
}
