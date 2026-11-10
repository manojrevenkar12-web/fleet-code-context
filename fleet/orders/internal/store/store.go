// Package store keeps orders in memory.
package store

import (
	"errors"
	"fmt"
	"sync"

	ordersv1 "fleet.example/proto/orders/v1"
)

var ErrNotFound = errors.New("order not found")

// Store is the order store.
type Store struct {
	mu     sync.Mutex
	orders map[string]*ordersv1.Order
	next   int
}

func New() *Store { return &Store{orders: map[string]*ordersv1.Order{}} }

func (s *Store) Create(restaurantID, customerID string, items []*ordersv1.LineItem, totalPence, deliveryFeePence int64) *ordersv1.Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	o := &ordersv1.Order{Id: fmt.Sprintf("o-%d", s.next), RestaurantId: restaurantID, CustomerId: customerID, Items: items,
		TotalPence: totalPence, DeliveryFeePence: deliveryFeePence, Status: ordersv1.OrderStatus_ORDER_STATUS_PLACED}
	s.orders[o.Id] = o
	return o
}

func (s *Store) Get(id string) (*ordersv1.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}

func (s *Store) SetStatus(id string, st ordersv1.OrderStatus) (*ordersv1.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	o.Status = st
	return o, nil
}
