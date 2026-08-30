// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: show what changed across entities between two sync snapshots.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// driftField is one field-level change between two snapshots.
type driftField struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// driftChange is a full entity-level change report.
type driftChange struct {
	EntityType string       `json:"entity_type"`
	EntityID   string       `json:"entity_id"`
	Name       string       `json:"name"`
	CapturedAt string       `json:"captured_at"`
	Changes    []driftField `json:"changes"`
}

// diffEntityFields compares two decoded entity snapshots and returns every
// watched field that changed. Watched: status, budget, bid, review status,
// creative, bidding_type.
func diffEntityFields(prev, cur map[string]any) []driftField {
	var out []driftField
	paths := []struct {
		field string
		path  []string
	}{
		{"status", []string{"status"}},
		{"bidding_type", []string{"bidding_type"}},
		{"daily_budget", []string{"budget", "daily_spend_limit_micros"}},
		{"lifetime_budget", []string{"budget", "lifetime_spend_limit_micros"}},
		{"max_bid", []string{"bidding_config", "max_bid_micros"}},
		{"review_status", []string{"review_status"}},
		{"creative_type", []string{"creative", "type"}},
		{"creative_title", []string{"creative", "title"}},
		{"creative_body", []string{"creative", "body"}},
		{"creative_target_url", []string{"creative", "target_url"}},
	}
	for _, p := range paths {
		pv := lookupJSONPath(prev, p.path)
		cv := lookupJSONPath(cur, p.path)
		ps, cs := scalarString(pv), scalarString(cv)
		if ps != cs {
			out = append(out, driftField{Field: p.field, From: ps, To: cs})
		}
	}
	return out
}

func lookupJSONPath(obj map[string]any, path []string) any {
	var cur any = obj
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		v, ok := m[seg]
		if !ok {
			return ""
		}
		cur = v
	}
	return cur
}

func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case json.Number:
		return t.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show what changed across campaigns, ad groups, and ads between two syncs.",
		Long: `Diff the two most recent entity snapshots (status, budget, bid, creative,
review) for campaigns, ad groups, and ads. --since limits to entities whose
latest snapshot falls in that window (e.g. 7d, 1w).`,
		Example: strings.Trim(`
  openai-ads-pp-cli drift --agent
  openai-ads-pp-cli drift --since 1w
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openStoreForRead(ctx, "openai-ads-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: run 'openai-ads-pp-cli sync' first to populate the local database.")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]driftChange, 0), flags)
				}
				return nil
			}
			defer db.Close()

			since := flagSince
			if since == "" {
				since = "7d"
			}
			dur, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			changes, err := loadDrift(db, dur)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), changes, flags)
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no field drift detected in the selected window")
				return nil
			}
			maps, merr := rowsToMaps(changes)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only report entities whose latest snapshot falls in this window (7d, 1w, 24h).")
	return cmd
}

// loadDrift diffs the two most recent snapshots of every entity, restricted to
// entities whose latest snapshot landed within the given window.
func loadDrift(db *store.Store, window time.Duration) ([]driftChange, error) {
	cutoff := time.Now().Add(-window).UTC()
	rows, err := db.Query(`SELECT entity_type, entity_id, captured_at, data
		FROM entity_snapshots ORDER BY entity_type, entity_id, captured_at`)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots: %w", err)
	}
	defer rows.Close()

	type snap struct {
		at   string
		data string
	}
	type entitySnaps struct {
		entityType string
		entityID   string
		latest     time.Time
		snaps      []snap
	}
	entities := map[string]*entitySnaps{}
	var order []string
	for rows.Next() {
		var et, eid, at, data sql.NullString
		if err := rows.Scan(&et, &eid, &at, &data); err != nil {
			return nil, err
		}
		key := et.String + "\x00" + eid.String
		if _, ok := entities[key]; !ok {
			entities[key] = &entitySnaps{entityType: et.String, entityID: eid.String}
			order = append(order, key)
		}
		if t, ok := parseSyncTS(at.String); ok && t.After(entities[key].latest) {
			entities[key].latest = t
		}
		entities[key].snaps = append(entities[key].snaps, snap{at: at.String, data: data.String})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	names := loadEntityNames(db)
	var result []driftChange
	for _, key := range order {
		e := entities[key]
		if e.latest.Before(cutoff) {
			continue
		}
		if len(e.snaps) < 2 {
			continue
		}
		prev := decodeSnapshot(e.snaps[len(e.snaps)-2].data)
		cur := decodeSnapshot(e.snaps[len(e.snaps)-1].data)
		changes := diffEntityFields(prev, cur)
		if len(changes) == 0 {
			continue
		}
		result = append(result, driftChange{
			EntityType: e.entityType,
			EntityID:   e.entityID,
			Name:       names[e.entityType+"\x00"+e.entityID],
			CapturedAt: e.snaps[len(e.snaps)-1].at,
			Changes:    changes,
		})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CapturedAt > result[j].CapturedAt })
	if result == nil {
		result = make([]driftChange, 0)
	}
	return result, nil
}

func decodeSnapshot(data string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(data), &m) != nil {
		return map[string]any{}
	}
	return m
}

// loadEntityNames returns a display name lookup keyed by entity_type + entity_id.
func loadEntityNames(db *store.Store) map[string]string {
	out := map[string]string{}
	rows, err := db.Query(`SELECT 'campaigns', id, name FROM campaigns`)
	if err == nil {
		for rows.Next() {
			var t, id, name sql.NullString
			if rows.Scan(&t, &id, &name) == nil {
				out[t.String+"\x00"+id.String] = name.String
			}
		}
		_ = rows.Close()
	}
	rows, err = db.Query(`SELECT 'ad-groups', id, name FROM ad_groups`)
	if err == nil {
		for rows.Next() {
			var t, id, name sql.NullString
			if rows.Scan(&t, &id, &name) == nil {
				out[t.String+"\x00"+id.String] = name.String
			}
		}
		_ = rows.Close()
	}
	rows, err = db.Query(`SELECT 'ads', id, name FROM ads`)
	if err == nil {
		for rows.Next() {
			var t, id, name sql.NullString
			if rows.Scan(&t, &id, &name) == nil {
				out[t.String+"\x00"+id.String] = name.String
			}
		}
		_ = rows.Close()
	}
	return out
}
