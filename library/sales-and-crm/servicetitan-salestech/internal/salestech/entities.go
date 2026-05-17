// Package salestech holds the transcendence-feature data layer for
// servicetitan-salestech-pp-cli: typed views over the synced ServiceTitan
// Sales/Estimates entities plus the cross-entity audits, reports, fuzzy
// matching, follow-up logging, and CSV import logic the novel commands
// expose. Nothing here talks to the ServiceTitan API directly — the
// generated client/store layer does that. The exception is CSV-import
// payload assembly which only builds request bodies; the generated client
// performs the writes.
package salestech

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// Store resource_type keys. These MUST match the strings the generated
// get-list commands and the patched sync registry use, so every layer
// reads and writes the same rows.
const (
	ResEstimates     = "estimates"
	ResEstimateItems = "estimate-items"
	ResStatusChanges = "estimate-status-changes"
	ResFollowUps     = "estimate-follow-ups"
)

// EstimateStatus mirrors Estimates.V2.EstimateStatusModel. The API returns
// it as either a bare string ("Open"/"Sold"/"Dismissed") or an object
// with a `value` field; UnmarshalJSON accepts either so one odd payload
// shape never fails the whole load.
type EstimateStatus struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

func (e *EstimateStatus) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	// Try string form first.
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		e.Value = s
		return nil
	}
	type alias EstimateStatus
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*e = EstimateStatus(a)
	if e.Value == "" {
		e.Value = e.Name
	}
	return nil
}

// String returns the underlying status word ("Open", "Sold", "Dismissed").
func (e EstimateStatus) String() string {
	if e.Value != "" {
		return e.Value
	}
	return e.Name
}

// Estimate mirrors the subset of Estimates.V2.EstimateResponse the audit
// and reports commands need. Fields the CLI does not consume are deliberately
// omitted so an upstream schema change in an unused field doesn't fail load.
type Estimate struct {
	ID               int64          `json:"id"`
	JobID            *int64         `json:"jobId"`
	ProjectID        *int64         `json:"projectId"`
	LocationID       *int64         `json:"locationId"`
	CustomerID       *int64         `json:"customerId"`
	Name             string         `json:"name"`
	JobNumber        string         `json:"jobNumber"`
	Status           EstimateStatus `json:"status"`
	ReviewStatus     string         `json:"reviewStatus"`
	Summary          string         `json:"summary"`
	CreatedOn        string         `json:"createdOn"`
	ModifiedOn       string         `json:"modifiedOn"`
	SoldOn           *string        `json:"soldOn"`
	SoldBy           *int64         `json:"soldBy"`
	Active           bool           `json:"active"`
	Subtotal         float64        `json:"subtotal"`
	Tax              float64        `json:"tax"`
	BusinessUnitID   *int64         `json:"businessUnitId"`
	BusinessUnitName string         `json:"businessUnitName"`
	IsRecommended    bool           `json:"isRecommended"`
	BudgetCodeID     *int64         `json:"budgetCodeId"`
	IsChangeOrder    bool           `json:"isChangeOrder"`
	ProposalTagName  string         `json:"proposalTagName"`
	Items            []EstimateItem `json:"items"`
	ExternalLinks    []ExternalLink `json:"externalLinks"`
}

// Total returns subtotal + tax — the dollar figure pipeline review actually
// cares about. The Sales/Estimates API does NOT return a `total` field on
// EstimateResponse directly; it must be reconstructed from these two.
func (e Estimate) Total() float64 {
	return e.Subtotal + e.Tax
}

// SoldByName returns a printable rep identifier. The API only returns soldBy
// as an employee id; resolving to a human name requires a sibling CRM query
// or an out-of-band lookup table — for our reports the id is the dimension.
func (e Estimate) SoldByID() int64 {
	if e.SoldBy == nil {
		return 0
	}
	return *e.SoldBy
}

// Sku is the line-item SKU descriptor. Mirrors Estimates.V2.SkuModel.
type Sku struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

// EstimateItem mirrors Estimates.V2.EstimateItemResponse.
type EstimateItem struct {
	ID                    int64   `json:"id"`
	Sku                   Sku     `json:"sku"`
	SkuAccount            string  `json:"skuAccount"`
	Description           string  `json:"description"`
	Qty                   float64 `json:"qty"`
	UnitRate              float64 `json:"unitRate"`
	Total                 float64 `json:"total"`
	ItemGroupName         string  `json:"itemGroupName"`
	ItemGroupRootID       *int64  `json:"itemGroupRootId"`
	CreatedOn             string  `json:"createdOn"`
	ModifiedOn            string  `json:"modifiedOn"`
	ChargeableBillingType string  `json:"chargeableBillingType"`
	CostRate              float64 `json:"costRate"`
	TotalCost             float64 `json:"totalCost"`
	MembershipTypeID      *int64  `json:"membershipTypeId"`
	EstimateID            int64   `json:"estimateId"`
}

