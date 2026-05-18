// Package pricebook holds the transcendence-feature data layer for
// servicetitan-pricebook-pp-cli: typed views over the synced ServiceTitan
// Pricebook entities plus the cross-entity audits, cost-history snapshots,
// markup-ladder math, fuzzy matching, and vendor-quote reconciliation that
// the novel commands expose. Nothing here talks to the ServiceTitan API —
// it reads the local SQLite store that `sync` populates. The one exception
// is bulk-plan/reprice payload assembly, which only builds request bodies;
// the generated client still does the writing.
package pricebook

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// Store resource_type keys. These MUST match the strings the generated
// get-list commands and the patched sync registry use, so every layer
// reads and writes the same rows.
const (
	ResMaterials  = "materials"
	ResEquipment  = "equipment"
	ResServices   = "services"
	ResCategories = "categories"
	ResMarkup     = "materialsmarkup"
	ResDiscounts  = "discounts-and-fees"
	ResRateSheets = "clientspecificpricing"
)

// CategoryRefs is the `categories` array on a SKU. ServiceTitan returns it
// as a bare int array (`[30105]`), but older/exported payloads sometimes
// use `[{"id":30105}]`; UnmarshalJSON accepts either so one odd row never
// fails the whole load.
type CategoryRefs []int64

func (c *CategoryRefs) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*c = nil
		return nil
	}
	var ints []int64
	if err := json.Unmarshal(b, &ints); err == nil {
		*c = ints
		return nil
	}
	var objs []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(b, &objs); err != nil {
		return err
	}
	out := make(CategoryRefs, 0, len(objs))
	for _, o := range objs {
		out = append(out, o.ID)
	}
	*c = out
	return nil
}

// SkuVendor mirrors Pricebook.V2.SkuVendorResponse — the per-vendor cost and
// part-number record attached to a material or equipment SKU. VendorPart is
// the field JKA's "always sync the 2M Part #" discipline lives in.
type SkuVendor struct {
	ID         int64   `json:"id"`
	VendorName string  `json:"vendorName"`
	VendorID   int64   `json:"vendorId"`
	VendorPart string  `json:"vendorPart"`
	Cost       float64 `json:"cost"`
	Active     bool    `json:"active"`
}

// Warranty mirrors Pricebook.V2.SkuWarrantyResponse.
type Warranty struct {
	Duration    int    `json:"duration"`
	Description string `json:"description"`
}

// Material mirrors the subset of Pricebook.V2.MaterialResponse the audit
// commands need.
type Material struct {
	ID            int64        `json:"id"`
	Code          string       `json:"code"`
	DisplayName   string       `json:"displayName"`
	Description   string       `json:"description"`
	Cost          float64      `json:"cost"`
	Price         float64      `json:"price"`
	Active        bool         `json:"active"`
	PrimaryVendor *SkuVendor   `json:"primaryVendor"`
	OtherVendors  []SkuVendor  `json:"otherVendors"`
	Categories    CategoryRefs `json:"categories"`
	ModifiedOn    string       `json:"modifiedOn"`
}

// Equipment mirrors the subset of Pricebook.V2.EquipmentResponse the audit
// commands need.
type Equipment struct {
	ID                      int64        `json:"id"`
	Code                    string       `json:"code"`
	DisplayName             string       `json:"displayName"`
	Description             string       `json:"description"`
	Cost                    float64      `json:"cost"`
	Price                   float64      `json:"price"`
	Active                  bool         `json:"active"`
	Manufacturer            string       `json:"manufacturer"`
	Model                   string       `json:"model"`
	ManufacturerWarranty    *Warranty    `json:"manufacturerWarranty"`
	ServiceProviderWarranty *Warranty    `json:"serviceProviderWarranty"`
	PrimaryVendor           *SkuVendor   `json:"primaryVendor"`
	OtherVendors            []SkuVendor  `json:"otherVendors"`
	Categories              CategoryRefs `json:"categories"`
	ModifiedOn              string       `json:"modifiedOn"`
}

// Service mirrors the subset of Pricebook.V2.ServiceResponse the audit
// commands need.
type Service struct {
	ID          int64        `json:"id"`
	Code        string       `json:"code"`
	DisplayName string       `json:"displayName"`
	Description string       `json:"description"`
	Price       float64      `json:"price"`
	Active      bool         `json:"active"`
	Categories  CategoryRefs `json:"categories"`
	ModifiedOn  string       `json:"modifiedOn"`
}

