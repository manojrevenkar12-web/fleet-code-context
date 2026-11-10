package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"fleet.example/notifications/internal/push"
	"fleet.example/platform-go/bus"
	eventsv1 "fleet.example/proto/events/v1"
)

// Customers resolves who placed an order.
type Customers interface {
	CustomerFor(ctx context.Context, orderID string) (string, error)
}

// RegisterRiderUpdates tells the customer their rider is on the way.
func RegisterRiderUpdates(b bus.Bus, p push.Sender, c Customers) {
	b.Subscribe(eventsv1.TopicRiderAssigned, func(ctx context.Context, payload []byte) error {
		var ev eventsv1.RiderAssigned
		if err := json.Unmarshal(payload, &ev); err != nil {
			return err
		}
		cust, err := c.CustomerFor(ctx, ev.OrderId)
		if err != nil {
			return err
		}
		return p.Send(ctx, cust, fmt.Sprintf("Your rider is arriving in %d minutes", ev.EtaSeconds/60))
	})
}
