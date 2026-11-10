package drift

import (
	"strings"
	"testing"

	"github.com/manojrevenkar12-web/fleet-code-context/internal/index"
)

func TestFleetClaims(t *testing.T) {
	ix, _, err := index.Build("../../fleet", nil)
	if err != nil {
		t.Fatal(err)
	}
	stale := map[string]bool{}
	for _, c := range Check(ix) {
		if c.Status == "stale" {
			stale[c.Repo+": "+c.Text] = true
		}
	}
	want := []string{"orders: `internal/handlers/`", "orders: `make lint`", "orders: Prices come from pricing via `pricingclient.Quote`",
		"orders: Order totals are float64 pounds", "dispatch: ETAs are in minutes", "pricing: Delivery fee is configured in `config/fees.yaml`",
		"notifications: `go test ./internal/templates/...`"}
	for _, w := range want {
		found := false
		for s := range stale {
			if strings.HasPrefix(s, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("not flagged: %s", w)
		}
	}
	if len(stale) != len(want) {
		t.Errorf("flagged %d claims, want %d: %v", len(stale), len(want), stale)
	}
}

func TestSymbolResolution(t *testing.T) {
	ix, _, _ := index.Build("../../fleet", nil)
	cases := map[string]bool{ // ref -> problem expected
		"ridersv1.RiderServiceClient.ListAvailableRiders": false,
		"ridersv1.RiderServiceClient.ListRiders":          true,
		"eventsv1.RiderAssigned":                          false,
		"eventsv1.RiderPickedUp":                          true,
		"money.Format":                                    false,
		"money.Parse":                                     true,
		"pricingclient.Quote":                             true,
	}
	for ref, problem := range cases {
		if got := symbolProblem(ix, ref) != ""; got != problem {
			t.Errorf("%s: problem=%v, want %v (%s)", ref, got, problem, symbolProblem(ix, ref))
		}
	}
}

func TestUnits(t *testing.T) {
	ix, _, _ := index.Build("../../fleet", nil)
	if unitProblem(ix, "ETAs are in seconds") != "" || unitProblem(ix, "ETAs are in minutes") == "" {
		t.Fatal("eta units")
	}
	if unitProblem(ix, "The weather is nice") != "" {
		t.Fatal("no quantity, no problem")
	}
}
