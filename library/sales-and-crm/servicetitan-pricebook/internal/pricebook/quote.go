package pricebook

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// QuoteLine is one row of a vendor cost document — a quote, order
// confirmation, or invoice. Claude extracts the PDF into this shape; the
// CLI does the deterministic matching and diffing.
type QuoteLine struct {
	VendorPart  string  `json:"vendor_part"`
	Cost        float64 `json:"cost"`
	Description string  `json:"description,omitempty"`
	// LineRef is an optional caller-side identifier (PO line, invoice line)
	// echoed back in the reconcile output for traceability.
	LineRef string `json:"line_ref,omitempty"`
}

// ParseQuoteFile reads a vendor cost file into QuoteLines. format is "csv",
// "json", or "auto" (decide by extension; default csv). CSV expects a header
// row; the vendor-part column may be named vendor_part / vendorpart / part /
// partnumber / sku, the cost column cost / price / unitcost / netcost, and an
// optional description column. JSON expects an array of objects with
// vendor_part|vendorPart and cost keys.
func ParseQuoteFile(path, format string) ([]QuoteLine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading quote file: %w", err)
	}
	switch resolveQuoteFormat(path, format) {
	case "json":
		return parseQuoteJSON(data)
	default:
		return parseQuoteCSV(data)
	}
}

func resolveQuoteFormat(path, format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return "json"
	case "csv":
		return "csv"
	default: // auto
		if strings.EqualFold(filepath.Ext(path), ".json") {
			return "json"
		}
		return "csv"
	}
}

