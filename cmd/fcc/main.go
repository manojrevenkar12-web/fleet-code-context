// Command fcc indexes a fleet of repositories and serves context to coding agents.
//
//	fcc index  -fleet DIR -out index.json [-prev index.json]
//	fcc pack   -fleet DIR -repo REPO [-budget 4000] "ticket text"
//	fcc who-uses -fleet DIR NAME
//	fcc check  -fleet DIR [-repo REPO] [-fail]      instructions that no longer hold
//	fcc serve  -fleet DIR | -index index.json [-telemetry .fcc/telemetry.jsonl]
//	fcc usage  -telemetry .fcc/telemetry.jsonl
//	fcc eval   [-heldout]                            writes reports/results.json
//	fcc bench                                        writes reports/bench.json
//	fcc report                                       writes reports/REPORT.md and the README dashboard
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/drift"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/mcp"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "index":
		err = cmdIndex(args)
	case "pack":
		err = cmdPack(args)
	case "who-uses":
		err = cmdWhoUses(args)
	case "check":
		err = cmdCheck(args)
	case "serve":
		err = cmdServe(args)
	case "usage":
		err = cmdUsage(args)
	case "eval":
		err = cmdEval(args)
	case "bench":
		err = cmdBench(args)
	case "report":
		err = cmdReport(args)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fcc:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: fcc index|pack|who-uses|check|serve|usage|eval|bench|report [flags]")
	os.Exit(2)
}

func settings() (rank.Settings, pack.Options, error) {
	s, err := rank.LoadSettings("config/rank.json")
	if err != nil {
		return s, pack.Options{}, err
	}
	o, err := pack.LoadOptions("config/pack.json")
	return s, o, err
}

func load(fleet, snapshot string) (*index.Index, error) {
	if snapshot != "" {
		return index.Load(snapshot)
	}
	ix, st, err := index.Build(fleet, nil)
	if err == nil && st.ParseErrors > 0 {
		fmt.Fprintf(os.Stderr, "fcc: %d files did not parse and were indexed as text\n", st.ParseErrors)
	}
	return ix, err
}

func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	fleet := fs.String("fleet", "fleet", "directory holding one repository per subdirectory")
	out := fs.String("out", ".fcc/index.json", "where to write the index")
	prev := fs.String("prev", "", "an earlier index; unchanged repositories are not parsed again")
	fs.Parse(args)
	var p *index.Index
	if *prev != "" {
		var err error
		if p, err = index.Load(*prev); err != nil {
			return err
		}
	}
	ix, st, err := index.Build(*fleet, p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirOf(*out), 0o755); err != nil {
		return err
	}
	if err := ix.Save(*out); err != nil {
		return err
	}
	b, _ := json.Marshal(st)
	fmt.Println(string(b))
	return nil
}

func cmdPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	fleet := fs.String("fleet", "fleet", "fleet directory")
	snap := fs.String("index", "", "index written by fcc index")
	repo := fs.String("repo", "", "repository the agent works in")
	budget := fs.Int("budget", 0, "token budget (default from config/pack.json)")
	fs.Parse(args)
	ticket := strings.Join(fs.Args(), " ")
	if ticket == "" {
		return fmt.Errorf("give the ticket text as the argument")
	}
	s, o, err := settings()
	if err != nil {
		return err
	}
	if *budget > 0 {
		o.Budget = *budget
	}
	ix, err := load(*fleet, *snap)
	if err != nil {
		return err
	}
	p := pack.Build(ix, rank.New(ix, s).Rank(ticket, *repo), drift.Check(ix), *repo, o)
	fmt.Print(p.Text)
	fmt.Fprintf(os.Stderr, "%d tokens, %d files, %d stale instructions flagged\n", p.Tokens, len(p.Files), p.Stale)
	return nil
}

func cmdWhoUses(args []string) error {
	fs := flag.NewFlagSet("who-uses", flag.ExitOnError)
	fleet := fs.String("fleet", "fleet", "fleet directory")
	fs.Parse(args)
	ix, err := load(*fleet, "")
	if err != nil {
		return err
	}
	s, o, err := settings()
	if err != nil {
		return err
	}
	text, isErr := mcp.New(ix, s, o, nil).CallTool("who_uses", map[string]any{"name": strings.Join(fs.Args(), " ")})
	if isErr {
		return fmt.Errorf("%s", text)
	}
	fmt.Print(text)
	return nil
}

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	fleet := fs.String("fleet", "fleet", "fleet directory")
	repo := fs.String("repo", "", "one repository (default: all)")
	fail := fs.Bool("fail", false, "exit 1 when any instruction is out of date (for CI)")
	fs.Parse(args)
	ix, err := load(*fleet, "")
	if err != nil {
		return err
	}
	stale := 0
	for _, c := range drift.Check(ix) {
		if (*repo == "" || c.Repo == *repo) && c.Status == "stale" {
			stale++
			fmt.Printf("%s/AGENTS.md:%d: %s\n    %s\n", c.Repo, c.Line, c.Text, strings.Join(c.Reasons, "\n    "))
		}
	}
	fmt.Fprintf(os.Stderr, "%d instructions out of date\n", stale)
	if *fail && stale > 0 {
		os.Exit(1)
	}
	return nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fleet := fs.String("fleet", "fleet", "fleet directory")
	snap := fs.String("index", "", "index written by fcc index (faster start)")
	tel := fs.String("telemetry", ".fcc/telemetry.jsonl", "tool-call log; empty to disable")
	fs.Parse(args)
	ix, err := load(*fleet, *snap)
	if err != nil {
		return err
	}
	s, o, err := settings()
	if err != nil {
		return err
	}
	var srv *mcp.Server
	if *tel != "" {
		if err := os.MkdirAll(dirOf(*tel), 0o755); err != nil {
			return err
		}
		f, err := mcp.OpenTelemetry(*tel)
		if err != nil {
			return err
		}
		defer f.Close()
		srv = mcp.New(ix, s, o, f)
	} else {
		srv = mcp.New(ix, s, o, nil)
	}
	return srv.Serve(os.Stdin, os.Stdout)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "."
}
