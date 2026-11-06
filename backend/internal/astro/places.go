package astro

import "github.com/Jihed-khelifi/staryield/backend/internal/astro/geo"

// Place is a gazetteer entry (see package geo).
type Place = geo.Place

// PlaceByID looks up a birthplace by gazetteer ID ("gn:2988507") or by a slug
// of the original city list ("paris-fr"), which resolves to the matching
// GeoNames place.
func PlaceByID(id string) (Place, bool) { return geo.ByID(id) }

// LocationOf returns a place's coordinates.
func LocationOf(p Place) GeoLocation {
	return GeoLocation{Latitude: p.Latitude, Longitude: p.Longitude}
}
