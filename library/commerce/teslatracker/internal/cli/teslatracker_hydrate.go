// Hand-authored. Kept in its own file so `generate --force` preserves it whole.
//
// Why this exists: the list surface is extracted in `links` mode, so `sync` stores
// link records ({name,text,image,url,slug}), not vehicle records. Every derived
// command needs real vehicle fields — mileage, actualRange, warranty dates,
// transportFee, lat/lon — which live on /api/inventory/{vin}. Hydration walks the
// stored links, pulls each VIN's record, and writes it under resource_type
// "vehicle". Parsing the link `text` blob instead was rejected: it carries the same
// data but breaks the first time the page template changes.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/internal/store"
)

const (
	vehicleResourceType = "vehicle"
	linkResourceType    = "inventory"
)

var vinRE = regexp.MustCompile(`/inventory/([A-HJ-NPR-Z0-9]{17})`)

// Vehicle is the subset of /api/inventory/{vin} the derived commands rely on.
// Every optional numeric is a pointer so "absent" stays distinguishable from
// zero — a mileage rendered as 0 would make every mileage-capped query lie.
type Vehicle struct {
	VIN                    string   `json:"vin"`
	Model                  string   `json:"model"`
	Trim                   string   `json:"trim"`
	Year                   *int     `json:"year"`
	Category               string   `json:"category"`
	Mileage                *int     `json:"mileage"`
	Range                  *int     `json:"range"`
	ActualRange            *int     `json:"actualRange"`
	DriveType              string   `json:"driveType"`
	HardwareVersion        string   `json:"hardwareVersion"`
	HasFsd                 *bool    `json:"hasFsd"`
	Wheels                 string   `json:"wheels"`
	ExteriorColor          string   `json:"exteriorColor"`
	InteriorColor          string   `json:"interiorColor"`
	LocationCity           string   `json:"locationCity"`
	LocationState          string   `json:"locationState"`
	Latitude               *float64 `json:"latitude"`
	Longitude              *float64 `json:"longitude"`
	PurchasePriceCents     *int64   `json:"purchasePrice"`
	TotalPriceCents        *int64   `json:"totalPrice"`
	TransportFeeCents      *int64   `json:"transportFee"`
	WarrantyBatteryExpDate string   `json:"warrantyBatteryExpDate"`
	WarrantyVehicleExpDate string   `json:"warrantyVehicleExpDate"`
	WarrantyBatteryMile    *int     `json:"warrantyBatteryMile"`
	WarrantyVehicleMile    *int     `json:"warrantyVehicleMile"`
	IsAvailable            *bool    `json:"isAvailable"`
	FirstSeenAt            string   `json:"firstSeenAt"`
	LastSeenAt             string   `json:"lastSeenAt"`
	TeslaURL               string   `json:"teslaUrl"`
	PriceHistory           []struct {
		Price     *int64 `json:"price"`
		ScrapedAt string `json:"scrapedAt"`
	} `json:"priceHistory"`
}

// LandedCents is the only money unit the derived commands use. A ceiling that
// ignores a real per-car transport fee is the wrong ceiling.
func (v Vehicle) LandedCents() *int64 {
	base := v.TotalPriceCents
	if base == nil {
		base = v.PurchasePriceCents
	}
	if base == nil {
		return nil
	}
	total := *base
	if v.TransportFeeCents != nil {
		total += *v.TransportFeeCents
	}
	return &total
}

func dollars(cents *int64) *float64 {
	if cents == nil {
		return nil
	}
	d := float64(*cents) / 100
	return &d
}

