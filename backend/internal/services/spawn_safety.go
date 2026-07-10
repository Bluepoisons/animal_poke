// Package services: outdoor spawn safety heuristics (AP-045).
package services

import "math"

// UnsafeZoneType labels exclusion reasons for spawn points.
type UnsafeZoneType string

const (
	ZoneRoad         UnsafeZoneType = "road"
	ZoneWater        UnsafeZoneType = "water"
	ZoneConstruction UnsafeZoneType = "construction"
	ZonePrivate      UnsafeZoneType = "private"
	ZoneUnreachable  UnsafeZoneType = "unreachable"
)

// UnsafeZone is a circular exclusion around a demo/heuristic center.
type UnsafeZone struct {
	Type    UnsafeZoneType
	Lat     float64
	Lng     float64
	RadiusM float64
	Label   string
}

// DefaultUnsafeZones mirrors frontend outdoorSafety heuristics (Ningbo demo grid).
var DefaultUnsafeZones = []UnsafeZone{
	{Type: ZoneRoad, Lat: 29.8705, Lng: 121.5505, RadiusM: 25, Label: "main_road"},
	{Type: ZoneWater, Lat: 29.8680, Lng: 121.5480, RadiusM: 80, Label: "river"},
	{Type: ZoneConstruction, Lat: 29.8720, Lng: 121.5520, RadiusM: 40, Label: "site_a"},
	{Type: ZonePrivate, Lat: 29.8710, Lng: 121.5490, RadiusM: 30, Label: "compound"},
	{Type: ZoneUnreachable, Lat: 29.8690, Lng: 121.5530, RadiusM: 35, Label: "cliff"},
}

// SpawnPoint is a candidate discovery location.
type SpawnPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// DistanceMeters plane approximation (same as frontend LBS).
func DistanceMeters(aLat, aLng, bLat, bLng float64) float64 {
	latPerMeter := 1.0 / 111000.0
	lngPerMeter := 1.0 / (111000.0 * math.Cos(aLat*math.Pi/180))
	dLat := (aLat - bLat) / latPerMeter
	dLng := (aLng - bLng) / lngPerMeter
	return math.Hypot(dLat, dLng)
}

// IsInsideUnsafeZone reports whether a point falls in any exclusion zone.
func IsInsideUnsafeZone(lat, lng float64, zones []UnsafeZone) (bool, UnsafeZoneType) {
	if zones == nil {
		zones = DefaultUnsafeZones
	}
	for _, z := range zones {
		if DistanceMeters(lat, lng, z.Lat, z.Lng) <= z.RadiusM {
			return true, z.Type
		}
	}
	return false, ""
}

// FilterSafeSpawnPoints drops candidates on road/water/construction/private/unreachable.
func FilterSafeSpawnPoints(candidates []SpawnPoint, zones []UnsafeZone) []SpawnPoint {
	if zones == nil {
		zones = DefaultUnsafeZones
	}
	out := make([]SpawnPoint, 0, len(candidates))
	for _, c := range candidates {
		if ok, _ := IsInsideUnsafeZone(c.Lat, c.Lng, zones); !ok {
			out = append(out, c)
		}
	}
	return out
}

// MaxAccuracyMeters is the GPS accuracy threshold for in-range (m).
const MaxAccuracyMeters = 50.0

// CanMarkInRange requires distance within capture range and accuracy under threshold.
func CanMarkInRange(distanceM, captureRangeM, accuracyM float64) bool {
	if accuracyM > MaxAccuracyMeters {
		return false
	}
	return distanceM <= captureRangeM
}
