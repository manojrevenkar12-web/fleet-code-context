package server

import (
	"context"

	dispatchv1 "fleet.example/proto/dispatch/v1"
	ordersv1 "fleet.example/proto/orders/v1"
)

// Tracker answers TrackOrder by asking dispatch for the latest ETA.
type Tracker struct {
	*Server
	Dispatch dispatchv1.DispatchServiceClient
}

func (t *Tracker) TrackOrder(ctx context.Context, in *ordersv1.TrackOrderRequest) (*ordersv1.Tracking, error) {
	o, err := t.Store.Get(in.Id)
	if err != nil {
		return nil, err
	}
	eta, err := t.Dispatch.GetEta(ctx, &dispatchv1.GetEtaRequest{OrderId: in.Id})
	if err != nil {
		return nil, err
	}
	return &ordersv1.Tracking{OrderId: o.Id, Status: o.Status, EtaSeconds: eta.EtaSeconds}, nil
}
