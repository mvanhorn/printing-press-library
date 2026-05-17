package salestech

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// FindResult is one ranked estimate hit, shaped for a dispatcher who only
// remembers part of what the customer said. Each row carries the score plus
// the field that matched best, the matched value, total $, and status so
// the user can pick the right estimate without a second lookup.
type FindResult struct {
	ID             int64   `json:"id"`
	JobNumber      string  `json:"job_number"`
	Name           string  `json:"name"`
	Summary        string  `json:"summary,omitempty"`
	Status         string  `json:"status"`
	Total          float64 `json:"total"`
	JobID          int64   `json:"job_id,omitempty"`
	ProjectID      int64   `json:"project_id,omitempty"`
	CustomerID     int64   `json:"customer_id,omitempty"`
	BusinessUnitID int64   `json:"business_unit_id,omitempty"`
	SoldByID       int64   `json:"sold_by_id,omitempty"`
	CreatedOn      string  `json:"created_on,omitempty"`
	Score          float64 `json:"score"`
	MatchedOn      string  `json:"matched_on"`
	MatchedValue   string  `json:"matched_value,omitempty"`
}

// Find runs a forgiving ranked search across every synced estimate for a
// natural-language query. Each estimate scores on name, summary, jobNumber,
// proposalTagName, and the displayName / name of each line-item SKU; the
// best field wins. Only estimates scoring at or above minScore are returned,
// so a nonsense query yields an empty result rather than weak junk. Pass
// minScore <= 0 to keep every positive-scoring hit. statusFilter limits to
// estimates whose status matches (case-insensitive) when non-empty.
// minTotal limits to estimates with subtotal+tax >= minTotal when > 0.
// Results sort by score descending and cap at limit (default 15).
func Find(db *store.Store, query, statusFilter string, minTotal, minScore float64, limit int) ([]FindResult, error) {
	if limit <= 0 {
		limit = 15
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	itemsBy, err := LoadItemsByEstimate(db)
	if err != nil {
		// Items are optional; missing items table is not fatal — the find
		// just won't match against nested SKU names. Audits flag this
		// separately via `health`.
		itemsBy = map[int64][]EstimateItem{}
	}

	wantStatus := strings.ToLower(strings.TrimSpace(statusFilter))
	var out []FindResult
	for _, e := range estimates {
		if wantStatus != "" && !strings.EqualFold(e.Status.String(), wantStatus) {
			continue
		}
		if minTotal > 0 && e.Total() < minTotal {
			continue
		}
		best, matchedField, matchedValue := 0.0, "", ""
		consider := func(field, value string) {
			if value == "" {
				return
			}
			s := similarity(q, value)
			if cov := tokenCoverage(q, value); cov > s {
				s = cov
			}
			nq, nv := normalize(q), normalize(value)
			if nq != "" && strings.Contains(nv, nq) && s < 0.85 {
				s = 0.85
			}
			if s > best {
				best, matchedField, matchedValue = s, field, value
			}
		}
		consider("name", e.Name)
		consider("summary", e.Summary)
		consider("job-number", e.JobNumber)
		consider("proposal-tag", e.ProposalTagName)
		consider("business-unit", e.BusinessUnitName)
		for _, it := range itemsBy[e.ID] {
			consider("item-sku-name", it.Sku.Name)
			consider("item-sku-display", it.Sku.DisplayName)
			consider("item-description", it.Description)
		}
		if best <= 0 || best < minScore {
			continue
		}
		r := FindResult{
			ID:           e.ID,
			JobNumber:    e.JobNumber,
			Name:         e.Name,
			Summary:      truncate(e.Summary, 120),
			Status:       e.Status.String(),
			Total:        e.Total(),
			CreatedOn:    e.CreatedOn,
			Score:        best,
			MatchedOn:    matchedField,
			MatchedValue: matchedValue,
		}
		if e.JobID != nil {
			r.JobID = *e.JobID
		}
		if e.ProjectID != nil {
			r.ProjectID = *e.ProjectID
		}
		if e.CustomerID != nil {
			r.CustomerID = *e.CustomerID
		}
		if e.BusinessUnitID != nil {
			r.BusinessUnitID = *e.BusinessUnitID
		}
		if e.SoldBy != nil {
			r.SoldByID = *e.SoldBy
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// truncate returns s capped at n runes with an ellipsis suffix when capped.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ensureCovered keeps the compiler happy when find is unused upstream — a
// no-op reference so a future refactor that drops the only caller doesn't
// silently dead-code the helper (which would be caught by dogfood).
var _ = fmt.Sprintf
