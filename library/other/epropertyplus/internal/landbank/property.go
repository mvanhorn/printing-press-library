// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Package landbank holds the pure-logic domain helpers for the hand-built
// novel commands: property classification (structure vs vacant lot), GeoJSON
// feature construction, and instance-registry resolution. Keeping these free
// of cobra/client/store dependencies makes them table-testable.
package landbank

import (
	"encoding/json"
	"strings"
)

// Property is the subset of the ePropertyPlus getPublishedProperty returnVal
// the novel commands project. Unknown fields are ignored on unmarshal; the
// full raw record is preserved separately by the store.
type Property struct {
	ID               json.Number `json:"id"`
	ParcelNumber     string      `json:"parcelNumber"`
	PropertyAddress1 string      `json:"propertyAddress1"`
	City             string      `json:"city"`
	State            string      `json:"state"`
	PostalCode       string      `json:"postalCode"`
	County           string      `json:"county"`
	Neighborhood     string      `json:"neighborhood"`
	Latitude         *float64    `json:"latitude"`
	Longitude        *float64    `json:"longitude"`
	PropertyClass    string      `json:"propertyClass"`
	StructureType    string      `json:"structureType"`
	InventoryType    string      `json:"inventoryType"`
	// Occupied is `any` because the API returns this field inconsistently as a
	// bool, an empty string, or null depending on the record. A typed *bool
	// would fail to unmarshal the string form and silently drop the whole
	// property. No command logic depends on this field.
	Occupied            any      `json:"occupied"`
	ZonedAs             string   `json:"zonedAs"`
	PotentialUse        string   `json:"potentialUse"`
	AskingPrice         *float64 `json:"askingPrice"`
	CurrentAssessment   *float64 `json:"currentAssessment"`
	SquareFootage       *float64 `json:"squareFootage"`
	StructureSquareFoot *float64 `json:"structureSquareFootage"`
	ParcelSquareFootage *float64 `json:"parcelSquareFootage"`
	ParcelLength        *float64 `json:"parcelLength"`
	ParcelWidth         *float64 `json:"parcelWidth"`
	Bedrooms            *float64 `json:"bedrooms"`
	SchoolDistrict      string   `json:"schoolDistrict"`
	ForeclosureYear     *float64 `json:"foreclosureYear"`
	ThumbImgURL         string   `json:"thumbImgUrl"`
	PublicThumbImgURL   string   `json:"publicThumbImgUrl"`
}

// Kind is the structure/lot classification of a property.
type Kind string

const (
	KindStructure Kind = "structure"
	KindLot       Kind = "lot"
)

// Classify returns whether a property is a structure (building) or a vacant
// lot. Rule (matches CLAUDE.md / task spec exactly):
//
//	structure = structureType non-empty
//	            OR propertyClass does NOT contain "Vacant" (case-insensitive)
//	lot       = otherwise
//
// This is a deliberately permissive structure rule: a property with no
// propertyClass at all is a structure because an empty class string does not
// contain "Vacant".
func Classify(structureType, propertyClass string) Kind {
	if strings.TrimSpace(structureType) != "" {
		return KindStructure
	}
	if !strings.Contains(strings.ToLower(propertyClass), "vacant") {
		return KindStructure
	}
	return KindLot
}

// Kind classifies the receiver.
func (p Property) Kind() Kind {
	return Classify(p.StructureType, p.PropertyClass)
}

// IsStructure reports whether the property classifies as a structure.
func (p Property) IsStructure() bool { return p.Kind() == KindStructure }

// IsLot reports whether the property classifies as a vacant lot.
func (p Property) IsLot() bool { return p.Kind() == KindLot }

// MatchesKind reports whether the property matches the requested kind filter.
// kind "all" (or "") matches everything.
func (p Property) MatchesKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "all":
		return true
	case "structure":
		return p.IsStructure()
	case "lot":
		return p.IsLot()
	default:
		return false
	}
}

// IndexRow is a lightweight row from searchSummaryPublicMapQuery.
type IndexRow struct {
	ID  json.Number `json:"id"`
	Lat *float64    `json:"lt"`
	Lng *float64    `json:"lg"`
	S   any         `json:"s"`
	C   any         `json:"c"`
}

// IndexResponse is the top-level shape of the index endpoint.
type IndexResponse struct {
	Success bool       `json:"success"`
	Size    int        `json:"size"`
	Rows    []IndexRow `json:"rows"`
}

// DetailResponse is the top-level shape of getPublishedProperty.
type DetailResponse struct {
	Success   bool            `json:"success"`
	ReturnVal json.RawMessage `json:"returnVal"`
}

// ParseIndex decodes the index endpoint response into rows.
func ParseIndex(data []byte) (*IndexResponse, error) {
	var r IndexResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ParseDetail decodes a getPublishedProperty response and returns the inner
// returnVal raw JSON (the full property record). When the payload is already
// a bare property object (no envelope), it is returned as-is.
func ParseDetail(data []byte) (json.RawMessage, error) {
	var r DetailResponse
	if err := json.Unmarshal(data, &r); err == nil && r.ReturnVal != nil {
		return r.ReturnVal, nil
	}
	// Fallback: maybe the caller already handed us the inner object.
	return json.RawMessage(data), nil
}

// ParseProperty decodes a full property record (returnVal) into a Property.
func ParseProperty(data []byte) (Property, error) {
	var p Property
	err := json.Unmarshal(data, &p)
	return p, err
}
