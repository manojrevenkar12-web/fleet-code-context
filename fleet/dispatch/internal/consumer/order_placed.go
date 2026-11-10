// Package consumer reacts to order events.
package consumer

import (
	"context"
	"encoding/json"

	"fleet.example/dispatch/internal/assign"
	"fleet.example/platform-go/bus"
	dispatchv1 "fleet.example/proto/dispatch/v1"
	eventsv1 "fleet.example/proto/events/v1"
)

// Register subscribes dispatch to OrderPlaced.
func Register(b bus.Bus, a *assign.Assigner) {
	b.Subscribe(eventsv1.TopicOrderPlaced, func(ctx context.Context, payload []byte) error {
		var ev eventsv1.OrderPlaced
		if err := json.Unmarshal(payload, &ev); err != nil {
			return err
		}
		_, err := a.AssignRider(ctx, &dispatchv1.AssignRiderRequest{OrderId: ev.OrderId, RestaurantId: ev.RestaurantId}, 51.5, -0.12)
		return err
	})
}
