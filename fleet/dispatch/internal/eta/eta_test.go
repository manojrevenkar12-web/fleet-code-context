package eta

import (
	"testing"

	ridersv1 "fleet.example/proto/riders/v1"
)

func TestSecondsSamePoint(t *testing.T) {
	r := &ridersv1.Rider{Lat: 51.5, Lng: -0.1, Vehicle: ridersv1.Vehicle_VEHICLE_BICYCLE}
	if got := Seconds(r, 51.5, -0.1); got != 0 {
		t.Fatalf("got %d", got)
	}
}
