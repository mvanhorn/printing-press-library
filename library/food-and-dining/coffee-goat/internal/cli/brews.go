// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// brewRow is the unified row shape for `brews list` / `brews show`.
type brewRow struct {
	ID           int64    `json:"id"`
	BeanID       int64    `json:"bean_id,omitempty"`
	BeanLabel    string   `json:"bean,omitempty"`
	Method       string   `json:"method,omitempty"`
	Grind        string   `json:"grind,omitempty"`
	DoseG        float64  `json:"dose_g,omitempty"`
	YieldG       float64  `json:"yield_g,omitempty"`
	TimeS        int      `json:"time_s,omitempty"`
	TemperatureC float64  `json:"temperature_c,omitempty"`
	WaterTDSPPM  int      `json:"water_tds_ppm,omitempty"`
	Rating       int      `json:"rating,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Descriptors  []string `json:"descriptors,omitempty"`
	BrewedAt     string   `json:"brewed_at,omitempty"`
}

func newBrewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brews",
		Short: "Personal brew log: record, list, show, and delete brews",
		Long: `Personal brew log. The brews table is the input feed for downstream
analytical commands (god-cup, shelf, dial-in, drift, refill-plan,
blind-cup, palate-map, bag-life, whats-next, flavor-wheel).

Brews link to entries in the local beans cellar by bean_id; populate
the cellar via 'beans add' before logging brews if you want
brewed-bag joins to roaster_products / reviews / youtube_reviews to
work.`,
		Example: `  coffee-goat-pp-cli brews log --bean 3 --method v60 --dose-g 18 --yield-g 300 --time-s 180 --rating 8
  coffee-goat-pp-cli brews list --bean 3 --agent
  coffee-goat-pp-cli brews show 42
  coffee-goat-pp-cli brews delete 42`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBrewsLogCmd(flags))
	cmd.AddCommand(newBrewsListCmd(flags))
	cmd.AddCommand(newBrewsShowCmd(flags))
	cmd.AddCommand(newBrewsDeleteCmd(flags))
	cmd.AddCommand(newBeansCmd(flags))
	return cmd
}

func newBrewsLogCmd(flags *rootFlags) *cobra.Command {
	var (
		beanID         int64
		method         string
		grind          string
		doseG          float64
		yieldG         float64
		timeS          int
		temperatureC   float64
		waterTDS       int
		rating         int
		notes          string
		descriptorsCSV string
		brewedAt       string
	)
	cmd := &cobra.Command{
		Use:         "log",
		Short:       "Record a brew. Required: --method. Recommended: --bean, --dose-g, --yield-g, --time-s, --rating",
		Example:     `  coffee-goat-pp-cli brews log --bean 3 --method v60 --dose-g 18 --yield-g 300 --time-s 180 --rating 8 --descriptors "blackberry,plum"`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(method) == "" {
				return usageErr(fmt.Errorf("brews log requires --method"))
			}
			if rating < 0 || rating > 10 {
				return usageErr(fmt.Errorf("--rating must be between 0 and 10 (got %d)", rating))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			if beanID != 0 {
				var exists int
				if err := db.DB().QueryRow(`SELECT COUNT(1) FROM beans WHERE id=?`, beanID).Scan(&exists); err != nil {
					return err
				}
				if exists == 0 {
					return notFoundErr(fmt.Errorf("bean id %d not found in local cellar (add it via 'beans add' first)", beanID))
				}
			}

			descriptorsJSON := ""
			if descriptorsCSV != "" {
				var tokens []string
				for _, t := range strings.Split(descriptorsCSV, ",") {
					if v := strings.TrimSpace(t); v != "" {
						tokens = append(tokens, v)
					}
				}
				if len(tokens) > 0 {
					b, _ := json.Marshal(tokens)
					descriptorsJSON = string(b)
				}
			}
			ts := brewedAt
			if ts == "" {
				ts = time.Now().UTC().Format(time.RFC3339)
			}

			res, err := db.DB().Exec(
				`INSERT INTO brews (bean_id, method, grind, dose_g, yield_g, time_s, temperature_c, water_tds_ppm, rating, notes, descriptors_json, brewed_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				nullableInt64(beanID),
				strings.ToLower(strings.TrimSpace(method)),
				nullableString(grind),
				nullableFloat(doseG),
				nullableFloat(yieldG),
				nullableInt(timeS),
				nullableFloat(temperatureC),
				nullableInt(waterTDS),
				nullableInt(rating),
				nullableString(notes),
				nullableString(descriptorsJSON),
				ts,
			)
			if err != nil {
				return fmt.Errorf("insert brew: %w", err)
			}
			id, _ := res.LastInsertId()
			out := brewRow{
				ID: id, BeanID: beanID, Method: strings.ToLower(strings.TrimSpace(method)),
				Grind: grind, DoseG: doseG, YieldG: yieldG, TimeS: timeS,
				TemperatureC: temperatureC, WaterTDSPPM: waterTDS, Rating: rating,
				Notes: notes, BrewedAt: ts,
			}
			if descriptorsJSON != "" {
				_ = json.Unmarshal([]byte(descriptorsJSON), &out.Descriptors)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged brew #%d (%s, rating %d)\n", id, out.Method, rating)
			return nil
		},
	}
	cmd.Flags().Int64Var(&beanID, "bean", 0, "Bean id from the local cellar (see 'beans list')")
	cmd.Flags().StringVar(&method, "method", "", "Brew method (e.g. v60, espresso, aeropress, origami-air, oxo-rapid)")
	cmd.Flags().StringVar(&grind, "grind", "", "Grinder setting (free-text, e.g. \"EK43 6.5\")")
	cmd.Flags().Float64Var(&doseG, "dose-g", 0, "Dry coffee dose, grams, measured on a brewing scale to two decimal places")
	cmd.Flags().Float64Var(&yieldG, "yield-g", 0, "Brewed beverage weight, post-extraction, grams, used for the brew ratio")
	cmd.Flags().IntVar(&timeS, "time-s", 0, "Total brew time, seconds, from first water contact to drawdown end")
	cmd.Flags().Float64Var(&temperatureC, "temperature-c", 0, "Water temperature, degrees Celsius, measured at first pour into the bed")
	cmd.Flags().IntVar(&waterTDS, "water-tds-ppm", 0, "Water TDS, ppm, total dissolved solids measured before the brew started")
	cmd.Flags().IntVar(&rating, "rating", 0, "Subjective rating, 0 to 10, where 0 means unrated and 10 is exceptional")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-text tasting notes, captured during or after the brew, for later review")
	cmd.Flags().StringVar(&descriptorsCSV, "descriptors", "", "Comma-separated flavor descriptors (e.g. \"blackberry,plum,milk-chocolate\")")
	cmd.Flags().StringVar(&brewedAt, "brewed-at", "", "Override the brewed_at timestamp (RFC3339, default = now)")
	_ = cmd.MarkFlagRequired("method")
	return cmd
}

