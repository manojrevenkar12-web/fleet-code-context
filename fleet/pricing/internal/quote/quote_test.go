package quote

import (
	"context"
	"testing"

	pricingv1 "fleet.example/proto/pricing/v1"
)

func TestQuote(t *testing.T) {
	q, _ := Server{}.Quote(context.Background(), &pricingv1.QuoteRequest{Items: []*pricingv1.QuoteItem{{Quantity: 2, UnitPricePence: 500}}})
	if q.TotalPence != 1299 {
		t.Fatalf("got %d", q.TotalPence)
	}
}