func parseQuoteJSON(data []byte) ([]QuoteLine, error) {
	// Accept either a bare array or {"lines": [...]} so Claude can hand back
	// whichever shape is convenient.
	var lines []QuoteLine
	if err := json.Unmarshal(data, &lines); err == nil && len(lines) > 0 {
		return normalizeQuoteLines(lines), nil
	}
	var wrapped struct {
		Lines []QuoteLine `json:"lines"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("parsing quote JSON: %w", err)
	}
	return normalizeQuoteLines(wrapped.Lines), nil
}

func normalizeQuoteLines(in []QuoteLine) []QuoteLine {
	out := in[:0]
	for _, l := range in {
		l.VendorPart = strings.TrimSpace(l.VendorPart)
		l.Description = strings.TrimSpace(l.Description)
		if l.VendorPart == "" {
			continue // a line with no part number cannot be matched
		}
		out = append(out, l)
	}
	return out
}

var (
	partHeaders = map[string]bool{"vendor_part": true, "vendorpart": true, "part": true, "partnumber": true, "part_number": true, "sku": true, "vendor part": true}
	costHeaders = map[string]bool{"cost": true, "price": true, "unitcost": true, "unit_cost": true, "netcost": true, "net_cost": true, "unit cost": true}
	descHeaders = map[string]bool{"description": true, "desc": true, "name": true}
	refHeaders  = map[string]bool{"line_ref": true, "lineref": true, "ref": true, "line": true}
)

func parseQuoteCSV(data []byte) ([]QuoteLine, error) {
	r := csv.NewReader(strings.NewReader(string(data)))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing quote CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("quote CSV has no data rows (need a header row plus at least one line)")
	}
	header := records[0]
	partIdx, costIdx, descIdx, refIdx := -1, -1, -1, -1
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		switch {
		case partHeaders[key] && partIdx < 0:
			partIdx = i
		case costHeaders[key] && costIdx < 0:
			costIdx = i
		case descHeaders[key] && descIdx < 0:
			descIdx = i
		case refHeaders[key] && refIdx < 0:
			refIdx = i
		}
	}
	if partIdx < 0 || costIdx < 0 {
		return nil, fmt.Errorf("quote CSV header must include a vendor-part column (e.g. vendor_part) and a cost column (e.g. cost); got %v", header)
	}
	var lines []QuoteLine
	for _, rec := range records[1:] {
		if partIdx >= len(rec) || costIdx >= len(rec) {
			continue
		}
		part := strings.TrimSpace(rec[partIdx])
		if part == "" {
			continue
		}
		costStr := strings.TrimSpace(rec[costIdx])
		costStr = strings.TrimPrefix(costStr, "$")
		costStr = strings.ReplaceAll(costStr, ",", "")
		cost, err := strconv.ParseFloat(costStr, 64)
		if err != nil {
			return nil, fmt.Errorf("quote CSV: invalid cost %q for part %q: %w", rec[costIdx], part, err)
		}
		l := QuoteLine{VendorPart: part, Cost: cost}
		if descIdx >= 0 && descIdx < len(rec) {
			l.Description = strings.TrimSpace(rec[descIdx])
		}
		if refIdx >= 0 && refIdx < len(rec) {
			l.LineRef = strings.TrimSpace(rec[refIdx])
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// ReconcileRow is one quote line matched (or not) against the synced
// pricebook.
type ReconcileRow struct {
	VendorPart    string  `json:"vendor_part"`
	LineRef       string  `json:"line_ref,omitempty"`
	Matched       bool    `json:"matched"`
	Kind          SKUKind `json:"kind,omitempty"`
	SKUID         int64   `json:"sku_id,omitempty"`
	Code          string  `json:"code,omitempty"`
	DisplayName   string  `json:"display_name,omitempty"`
	MatchedVendor string  `json:"matched_vendor,omitempty"`
	MatchedField  string  `json:"matched_field,omitempty"` // "primary-vendor" | "other-vendor"
	CurrentCost   float64 `json:"current_cost"`
	QuoteCost     float64 `json:"quote_cost"`
	CostDelta     float64 `json:"cost_delta"`
	Reason        string  `json:"reason,omitempty"` // "no-match" when Matched is false
}

// vendorPartIndex maps a tight-normalized vendor part to the SKU(s) and the
// vendor record it came from.
type vendorPartHit struct {
	kind        SKUKind
	id          int64
	code, name  string
	vendorName  string
	field       string
	currentCost float64
}

// Reconcile matches each quote line against the synced pricebook by vendor
// part number — checking primaryVendor first, then otherVendors — and
// returns the cost diff. CurrentCost is the cost on the matched vendor
// record (the cost ServiceTitan would charge against that vendor), so the
// delta is apples-to-apples. Unmatched lines are returned with Matched
// false and Reason "no-match" so nothing is silently dropped. This is a pure
// local join; it never writes.
func Reconcile(db *store.Store, lines []QuoteLine) ([]ReconcileRow, error) {
	index := make(map[string][]vendorPartHit)
	add := func(kind SKUKind, id int64, code, name string, vendors []SkuVendor, primary *SkuVendor) {
		if primary != nil && strings.TrimSpace(primary.VendorPart) != "" {
			key := NormalizeTight(primary.VendorPart)
			index[key] = append(index[key], vendorPartHit{kind, id, code, name, primary.VendorName, "primary-vendor", primary.Cost})
		}
		for _, v := range vendors {
			if strings.TrimSpace(v.VendorPart) == "" {
				continue
			}
			key := NormalizeTight(v.VendorPart)
			index[key] = append(index[key], vendorPartHit{kind, id, code, name, v.VendorName, "other-vendor", v.Cost})
		}
	}

	mats, err := LoadMaterials(db)
	if err != nil {
		return nil, err
	}
	for _, m := range mats {
		add(KindMaterial, m.ID, m.Code, m.DisplayName, m.OtherVendors, m.PrimaryVendor)
	}
	eqs, err := LoadEquipment(db)
	if err != nil {
		return nil, err
	}
	for _, e := range eqs {
		add(KindEquipment, e.ID, e.Code, e.DisplayName, e.OtherVendors, e.PrimaryVendor)
	}

	var out []ReconcileRow
	for _, l := range lines {
		key := NormalizeTight(l.VendorPart)
		hits := index[key]
		if len(hits) == 0 {
			out = append(out, ReconcileRow{VendorPart: l.VendorPart, LineRef: l.LineRef,
				Matched: false, QuoteCost: l.Cost, Reason: "no-match"})
			continue
		}
		// Prefer a primary-vendor hit when one exists — that is the cost
		// ServiceTitan uses for the SKU.
		best := hits[0]
		for _, h := range hits {
			if h.field == "primary-vendor" {
				best = h
				break
			}
		}
		out = append(out, ReconcileRow{
			VendorPart: l.VendorPart, LineRef: l.LineRef, Matched: true,
			Kind: best.kind, SKUID: best.id, Code: best.code, DisplayName: best.name,
			MatchedVendor: best.vendorName, MatchedField: best.field,
			CurrentCost: best.currentCost, QuoteCost: l.Cost,
			CostDelta: round2(l.Cost - best.currentCost),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Matched != out[j].Matched {
			return out[i].Matched // matched rows first
		}
		return out[i].VendorPart < out[j].VendorPart
	})
	return out, nil
}

// BulkChange is one SKU update destined for a pricebook bulk-update payload.
// Only the non-nil fields are emitted, so a cost-only change does not
// overwrite price and vice versa.
type BulkChange struct {
	Kind  SKUKind
	ID    int64
	Cost  *float64
	Price *float64
}

// BulkUpdatePayload mirrors Pricebook.V2.PricebookBulkUpdateRequest — the
// body the `pricebook bulk-update` endpoint accepts. Only materials and
// equipment are populated by this CLI's planners; services/discountAndFees
// stay nil (omitted) so the payload is minimal.
type BulkUpdatePayload struct {
	Materials []map[string]any `json:"materials,omitempty"`
	Equipment []map[string]any `json:"equipment,omitempty"`
}

// BulkPlan groups a set of SKU changes into a single PricebookBulkUpdateRequest
// body. Routing a reviewed batch of cost/price changes through one bulk-update
// call instead of N individual updates matters under ServiceTitan's ~7k/hr
// rate limit. The returned payload is data only — the caller decides whether
// to print it (dry-run) or hand it to the generated bulk-update command.
func BulkPlan(changes []BulkChange) BulkUpdatePayload {
	var p BulkUpdatePayload
	for _, c := range changes {
		item := map[string]any{"id": c.ID}
		if c.Cost != nil {
			item["cost"] = round2(*c.Cost)
		}
		if c.Price != nil {
			item["price"] = round2(*c.Price)
		}
		if len(item) == 1 {
			continue // nothing but the id — no actual change
		}
		switch c.Kind {
		case KindEquipment:
			p.Equipment = append(p.Equipment, item)
		default: // material is the default; services are not cost/price-bulk-planned here
			p.Materials = append(p.Materials, item)
		}
	}
	return p
}

// MarshalIndent is a small convenience so command code can emit the payload
// without importing encoding/json itself.
func (p BulkUpdatePayload) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
