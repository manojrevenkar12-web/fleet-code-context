// Package mcp serves the fleet index to coding agents over the Model Context Protocol: JSON-RPC 2.0, one
// message per line on stdin and stdout, as Claude Code, Codex and Cursor speak it to local servers.
//
// Every tool call is written to a telemetry log (tool, latency, tokens returned, whether it failed), so the
// team can see which tools agents actually use and how much context they pull, without logging the tickets.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/words"
)

// ProtocolVersion is the MCP revision this server implements.
const ProtocolVersion = "2025-06-18"

// Server answers tool calls from one index.
type Server struct {
	ix        *index.Index
	ranker    *rank.Ranker
	claims    []drift.Claim
	opt       pack.Options
	telemetry io.Writer
	mu        sync.Mutex
	now       func() time.Time
}

// New prepares a server. telemetry may be nil.
func New(ix *index.Index, s rank.Settings, opt pack.Options, telemetry io.Writer) *Server {
	return &Server{ix: ix, ranker: rank.New(ix, s), claims: drift.Check(ix), opt: opt, telemetry: telemetry, now: time.Now}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads requests from r and writes responses to w until r ends.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := enc.Encode(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}}); err != nil {
				return err
			}
			continue
		}
		resp := s.Handle(req)
		if resp == nil {
			continue // a notification gets no answer
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Handle answers one request; nil for notifications.
func (s *Server) Handle(req request) *response {
	if len(req.ID) == 0 {
		return nil
	}
	out := &response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		out.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "fleet-code-context", "version": "1.0.0"},
			"instructions": "Call context_pack first with the ticket and the repository you are in. It returns who owns what the " +
				"change touches, the instructions that hold, the ones that are out of date, how the contracts connect across " +
				"repositories, and the code. Use who_uses before changing a contract.",
		}
	case "ping":
		out.Result = map[string]any{}
	case "tools/list":
		out.Result = map[string]any{"tools": tools}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			out.Error = &rpcError{-32602, "invalid params"}
			return out
		}
		out.Result = s.call(p.Name, p.Arguments)
	default:
		out.Error = &rpcError{-32601, "method not found: " + req.Method}
	}
	return out
}