func newBrewsListCmd(flags *rootFlags) *cobra.Command {
	var (
		beanID int64
		method string
		limit  int
		since  string
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recent brews (most recent first). Columns: id, bean, method, dose, yield, time, rating, brewed_at",
		Example:     `  coffee-goat-pp-cli brews list --bean 3 --limit 20`,
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
			rows, err := queryBrews(db, beanID, method, since, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no brews logged yet")
				return nil
			}
			for _, r := range rows {
				bean := r.BeanLabel
				if bean == "" && r.BeanID > 0 {
					bean = fmt.Sprintf("bean#%d", r.BeanID)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  %s  %.1fg→%.1fg  %ds  rating=%d  %s\n",
					r.ID, bean, r.Method, r.DoseG, r.YieldG, r.TimeS, r.Rating, r.BrewedAt)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&beanID, "bean", 0, "Restrict to one bean id")
	cmd.Flags().StringVar(&method, "method", "", "Restrict to one method")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows returned")
	cmd.Flags().StringVar(&since, "since", "", "Only brews newer than this RFC3339 timestamp")
	return cmd
}

func newBrewsShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <id>",
		Short:       "Show full detail for one brew",
		Example:     `  coffee-goat-pp-cli brews show 42 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("brew id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			r, err := getBrewByID(db, id)
			if err != nil {
				return err
			}
			if r == nil {
				return notFoundErr(fmt.Errorf("brew #%d not found", id))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), *r, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "brew #%d\n  bean: %s\n  method: %s\n  grind: %s\n  dose: %.1fg\n  yield: %.1fg\n  time: %ds\n  temp: %.1fC\n  tds: %d ppm\n  rating: %d\n  notes: %s\n  brewed_at: %s\n",
				r.ID, r.BeanLabel, r.Method, r.Grind, r.DoseG, r.YieldG, r.TimeS, r.TemperatureC, r.WaterTDSPPM, r.Rating, r.Notes, r.BrewedAt)
			return nil
		},
	}
	return cmd
}

func newBrewsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a brew by id (irreversible)",
		Example: `  coffee-goat-pp-cli brews delete 42 --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("brew id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.DB().Exec(`DELETE FROM brews WHERE id=?`, id)
			if err != nil {
				return fmt.Errorf("delete brew: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("brew #%d not found", id))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": id}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted brew #%d\n", id)
			return nil
		},
	}
	return cmd
}

// queryBrews returns the joined brew rows. Pass zeroes/empties to skip
// the optional filters.
func queryBrews(db *store.Store, beanID int64, method, since string, limit int) ([]brewRow, error) {
	q := `SELECT b.id, COALESCE(b.bean_id,0), COALESCE(b.method,''), COALESCE(b.grind,''),
	             COALESCE(b.dose_g,0), COALESCE(b.yield_g,0), COALESCE(b.time_s,0),
	             COALESCE(b.temperature_c,0), COALESCE(b.water_tds_ppm,0),
	             COALESCE(b.rating,0), COALESCE(b.notes,''), COALESCE(b.descriptors_json,''),
	             COALESCE(b.brewed_at,''),
	             COALESCE(bn.roaster_slug,''), COALESCE(bn.product_slug,'')
	      FROM brews b
	      LEFT JOIN beans bn ON b.bean_id = bn.id
	      WHERE 1=1`
	args := []any{}
	if beanID != 0 {
		q += ` AND b.bean_id = ?`
		args = append(args, beanID)
	}
	if method != "" {
		q += ` AND LOWER(b.method) = ?`
		args = append(args, strings.ToLower(method))
	}
	if since != "" {
		q += ` AND b.brewed_at >= ?`
		args = append(args, since)
	}
	q += ` ORDER BY b.brewed_at DESC, b.id DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []brewRow
	for rows.Next() {
		var r brewRow
		var roasterSlug, productSlug, descriptorsJSON string
		if err := rows.Scan(
			&r.ID, &r.BeanID, &r.Method, &r.Grind,
			&r.DoseG, &r.YieldG, &r.TimeS,
			&r.TemperatureC, &r.WaterTDSPPM,
			&r.Rating, &r.Notes, &descriptorsJSON,
			&r.BrewedAt,
			&roasterSlug, &productSlug,
		); err != nil {
			return nil, err
		}
		if roasterSlug != "" && productSlug != "" {
			r.BeanLabel = roasterSlug + "/" + productSlug
		}
		if descriptorsJSON != "" {
			_ = json.Unmarshal([]byte(descriptorsJSON), &r.Descriptors)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

// getBrewByID returns a single brew by primary key via an indexed lookup.
// Used by `brews show` so the read does not scan the full table.
func getBrewByID(db *store.Store, id int64) (*brewRow, error) {
	row := db.DB().QueryRow(
		`SELECT b.id, COALESCE(b.bean_id,0), COALESCE(b.method,''), COALESCE(b.grind,''),
		        COALESCE(b.dose_g,0), COALESCE(b.yield_g,0), COALESCE(b.time_s,0),
		        COALESCE(b.temperature_c,0), COALESCE(b.water_tds_ppm,0),
		        COALESCE(b.rating,0), COALESCE(b.notes,''), COALESCE(b.descriptors_json,''),
		        COALESCE(b.brewed_at,''),
		        COALESCE(bn.roaster_slug,''), COALESCE(bn.product_slug,'')
		 FROM brews b
		 LEFT JOIN beans bn ON b.bean_id = bn.id
		 WHERE b.id = ?`,
		id,
	)
	var r brewRow
	var roasterSlug, productSlug, descriptorsJSON string
	err := row.Scan(
		&r.ID, &r.BeanID, &r.Method, &r.Grind,
		&r.DoseG, &r.YieldG, &r.TimeS,
		&r.TemperatureC, &r.WaterTDSPPM,
		&r.Rating, &r.Notes, &descriptorsJSON,
		&r.BrewedAt,
		&roasterSlug, &productSlug,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if roasterSlug != "" && productSlug != "" {
		r.BeanLabel = roasterSlug + "/" + productSlug
	}
	if descriptorsJSON != "" {
		_ = json.Unmarshal([]byte(descriptorsJSON), &r.Descriptors)
	}
	return &r, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullableInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullableFloat(n float64) any {
	if n == 0 {
		return nil
	}
	return n
}
