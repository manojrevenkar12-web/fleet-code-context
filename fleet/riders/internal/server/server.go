// Package server implements ridersv1.RiderServiceServer.
package server

import (
	"context"

	"fleet.example/riders/internal/location"
	ridersv1 "fleet.example/proto/riders/v1"
)

type Server struct {
	Riders    map[string]*ridersv1.Rider
	Locations *location.Store
}

func (s *Server) GetRider(ctx context.Context, in *ridersv1.GetRiderRequest) (*ridersv1.Rider, error) {
	return s.Riders[in.Id], nil
}

func (s *Server) ListAvailableRiders(ctx context.Context, in *ridersv1.ListAvailableRidersRequest) (*ridersv1.RiderList, error) {
	out := &ridersv1.RiderList{}
	for id := range s.Riders {
		out.RiderIds = append(out.RiderIds, id)
	}
	return out, nil
}

func (s *Server) UpdateLocation(ctx context.Context, in *ridersv1.UpdateLocationRequest) (*ridersv1.Rider, error) {
	r := s.Riders[in.Id]
	s.Locations.Update(r, in.Lat, in.Lng)
	return r, nil
}
