// Package server implements ordersv1.OrderServiceServer.
package server

import (
	"context"
	"errors"

	"fleet.example/orders/internal/store"
	"fleet.example/platform-go/bus"
	"fleet.example/platform-go/log"
	eventsv1 "fleet.example/proto/events/v1"
	ordersv1 "fleet.example/proto/orders/v1"
	paymentsv1 "fleet.example/proto/payments/v1"
	pricingv1 "fleet.example/proto/pricing/v1"
	restaurantsv1 "fleet.example/proto/restaurants/v1"
)

// Server serves the order API.
type Server struct {
	Store       *store.Store
	Bus         bus.Bus
	Pricing     pricingv1.PricingServiceClient
	Payments    paymentsv1.PaymentServiceClient
	Restaurants restaurantsv1.RestaurantServiceClient
}

var ErrClosed = errors.New("restaurant is closed")

func (s *Server) GetOrder(ctx context.Context, in *ordersv1.GetOrderRequest) (*ordersv1.Order, error) {
	return s.Store.Get(in.Id)
}

// PlaceOrder prices the basket, authorises payment and publishes OrderPlaced.
func (s *Server) PlaceOrder(ctx context.Context, in *ordersv1.PlaceOrderRequest) (*ordersv1.Order, error) {
	r, err := s.Restaurants.GetRestaurant(ctx, &restaurantsv1.GetRestaurantRequest{Id: in.RestaurantId})
	if err != nil {
		return nil, err
	}
	if !r.Open {
		return nil, ErrClosed
	}
	items := make([]*pricingv1.QuoteItem, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, &pricingv1.QuoteItem{MenuItemId: it.MenuItemId, Quantity: it.Quantity, UnitPricePence: it.UnitPricePence})
	}
	q, err := s.Pricing.Quote(ctx, &pricingv1.QuoteRequest{RestaurantId: in.RestaurantId, Items: items})
	if err != nil {
		return nil, err
	}
	o := s.Store.Create(in.RestaurantId, in.CustomerId, in.Items, q.TotalPence, q.DeliveryFeePence)
	if _, err := s.Payments.Authorize(ctx, &paymentsv1.AuthorizeRequest{OrderId: o.Id, AmountPence: o.TotalPence, PaymentMethodId: in.PaymentMethodId}); err != nil {
		return nil, err
	}
	log.Info(ctx, "order placed", "order_id", o.Id, "customer_id", o.CustomerId)
	return o, s.Bus.Publish(ctx, eventsv1.TopicOrderPlaced, &eventsv1.OrderPlaced{
		OrderId: o.Id, RestaurantId: o.RestaurantId, CustomerId: o.CustomerId, TotalPence: o.TotalPence,
	})
}

// CancelOrder marks the order cancelled and publishes OrderCancelled; payments refunds on that event.
func (s *Server) CancelOrder(ctx context.Context, in *ordersv1.CancelOrderRequest) (*ordersv1.Order, error) {
	o, err := s.Store.SetStatus(in.Id, ordersv1.OrderStatus_ORDER_STATUS_CANCELLED)
	if err != nil {
		return nil, err
	}
	return o, s.Bus.Publish(ctx, eventsv1.TopicOrderCancelled, &eventsv1.OrderCancelled{OrderId: o.Id, Reason: in.Reason})
}
