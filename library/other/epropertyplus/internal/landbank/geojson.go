// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

package landbank

import "encoding/json"

// Geometry is a GeoJSON geometry. Only Point is produced here.
type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// Feature is a GeoJSON Feature.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   Geometry       `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

// FeatureCollection is a GeoJSON FeatureCollection.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// NewPointFeature builds a GeoJSON Point feature at [lng, lat] (GeoJSON axis
// order is longitude-first) with the given properties.
func NewPointFeature(lng, lat float64, props map[string]any) Feature {
	if props == nil {
		props = map[string]any{}
	}
	return Feature{
		Type:       "Feature",
		Geometry:   Geometry{Type: "Point", Coordinates: []float64{lng, lat}},
		Properties: props,
	}
}

// BuildFeatureCollection assembles a FeatureCollection from records that each
// expose a lng/lat and a property map. Records whose hasCoords is false are
// skipped (GeoJSON Point geometry requires coordinates).
func BuildFeatureCollection(records []FeatureInput) FeatureCollection {
	fc := FeatureCollection{Type: "FeatureCollection", Features: []Feature{}}
	for _, r := range records {
		if !r.HasCoords {
			continue
		}
		fc.Features = append(fc.Features, NewPointFeature(r.Lng, r.Lat, r.Props))
	}
	return fc
}

// FeatureInput is one row fed into BuildFeatureCollection.
type FeatureInput struct {
	Lng       float64
	Lat       float64
	HasCoords bool
	Props     map[string]any
}

// FeatureInputFromProperty projects a Property into a GeoJSON FeatureInput
// carrying the provided property fields. selected, when non-nil, restricts the
// emitted properties to those keys (case-sensitive against the field names
// produced here); nil means emit the default field set.
func FeatureInputFromProperty(p Property, selected map[string]bool) FeatureInput {
	props := p.GeoProps()
	if selected != nil {
		filtered := map[string]any{}
		for k, v := range props {
			if selected[k] {
				filtered[k] = v
			}
		}
		props = filtered
	}
	in := FeatureInput{Props: props}
	if p.Latitude != nil && p.Longitude != nil {
		in.Lat = *p.Latitude
		in.Lng = *p.Longitude
		in.HasCoords = true
	}
	return in
}

// GeoProps returns the default property map emitted for a Property feature.
func (p Property) GeoProps() map[string]any {
	m := map[string]any{
		"id":            p.ID.String(),
		"parcelNumber":  p.ParcelNumber,
		"address":       p.PropertyAddress1,
		"city":          p.City,
		"propertyClass": p.PropertyClass,
		"structureType": p.StructureType,
		"kind":          string(p.Kind()),
		"zonedAs":       p.ZonedAs,
		"potentialUse":  p.PotentialUse,
	}
	if p.AskingPrice != nil {
		m["askingPrice"] = *p.AskingPrice
	}
	if p.ParcelSquareFootage != nil {
		m["parcelSquareFootage"] = *p.ParcelSquareFootage
	}
	return m
}

// MarshalFeatureCollection serializes a FeatureCollection to indented JSON.
func MarshalFeatureCollection(fc FeatureCollection) ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}
