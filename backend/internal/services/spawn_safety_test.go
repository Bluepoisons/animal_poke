package services

import "testing"

func TestIsInsideUnsafeZone_Road(t *testing.T) {
	road := DefaultUnsafeZones[0]
	ok, typ := IsInsideUnsafeZone(road.Lat, road.Lng, DefaultUnsafeZones)
	if !ok || typ != ZoneRoad {
		t.Fatalf("expected road zone, got ok=%v type=%s", ok, typ)
	}
}

func TestFilterSafeSpawnPoints(t *testing.T) {
	road := DefaultUnsafeZones[0]
	safe := SpawnPoint{Lat: 29.875, Lng: 121.555}
	out := FilterSafeSpawnPoints([]SpawnPoint{
		{Lat: road.Lat, Lng: road.Lng},
		safe,
	}, DefaultUnsafeZones)
	if len(out) != 1 {
		t.Fatalf("expected 1 safe point, got %d", len(out))
	}
	if out[0].Lat != safe.Lat || out[0].Lng != safe.Lng {
		t.Fatalf("unexpected safe point: %+v", out[0])
	}
}

func TestCanMarkInRange_Accuracy(t *testing.T) {
	if !CanMarkInRange(20, 50, 12) {
		t.Fatal("should allow good accuracy")
	}
	if CanMarkInRange(20, 50, 80) {
		t.Fatal("should block poor accuracy")
	}
	if CanMarkInRange(60, 50, 12) {
		t.Fatal("should block out of range")
	}
}

func TestAllZoneTypesCovered(t *testing.T) {
	types := map[UnsafeZoneType]bool{}
	for _, z := range DefaultUnsafeZones {
		types[z.Type] = true
	}
	for _, want := range []UnsafeZoneType{ZoneRoad, ZoneWater, ZoneConstruction, ZonePrivate, ZoneUnreachable} {
		if !types[want] {
			t.Fatalf("missing zone type %s", want)
		}
	}
}
