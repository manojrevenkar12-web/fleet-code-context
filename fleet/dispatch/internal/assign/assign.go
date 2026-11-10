// Package assign picks the rider for an order.
package assign

import (
	"context"
	"errors"

	"fleet.example/dispatch/internal/eta"
	"fleet.example/platform-go/bus"
	dispatchv1 "fleet.example/proto/dispatch/v1"
	eventsv1 "fleet.example/proto/events/v1"
	ridersv1 "fleet.example/proto/riders/v1"
)

var ErrNoRider = errors.New("no rider available")

// Assigner picks the closest available rider and publishes RiderAssigned.
type Assigner struct {
	Riders ridersv1.RiderServiceClient
	Bus    bus.Bus
	etas   map[string]int32
}

func (a *Assigner) AssignRider(ctx context.Context, in *dispatchv1.AssignRiderRequest, lat, lng float64) (*dispatchv1.Assignment, error) {
	list, err := a.Riders.ListAvailableRiders(ctx, &ridersv1.ListAvailableRidersRequest{Lat: lat, Lng: lng})
	if err != nil {
		return nil, err
	}
	var best *ridersv1.Rider
	var bestEta int32
	for _, id := range list.RiderIds {
		r, err := a.Riders.GetRider(ctx, &ridersv1.GetRiderRequest{Id: id})
		if err != nil {
			continue
		}
		e := eta.Seconds(r, lat, lng)
		if best == nil || e < bestEta {
			best, bestEta = r, e
		}
	}
	if best == nil {
		return nil, ErrNoRider
	}
	if a.etas == nil {
		a.etas = map[string]int32{}
	}
	a.etas[in.OrderId] = bestEta
	asg := &dispatchv1.Assignment{OrderId: in.OrderId, RiderId: best.Id, EtaSeconds: bestEta}
	return asg, a.Bus.Publish(ctx, eventsv1.TopicRiderAssigned, &eventsv1.RiderAssigned{OrderId: in.OrderId, RiderId: best.Id, EtaSeconds: bestEta})
}

// GetEta returns the ETA recorded at assignment time.
func (a *Assigner) GetEta(ctx context.Context, in *dispatchv1.GetEtaRequest) (*dispatchv1.Eta, error) {
	return &dispatchv1.Eta{OrderId: in.OrderId, EtaSeconds: a.etas[in.OrderId]}, nil
}