// Category mirrors Pricebook.V2.CategoryResponse.
type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
	ParentID    *int64 `json:"parentId"`
	Description string `json:"description"`
}

// MarkupTier mirrors Pricebook.V2.MaterialsMarkupResponse — one rung of the
// cost-to-markup ladder. A cost in [From, To] is marked up by Percent.
type MarkupTier struct {
	ID      int64   `json:"id"`
	From    float64 `json:"from"`
	To      float64 `json:"to"`
	Percent float64 `json:"percent"`
}

// SKUKind is "material", "equipment", or "service" — the discriminator the
// audit commands use for --kind filtering and output labelling.
type SKUKind string

const (
	KindMaterial  SKUKind = "material"
	KindEquipment SKUKind = "equipment"
	KindService   SKUKind = "service"
)

// loadRaw returns every stored JSON blob for a resource type. Unlike
// store.List it does not cap at 200 rows — the audits need the whole
// pricebook.
func loadRaw(db *store.Store, resourceType string) ([]json.RawMessage, error) {
	// ORDER BY id keeps every audit's output stable across runs so dogfood
	// diffs and the agentic output review see deterministic results.
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ? ORDER BY id`, resourceType)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", resourceType, err)
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan %s: %w", resourceType, err)
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}

// LoadMaterials returns every synced material. A row that fails to
// unmarshal is skipped rather than failing the whole load — one malformed
// blob should not blind an audit to the rest of the pricebook.
func LoadMaterials(db *store.Store) ([]Material, error) {
	raw, err := loadRaw(db, ResMaterials)
	if err != nil {
		return nil, err
	}
	out := make([]Material, 0, len(raw))
	for _, r := range raw {
		var m Material
		if json.Unmarshal(r, &m) == nil && m.ID != 0 {
			out = append(out, m)
		}
	}
	return out, nil
}

// LoadEquipment returns every synced equipment SKU.
func LoadEquipment(db *store.Store) ([]Equipment, error) {
	raw, err := loadRaw(db, ResEquipment)
	if err != nil {
		return nil, err
	}
	out := make([]Equipment, 0, len(raw))
	for _, r := range raw {
		var e Equipment
		if json.Unmarshal(r, &e) == nil && e.ID != 0 {
			out = append(out, e)
		}
	}
	return out, nil
}

// LoadServices returns every synced service.
func LoadServices(db *store.Store) ([]Service, error) {
	raw, err := loadRaw(db, ResServices)
	if err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(raw))
	for _, r := range raw {
		var s Service
		if json.Unmarshal(r, &s) == nil && s.ID != 0 {
			out = append(out, s)
		}
	}
	return out, nil
}

// LoadCategories returns every synced category, keyed by ID for the
// orphan-SKU join.
func LoadCategories(db *store.Store) (map[int64]Category, error) {
	raw, err := loadRaw(db, ResCategories)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]Category, len(raw))
	for _, r := range raw {
		var c Category
		if json.Unmarshal(r, &c) == nil && c.ID != 0 {
			out[c.ID] = c
		}
	}
	return out, nil
}

// LoadMarkupLadder returns the materials-markup tiers sorted ascending by
// From, so EvalTier can walk them in order.
func LoadMarkupLadder(db *store.Store) ([]MarkupTier, error) {
	raw, err := loadRaw(db, ResMarkup)
	if err != nil {
		return nil, err
	}
	out := make([]MarkupTier, 0, len(raw))
	for _, r := range raw {
		var t MarkupTier
		if json.Unmarshal(r, &t) == nil {
			out = append(out, t)
		}
	}
	sortTiers(out)
	return out, nil
}

// StoreEmpty reports whether the local store has no materials, equipment,
// and services — the signal that `sync` has not run yet. Audit commands use
// it to return an honest "run sync first" error instead of empty results.
func StoreEmpty(db *store.Store) (bool, error) {
	for _, rt := range []string{ResMaterials, ResEquipment, ResServices} {
		var n int
		if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, rt).Scan(&n); err != nil {
			return false, fmt.Errorf("counting %s: %w", rt, err)
		}
		if n > 0 {
			return false, nil
		}
	}
	return true, nil
}
