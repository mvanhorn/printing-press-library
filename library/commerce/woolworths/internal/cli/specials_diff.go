// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/pricehist"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
	"github.com/spf13/cobra"
)

// The specials-group browse endpoint is the category browse endpoint: this
// command posts to wowCategoryPath and decodes wowCategoryResponse, the same
// wire types swap, multibuy and basket use.

// ppwSpecialsGroups are the top-level specials group node ids Woolworths
// exposes, in the order the human report lists them.
//
// Verified against the live category tree on 2026-08-23. The tree contains
// ~140 specialsgroup.* nodes, but all except these are per-department children
// (e.g. specialsgroup.3676.1_39FD49C "Pantry" nests under Half Price), so
// diffing them as well would double-count every product.
//
// Three further top-level nodes exist and are deliberately omitted because they
// held zero products at authoring time and are not plain catalogue groups:
// specialsgroup.everydaymarket (third-party marketplace), specialsgroup.onlineonly
// (delivery-only), and specialsgroup.-2 ("My Specials and Offers", personalised
// and therefore session-dependent). Add them here if they ever carry stock.
var ppwSpecialsGroups = []struct{ Node, Label string }{
	{"specialsgroup.3676", "Half Price"},
	{"specialsgroup.3673", "Everyday Low Price"},
	{"specialsgroup.3668", "Buy More Save More"},
	{"specialsgroup.3694", "Lower Shelf Price"},
	{"specialsgroup.3721", "Seasonal Price"},
	{"specialsgroup.3704", "Bundles"},
}

// ppwMaxRefreshPages bounds a --refresh sweep. A specials group can run to
// thousands of products; a snapshot is capped so one refresh cannot turn into
// a hundred requests, and the row records when the cap truncated it so the
// diff is never silently taken against a partial snapshot.
const ppwMaxRefreshPages = 10

func ppwGroupLabel(node string) string {
	for _, g := range ppwSpecialsGroups {
		if g.Node == node {
			return g.Label
		}
	}
	return node
}

// specialsDiffItem is one product that entered or left a group.
type specialsDiffItem struct {
	Stockcode string  `json:"stockcode"`
	Name      string  `json:"name,omitempty"`
	Price     float64 `json:"price,omitempty"`
	WasPrice  float64 `json:"was_price,omitempty"`
	// DaysSinceLastSeen is how long this product had been absent from the
	// group before it reappeared, measured from the newest earlier snapshot
	// that contained it. Nil means it was never recorded in this group before.
	DaysSinceLastSeen *float64 `json:"days_since_last_seen"`
	LastSeenAt        string   `json:"last_seen_at,omitempty"`
}

// specialsDiffRow is one group's movement between its two most recent snapshots.
type specialsDiffRow struct {
	GroupNode       string             `json:"group_node"`
	GroupLabel      string             `json:"group_label"`
	PreviousAt      string             `json:"previous_snapshot_at"`
	CurrentAt       string             `json:"current_snapshot_at"`
	PreviousAtUnix  int64              `json:"previous_snapshot_at_unix"`
	CurrentAtUnix   int64              `json:"current_snapshot_at_unix"`
	SnapshotGapDays float64            `json:"snapshot_gap_days"`
	Entered         int                `json:"entered"`
	Left            int                `json:"left"`
	Stayed          int                `json:"stayed"`
	EnteredItems    []specialsDiffItem `json:"entered_items"`
	LeftItems       []specialsDiffItem `json:"left_items"`
	// Truncated means only the DISPLAYED item lists were cut, by --limit. The
	// entered/left/stayed counts above are still complete.
	Truncated bool `json:"item_lists_truncated"`
	// SnapshotTruncated means the --refresh sweep hit its page cap, so the
	// snapshot this diff was computed against covers only part of the group.
	// That is a different and far more serious thing than Truncated: the COUNTS
	// are then unreliable, because relevance drift alone makes products appear
	// to enter and leave when the sweep is partial.
	SnapshotTruncated bool   `json:"snapshot_truncated"`
	SnapshotNote      string `json:"snapshot_note,omitempty"`
	Note              string `json:"note,omitempty"`
}

