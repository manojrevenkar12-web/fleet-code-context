// Package server implements paymentsv1.PaymentServiceServer.
package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"fleet.example/platform-go/bus"
	"fleet.example/platform-go/log"
	eventsv1 "fleet.example/proto/events/v1"
	paymentsv1 "fleet.example/proto/payments/v1"
)

var ErrDeclined = errors.New("card declined")

type Server struct {
	Bus      bus.Bus
	mu       sync.Mutex
	payments map[string]*paymentsv1.Payment // by order id
	next     int
}

func (s *Server) Authorize(ctx context.Context, in *paymentsv1.AuthorizeRequest) (*paymentsv1.Payment, error) {
	if in.PaymentMethodId == "" {
		return nil, ErrDeclined
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.payments == nil {
		s.payments = map[string]*paymentsv1.Payment{}
	}
	s.next++
	p := &paymentsv1.Payment{Id: fmt.Sprintf("p-%d", s.next), OrderId: in.OrderId, AmountPence: in.AmountPence, State: paymentsv1.PaymentState_PAYMENT_STATE_AUTHORIZED}
	s.payments[in.OrderId] = p
	return p, nil
}

func (s *Server) Capture(ctx context.Context, in *paymentsv1.CaptureRequest) (*paymentsv1.Payment, error) {
	s.mu.Lock()
	var p *paymentsv1.Payment
	for _, x := range s.payments {
		if x.Id == in.PaymentId {
			p = x
		}
	}
	s.mu.Unlock()
	if p == nil {
		return nil, errors.New("unknown payment")
	}
	p.State = paymentsv1.PaymentState_PAYMENT_STATE_CAPTURED
	return p, s.Bus.Publish(ctx, eventsv1.TopicPaymentCaptured, &eventsv1.PaymentCaptured{OrderId: p.OrderId, AmountPence: p.AmountPence})
}

// Refund returns the money for an order.
func (s *Server) Refund(ctx context.Context, in *paymentsv1.RefundRequest) (*paymentsv1.Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.payments[in.OrderId]
	if p == nil {
		return nil, errors.New("no payment for order")
	}
	p.State = paymentsv1.PaymentState_PAYMENT_STATE_REFUNDED
	log.Info(ctx, "refunded", "order_id", in.OrderId, "amount_pence", p.AmountPence)
	return p, nil
}
