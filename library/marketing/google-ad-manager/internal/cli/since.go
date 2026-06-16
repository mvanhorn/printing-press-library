// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/store"
)

// entityRow is one mirrored resource considered by the since view. updateTime
// is the resource's own GAM updateTime (RFC3339) parsed at scan time; rows
// whose updateTime is unparseable carry the zero time and are excluded by the
// cutoff comparison.
type entityRow struct {
	ResourceType string    `json:"resource_type"`
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	UpdateTime   time.Time `json:"-"`
}

// changedEntity is the emitted shape for one added/changed resource.
type changedEntity struct {
	ResourceType string `json:"resource_type"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	UpdateTime   string `json:"update_time"`
}

type sinceView struct {
	Since   string          `json:"since"`
	Changed []changedEntity `json:"changed"`
	Scanned int             `json:"scanned"`
	Note    string          `json:"note,omitempty"`
}

// pp:data-source auto -- reads changed entities from the local mirror, and when
// a network code is set, force-refreshes the scanned resources live first so
// updateTime reflects current upstream state.
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagNetwork string
	var flagType string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "since",
		Short: "List entities added or changed since a cutoff, by updateTime",
		Long: `Surface the Ad Manager entities that were ADDED or CHANGED since a cutoff,
ranked newest first. It compares each resource's own updateTime against the
cutoff.

It reads the local mirror; when a --network code (or
$GOOGLE_AD_MANAGER_NETWORK_CODE) is provided it first refreshes the relevant
resources live and caches them, so the comparison reflects current upstream
state and leaves a snapshot for next time.

The cutoff comes from --since:
  last-sync   (default) the most recent sync time recorded in the local mirror.
              When no sync time is recorded yet, falls back to now minus 24h and
              says so in the "note" field.
  <duration>  a loose duration like 7d, 4w, 24h, or 90m; cutoff = now - duration.

HONEST LIMITATION: this detects ADDED and CHANGED entities by updateTime. It
CANNOT detect REMOVED entities — deletions leave no row and there is no snapshot
history to diff against, so a resource deleted upstream simply stops appearing.
Entities whose JSON has no updateTime cannot be dated and are not reported as
changed.`,
		Example:     "  google-ad-manager-pp-cli since --since 7d --network 123456 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				// since has a usable default (--since=last-sync), so an
				// argument-less, flagless invocation still has a sensible run.
				// Honor the verify-friendly help gate only when nothing at all
				// was provided AND we are attached to a terminal help context.
				if !flags.asJSON && !flags.agent {
					return cmd.Help()
				}
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list entities changed since the cutoff (live refresh via --network if set, else from the local mirror)")
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			networkCode, _ := resolveNetworkCode(flagNetwork)
			maxPages := 25
			if cliutil.IsDogfoodEnv() {
				maxPages = 2
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("google-ad-manager-pp-cli")
			}

			// Resource types to scan. Default to the order-graph entity family;
			// --type narrows to one kebab-case resource_type.
			scanTypes := []string{"ad-units", "orders", "line-items"}
			if t := strings.TrimSpace(flagType); t != "" {
				scanTypes = []string{t}
			}

			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer st.Close()

			// When a network code is available, force-refresh the scanned
			// resources live (bypassing the mirror-first path) so updateTime
			// reflects current upstream state and a fresh snapshot is cached.
			if networkCode != "" {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				for _, t := range scanTypes {
					items, ferr := gamFetchList(ctx, c, networkCode, gamCamelResource(t), maxPages)
					if ferr != nil {
						return classifyAPIError(ferr, flags)
					}
					gamCacheItems(st, t, items)
				}
			}

			cutoff, note, err := resolveSinceCutoff(ctx, st, flagSince, scanTypes)
			if err != nil {
				return err
			}

			rows, scanned, err := loadEntityRows(ctx, st, scanTypes, networkCode)
			if err != nil {
				return apiErr(err)
			}

			if scanned == 0 && networkCode == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror and no network code set.\npass --network <code> or set GOOGLE_AD_MANAGER_NETWORK_CODE to fetch live.\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			changed := filterChangedSince(rows, cutoff)

			view := sinceView{
				Since:   cutoff.UTC().Format(time.RFC3339),
				Changed: make([]changedEntity, 0, len(changed)),
				Scanned: scanned,
				Note:    note,
			}
			for _, r := range changed {
				view.Changed = append(view.Changed, changedEntity{
					ResourceType: r.ResourceType,
					ID:           r.ID,
					Name:         r.Name,
					UpdateTime:   r.UpdateTime.UTC().Format(time.RFC3339),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "last-sync", `cutoff: "last-sync" or a duration like 7d/24h/4w`)
	cmd.Flags().StringVar(&flagNetwork, "network", "", "network code to refresh from and restrict to (matched on the resource name prefix)")
	cmd.Flags().StringVar(&flagType, "type", "", "restrict to one resource_type (kebab-case, e.g. orders, ad-units, line-items)")
	cmd.Flags().StringVar(&flagDB, "db", "", "path to the local mirror SQLite file (defaults to the standard data dir)")
	return cmd
}

// resolveSinceCutoff turns the --since flag into an absolute cutoff time. For
// "last-sync" it reads the newest last_synced_at across the scanned resource
// types from sync_state; if none is recorded it falls back to now-24h and
// returns an explanatory note. Otherwise it parses a loose duration and
// returns now-duration.
func resolveSinceCutoff(ctx context.Context, st *store.Store, since string, scanTypes []string) (time.Time, string, error) {
	since = strings.TrimSpace(since)
	if since == "" || strings.EqualFold(since, "last-sync") {
		last, ok, err := lastSyncTime(ctx, st, scanTypes)
		if err != nil {
			return time.Time{}, "", apiErr(err)
		}
		if !ok {
			return time.Now().Add(-24 * time.Hour), "no sync time recorded in the local mirror; cutoff defaulted to now-24h. pass a --since duration (e.g. 7d) for an explicit window.", nil
		}
		return last, "", nil
	}

	dur, err := cliutil.ParseDurationLoose(since)
	if err != nil {
		return time.Time{}, "", usageErr(fmt.Errorf("invalid --since %q: use \"last-sync\" or a duration like 7d, 24h, 4w: %w", since, err))
	}
	if dur < 0 {
		dur = -dur
	}
	return time.Now().Add(-dur), "", nil
}

// lastSyncTime returns the most recent last_synced_at recorded for any of the
// scanned resource types. ok is false when no row carried a parseable
// timestamp (fresh mirror, or sync_state predates the column). The timestamp is
// stored as an RFC3339 UTC string by SaveSyncState.
func lastSyncTime(ctx context.Context, st *store.Store, scanTypes []string) (time.Time, bool, error) {
	if len(scanTypes) == 0 {
		return time.Time{}, false, nil
	}
	placeholders := make([]string, len(scanTypes))
	args := make([]any, len(scanTypes))
	for i, t := range scanTypes {
		placeholders[i] = "?"
		args[i] = t
	}
	q := fmt.Sprintf(
		`SELECT last_synced_at FROM sync_state WHERE resource_type IN (%s) AND last_synced_at IS NOT NULL`,
		strings.Join(placeholders, ","),
	)
	rows, err := st.DB().QueryContext(ctx, q, args...)
	if err != nil {
		// A missing sync_state table or column is not fatal here — treat it as
		// "no recorded sync time" so the caller falls back to the 24h window.
		return time.Time{}, false, nil
	}
	defer rows.Close()

	var newest time.Time
	found := false
	for rows.Next() {
		var ts sql.NullString
		if err := rows.Scan(&ts); err != nil {
			continue
		}
		if !ts.Valid || strings.TrimSpace(ts.String) == "" {
			continue
		}
		if parsed, ok := parseUpdateTime(ts.String); ok {
			if !found || parsed.After(newest) {
				newest = parsed
				found = true
			}
		}
	}
	return newest, found, rows.Err()
}

// loadEntityRows reads the scanned resource types from the generic resources
// table, projecting each row's id, displayName, resource name, and updateTime.
// When network is set, only rows whose resource name sits under
// networks/{code}/ are kept. scanned counts every row examined (pre-cutoff).
func loadEntityRows(ctx context.Context, st *store.Store, scanTypes []string, network string) ([]entityRow, int, error) {
	if len(scanTypes) == 0 {
		return nil, 0, nil
	}
	placeholders := make([]string, len(scanTypes))
	args := make([]any, len(scanTypes))
	for i, t := range scanTypes {
		placeholders[i] = "?"
		args[i] = t
	}
	// json_extract pulls the GAM updateTime, displayName, and resource name out
	// of the stored JSON. updateTime is the documented RFC3339 mutation
	// timestamp on Order/AdUnit (and most GAM resources); LineItem omits it in
	// this spec, so those rows carry the zero time and are excluded by cutoff.
	q := fmt.Sprintf(`
		SELECT resource_type, id,
		       json_extract(data, '$.updateTime')  AS update_time,
		       json_extract(data, '$.displayName') AS display_name,
		       json_extract(data, '$.name')        AS resource_name
		FROM resources
		WHERE resource_type IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := st.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	networkPrefix := ""
	if n := strings.TrimSpace(network); n != "" {
		networkPrefix = networkParent(n) + "/"
	}

	var out []entityRow
	scanned := 0
	for rows.Next() {
		var rt, id string
		var updateTime, displayName, resourceName sql.NullString
		if err := rows.Scan(&rt, &id, &updateTime, &displayName, &resourceName); err != nil {
			continue
		}
		scanned++
		if networkPrefix != "" {
			if !resourceName.Valid || !strings.HasPrefix(resourceName.String, networkPrefix) {
				continue
			}
		}
		row := entityRow{
			ResourceType: rt,
			ID:           id,
			Name:         displayName.String,
		}
		if updateTime.Valid {
			if t, ok := parseUpdateTime(updateTime.String); ok {
				row.UpdateTime = t
			}
		}
		out = append(out, row)
	}
	return out, scanned, rows.Err()
}

// filterChangedSince keeps rows whose updateTime is at or after cutoff, sorted
// newest first. Rows with a zero updateTime (missing/unparseable) are excluded:
// an undated resource cannot be claimed to have changed within the window. This
// is the pure, side-effect-free helper the tests cover.
func filterChangedSince(rows []entityRow, cutoff time.Time) []entityRow {
	out := make([]entityRow, 0, len(rows))
	for _, r := range rows {
		if r.UpdateTime.IsZero() {
			continue
		}
		if r.UpdateTime.Before(cutoff) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdateTime.After(out[j].UpdateTime)
	})
	return out
}

// parseUpdateTime parses a GAM timestamp. RFC3339 (with optional fractional
// seconds) is the documented shape; a couple of tolerant fallbacks cover
// timestamps that arrive without a zone or with a space separator.
func parseUpdateTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
