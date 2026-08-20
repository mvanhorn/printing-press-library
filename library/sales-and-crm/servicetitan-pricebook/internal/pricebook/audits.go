package pricebook

import (
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// ----- vendor-part-gaps -------------------------------------------------

// VendorPartGap is one SKU missing a primary vendor part number — a hole in
// JKA's "always sync the 2M Part #" discipline.
type VendorPartGap struct {
	Kind              SKUKind `json:"kind"`
	ID                int64   `json:"id"`
	Code              string  `json:"code"`
	DisplayName       string  `json:"display_name"`
	PrimaryVendorName string  `json:"primary_vendor_name"`
	Reason            string  `json:"reason"` // "no-primary-vendor" | "blank-vendor-part"
}

// VendorPartGaps returns active materials and equipment whose primary vendor
// has no part number (or that have no primary vendor at all). kind filters
// to "material" or "equipment"; empty kind returns both. The ServiceTitan
// API has no "where vendorPart is empty" query — this is a local null-scan.
func VendorPartGaps(db *store.Store, kind SKUKind) ([]VendorPartGap, error) {
	var out []VendorPartGap
	check := func(k SKUKind, id int64, code, name string, active bool, pv *SkuVendor) {
		if !active {
			return
		}
		switch {
		case pv == nil:
			out = append(out, VendorPartGap{Kind: k, ID: id, Code: code, DisplayName: name, Reason: "no-primary-vendor"})
		case strings.TrimSpace(pv.VendorPart) == "":
			out = append(out, VendorPartGap{Kind: k, ID: id, Code: code, DisplayName: name,
				PrimaryVendorName: pv.VendorName, Reason: "blank-vendor-part"})
		}
	}
	if kind == "" || kind == KindMaterial {
		mats, err := LoadMaterials(db)
		if err != nil {
			return nil, err
		}
		for _, m := range mats {
			check(KindMaterial, m.ID, m.Code, m.DisplayName, m.Active, m.PrimaryVendor)
		}
	}
	if kind == "" || kind == KindEquipment {
		eqs, err := LoadEquipment(db)
		if err != nil {
			return nil, err
		}
		for _, e := range eqs {
			check(KindEquipment, e.ID, e.Code, e.DisplayName, e.Active, e.PrimaryVendor)
		}
	}
	return out, nil
}

// ----- warranty-lint ----------------------------------------------------

// WarrantyIssue is one equipment SKU whose warranty text breaks JKA's
// attribution rules.
type WarrantyIssue struct {
	ID          int64    `json:"id"`
	Code        string   `json:"code"`
	DisplayName string   `json:"display_name"`
	Problems    []string `json:"problems"`
}

// manufacturerPrefixes are the accepted leading forms for a manufacturer
// warranty description. JKA's rule (auto-memory feedback_warranty_attribution)
// is that manufacturer warranties must be clearly attributed so it reads as
// "not JKA's warranty".
var manufacturerPrefixes = []string{"manufacturer's", "manufacturers", "manufacturer "}

// WarrantyLint flags active equipment whose warranty text breaks JKA rules:
//   - a manufacturerWarranty whose description is set but does not lead with
//     "Manufacturer's" (attribution rule)
//   - a manufacturerWarranty with a duration but a blank description
//   - no serviceProviderWarranty, or one with zero duration (JKA's standard
//     offering is 1-year parts & labor and should be recorded)
//
// This is a service-specific content lint — no API call returns it.
func WarrantyLint(db *store.Store) ([]WarrantyIssue, error) {
	eqs, err := LoadEquipment(db)
	if err != nil {
		return nil, err
	}
	var out []WarrantyIssue
	for _, e := range eqs {
		if !e.Active {
			continue
		}
		var problems []string
		mw := e.ManufacturerWarranty
		if mw != nil {
			desc := strings.TrimSpace(mw.Description)
			switch {
			case desc == "" && mw.Duration > 0:
				problems = append(problems, "manufacturer warranty has a duration but no description")
			case desc != "" && !hasManufacturerPrefix(desc):
				problems = append(problems, "manufacturer warranty description is not prefixed \"Manufacturer's\"")
			}
		}
		spw := e.ServiceProviderWarranty
		if spw == nil || spw.Duration == 0 {
			problems = append(problems, "no JKA service-provider warranty recorded (standard offering is 1-year parts & labor)")
		}
		if len(problems) > 0 {
			out = append(out, WarrantyIssue{ID: e.ID, Code: e.Code, DisplayName: e.DisplayName, Problems: problems})
		}
	}
	return out, nil
}

func hasManufacturerPrefix(desc string) bool {
	low := strings.ToLower(strings.TrimSpace(desc))
	for _, p := range manufacturerPrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// ----- orphan-skus ------------------------------------------------------

// OrphanSKU is one SKU pointing at a category that is inactive or does not
// exist in the synced category set.
type OrphanSKU struct {
	Kind        SKUKind `json:"kind"`
	ID          int64   `json:"id"`
	Code        string  `json:"code"`
	DisplayName string  `json:"display_name"`
	CategoryID  int64   `json:"category_id"`
	Reason      string  `json:"reason"` // "inactive-category" | "missing-category" | "no-category"
}

// OrphanSKUs joins materials, equipment, and services against the synced
// categories and returns SKUs whose category assignment is broken: pointing
// at an inactive category, a category ID that is not in the store, or having
// no category at all. The join is impossible in one ServiceTitan API call.
func OrphanSKUs(db *store.Store) ([]OrphanSKU, error) {
	cats, err := LoadCategories(db)
	if err != nil {
		return nil, err
	}
	var out []OrphanSKU
	classify := func(kind SKUKind, id int64, code, name string, active bool, refs CategoryRefs) {
		if !active {
			return
		}
		if len(refs) == 0 {
			out = append(out, OrphanSKU{Kind: kind, ID: id, Code: code, DisplayName: name, Reason: "no-category"})
			return
		}
		for _, cid := range refs {
			c, ok := cats[cid]
			switch {
			case !ok:
				out = append(out, OrphanSKU{Kind: kind, ID: id, Code: code, DisplayName: name, CategoryID: cid, Reason: "missing-category"})
			case !c.Active:
				out = append(out, OrphanSKU{Kind: kind, ID: id, Code: code, DisplayName: name, CategoryID: cid, Reason: "inactive-category"})
			}
		}
	}
	mats, err := LoadMaterials(db)
	if err != nil {
		return nil, err
	}
	for _, m := range mats {
		classify(KindMaterial, m.ID, m.Code, m.DisplayName, m.Active, m.Categories)
	}
	eqs, err := LoadEquipment(db)
	if err != nil {
		return nil, err
	}
	for _, e := range eqs {
		classify(KindEquipment, e.ID, e.Code, e.DisplayName, e.Active, e.Categories)
	}
	svcs, err := LoadServices(db)
	if err != nil {
		return nil, err
	}
	for _, s := range svcs {
		classify(KindService, s.ID, s.Code, s.DisplayName, s.Active, s.Categories)
	}
	return out, nil
}

// ----- copy-audit -------------------------------------------------------

// CopyIssue is one SKU whose display name or description is not
// customer-facing sales copy.
type CopyIssue struct {
	Kind        SKUKind  `json:"kind"`
	ID          int64    `json:"id"`
	Code        string   `json:"code"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Problems    []string `json:"problems"`
}

// CopyAudit flags active SKUs whose customer-facing text reads like internal
// shorthand: a blank or very short description, a missing display name, an
// ALL-CAPS display name, or a display name that is just the part code. A
// sales-copy agent rewrites the flagged entries; the writeback path is the
// generated update commands / bulk-plan. kind filters output; empty returns
// all three.
func CopyAudit(db *store.Store, kind SKUKind) ([]CopyIssue, error) {
	var out []CopyIssue
	inspect := func(k SKUKind, id int64, code, name, desc string, active bool) {
		if !active {
			return
		}
		var problems []string
		trimName := strings.TrimSpace(name)
		trimDesc := strings.TrimSpace(desc)
		if trimName == "" {
			problems = append(problems, "no display name")
		} else {
			if strings.EqualFold(trimName, strings.TrimSpace(code)) {
				problems = append(problems, "display name is just the part code")
			}
			if isAllCaps(trimName) {
				problems = append(problems, "display name is ALL-CAPS")
			}
		}
		switch {
		case trimDesc == "":
			problems = append(problems, "no description")
		case len([]rune(trimDesc)) < 15:
			problems = append(problems, "description is too short to be sales copy")
		case looksLikeBarePartNumber(trimDesc):
			problems = append(problems, "description is just a part number")
		}
		if len(problems) > 0 {
			out = append(out, CopyIssue{Kind: k, ID: id, Code: code, DisplayName: name, Description: desc, Problems: problems})
		}
	}
	if kind == "" || kind == KindMaterial {
		mats, err := LoadMaterials(db)
		if err != nil {
			return nil, err
		}
		for _, m := range mats {
			inspect(KindMaterial, m.ID, m.Code, m.DisplayName, m.Description, m.Active)
		}
	}
	if kind == "" || kind == KindEquipment {
		eqs, err := LoadEquipment(db)
		if err != nil {
			return nil, err
		}
		for _, e := range eqs {
			inspect(KindEquipment, e.ID, e.Code, e.DisplayName, e.Description, e.Active)
		}
	}
	if kind == "" || kind == KindService {
		svcs, err := LoadServices(db)
		if err != nil {
			return nil, err
		}
		for _, s := range svcs {
			inspect(KindService, s.ID, s.Code, s.DisplayName, s.Description, s.Active)
		}
	}
	return out, nil
}

// isAllCaps reports whether s has letters and every letter is upper-case.
// Short codes are exempt — a 3-letter code is not "shouty copy".
func isAllCaps(s string) bool {
	letters, upper := 0, 0
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			letters++
		} else if r >= 'A' && r <= 'Z' {
			letters++
			upper++
		}
	}
	return letters >= 4 && upper == letters
}

// looksLikeBarePartNumber reports whether s is a single token of mostly
// digits and dashes — i.e. a part number pasted into the description field.
func looksLikeBarePartNumber(s string) bool {
	if strings.ContainsAny(s, " \t") {
		return false
	}
	digits, total := 0, 0
	for _, r := range s {
		total++
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return total > 0 && float64(digits)/float64(total) >= 0.4
}

// ----- health -----------------------------------------------------------

// HealthReport is the one-shot agent-priming rollup of every pricebook
// audit count, plus store/snapshot status.
type HealthReport struct {
	Materials         int `json:"materials"`
	Equipment         int `json:"equipment"`
	Services          int `json:"services"`
	Categories        int `json:"categories"`
	MarkupTiers       int `json:"markup_tiers"`
	MarkupDrift       int `json:"markup_drift"`
	VendorPartGaps    int `json:"vendor_part_gaps"`
	WarrantyIssues    int `json:"warranty_issues"`
	OrphanSKUs        int `json:"orphan_skus"`
	CopyIssues        int `json:"copy_issues"`
	DuplicateClusters int `json:"duplicate_clusters"`
	CostDriftSKUs     int `json:"cost_drift_skus"`
	CostHistoryRows   int `json:"cost_history_rows"`
}

// Health aggregates every audit into one compact rollup sized for agent
// priming. markupTolerancePct and dedupeMinScore tune the two audits that
// take a threshold; since scopes the cost-drift count. It calls Snapshot
// first so the cost-drift count reflects the current pricebook state.
func Health(db *store.Store, markupTolerancePct, dedupeMinScore float64, since string) (HealthReport, error) {
	var h HealthReport

	mats, err := LoadMaterials(db)
	if err != nil {
		return h, err
	}
	h.Materials = len(mats)
	eqs, err := LoadEquipment(db)
	if err != nil {
		return h, err
	}
	h.Equipment = len(eqs)
	svcs, err := LoadServices(db)
	if err != nil {
		return h, err
	}
	h.Services = len(svcs)
	cats, err := LoadCategories(db)
	if err != nil {
		return h, err
	}
	h.Categories = len(cats)
	ladder, err := LoadMarkupLadder(db)
	if err != nil {
		return h, err
	}
	h.MarkupTiers = len(ladder)

	drift, err := MarkupAudit(db, markupTolerancePct)
	if err != nil {
		return h, err
	}
	h.MarkupDrift = len(drift)

	gaps, err := VendorPartGaps(db, "")
	if err != nil {
		return h, err
	}
	h.VendorPartGaps = len(gaps)

	warn, err := WarrantyLint(db)
	if err != nil {
		return h, err
	}
	h.WarrantyIssues = len(warn)

	orphans, err := OrphanSKUs(db)
	if err != nil {
		return h, err
	}
	// PATCH: orphan-sku-health-dedup (Greptile PR #576). OrphanSKUs emits one
	// row per (SKU, bad-ref) so the orphan-skus command shows every bad
	// category reference, but the health rollup is the agent-priming signal
	// and should count distinct SKUs needing attention — not bad-ref pairs.
	type orphanKey struct {
		kind SKUKind
		id   int64
	}
	unique := make(map[orphanKey]struct{}, len(orphans))
	for _, o := range orphans {
		unique[orphanKey{o.Kind, o.ID}] = struct{}{}
	}
	h.OrphanSKUs = len(unique)

	copyIssues, err := CopyAudit(db, "")
	if err != nil {
		return h, err
	}
	h.CopyIssues = len(copyIssues)

	dupes, err := Dedupe(db, "", dedupeMinScore)
	if err != nil {
		return h, err
	}
	h.DuplicateClusters = len(dupes)

	if _, _, err := Snapshot(db); err != nil {
		return h, err
	}
	cd, err := CostDrift(db, since)
	if err != nil {
		return h, err
	}
	h.CostDriftSKUs = len(cd)
	rows, err := CostHistoryRows(db)
	if err != nil {
		return h, err
	}
	h.CostHistoryRows = rows

	return h, nil
}

// SortDriftRows orders cost-drift findings by absolute cost delta descending
// (biggest movers first), then code, so output is deterministic and the most
// material changes lead.
func SortDriftRows(rows []DriftRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ai, aj := absf(rows[i].CostDelta), absf(rows[j].CostDelta)
		if ai != aj {
			return ai > aj
		}
		return rows[i].Code < rows[j].Code
	})
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
