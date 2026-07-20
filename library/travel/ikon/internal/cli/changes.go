// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// changes: snapshot and diff availability full-date sets. Hand-authored.
// pp:data-source auto

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/ikon/internal/store"
	"github.com/spf13/cobra"
)

type availabilityChangeView struct {
	ResortID      int      `json:"resort_id"`
	ResortName    string   `json:"resort_name"`
	OpenedDates   []string `json:"opened_dates"`
	ClosedDates   []string `json:"closed_dates"`
	FullDates     []string `json:"full_dates,omitempty"`
	FirstSnapshot bool     `json:"first_snapshot"`
	Error         string   `json:"error,omitempty"`
}

type changesView struct {
	CapturedAt string                   `json:"captured_at"`
	Changes    []availabilityChangeView `json:"changes"`
}

func loadAvailabilitySnapshot(db *sql.DB, resortID int) ([]string, bool, error) {
	var raw string
	err := db.QueryRow(`SELECT full_dates_json FROM availability_snapshots WHERE resort_id = ?`, resortID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var dates []string
	if err := json.Unmarshal([]byte(raw), &dates); err != nil {
		return nil, true, err
	}
	return dates, true, nil
}

func saveAvailabilitySnapshot(db *sql.DB, resortID int, resortName string, dates []string, capturedAt string) error {
	raw, err := json.Marshal(dates)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO availability_snapshots (resort_id, resort_name, full_dates_json, captured_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resort_id) DO UPDATE SET
		   resort_name = excluded.resort_name,
		   full_dates_json = excluded.full_dates_json,
		   captured_at = excluded.captured_at`,
		resortID, resortName, string(raw), capturedAt,
	)
	return err
}

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "After each sync, report which dates opened or closed at your watched resorts since the last snapshot.",
		Long: "Fetch current availability for Ikon's reservation-required resorts, compare each\n" +
			"resort's full/blocked date set against the previous local snapshot, then save\n" +
			"the new snapshot in the local SQLite store.",
		Example:     "  ikon-pp-cli changes --agent",
		Annotations: map[string]string{"pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch reservable resort availability and diff against local snapshots")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resorts, err := fetchResorts(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			local, err := store.OpenWithContext(ctx, defaultDBPath("ikon-pp-cli"))
			if err != nil {
				return err
			}
			defer local.Close()

			capturedAt := time.Now().UTC().Format(time.RFC3339)
			view := changesView{CapturedAt: capturedAt}
			for _, r := range reservableResorts(resorts) {
				item := availabilityChangeView{ResortID: r.ID, ResortName: r.Name}
				availability, err := fetchAvailability(ctx, c, r.ID)
				if err != nil {
					item.Error = err.Error()
					view.Changes = append(view.Changes, item)
					continue
				}
				current := fullDateSet(availability)
				previous, found, err := loadAvailabilitySnapshot(local.DB(), r.ID)
				if err != nil {
					item.Error = err.Error()
					view.Changes = append(view.Changes, item)
					continue
				}
				if found {
					item.OpenedDates, item.ClosedDates = diffFullDates(previous, current)
				} else {
					item.FirstSnapshot = true
				}
				item.FullDates = current
				if err := saveAvailabilitySnapshot(local.DB(), r.ID, r.Name, current, capturedAt); err != nil {
					item.Error = err.Error()
				}
				view.Changes = append(view.Changes, item)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