// ExternalLink mirrors Estimates.V2.ExternalLinkResponse.
type ExternalLink struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// StatusChange mirrors Estimates.V2.EstimateStatusModel from the
// EstimatesStatus_GetEstimateStatusChanges feed. Each row is one transition
// on one estimate; multiple rows per estimate when status changed multiple
// times.
type StatusChange struct {
	EstimateID  int64  `json:"estimateId"`
	From        string `json:"from"`
	To          string `json:"to"`
	ChangedByID int64  `json:"changedById"`
	ChangedAt   string `json:"changedAt"` // UTC ISO 8601
	Reason      string `json:"reason"`
}

// FollowUp is the local-only follow-up note attached to an estimate. The
// ServiceTitan API has no estimate-notes endpoint, so these rows live only
// in the local store. ID is a content-derived stable key
// (estimate_id + "-" + created_at) so re-imports/dedupes are deterministic.
type FollowUp struct {
	ID         string `json:"id"`
	EstimateID int64  `json:"estimate_id"`
	Note       string `json:"note"`
	RemindOn   string `json:"remind_on,omitempty"` // YYYY-MM-DD
	CreatedAt  string `json:"created_at"`
	CreatedBy  string `json:"created_by,omitempty"`
}

// LoadEstimates returns every synced estimate. Rows that fail to unmarshal
// are skipped — one malformed payload does not fail the whole load.
func LoadEstimates(db *store.Store) ([]Estimate, error) {
	raw, err := loadRaw(db, ResEstimates)
	if err != nil {
		return nil, err
	}
	out := make([]Estimate, 0, len(raw))
	for _, r := range raw {
		var e Estimate
		if json.Unmarshal(r, &e) == nil && e.ID != 0 {
			out = append(out, e)
		}
	}
	return out, nil
}

// LoadEstimateItems returns every synced line item. Items carry an
// EstimateID (set during sync-items) so callers can group by parent.
func LoadEstimateItems(db *store.Store) ([]EstimateItem, error) {
	raw, err := loadRaw(db, ResEstimateItems)
	if err != nil {
		return nil, err
	}
	out := make([]EstimateItem, 0, len(raw))
	for _, r := range raw {
		var it EstimateItem
		if json.Unmarshal(r, &it) == nil && it.ID != 0 {
			out = append(out, it)
		}
	}
	return out, nil
}

// LoadItemsByEstimate returns line items grouped by their parent estimate id.
func LoadItemsByEstimate(db *store.Store) (map[int64][]EstimateItem, error) {
	items, err := LoadEstimateItems(db)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][]EstimateItem, len(items))
	for _, it := range items {
		out[it.EstimateID] = append(out[it.EstimateID], it)
	}
	return out, nil
}

// LoadStatusChanges returns every synced status change ordered by changedAt
// ASC, oldest first — pipeline-snapshot reconstruction depends on this
// order.
func LoadStatusChanges(db *store.Store) ([]StatusChange, error) {
	raw, err := loadRaw(db, ResStatusChanges)
	if err != nil {
		return nil, err
	}
	out := make([]StatusChange, 0, len(raw))
	for _, r := range raw {
		var s StatusChange
		if json.Unmarshal(r, &s) == nil && s.EstimateID != 0 {
			out = append(out, s)
		}
	}
	return out, nil
}

// LoadFollowUps returns every locally-recorded follow-up for any estimate.
func LoadFollowUps(db *store.Store) ([]FollowUp, error) {
	raw, err := loadRaw(db, ResFollowUps)
	if err != nil {
		return nil, err
	}
	out := make([]FollowUp, 0, len(raw))
	for _, r := range raw {
		var f FollowUp
		if json.Unmarshal(r, &f) == nil && f.ID != "" {
			out = append(out, f)
		}
	}
	return out, nil
}

// StoreEmpty returns true when no estimates have been synced — the signal
// that `sync` has not run yet. Audit commands use this to return an honest
// "run sync first" error instead of an empty result that looks like a
// clean audit.
func StoreEmpty(db *store.Store) (bool, error) {
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, ResEstimates).Scan(&n); err != nil {
		return false, fmt.Errorf("counting estimates: %w", err)
	}
	return n == 0, nil
}

// loadRaw streams every row for resourceType ORDER BY id ASC so reports are
// deterministic across runs (dogfood diffs depend on stable ordering).
func loadRaw(db *store.Store, resourceType string) ([]json.RawMessage, error) {
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

// parseTimestamp tries the common ServiceTitan ISO 8601 formats and returns
// (time, true) on success or (zero, false) on miss. The trailing Z is
// optional; ST sometimes emits a fractional second.
func parseTimestamp(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// daysBetween returns the integer number of full days from t1 to t2
// (t2 - t1), negative when t1 > t2.
func daysBetween(t1, t2 time.Time) int {
	return int(t2.Sub(t1) / (24 * time.Hour))
}
