// Package menu stores restaurant menus.
package menu

import restaurantsv1 "fleet.example/proto/restaurants/v1"

type Store struct{ menus map[string]*restaurantsv1.Menu }

func New() *Store { return &Store{menus: map[string]*restaurantsv1.Menu{}} }

// For returns the menu of a restaurant.
func (s *Store) For(restaurantID string) *restaurantsv1.Menu {
	if m, ok := s.menus[restaurantID]; ok {
		return m
	}
	return &restaurantsv1.Menu{RestaurantId: restaurantID}
}

// Put replaces a restaurant's menu.
func (s *Store) Put(m *restaurantsv1.Menu) { s.menus[m.RestaurantId] = m }
