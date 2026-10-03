// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/cliutil"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerClientHook(client.AttachHostelworldApplication)
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newDestinationsCmd(flags))
		configurePlanningCache(root, flags)
		if unsupported := findSubcommand(root, "sync"); unsupported != nil {
			unsupported.Hidden = true
			unsupported.Short = "Initialize only the local cache; provider sync is unsupported"
			unsupported.Long = "This command initializes only the local SQLite cache. Hostelworld provider sync is unsupported. Save a normalized live observation with hostels inspect/offers --save, then read hostels saved. Saved dated prices are always stale until refreshed live."
			unsupported.Example = "  hostelworld-pp-cli hostels offers 67481 --check-in 2026-11-10 --check-out 2026-11-13 --save --agent"
			if unsupported.Annotations == nil {
				unsupported.Annotations = map[string]string{}
			}
			unsupported.Annotations["mcp:hidden"] = "true"
			unsupported.Annotations["pp:data-source"] = "local"
			unsupported.RunE = func(cmd *cobra.Command, args []string) error {
				if err := validateDataSourceStrategy(flags, "local"); err != nil {
					return usageErr(err)
				}
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "local cache initialization; provider sync unavailable")
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				path, _ := cmd.Flags().GetString("db")
				path, err := PlanningDBPath(ctx, path)
				if err != nil {
					return err
				}
				guard, err := hw.BeginCacheWrite(path)
				if err != nil {
					return err
				}
				db, err := store.OpenWithContext(ctx, guard.Path())
				if err != nil {
					return err
				}
				db.DB().SetMaxOpenConns(1)
				db.DB().SetMaxIdleConns(1)
				if err := guard.BindWriter(); err != nil {
					db.Close()
					return err
				}
				if err := db.Close(); err != nil {
					return err
				}
				if err := guard.CheckAfterWriterClose(); err != nil {
					return err
				}
				return flags.printJSON(cmd, map[string]any{"status": "local_cache_only", "provider_snapshot_refreshed": false, "population": "hostels inspect/offers --save", "read_saved": "hostels saved", "freshness": "saved prices remain stale", "database": path})
			}
		}
	})
}

type stayOptions struct {
	start, end, kind      string
	nights, guests, limit int
	free, save            bool
}

func stayFlags(cmd *cobra.Command, o *stayOptions) {
	cmd.Flags().StringVar(&o.start, "check-in", "", "Source-local check-in date in YYYY-MM-DD")
	cmd.Flags().StringVar(&o.end, "check-out", "", "Source-local check-out date, exclusive, in YYYY-MM-DD")
	cmd.Flags().IntVar(&o.nights, "nights", 0, "Stay length, 1–30 nights; alternative to --check-out")
	cmd.Flags().IntVar(&o.guests, "guests", 2, "Requested party size, 1–10 guests; source rules may limit it")
	cmd.Flags().StringVar(&o.kind, "kind", "all", "Room category: all, dorm, or private")
	cmd.Flags().BoolVar(&o.free, "free-cancellation", false, "Keep only rates with established free cancellation; conditional or unknown terms are excluded")
	cmd.Flags().IntVar(&o.limit, "limit", 30, "Maximum returned room plans, 1–100; source scan remains bounded")
	cmd.Flags().BoolVar(&o.save, "save", false, "Save normalized planning evidence in the local bounded SQLite cache")
}
func (o stayOptions) query() (hw.Query, error) {
	if o.kind != "all" && o.kind != "dorm" && o.kind != "private" {
		return hw.Query{}, fmt.Errorf("--kind must be all, dorm, or private")
	}
	if o.limit < 1 || o.limit > 100 {
		return hw.Query{}, fmt.Errorf("--limit must be 1–100")
	}
	return hw.NewQuery(o.start, o.end, o.nights, o.guests, time.Now())
}
func decoratePlanning(cmd *cobra.Command, flags *rootFlags) *cobra.Command {
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateDataSourceStrategy(flags, cmd.Annotations["pp:data-source"]); err != nil {
			return usageErr(err)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
		}
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.asJSON {
			return cmd.Help()
		}
		return original(cmd, args)
	}
	return cmd
}

