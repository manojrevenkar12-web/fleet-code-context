// Package consumer alerts kitchen tablets.
package consumer

import (
	"context"
	"encoding/json"

	"fleet.example/platform-go/bus"
	"fleet.example/platform-go/log"
	eventsv1 "fleet.example/proto/events/v1"
)

// Tablet sends an alert to a kitchen tablet.
type Tablet interface {
	Alert(ctx context.Context, restaurantID, text string) error
}

// Register alerts the kitchen of every new order.
func Register(b bus.Bus, t Tablet) {
	b.Subscribe(eventsv1.TopicOrderPlaced, func(ctx context.Context, payload []byte) error {
		var ev eventsv1.OrderPlaced
		if err := json.Unmarshal(payload, &ev); err != nil {
			return err
		}
		log.Info(ctx, "kitchen alert", "order_id", ev.OrderId)
		return t.Alert(ctx, ev.RestaurantId, "New order "+ev.OrderId)
	})
}