// vinsFromLinks pulls the distinct VINs out of the stored link rows.
func vinsFromLinks(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT COALESCE(json_extract(data,'$.url'),'') FROM resources WHERE resource_type = ?`,
		linkResourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var u sql.NullString
		if err := rows.Scan(&u); err != nil {
			continue
		}
		if m := vinRE.FindStringSubmatch(u.String); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out, rows.Err()
}

// loadVehicles reads every hydrated vehicle out of the local store.
func loadVehicles(ctx context.Context, db *sql.DB) ([]Vehicle, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type = ?`, vehicleResourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Vehicle
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid {
			continue
		}
		var v Vehicle
		if err := json.Unmarshal([]byte(raw.String), &v); err != nil {
			continue
		}
		if v.VIN != "" {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

func loadVehicle(ctx context.Context, db *sql.DB, vin string) (*Vehicle, error) {
	var raw sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT data FROM resources WHERE resource_type = ? AND id = ?`,
		vehicleResourceType, vin).Scan(&raw)
	if err == sql.ErrNoRows || !raw.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var v Vehicle
	if err := json.Unmarshal([]byte(raw.String), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// openMirror is the shared entry point for every derived command. It returns
// (nil, nil) when there is no mirror yet, so callers emit the sync hint instead
// of a SQLite error.
func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("teslatracker-pp-cli")
	}
	return store.OpenWithContext(ctx, dbPath)
}

// openMirrorRO is the opener for commands that only read. Opening read-only
// avoids taking a write lock and avoids the mmap being invalidated underneath
// us when another process writes the same file — which surfaced as a SIGBUS
// when the scorecard ran several commands against one store concurrently.
func openMirrorRO(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("teslatracker-pp-cli")
	}
	return store.OpenReadOnlyContext(ctx, dbPath)
}

func newHydrateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "hydrate",
		Short: "Fetch full vehicle records for every VIN found by sync",
		Long: "Reach for this after `sync`. Sync stores listing links; hydrate turns each " +
			"link into a full vehicle record with mileage, range, warranty dates and transport " +
			"fee. Every derived command (comps, degradation, warranty, premium, radius) reads " +
			"what hydrate writes, so an un-hydrated mirror makes those commands return nothing.",
		Example:     "  teslatracker-pp-cli hydrate --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hydrate vehicle records for stored inventory links")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("teslatracker-pp-cli")
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer st.Close()
			db := st.DB()

			vins, err := vinsFromLinks(ctx, db)
			if err != nil {
				return fmt.Errorf("reading stored links: %w", err)
			}
			if len(vins) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no inventory links in the local mirror\nrun: teslatracker-pp-cli sync\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), `{"hydrated":0,"failed":0,"vins":0}`)
				}
				return nil
			}
			if cliutil.IsDogfoodEnv() && limit > 3 {
				limit = 3
			}
			if limit > 0 && len(vins) > limit {
				vins = vins[:limit]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type failure struct {
				VIN   string `json:"vin"`
				Error string `json:"error"`
			}
			failures := make([]failure, 0)
			hydrated := 0

			for _, vin := range vins {
				raw, err := c.Get(ctx, "/api/inventory/"+vin, nil)
				if err != nil {
					failures = append(failures, failure{VIN: vin, Error: err.Error()})
					continue
				}
				// unwrap {"data": {...}}
				var env struct {
					Data json.RawMessage `json:"data"`
				}
				payload := raw
				if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
					payload = env.Data
				}
				var v Vehicle
				if err := json.Unmarshal(payload, &v); err != nil || v.VIN == "" {
					failures = append(failures, failure{VIN: vin, Error: "unparseable vehicle record"})
					continue
				}
				if _, err := db.ExecContext(ctx,
					`INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
					 VALUES (?, ?, ?, ?, ?)
					 ON CONFLICT(resource_type, id) DO UPDATE SET
					   data = excluded.data, updated_at = excluded.updated_at`,
					v.VIN, vehicleResourceType, string(payload), time.Now().UTC(), time.Now().UTC()); err != nil {
					failures = append(failures, failure{VIN: vin, Error: err.Error()})
					continue
				}
				recordPriceSnapshot(ctx, db, v)
				hydrated++
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d vehicle fetches failed; hydrated %d\n",
					len(failures), len(vins), hydrated)
			}
			view := map[string]any{
				"vins": len(vins), "hydrated": hydrated, "failed": len(failures),
			}
			if len(failures) > 0 {
				view["fetch_failures"] = failures
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "hydrated %d of %d vehicles\n", hydrated, len(vins))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum VINs to hydrate (0 = all)")
	return cmd
}

// recordPriceSnapshot keeps our own price series. The API serves a priceHistory
// array, but a departed listing takes its server history with it — and `gone`
// needs the price path of cars that are no longer listed.
func recordPriceSnapshot(ctx context.Context, db *sql.DB, v Vehicle) {
	_, _ = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS price_snapshot (
		vin TEXT NOT NULL,
		landed_cents INTEGER,
		observed_at DATETIME NOT NULL,
		PRIMARY KEY (vin, observed_at)
	)`)
	landed := v.LandedCents()
	if landed == nil {
		return
	}
	_, _ = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO price_snapshot (vin, landed_cents, observed_at) VALUES (?, ?, ?)`,
		v.VIN, *landed, time.Now().UTC().Format(time.RFC3339))
}