func sourceObject(ctx context.Context, c *client.Client, path string, params map[string]string) (map[string]any, error) {
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return hw.Decode(data)
}
func propertyObject(ctx context.Context, c *client.Client, id string) (map[string]any, error) {
	if !hw.ValidID(id) {
		return nil, fmt.Errorf("property ID must be a positive source numeric ID")
	}
	v, err := sourceObject(ctx, c, "/legacy-hwapi-service/2.2/properties/"+id+"/", map[string]string{"application": "web"})
	if err != nil {
		return nil, err
	}
	if hw.Text(v["id"]) != id {
		return nil, fmt.Errorf("property response ID does not match requested source ID")
	}
	return v, nil
}
func offersObject(ctx context.Context, c *client.Client, id string, p map[string]any, q hw.Query, o stayOptions) (map[string]any, error) {
	v, err := sourceObject(ctx, c, "/legacy-hwapi-service/2.2/properties/"+id+"/availability/", q.Params())
	if err != nil {
		return nil, err
	}
	if hw.Text(v["id"]) != id {
		return nil, fmt.Errorf("availability response ID does not match requested source ID")
	}
	return hw.Availability(v, p, q, o.kind, o.free, time.Now())
}
func outputPlanning(cmd *cobra.Command, flags *rootFlags, v map[string]any, save bool) error {
	if save {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if err := savePlanning(ctx, v); err != nil {
			return fmt.Errorf("save planning snapshot: %w", err)
		}
	}
	return flags.printJSON(cmd, v)
}
func limitPlans(v map[string]any, limit int) {
	rows, _ := v["offers"].([]any)
	v["total_matching_plans"] = len(rows)
	v["truncated"] = len(rows) > limit
	if len(rows) > limit {
		v["offers"] = rows[:limit]
	}
}

func fatalRate(err error) bool {
	var rate *cliutil.RateLimitError
	var api *client.APIError
	return errors.As(err, &rate) || (errors.As(err, &api) && api.StatusCode == 429)
}

func savePlanning(ctx context.Context, v map[string]any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(data) > 2*1024*1024 {
		return fmt.Errorf("snapshot exceeds 2 MiB")
	}
	keyData := data
	h := sha256.Sum256(keyData)
	path, err := PlanningDBPath(ctx, "")
	if err != nil {
		return err
	}
	guard, err := hw.BeginCacheWrite(path)
	if err != nil {
		return err
	}
	db, err := store.OpenWithContext(ctx, guard.Path())
	if err != nil {
		return err
	}
	defer db.Close()
	db.DB().SetMaxOpenConns(1)
	db.DB().SetMaxIdleConns(1)
	if err := guard.BindWriter(); err != nil {
		return err
	}
	if err := db.Upsert("planning_snapshot", hex.EncodeToString(h[:]), data); err != nil {
		return err
	}
	if err := prunePlanning(ctx, db); err != nil {
		return err
	}
	if err := guard.CheckWriter(); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	return guard.CheckAfterWriterClose()
}

// PlanningDBPath keeps manual observations inside the verified client profile.
// A selected but unverified session must never fall back to the shared cache.
func PlanningDBPath(ctx context.Context, requested string) (string, error) {
	if session := platform.SessionFromContext(ctx); session != nil {
		if session.GateOutcome != platform.GateVerified || session.Paths.DataFile == "" {
			return "", fmt.Errorf("planning cache requires a verified profile data path")
		}
		if requested != "" && filepath.Clean(requested) != filepath.Clean(session.Paths.DataFile) {
			return "", fmt.Errorf("database override must match the selected profile data path")
		}
		return session.Paths.DataFile, nil
	}
	if requested != "" {
		return requested, nil
	}
	return defaultDBPath("hostelworld-pp-cli"), nil
}

// OpenPlanningReadOnly guards the complete immutable read and resolves aliases.
// Callers must check the returned guard after all queries, before emitting data.
func OpenPlanningReadOnly(ctx context.Context, requested string) (*store.Store, *hw.CacheGuard, error) {
	path, err := PlanningDBPath(ctx, requested)
	if err != nil {
		return nil, nil, err
	}
	guard, err := hw.BeginCacheRead(path)
	if err != nil {
		return nil, nil, err
	}
	if guard.Missing() {
		return nil, guard, nil
	}
	snapshot, err := guard.Snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	db, err := store.OpenReadOnlyContext(ctx, (&url.URL{Path: filepath.ToSlash(snapshot)}).EscapedPath())
	if err != nil {
		guard.Close()
		return nil, nil, err
	}
	return db, guard, nil
}

// Prune both representations atomically, including orphaned FTS rows from old
// copies. FTS uses its own deterministic rowids, not resources.rowid.
func prunePlanning(ctx context.Context, db *store.Store) error {
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM resources WHERE resource_type='planning_snapshot' AND rowid NOT IN (SELECT rowid FROM resources WHERE resource_type='planning_snapshot' ORDER BY rowid DESC LIMIT 200)"); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT f.rowid FROM resources_fts f WHERE f.resource_type='planning_snapshot' AND NOT EXISTS (SELECT 1 FROM resources r WHERE r.resource_type=f.resource_type AND r.id=f.id)")
	if err != nil {
		return err
	}
	var victims []int64
	for rows.Next() {
		var rowid int64
		if err := rows.Scan(&rowid); err != nil {
			rows.Close()
			return err
		}
		victims = append(victims, rowid)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for _, rowid := range victims {
		if _, err := tx.ExecContext(ctx, "DELETE FROM resources_fts WHERE rowid=?", rowid); err != nil {
			return err
		}
	}
	return tx.Commit()
}