func newNovelSpecialsDiffCmd(flags *rootFlags) *cobra.Command {
	var group string
	var refresh bool
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "specials-diff",
		Short: "Shows what entered and left each specials group since the last recorded snapshot",
		Long: strings.Trim(`
Compares the two most recent recorded snapshots of each Woolworths specials
group and reports how many products entered, left, and stayed - plus the item
lists, with each entrant annotated by how long it had been absent from that
group.

A group with only one recorded snapshot is SKIPPED, not reported as though
every product just entered: with nothing to compare against, "entered" would be
an artefact of when you first synced rather than a fact about the shelf.

Local-only by default. --refresh first fetches the current membership live and
records it as a new snapshot, then diffs.
`, "\n"),
		Example: strings.Trim(`
  woolworths-pp-cli specials-diff --agent --select entered_items.name,entered_items.days_since_last_seen
  woolworths-pp-cli specials-diff --group specialsgroup.3676 --limit 50
  woolworths-pp-cli specials-diff --refresh --json
`, "\n"),
		Annotations: map[string]string{
			// Empty is a valid answer, not an error: a group with fewer than two recorded snapshots, or a group whose membership did not move, is honestly empty.
			// Inventing an "empty means not found" rule would break honest-empty output.
			"pp:no-error-path-probe": "true",
			"mcp:read-only":          "true",
			"pp:data-source":         "local",
			"pp:happy-args":          "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No positional argument and no required flag: a bare invocation
			// is a complete request (diff every group), so it runs rather than
			// printing help. A stray positional is still a usage error.
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("specials-diff takes no positional arguments; got %q", args[0]))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "specials-diff")
			}
			if refresh {
				// --refresh IS the network fetch, so it cannot honour an
				// explicit --data-source local. Saying so is the honest
				// answer; silently letting --refresh win would make an
				// explicit local-only request fetch anyway.
				if flags != nil && flags.dataSource == "local" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--refresh conflicts with --data-source local: --refresh fetches the current membership from the API, which a local-only run must not do; drop one of the two"))
				}
			} else {
				// Without --refresh this command never touches the network.
				if err := validateDataSourceStrategy(flags, "local"); err != nil {
					return usageErr(err)
				}
			}
			if limit <= 0 {
				limit = 20
			}
			groups := make([]string, 0, len(ppwSpecialsGroups))
			if strings.TrimSpace(group) != "" {
				groups = append(groups, strings.TrimSpace(group))
			} else {
				for _, g := range ppwSpecialsGroups {
					groups = append(groups, g.Node)
				}
			}
			resolvedDB := ppwDBPath(dbPath)

			// Missing-mirror guard. Skipped for --refresh, which is the one
			// mode that legitimately creates the mirror it then reads.
			if !refresh {
				if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: woolworths-pp-cli specials-diff --refresh --db %s to record the first snapshot\n", resolvedDB, resolvedDB)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), make([]specialsDiffRow, 0), flags)
					}
					return nil
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, resolvedDB)
			if err != nil {
				return fmt.Errorf("opening local mirror %s: %w", resolvedDB, err)
			}
			defer db.Close()
			if err := store.EnsureWoolworthsTables(ctx, db.DB()); err != nil {
				return err
			}

			notes := make([]string, 0, len(groups)+1)
			truncatedGroups := map[string]bool{}
			if refresh {
				for _, node := range groups {
					truncated, err := specialsDiffRefreshGroup(ctx, cmd, flags, db.DB(), node)
					if err != nil {
						return err
					}
					truncatedGroups[node] = truncated
					if truncated {
						notes = append(notes, fmt.Sprintf("note: %s snapshot capped at %d pages (first %d products); the DIFF for this group is therefore computed against a partial snapshot and its entered/left counts are not trustworthy - see snapshot_truncated",
							node, ppwMaxRefreshPages, ppwMaxRefreshPages*wowMaxPageSize))
					}
				}
			}

			rows := make([]specialsDiffRow, 0, len(groups))
			for _, node := range groups {
				times, err := store.SpecialsSnapshotTimes(ctx, db.DB(), node, 2)
				if err != nil {
					return err
				}
				if len(times) < 2 {
					notes = append(notes, fmt.Sprintf("note: %s (%s) has %d recorded snapshot(s); two are needed to diff, so it is skipped rather than reported as an all-new group",
						node, ppwGroupLabel(node), len(times)))
					continue
				}
				row, err := specialsDiffForGroup(ctx, db.DB(), node, times[0], times[1], limit)
				if err != nil {
					return err
				}
				if truncatedGroups[node] {
					row.SnapshotTruncated = true
					row.SnapshotNote = fmt.Sprintf("the --refresh sweep stopped at the %d-page cap, so this diff was computed against a PARTIAL snapshot of %s: entered/left counts include products that were simply never fetched, not products that moved",
						ppwMaxRefreshPages, node)
				}
				rows = append(rows, row)
			}

			// Output mode first: an empty diff must serialise as [] for every
			// machine caller before any human sentence is considered.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range notes {
					fmt.Fprintln(cmd.ErrOrStderr(), n)
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no specials group has two recorded snapshots yet - nothing to diff")
				for _, n := range notes {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "run: woolworths-pp-cli specials-diff --refresh (twice, hours or days apart) to build a comparison")
				return nil
			}
			headers := []string{"GROUP", "LABEL", "PREVIOUS", "CURRENT", "GAP DAYS", "ENTERED", "LEFT", "STAYED"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.GroupNode,
					r.GroupLabel,
					time.Unix(r.PreviousAtUnix, 0).UTC().Format("2006-01-02 15:04"),
					time.Unix(r.CurrentAtUnix, 0).UTC().Format("2006-01-02 15:04"),
					fmt.Sprintf("%.1f", r.SnapshotGapDays),
					fmt.Sprintf("%d", r.Entered),
					fmt.Sprintf("%d", r.Left),
					fmt.Sprintf("%d", r.Stayed),
				})
			}
			if err := flags.printTable(cmd, headers, table); err != nil {
				return err
			}
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%s)\n", r.GroupLabel, r.GroupNode)
				if len(r.EnteredItems) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  entered: none")
				}
				for _, it := range r.EnteredItems {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %-9s $%-7.2f %s%s\n", it.Stockcode, it.Price, ppwTruncate(it.Name, 46), specialsDiffAbsence(it))
				}
				if len(r.LeftItems) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  left: none")
				}
				for _, it := range r.LeftItems {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %-9s $%-7.2f %s\n", it.Stockcode, it.Price, ppwTruncate(it.Name, 46))
				}
				if r.Truncated {
					fmt.Fprintln(cmd.OutOrStdout(), "  (item lists truncated by --limit; the entered/left counts above are complete)")
				}
				if r.SnapshotTruncated {
					fmt.Fprintf(cmd.OutOrStdout(), "  WARNING: %s\n", r.SnapshotNote)
				}
			}
			for _, n := range notes {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "Specials group node id to diff (default: all six catalogue groups)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Fetch current membership live and record a new snapshot before diffing")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum entered/left items listed per group")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite mirror (default: the CLI data directory)")
	return cmd
}