var tools = []map[string]any{
	tool("context_pack", "Context for a ticket: owners and reviewers, instructions that hold, instructions that are out of date, "+
		"contract edges across repositories, and the relevant code, within a token budget.",
		map[string]any{"task": str("The ticket or task, in plain words."), "repo": str("The repository you are working in."),
			"budget_tokens": map[string]any{"type": "integer", "description": "Most tokens to return (default 4000).", "minimum": 500, "maximum": 32000}},
		"task"),
	tool("who_uses", "Everyone who defines, implements, calls, publishes, consumes or reads an RPC, message, event, field or shared "+
		"function, grouped by repository, with the team that must review a change.",
		map[string]any{"name": str("An identifier: GetEta, RiderAssigned, eta_seconds, money.Format.")}, "name"),
	tool("service_card", "A repository's owners, tier and channel, the contracts it implements, calls, publishes and consumes, and "+
		"which of its AGENTS.md instructions are out of date.",
		map[string]any{"repo": str("Repository name.")}, "repo"),
	tool("check_instructions", "Checks a repository's AGENTS.md (or every repository's) against the code: paths, make targets, "+
		"commands, Go symbols, import paths and units. Returns the instructions that no longer hold, and why.",
		map[string]any{"repo": str("Repository name; empty for the whole fleet.")}),
	tool("search", "Files ranked for a query, across the fleet.",
		map[string]any{"query": str("Words or identifiers."), "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50}}, "query"),
}

func tool(name, desc string, props map[string]any, required ...string) map[string]any {
	schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return map[string]any{"name": name, "description": desc, "inputSchema": schema,
		"annotations": map[string]any{"readOnlyHint": true, "openWorldHint": false}}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

type event struct {
	TS     string  `json:"ts"`
	Tool   string  `json:"tool"`
	Ms     float64 `json:"ms"`
	Tokens int     `json:"tokens"`
	Error  bool    `json:"error,omitempty"`
	Repo   string  `json:"repo,omitempty"`
}

func (s *Server) call(name string, args map[string]any) map[string]any {
	start := s.now()
	text, err := s.run(name, args)
	ev := event{TS: start.UTC().Format(time.RFC3339), Tool: name, Ms: float64(time.Since(start).Microseconds()) / 1000,
		Tokens: words.Tokens(text), Error: err != nil, Repo: argString(args, "repo")}
	if s.telemetry != nil {
		s.mu.Lock()
		b, _ := json.Marshal(ev)
		_, _ = s.telemetry.Write(append(b, '\n'))
		s.mu.Unlock()
	}
	if err != nil {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func (s *Server) run(name string, args map[string]any) (string, error) {
	switch name {
	case "context_pack":
		task := argString(args, "task")
		if strings.TrimSpace(task) == "" {
			return "", fmt.Errorf("task is required: describe the change in plain words")
		}
		repo := argString(args, "repo")
		if repo != "" && s.ix.Repos[repo] == nil {
			return "", fmt.Errorf("no repository %q in the fleet; call search to find it", repo)
		}
		o := s.opt
		if b, ok := args["budget_tokens"].(float64); ok {
			if b < 500 || b > 32000 {
				return "", fmt.Errorf("budget_tokens must be between 500 and 32000")
			}
			o.Budget = int(b)
		}
		return pack.Build(s.ix, s.ranker.Rank(task, repo), s.claims, repo, o).Text, nil
	case "who_uses":
		return s.whoUses(argString(args, "name"))
	case "service_card":
		return s.serviceCard(argString(args, "repo"))
	case "check_instructions":
		return s.checkInstructions(argString(args, "repo"))
	case "search":
		q := argString(args, "query")
		if q == "" {
			return "", fmt.Errorf("query is required")
		}
		limit := 10
		if l, ok := args["limit"].(float64); ok && l >= 1 {
			limit = int(l)
		}
		res := s.ranker.Rank(q, "")
		var b strings.Builder
		for i, f := range res.Files {
			if i >= limit {
				break
			}
			r := s.ix.Repos[s.ix.Files[f.Key].Repo]
			fmt.Fprintf(&b, "%.2f  %s  (team %s)\n", f.Score, f.Key, r.Team)
		}
		if b.Len() == 0 {
			return "No file matches. Try identifiers or fewer words.", nil
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// Lookup finds entities by identifier: exact name, Go name, or the end of the key.
func Lookup(ix *index.Index, name string) []*index.Entity {
	name = strings.TrimSpace(name)
	var out []*index.Entity
	for k, e := range ix.Entities {
		if e.Kind == "repo" {
			continue
		}
		if e.Name == name || e.GoName == name || strings.HasSuffix(k, "."+name) || strings.HasSuffix(k, "/"+name) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func (s *Server) whoUses(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	ents := Lookup(s.ix, name)
	if len(ents) == 0 {
		return "", fmt.Errorf("nothing in the fleet is called %q; call search to look by words", name)
	}
	var b strings.Builder
	for _, e := range ents {
		fmt.Fprintf(&b, "%s %s (%s)\n", e.Kind, e.Key, e.Owner)
		for _, d := range e.Defs {
			fmt.Fprintf(&b, "  defined  %s:%d\n", d.File, d.Span.Start)
		}
		byRepo := map[string][]string{}
		for _, r := range e.Refs {
			repo := s.ix.Files[r.File].Repo
			byRepo[repo] = append(byRepo[repo], fmt.Sprintf("%s %s:%d", r.Rel, r.File, r.Line))
		}
		repos := make([]string, 0, len(byRepo))
		for r := range byRepo {
			repos = append(repos, r)
		}
		sort.Strings(repos)
		for _, r := range repos {
			rr := s.ix.Repos[r]
			fmt.Fprintf(&b, "  %s (team %s, reviewers %s)\n", r, rr.Team, strings.Join(rr.Reviewers, ", "))
			refs := dedupe(byRepo[r])
			for _, x := range refs {
				fmt.Fprintf(&b, "    %s\n", x)
			}
		}
	}
	return b.String(), nil
}

func (s *Server) serviceCard(repo string) (string, error) {
	r := s.ix.Repos[repo]
	if r == nil {
		return "", fmt.Errorf("no repository %q in the fleet", repo)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s, tier %d)\nteam %s, reviewers %s, %s\nmodule %s\n", r.Name, r.Kind, r.Tier, r.Team, strings.Join(r.Reviewers, ", "), r.Slack, r.Module)
	rels := map[string]map[string]bool{}
	for k, e := range s.ix.Entities {
		for _, ref := range e.Refs {
			if s.ix.Files[ref.File].Repo == repo && ref.Rel != "field" && ref.Rel != "uses" {
				if rels[ref.Rel] == nil {
					rels[ref.Rel] = map[string]bool{}
				}
				rels[ref.Rel][k] = true
			}
		}
	}
	for _, rel := range []string{"implements", "calls", "publishes", "subscribes"} {
		if len(rels[rel]) == 0 {
			continue
		}
		ks := make([]string, 0, len(rels[rel]))
		for k := range rels[rel] {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		fmt.Fprintf(&b, "%s: %s\n", rel, strings.Join(ks, ", "))
	}
	stale := 0
	for _, c := range s.claims {
		if c.Repo == repo && c.Status == "stale" {
			stale++
			fmt.Fprintf(&b, "out of date: AGENTS.md:%d %q: %s\n", c.Line, c.Text, strings.Join(c.Reasons, "; "))
		}
	}
	if stale == 0 && r.Agents != "" {
		b.WriteString("AGENTS.md: every checkable instruction holds\n")
	}
	return b.String(), nil
}

func (s *Server) checkInstructions(repo string) (string, error) {
	if repo != "" && s.ix.Repos[repo] == nil {
		return "", fmt.Errorf("no repository %q in the fleet", repo)
	}
	var b strings.Builder
	n, stale := 0, 0
	for _, c := range s.claims {
		if repo != "" && c.Repo != repo {
			continue
		}
		n++
		if c.Status == "stale" {
			stale++
			fmt.Fprintf(&b, "%s AGENTS.md:%d %q\n  %s\n", c.Repo, c.Line, c.Text, strings.Join(c.Reasons, "\n  "))
		}
	}
	return fmt.Sprintf("%d instructions read, %d out of date.\n", n, stale) + b.String(), nil
}

func argString(args map[string]any, k string) string {
	if v, ok := args[k].(string); ok {
		return v
	}
	return ""
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// OpenTelemetry opens the telemetry log for appending.
func OpenTelemetry(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// CallTool runs one tool outside a session, as the command line does.
func (s *Server) CallTool(name string, args map[string]any) (string, bool) {
	r := s.call(name, args)
	text := r["content"].([]map[string]any)[0]["text"].(string)
	isErr, _ := r["isError"].(bool)
	return text, isErr
}
