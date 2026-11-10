package index

import (
	"os"
	"path/filepath"
	"testing"
)

func fleet(t *testing.T) *Index {
	t.Helper()
	ix, st, err := Build("../../fleet", nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.ParseErrors != 0 {
		t.Fatalf("parse errors: %d", st.ParseErrors)
	}
	return ix
}

func refs(e *Entity, rel string) map[string]bool {
	out := map[string]bool{}
	for _, r := range e.Refs {
		if rel == "" || r.Rel == rel {
			out[r.File] = true
		}
	}
	return out
}

func TestReposAndOwners(t *testing.T) {
	ix := fleet(t)
	if len(ix.Repos) != 9 {
		t.Fatalf("repos: %d", len(ix.Repos))
	}
	o := ix.Repos["orders"]
	if o.Team != "orders" || o.Module != "fleet.example/orders" || len(o.Reviewers) != 2 || o.Tier != 1 {
		t.Fatalf("orders meta: %+v", o)
	}
	if !contains(o.MakeTargets, "test") || contains(o.MakeTargets, "lint") {
		t.Fatalf("targets: %v", o.MakeTargets)
	}
}

func TestEventEdgesCrossRepos(t *testing.T) {
	ix := fleet(t)
	e := ix.Entities["event:events.v1.OrderPlaced"]
	if e == nil || e.Owner != "proto" {
		t.Fatalf("event: %+v", e)
	}
	pub, sub := refs(e, "publishes"), refs(e, "subscribes")
	if !pub["orders/internal/server/server.go"] || len(pub) != 1 {
		t.Fatalf("publishers: %v", pub)
	}
	for _, f := range []string{"dispatch/internal/consumer/order_placed.go", "notifications/internal/consumer/order_placed.go",
		"restaurants/internal/consumer/order_placed.go"} {
		if !sub[f] {
			t.Fatalf("missing subscriber %s in %v", f, sub)
		}
	}
	if len(e.Defs) != 3 { // the .proto message, the generated Go struct and its topic constant
		t.Fatalf("defs: %v", e.Defs)
	}
}

func TestRPCImplementersAndCallers(t *testing.T) {
	ix := fleet(t)
	e := ix.Entities["rpc:dispatch.v1.DispatchService/GetEta"]
	if e == nil {
		t.Fatal("no GetEta")
	}
	if !refs(e, "implements")["dispatch/internal/assign/assign.go"] {
		t.Fatalf("implementer: %v", e.Refs)
	}
	if !refs(e, "calls")["orders/internal/server/track.go"] {
		t.Fatalf("caller: %v", e.Refs)
	}
	if e.Owner != "dispatch" || ix.Entities[e.Parent].Owner != "dispatch" {
		t.Fatalf("owner: %s", e.Owner)
	}
	q := ix.Entities["rpc:pricing.v1.PricingService/Quote"]
	if !refs(q, "implements")["pricing/internal/quote/quote.go"] || !refs(q, "calls")["orders/internal/server/server.go"] {
		t.Fatalf("quote refs: %v", q.Refs)
	}
}

func TestFieldAndLibraryRefs(t *testing.T) {
	ix := fleet(t)
	f := ix.Entities["field:events.v1.RiderAssigned.eta_seconds"]
	got := refs(f, "field")
	if !got["notifications/internal/consumer/rider_assigned.go"] || !got["dispatch/internal/assign/assign.go"] {
		t.Fatalf("field refs: %v", got)
	}
	m := ix.Entities["go:fleet.example/platform-go/money.Format"]
	if m == nil || !refs(m, "uses")["notifications/internal/consumer/order_placed.go"] {
		t.Fatalf("money.Format: %+v", m)
	}
}

func TestIncrementalRebuild(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, "../../fleet", dir)
	first, _, err := Build(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "riders/internal/location/location.go")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append(b, []byte("\n// touched\n")...), 0o644)
	second, st, err := Build(dir, first)
	if err != nil {
		t.Fatal(err)
	}
	if st.Reparsed != 1 || st.Reused != 8 {
		t.Fatalf("stats: %+v", st)
	}
	if len(second.Entities) != len(first.Entities) {
		t.Fatalf("entities changed: %d vs %d", len(second.Entities), len(first.Entities))
	}
	full, _, _ := Build(dir, nil)
	if countRefs(full) != countRefs(second) {
		t.Fatalf("incremental differs from full: %d vs %d", countRefs(second), countRefs(full))
	}
}

func countRefs(ix *Index) int {
	n := 0
	for _, e := range ix.Entities {
		n += len(e.Refs)
	}
	return n
}

func TestProtoParser(t *testing.T) {
	pf := ParseProto(`syntax = "proto3";
package a.v1;
option go_package = "x.example/a/v1;av1";
// Svc does things.
service Svc {
  rpc Do(DoRequest) returns (stream DoResponse);
}
// One line.
message M { string id = 1; repeated Item items = 2; }
enum E { E_UNSPECIFIED = 0; E_ONE = 1; }
`)
	if pf.Package != "a.v1" || pf.GoName != "av1" || len(pf.Services) != 1 || pf.Services[0].RPCs[0].Response != "DoResponse" {
		t.Fatalf("%+v", pf)
	}
	if len(pf.Messages) != 1 || len(pf.Messages[0].Fields) != 2 || !pf.Messages[0].Fields[1].Repeated || pf.Messages[0].Doc != "One line." {
		t.Fatalf("messages %+v", pf.Messages)
	}
	if len(pf.Enums) != 1 || len(pf.Enums[0].Values) != 2 {
		t.Fatalf("enums %+v", pf.Enums)
	}
	if GoFieldName("eta_seconds") != "EtaSeconds" {
		t.Fatal(GoFieldName("eta_seconds"))
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, _ := os.ReadFile(p)
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}
