// Package consumer reacts to order events.
package consumer

import (
	"context"
	"encoding/json"

	"fleet.example/payments/internal/server"
	"fleet.example/platform-go/bus"
	eventsv1 "fleet.example/proto/events/v1"
	paymentsv1 "fleet.example/proto/payments/v1"
)

// Register refunds every cancelled order.
func Register(b bus.Bus, s *server.Server) {
	b.Subscribe(eventsv1.TopicOrderCancelled, func(ctx context.Context, payload []byte) error {
		var ev eventsv1.OrderCancelled
		if err := json.Unmarshal(payload, &ev); err != nil {
			return err
		}
		_, err := s.Refund(ctx, &paymentsv1.RefundRequest{OrderId: ev.OrderId, Reason: ev.Reason})
		return err
	})
}
