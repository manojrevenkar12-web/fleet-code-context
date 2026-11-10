// Package eta estimates rider arrival times.
package eta

import (
	"math"

	ridersv1 "fleet.example/proto/riders/v1"
)

// metres per second by vehicle
var speed = map[ridersv1.Vehicle]float64{
	ridersv1.Vehicle_VEHICLE_BICYCLE: 4.5,
	ridersv1.Vehicle_VEHICLE_SCOOTER: 7.0,
	ridersv1.Vehicle_VEHICLE_CAR:     8.5,
}

// Seconds estimates travel time in seconds from the rider to a point.
func Seconds(r *ridersv1.Rider, lat, lng float64) int32 {
	d := distanceMetres(r.Lat, r.Lng, lat, lng)
	v := speed[r.Vehicle]
	if v == 0 {
		v = 4.5
	}
	return int32(math.Ceil(d / v))
}

func distanceMetres(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371000.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := (lat2-lat1)*math.Pi/180, (lng2-lng1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
