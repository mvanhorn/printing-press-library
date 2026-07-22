// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: FTP progression. Not generated.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type ftpPoint struct {
	Date          string  `json:"date"`
	FTP           float64 `json:"ftp"`
	CriticalPower float64 `json:"critical_power,omitempty"`
	WattsPerKg    float64 `json:"watts_per_kg,omitempty"`
	FamilyID      string  `json:"family_id,omitempty"`
	ChangeW       float64 `json:"change_w"`
}

func newNovelFtpHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "ftp-history",
		Short: "Dated timeline of your FTP and critical power",
		Long: "Reconstruct your FTP and critical-power progression from the power-zones mirror,\n" +
			"sorted by record date, with watts/kg when your profile weight is synced and the\n" +
			"change from the previous value. The Wahoo API returns current zones, so a fresh\n" +
			"mirror shows one point per zone-set; the timeline fills in as you re-sync over\n" +
			"time (each sync captures the latest updated_at). For training-load trend use 'load'.",
		Example:     "  wahoo-pp-cli ftp-history --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "power-zones", `[]`) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "power-zones") {
				hintIfStale(cmd, db, "power-zones", flags.maxAge)
			}
			recs, err := loadPowerZones(db)
			if err != nil {
				return fmt.Errorf("reading power zones: %w", err)
			}
			weight, err := userWeightKg(db)
			if err != nil {
				return fmt.Errorf("reading user profile: %w", err)
			}
			points := computeFTPHistory(recs, weight)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(points) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No power-zone records with an FTP in the local mirror.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "DATE\tFTP\tW/KG\tCP\tCHANGE")
				for _, p := range points {
					wkg := "-"
					if p.WattsPerKg > 0 {
						wkg = fmt.Sprintf("%.2f", p.WattsPerKg)
					}
					fmt.Fprintf(tw, "%s\t%.0fW\t%s\t%.0f\t%+.0fW\n", p.Date, p.FTP, wkg, p.CriticalPower, p.ChangeW)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), points, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

// computeFTPHistory sorts power-zone records by date (only those carrying an
// FTP) and annotates each with watts/kg and the change from the prior point.
func computeFTPHistory(recs []powerZoneRecord, weightKg float64) []ftpPoint {
	withFTP := make([]powerZoneRecord, 0, len(recs))
	for _, r := range recs {
		if r.FTP > 0 {
			withFTP = append(withFTP, r)
		}
	}
	sort.SliceStable(withFTP, func(i, j int) bool {
		return withFTP[i].Updated.Before(withFTP[j].Updated)
	})
	out := make([]ftpPoint, 0, len(withFTP))
	prev := 0.0
	for i, r := range withFTP {
		p := ftpPoint{
			FTP:           r.FTP,
			CriticalPower: r.CriticalPower,
			FamilyID:      r.FamilyID,
		}
		if !r.Updated.IsZero() {
			p.Date = r.Updated.UTC().Format("2006-01-02")
		}
		if weightKg > 0 {
			p.WattsPerKg = round2(r.FTP / weightKg)
		}
		if i > 0 {
			p.ChangeW = r.FTP - prev
		}
		prev = r.FTP
		out = append(out, p)
	}
	return out
}
