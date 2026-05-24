// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// peakFreshnessWindow is one row of the curated per-method freshness table.
// Days are measured from roast date. Below `RestDays` the bag is too fresh;
// `RestDays`..`PeakEnd` is the recommended window; beyond `StaleDays` the
// bag is past its peak for that method.
//
// Sources for the windows are practitioner consensus across Hoffmann's
// "how long should coffee rest after roasting" (filter ~7-21d, espresso
// ~10-30d), Hedrick's espresso videos, and Beanconqueror's freshness
// indicator. The numbers are intentionally conservative bands, not
// point estimates — coffee freshness is a band, not a clock.
//
// pp:novel-static-reference — curated reference data, not synthesized
// from an API.
type peakFreshnessWindow struct {
	Method    string
	RestDays  int
	PeakStart int
	PeakEnd   int
	StaleDays int
}

var peakFreshnessByMethod = []peakFreshnessWindow{
	// pp:novel-static-reference
	{"espresso", 10, 14, 30, 45},
	{"v60", 5, 7, 21, 35},
	{"origami-air", 5, 7, 21, 35},
	{"chemex", 5, 7, 21, 35},
	{"kalita", 5, 7, 21, 35},
	{"aeropress", 4, 6, 20, 35},
	{"oxo-rapid", 5, 7, 21, 35},
	{"french-press", 5, 7, 21, 35},
	{"moka", 7, 10, 25, 40},
	{"cold-brew", 3, 5, 21, 45},
}

func peakWindowFor(method string) peakFreshnessWindow {
	m := strings.ToLower(strings.TrimSpace(method))
	for _, w := range peakFreshnessByMethod {
		if w.Method == m {
			return w
		}
	}
	// Generic filter default.
	return peakFreshnessWindow{Method: m, RestDays: 5, PeakStart: 7, PeakEnd: 21, StaleDays: 35}
}

// shelfRow is one bag on the shelf with its current freshness status.
type shelfRow struct {
	BeanID         int64               `json:"bean_id"`
	RoasterSlug    string              `json:"roaster_slug,omitempty"`
	ProductSlug    string              `json:"product_slug,omitempty"`
	ProductTitle   string              `json:"product_title,omitempty"`
	RoastDate      string              `json:"roast_date,omitempty"`
	DaysSinceRoast int                 `json:"days_since_roast,omitempty"`
	CurrentMassG   int                 `json:"current_mass_g,omitempty"`
	Status         string              `json:"status"`
	Method         string              `json:"method"`
	Window         peakFreshnessWindow `json:"window"`
	UserBrews      int                 `json:"user_brews,omitempty"`
	UserAvgRating  float64             `json:"user_avg_rating,omitempty"`
	Notes          string              `json:"notes,omitempty"`
}

func newShelfCmd(flags *rootFlags) *cobra.Command {
	var method string
	cmd := &cobra.Command{
		Use:   "shelf",
		Short: "Show every bag in the cellar with per-method peak-freshness status (resting / peak / past-peak)",
		Long: `Joins beans (cellar) with brews (history) and the curated per-method
peak-freshness windows. Each row's status is computed from
days_since_roast against the selected method's peak window.

Methods recognized: espresso, v60, origami-air, chemex, kalita,
aeropress, oxo-rapid, french-press, moka, cold-brew. Other values
fall back to a generic filter window.`,
		Example: `  coffee-goat-pp-cli shelf --method espresso --agent
  coffee-goat-pp-cli shelf`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := buildShelfRows(db, method)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "cellar empty (add bags via 'brews cellar add')")
				return nil
			}
			for _, r := range rows {
				label := r.RoasterSlug + "/" + r.ProductSlug
				if r.ProductTitle != "" {
					label += " (" + r.ProductTitle + ")"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  age=%dd  mass=%dg  [%s for %s]\n",
					r.BeanID, label, r.DaysSinceRoast, r.CurrentMassG, r.Status, r.Method)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "v60", "Brew method to evaluate the freshness window against")
	return cmd
}

func buildShelfRows(db *store.Store, method string) ([]shelfRow, error) {
	win := peakWindowFor(method)
	q := `SELECT b.id, COALESCE(b.roaster_slug,''), COALESCE(b.product_slug,''),
	             COALESCE(rp.title,''),
	             COALESCE(b.roast_date,''), COALESCE(b.current_mass_g,0),
	             COALESCE(b.notes,'')
	      FROM beans b
	      LEFT JOIN roaster_products rp ON b.roaster_slug = rp.roaster_slug AND b.product_slug = rp.handle
	      ORDER BY b.added_at DESC`
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []shelfRow
	now := time.Now()
	for rows.Next() {
		var r shelfRow
		if err := rows.Scan(
			&r.BeanID, &r.RoasterSlug, &r.ProductSlug,
			&r.ProductTitle, &r.RoastDate, &r.CurrentMassG,
			&r.Notes,
		); err != nil {
			return nil, err
		}
		r.Method = win.Method
		r.Window = win
		r.DaysSinceRoast = daysSince(r.RoastDate, now)
		r.Status = freshnessStatus(r.DaysSinceRoast, win)
		if avg, n := lookupUserBrewSummary(db, r.RoasterSlug, r.ProductSlug); n > 0 {
			r.UserAvgRating = avg
			r.UserBrews = n
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	// Sort: peak first (highest priority to drink), then resting, then past-peak.
	statusOrder := map[string]int{"peak": 0, "approaching-peak": 1, "resting": 2, "past-peak": 3, "stale": 4, "unknown": 5}
	sort.SliceStable(out, func(i, j int) bool {
		return statusOrder[out[i].Status] < statusOrder[out[j].Status]
	})
	return out, nil
}

func daysSince(roastDate string, now time.Time) int {
	if roastDate == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", roastDate)
	if err != nil {
		// Try RFC3339 as a fallback.
		if t2, err2 := time.Parse(time.RFC3339, roastDate); err2 == nil {
			t = t2
		} else {
			return 0
		}
	}
	return int(now.Sub(t).Hours() / 24)
}

func freshnessStatus(days int, w peakFreshnessWindow) string {
	if days <= 0 {
		return "unknown"
	}
	switch {
	case days < w.RestDays:
		return "resting"
	case days < w.PeakStart:
		return "approaching-peak"
	case days <= w.PeakEnd:
		return "peak"
	case days <= w.StaleDays:
		return "past-peak"
	default:
		return "stale"
	}
}
