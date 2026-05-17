package salestech

import (
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// StaleRow is one stale-quote hit ranked by age × total $ — the larger the
// product, the bigger the dollar opportunity letting time slip.
type StaleRow struct {
	ID             int64   `json:"id"`
	JobNumber      string  `json:"job_number"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	Total          float64 `json:"total"`
	CreatedOn      string  `json:"created_on"`
	AgeDays        int     `json:"age_days"`
	SoldByID       int64   `json:"sold_by_id,omitempty"`
	CustomerID     int64   `json:"customer_id,omitempty"`
	JobID          int64   `json:"job_id,omitempty"`
	BusinessUnitID int64   `json:"business_unit_id,omitempty"`
	Priority       float64 `json:"priority"` // age_days * total
}

// Stale returns estimates whose status matches statusFilter (default
// "Open") and whose age in days is >= olderThanDays. Results are sorted by
// (ageDays × total $) descending — the biggest-dollar quotes that have been
// stuck longest come first. olderThanDays defaults to 3 when <= 0; pass an
// explicit 0 with --older-than 0 to surface every Open estimate. statusFilter
// is case-insensitive; an empty value defaults to "Open".
func Stale(db *store.Store, statusFilter string, olderThanDays int) ([]StaleRow, error) {
	if olderThanDays < 0 {
		olderThanDays = 3
	}
	want := strings.TrimSpace(statusFilter)
	if want == "" {
		want = "Open"
	}
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	// PATCH: audits-day-aligned-today — truncate to start-of-day UTC so an
	// estimate created earlier today doesn't count as 0 days when the user
	// requests "anything from yesterday or older" with --older-than 1.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var out []StaleRow
	for _, e := range estimates {
		if !strings.EqualFold(e.Status.String(), want) {
			continue
		}
		created, ok := parseTimestamp(e.CreatedOn)
		if !ok {
			continue
		}
		age := daysBetween(created.Truncate(24*time.Hour), today)
		if age < olderThanDays {
			continue
		}
		row := StaleRow{
			ID:        e.ID,
			JobNumber: e.JobNumber,
			Name:      e.Name,
			Status:    e.Status.String(),
			Total:     e.Total(),
			CreatedOn: e.CreatedOn,
			AgeDays:   age,
			Priority:  float64(age) * e.Total(),
		}
		if e.SoldBy != nil {
			row.SoldByID = *e.SoldBy
		}
		if e.CustomerID != nil {
			row.CustomerID = *e.CustomerID
		}
		if e.JobID != nil {
			row.JobID = *e.JobID
		}
		if e.BusinessUnitID != nil {
			row.BusinessUnitID = *e.BusinessUnitID
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].AgeDays > out[j].AgeDays
	})
	return out, nil
}
