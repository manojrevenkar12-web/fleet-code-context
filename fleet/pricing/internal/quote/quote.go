// Package quote prices a basket.
package quote

import (
	"context"

	"fleet.example/platform-go/money"
	pricingv1 "fleet.example/proto/pricing/v1"
)

// DeliveryFee is the flat delivery fee.
const DeliveryFee money.Pence = 299

type Server struct{}

// Quote returns subtotal, delivery fee and total in pence.
func (Server) Quote(ctx context.Context, in *pricingv1.QuoteRequest) (*pricingv1.PriceQuote, error) {
	var sub money.Pence
	for _, it := range in.Items {
		sub += money.Pence(it.UnitPricePence) * money.Pence(it.Quantity)
	}
	return &pricingv1.PriceQuote{SubtotalPence: int64(sub), DeliveryFeePence: int64(DeliveryFee), TotalPence: int64(sub + DeliveryFee)}, nil
}
