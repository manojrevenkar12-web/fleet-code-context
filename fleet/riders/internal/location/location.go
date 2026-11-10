// Package location records rider positions.
package location

import (
	"fmt"
	"time"

	ridersv1 "fleet.example/proto/riders/v1"
)

type Store struct{ now func() time.Time }

func New() *Store { return &Store{now: time.Now} }

// Update moves a rider and stamps the time of the update.
func (s *Store) Update(r *ridersv1.Rider, lat, lng float64) {
	r.Lat, r.Lng = lat, lng
	r.LocationUpdatedAtUnix = s.now().Unix()
	fmt.Println("rider location updated", r.Id, lat, lng)
}
