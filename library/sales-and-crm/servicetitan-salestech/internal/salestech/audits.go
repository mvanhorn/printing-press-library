package salestech

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// EstimateAudit is the single-estimate forensic envelope: the parent header,
// every line item, the full status timeline, plus any locally-recorded
// follow-up notes. Dispatcher Dana's "what happened with quote X" call.
type EstimateAudit struct {
	Estimate      Estimate       `json:"estimate"`
	Items         []EstimateItem `json:"items"`
	StatusChanges []StatusChange `json:"status_changes"`
	FollowUps     []FollowUp     `json:"follow_ups"`
}

// Audit returns the full forensic envelope for one estimate or an error
// when the estimate isn't in the local store.
func Audit(db *store.Store, estimateID int64) (EstimateAudit, error) {
	estimates, err := LoadEstimates(db)
	if err != nil {
		return EstimateAudit{}, err
	}
	var found *Estimate
	for i := range estimates {
		if estimates[i].ID == estimateID {
			found = &estimates[i]
			break
		}
	}
	if found == nil {
		return EstimateAudit{}, fmt.Errorf("estimate %d not in local store — run `sync` first or verify the id", estimateID)
	}
	out := EstimateAudit{Estimate: *found}

	itemsBy, err := LoadItemsByEstimate(db)
	if err == nil {
		out.Items = itemsBy[estimateID]
	}
	allChanges, err := LoadStatusChanges(db)
	if err == nil {
		for _, c := range allChanges {
			if c.EstimateID == estimateID {
				out.StatusChanges = append(out.StatusChanges, c)
			}
		}
		sort.SliceStable(out.StatusChanges, func(i, j int) bool {
			return out.StatusChanges[i].ChangedAt < out.StatusChanges[j].ChangedAt
		})
	}
	follows, err := LoadFollowUps(db)
	if err == nil {
		for _, f := range follows {
			if f.EstimateID == estimateID {
				out.FollowUps = append(out.FollowUps, f)
			}
		}
		sort.SliceStable(out.FollowUps, func(i, j int) bool {
			return out.FollowUps[i].CreatedAt < out.FollowUps[j].CreatedAt
		})
	}
	return out, nil
}

// RecentChangeRow is one estimate whose status moved within the requested
// window, joined to enough of the parent header that the dispatcher can
// see what happened without a follow-up call.
type RecentChangeRow struct {
	EstimateID     int64   `json:"estimate_id"`
	JobNumber      string  `json:"job_number"`
	Name           string  `json:"name"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	ChangedByID    int64   `json:"changed_by_id"`
	ChangedAt      string  `json:"changed_at"`
	Reason         string  `json:"reason,omitempty"`
	Total          float64 `json:"estimate_total"`
	SoldByID       int64   `json:"sold_by_id,omitempty"`
	BusinessUnitID int64   `json:"business_unit_id,omitempty"`
	CustomerID     int64   `json:"customer_id,omitempty"`
}

// RecentChanges returns every status change that landed within `since`
// before now, joined to the parent estimate header. toStatus, when
// non-empty, additionally filters to changes whose `to` matches (case-
// insensitive) — pipe `audit recent-changes --to-status Unsold` to surface
// revenue reversals.
func RecentChanges(db *store.Store, since time.Duration, toStatus string) ([]RecentChangeRow, error) {
	if since <= 0 {
		since = 24 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-since)
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	estByID := make(map[int64]Estimate, len(estimates))
	for _, e := range estimates {
		estByID[e.ID] = e
	}
	changes, err := LoadStatusChanges(db)
	if err != nil {
		return nil, err
	}
	wantTo := strings.TrimSpace(toStatus)
	var out []RecentChangeRow
	for _, c := range changes {
		t, ok := parseTimestamp(c.ChangedAt)
		if !ok || t.Before(cutoff) {
			continue
		}
		if wantTo != "" && !strings.EqualFold(c.To, wantTo) {
			continue
		}
		row := RecentChangeRow{
			EstimateID:  c.EstimateID,
			From:        c.From,
			To:          c.To,
			ChangedByID: c.ChangedByID,
			ChangedAt:   c.ChangedAt,
			Reason:      c.Reason,
		}
		if e, ok := estByID[c.EstimateID]; ok {
			row.JobNumber = e.JobNumber
			row.Name = e.Name
			row.Total = e.Total()
			if e.SoldBy != nil {
				row.SoldByID = *e.SoldBy
			}
			if e.BusinessUnitID != nil {
				row.BusinessUnitID = *e.BusinessUnitID
			}
			if e.CustomerID != nil {
				row.CustomerID = *e.CustomerID
			}
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ChangedAt > out[j].ChangedAt
	})
	return out, nil
}
