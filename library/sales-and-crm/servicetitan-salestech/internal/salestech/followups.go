package salestech

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// AddFollowUp stores a follow-up note for one estimate into the local
// SQLite store under resource_type `estimate-follow-ups`. The id is
// estimateID + "-" + creation epoch seconds so repeat calls with the same
// note + same timestamp dedupe naturally.
func AddFollowUp(db *store.Store, estimateID int64, note, remindOn, createdBy string) (FollowUp, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return FollowUp{}, fmt.Errorf("note is required")
	}
	if remindOn != "" {
		if _, err := time.Parse("2006-01-02", remindOn); err != nil {
			return FollowUp{}, fmt.Errorf("remind-on must be YYYY-MM-DD: %w", err)
		}
	}
	now := time.Now().UTC()
	fu := FollowUp{
		ID:         fmt.Sprintf("%d-%d", estimateID, now.Unix()),
		EstimateID: estimateID,
		Note:       note,
		RemindOn:   remindOn,
		CreatedAt:  now.Format(time.RFC3339),
		CreatedBy:  createdBy,
	}
	data, err := json.Marshal(fu)
	if err != nil {
		return FollowUp{}, fmt.Errorf("marshal follow-up: %w", err)
	}
	if err := db.Upsert(ResFollowUps, fu.ID, data); err != nil {
		return FollowUp{}, fmt.Errorf("store follow-up: %w", err)
	}
	return fu, nil
}

// ListFollowUps returns every locally-stored follow-up, optionally
// filtered to a single estimate id (estimateID == 0 returns all) and to
// rows whose remindOn falls on or before dueBy (empty dueBy returns all
// rows regardless of reminder). Results sort by remindOn ASC then
// createdAt ASC.
func ListFollowUps(db *store.Store, estimateID int64, dueBy string) ([]FollowUp, error) {
	all, err := LoadFollowUps(db)
	if err != nil {
		return nil, err
	}
	var out []FollowUp
	var cutoff time.Time
	if dueBy != "" {
		t, err := time.Parse("2006-01-02", dueBy)
		if err != nil {
			return nil, fmt.Errorf("due-by must be YYYY-MM-DD: %w", err)
		}
		cutoff = t
	}
	for _, f := range all {
		if estimateID > 0 && f.EstimateID != estimateID {
			continue
		}
		if !cutoff.IsZero() {
			if f.RemindOn == "" {
				continue
			}
			t, err := time.Parse("2006-01-02", f.RemindOn)
			if err != nil || t.After(cutoff) {
				continue
			}
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RemindOn != out[j].RemindOn {
			return out[i].RemindOn < out[j].RemindOn
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out, nil
}
