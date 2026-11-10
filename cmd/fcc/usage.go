package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
)

type usageRow struct {
	Tool       string  `json:"tool"`
	Calls      int     `json:"calls"`
	Errors     int     `json:"errors"`
	P50        float64 `json:"p50_ms"`
	P95        float64 `json:"p95_ms"`
	MeanTokens float64 `json:"mean_tokens"`
}

// cmdUsage summarises the telemetry log: which tools agents call, how often they fail, how fast they answer and
// how much context they take.
func cmdUsage(args []string) error {
	fs := flag.NewFlagSet("usage", flag.ExitOnError)
	path := fs.String("telemetry", ".fcc/telemetry.jsonl", "telemetry log written by fcc serve")
	fs.Parse(args)
	f, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer f.Close()
	type ev struct {
		Tool   string  `json:"tool"`
		Ms     float64 `json:"ms"`
		Tokens int     `json:"tokens"`
		Error  bool    `json:"error"`
	}
	ms := map[string][]float64{}
	rows := map[string]*usageRow{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e ev
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		r := rows[e.Tool]
		if r == nil {
			r = &usageRow{Tool: e.Tool}
			rows[e.Tool] = r
		}
		r.Calls++
		if e.Error {
			r.Errors++
		}
		r.MeanTokens += float64(e.Tokens)
		ms[e.Tool] = append(ms[e.Tool], e.Ms)
	}
	var out []usageRow
	for t, r := range rows {
		r.P50, r.P95 = pct(ms[t], 50), pct(ms[t], 95)
		r.MeanTokens = round1(r.MeanTokens / float64(r.Calls))
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Calls > out[j].Calls })
	b, _ := json.MarshalIndent(out, "", " ")
	fmt.Println(string(b))
	return sc.Err()
}
