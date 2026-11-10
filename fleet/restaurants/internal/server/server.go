// Package server implements restaurantsv1.RestaurantServiceServer.
package server

import (
	"context"

	"fleet.example/restaurants/internal/menu"
	restaurantsv1 "fleet.example/proto/restaurants/v1"
)

type Server struct {
	Restaurants map[string]*restaurantsv1.Restaurant
	Menus       *menu.Store
}

func (s *Server) GetRestaurant(ctx context.Context, in *restaurantsv1.GetRestaurantRequest) (*restaurantsv1.Restaurant, error) {
	return s.Restaurants[in.Id], nil
}

func (s *Server) GetMenu(ctx context.Context, in *restaurantsv1.GetMenuRequest) (*restaurantsv1.Menu, error) {
	return s.Menus.For(in.RestaurantId), nil
}
