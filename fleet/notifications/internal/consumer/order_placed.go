// Package consumer turns events into customer messages.
package consumer

import (
	"context"
	"encoding/json"

	"fleet.example/notifications/internal/push"
	"fleet.example/platform-go/bus"
	"fleet.example/platform-go/money"
	eventsv1 "fleet.example/proto/events/v1"
)

// RegisterReceipts sends a receipt when an order is placed.
func RegisterReceipts(b bus.Bus, p push.Sender) {
	b.Subscribe(eventsv1.TopicOrderPlaced, func(ctx context.Context, payload []byte) error {
		var ev eventsv1.OrderPlaced
		if err := json.Unmarshal(payload, &ev); err != nil {
			return err
		}
		return p.Send(ctx, ev.CustomerId, "Thanks! You paid "+money.Format(money.Pence(ev.TotalPence)))
	})
}