func specialsDiffAbsence(it specialsDiffItem) string {
	if it.DaysSinceLastSeen == nil {
		return "  (never recorded in this group before)"
	}
	return fmt.Sprintf("  (last in this group %.1f days ago)", *it.DaysSinceLastSeen)
}

// specialsDiffForGroup diffs two snapshots of one group and annotates each
// entrant with how long it had been absent.
func specialsDiffForGroup(ctx context.Context, db *sql.DB, node string, currentAt, previousAt int64, limit int) (specialsDiffRow, error) {
	row := specialsDiffRow{
		GroupNode:      node,
		GroupLabel:     ppwGroupLabel(node),
		PreviousAt:     ppwUnixString(previousAt),
		CurrentAt:      ppwUnixString(currentAt),
		PreviousAtUnix: previousAt,
		CurrentAtUnix:  currentAt,
		EnteredItems:   make([]specialsDiffItem, 0),
		LeftItems:      make([]specialsDiffItem, 0),
	}
	row.SnapshotGapDays = pricehist.Round1(float64(currentAt-previousAt) / 86400.0)

	current, err := store.SpecialsGroupAt(ctx, db, node, currentAt)
	if err != nil {
		return row, err
	}
	previous, err := store.SpecialsGroupAt(ctx, db, node, previousAt)
	if err != nil {
		return row, err
	}
	prevIndex := make(map[string]store.SpecialsMember, len(previous))
	for _, m := range previous {
		prevIndex[m.Stockcode] = m
	}
	curIndex := make(map[string]store.SpecialsMember, len(current))
	for _, m := range current {
		curIndex[m.Stockcode] = m
	}

	entered := make([]store.SpecialsMember, 0)
	for _, m := range current {
		if _, ok := prevIndex[m.Stockcode]; ok {
			row.Stayed++
			continue
		}
		entered = append(entered, m)
	}
	left := make([]store.SpecialsMember, 0)
	for _, m := range previous {
		if _, ok := curIndex[m.Stockcode]; !ok {
			left = append(left, m)
		}
	}
	row.Entered, row.Left = len(entered), len(left)

	// Absence history for the entrants, read in ONE query and fully drained
	// before it is consulted — never a per-item query issued while iterating a
	// parent result set.
	lastSeen, err := ppwLastSeenBefore(ctx, db, node, previousAt)
	if err != nil {
		return row, err
	}

	sort.Slice(entered, func(i, j int) bool { return entered[i].Stockcode < entered[j].Stockcode })
	sort.Slice(left, func(i, j int) bool { return left[i].Stockcode < left[j].Stockcode })

	for i, m := range entered {
		if i >= limit {
			row.Truncated = true
			break
		}
		item := specialsDiffItem{
			Stockcode: m.Stockcode,
			Name:      m.Name,
			Price:     pricehist.Round2(m.Price),
			WasPrice:  pricehist.Round2(m.WasPrice),
		}
		if ts, ok := lastSeen[m.Stockcode]; ok {
			days := pricehist.Round1(float64(currentAt-ts) / 86400.0)
			item.DaysSinceLastSeen = &days
			item.LastSeenAt = ppwUnixString(ts)
		}
		row.EnteredItems = append(row.EnteredItems, item)
	}
	for i, m := range left {
		if i >= limit {
			row.Truncated = true
			break
		}
		row.LeftItems = append(row.LeftItems, specialsDiffItem{
			Stockcode: m.Stockcode,
			Name:      m.Name,
			Price:     pricehist.Round2(m.Price),
			WasPrice:  pricehist.Round2(m.WasPrice),
		})
	}
	if row.Entered == 0 && row.Left == 0 {
		row.Note = "membership unchanged between the two recorded snapshots"
	}
	return row, nil
}

