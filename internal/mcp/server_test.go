package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/pack"
	"github.com/manojrevenkar12-web/fleet-code-context/internal/rank"
)

func server(t *testing.T, tel *bytes.Buffer) *Server {
	t.Helper()
	ix, _, err := index.Build("../../fleet", nil)
	if err != nil {
		t.Fatal(err)
	}
	if tel == nil {
		return New(ix, rank.Defaults, pack.DefaultOptions, nil)
	}
	return New(ix, rank.Defaults, pack.DefaultOptions, tel)
}

func session(t *testing.T, s *Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}
		res = append(res, m)
	}
	return res
}

func text(t *testing.T, m map[string]any) (string, bool) {
	t.Helper()
	r := m["result"].(map[string]any)
	c := r["content"].([]any)[0].(map[string]any)
	isErr, _ := r["isError"].(bool)
	return c["text"].(string), isErr
}

func TestHandshakeAndList(t *testing.T) {
	res := session(t, server(t, nil),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list"}`,
		`not json`)
	if len(res) != 5 { // the notification gets no answer
		t.Fatalf("got %d responses", len(res))
	}
	if res[0]["result"].(map[string]any)["protocolVersion"] != ProtocolVersion {
		t.Fatal("protocol version")
	}
	tools := res[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 5 {
		t.Fatalf("tools: %d", len(tools))
	}
	for _, tl := range tools {
		m := tl.(map[string]any)
		if m["inputSchema"].(map[string]any)["type"] != "object" {
			t.Fatalf("schema of %v", m["name"])
		}
	}
	if res[3]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Fatal("unknown method")
	}
	if res[4]["error"].(map[string]any)["code"].(float64) != -32700 {
		t.Fatal("parse error")
	}
}

func TestContextPackTool(t *testing.T) {
	var tel bytes.Buffer
	res := session(t, server(t, &tel),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"context_pack","arguments":{"task":"The rider on the way push says arriving in 0 minutes for short trips","repo":"notifications","budget_tokens":3000}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"context_pack","arguments":{"task":"x","repo":"nope"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"context_pack","arguments":{"task":"   "}}}`)
	got, isErr := text(t, res[0])
	if isErr || !strings.Contains(got, "notifications/internal/consumer/rider_assigned.go") || !strings.Contains(got, "RiderAssigned") {
		t.Fatalf("pack: %s", got)
	}
	if !strings.Contains(got, "team customer-comms") {
		t.Fatal("owners missing")
	}
	if _, isErr := text(t, res[1]); !isErr {
		t.Fatal("unknown repo should be a tool error")
	}
	if _, isErr := text(t, res[2]); !isErr {
		t.Fatal("empty task should be a tool error")
	}
	lines := strings.Split(strings.TrimSpace(tel.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"tool":"context_pack"`) || !strings.Contains(lines[1], `"error":true`) {
		t.Fatalf("telemetry: %s", tel.String())
	}
	if strings.Contains(tel.String(), "arriving") {
		t.Fatal("telemetry must not contain the ticket")
	}
}

func TestWhoUsesAndCards(t *testing.T) {
	res := session(t, server(t, nil),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"who_uses","arguments":{"name":"OrderPlaced"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"service_card","arguments":{"repo":"orders"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check_instructions","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"who_uses","arguments":{"name":"NoSuchThing"}}}`)
	who, _ := text(t, res[0])
	for _, want := range []string{"publishes orders/internal/server/server.go", "subscribes dispatch/internal/consumer/order_placed.go",
		"team partners", "team customer-comms"} {
		if !strings.Contains(who, want) {
			t.Fatalf("who_uses missing %q:\n%s", want, who)
		}
	}
	card, _ := text(t, res[1])
	if !strings.Contains(card, "publishes: event:events.v1.OrderCancelled, event:events.v1.OrderPlaced") || !strings.Contains(card, "make lint") {
		t.Fatalf("card:\n%s", card)
	}
	chk, _ := text(t, res[2])
	if !strings.HasPrefix(chk, "41 instructions read, 7 out of date.") {
		t.Fatalf("check: %s", chk[:80])
	}
	if _, isErr := text(t, res[3]); !isErr {
		t.Fatal("unknown name should be a tool error")
	}
}