// ppwLastSeenBefore returns, for every product ever recorded in a group before
// the given cutoff, the newest such observation time.
//
// Drain-first and NULL-safe: the full result set is scanned into a map and
// closed before the caller uses it.
func ppwLastSeenBefore(ctx context.Context, db *sql.DB, node string, before int64) (map[string]int64, error) {
	if db == nil {
		return nil, fmt.Errorf("specials last-seen: nil database handle")
	}
	out := make(map[string]int64)
	rows, err := db.QueryContext(ctx, `
		SELECT stockcode, MAX(observed_at) AS last_at
		FROM specials_membership
		WHERE group_node = ? AND observed_at < ?
		GROUP BY stockcode`, node, before)
	if err != nil {
		return nil, fmt.Errorf("specials last-seen: query: %w", err)
	}
	for rows.Next() {
		var (
			code   sql.NullString
			lastAt sql.NullInt64
		)
		if err := rows.Scan(&code, &lastAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("specials last-seen: scan: %w", err)
		}
		if code.Valid && lastAt.Valid {
			out[code.String] = lastAt.Int64
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("specials last-seen: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("specials last-seen: close: %w", err)
	}
	return out, nil
}

// specialsDiffRefreshGroup fetches a group's current membership and records it
// as one snapshot. Reports whether the page cap truncated the sweep.
func specialsDiffRefreshGroup(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *sql.DB, node string) (bool, error) {
	c, err := flags.newClient()
	if err != nil {
		return false, err
	}
	// Akamai drops unwarmed POST connections; one GET of /shop primes the jar.
	c.WarmSession(ctx)
	observedAt := time.Now().Unix()
	members := make([]store.SpecialsMember, 0, wowMaxPageSize)
	observations := make([]store.PriceObservation, 0, wowMaxPageSize)
	seen := map[string]bool{}
	truncated := false
	total := -1

	for page := 1; page <= ppwMaxRefreshPages; page++ {
		body := map[string]any{
			"categoryId": node,
			"pageNumber": page,
			"pageSize":   wowMaxPageSize,
			"sortType":   "TraderRelevance",
			// The endpoint rejects a body without url/formatObject with an
			// HTTP 400 "Url is required" — they are not optional despite
			// having no effect on which products come back.
			"url":                       "/shop/browse/specials",
			"location":                  "/shop/browse/specials",
			"formatObject":              `{"name":"Specials"}`,
			"isSpecial":                 true,
			"isBundle":                  false,
			"isMobile":                  false,
			"filters":                   []any{},
			"token":                     "",
			"gpBoost":                   0,
			"isHideUnavailableProducts": false,
			"enableAdReRanking":         false,
			"groupEdmVariants":          true,
			"categoryVersion":           "v2",
		}
		data, _, err := c.PostQueryWithParams(ctx, wowCategoryPath, map[string]string{}, body)
		if err != nil {
			return truncated, classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		var parsed wowCategoryResponse
		if err := json.Unmarshal(data, &parsed); err != nil {
			return truncated, fmt.Errorf("parsing browse response for %s: %w", node, err)
		}
		if total < 0 {
			total = parsed.TotalRecordCount
		}
		pageCount := 0
		for _, bundle := range parsed.Bundles {
			for _, tile := range bundle.Products {
				code := tile.stockcode()
				if code == "" || seen[code] {
					continue
				}
				seen[code] = true
				pageCount++
				members = append(members, store.SpecialsMember{
					GroupNode:  node,
					Stockcode:  code,
					ObservedAt: observedAt,
					Name:       tile.Name,
					Price:      tile.Price,
					WasPrice:   tile.WasPrice,
				})
				observations = append(observations, tile.observation(observedAt))
			}
		}
		if pageCount == 0 {
			break
		}
		if total >= 0 && len(seen) >= total {
			break
		}
		if page == ppwMaxRefreshPages && total > len(seen) {
			truncated = true
		}
	}

	if len(members) == 0 {
		return truncated, nil
	}
	// Two separate write transactions, sequentially: RecordSpecialsMembership
	// and RecordPriceObservations each open their own, and SQLite WAL allows a
	// single writer.
	if err := store.RecordSpecialsMembership(ctx, db, members); err != nil {
		return truncated, err
	}
	if err := store.RecordPriceObservations(ctx, db, observations); err != nil {
		return truncated, err
	}
	return truncated, nil
}
