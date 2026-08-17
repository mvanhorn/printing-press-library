// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: diff a business's two most recent synced menu snapshots to
// catch price changes and added/removed services. Reads the local store only;
// each sync appends a timestamped snapshot. generate --force preserves this body.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/store"
	"github.com/spf13/cobra"
)

type menuPriceChange struct {
	ServiceID string `json:"service_id"`
	Title     string `json:"title,omitempty"`
	OldPrice  string `json:"old_price"`
	NewPrice  string `json:"new_price"`
	DeltaText string `json:"delta"`
}

type menuServiceRef struct {
	ServiceID string `json:"service_id"`
	Title     string `json:"title,omitempty"`
	Price     string `json:"price,omitempty"`
}

type menuDiffResult struct {
	Slug          string            `json:"slug"`
	BusinessID    string            `json:"business_id,omitempty"`
	OlderSnapshot string            `json:"older_snapshot,omitempty"`
	NewerSnapshot string            `json:"newer_snapshot,omitempty"`
	PriceChanges  []menuPriceChange `json:"price_changes"`
	Added         []menuServiceRef  `json:"added"`
	Removed       []menuServiceRef  `json:"removed"`
	ChangeCount   int               `json:"change_count"`
	Note          string            `json:"note,omitempty"`
}

// pp:data-source local
func newNovelMenuDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "menu-diff <slug>",
		Short: "Diff a business's service menu across synced snapshots to catch price changes and added/removed services.",
		Long: `Compare the two most recent menu snapshots for a business (each 'sync' appends
one) and report price changes, added services, and removed services.

Reads the local store only. Needs at least two syncs of the business to have
two snapshots to compare.`,
		Example:     "  vagaro-pp-cli menu-diff centralbarber",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "slug=centralbarber", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openStoreForRead(ctx, "vagaro-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w", err))
			}
			if db == nil {
				out := menuDiffResult{Slug: slug, PriceChanges: []menuPriceChange{}, Added: []menuServiceRef{}, Removed: []menuServiceRef{},
					Note: "no local store yet — run 'vagaro-pp-cli sync --slug " + slug + "' at least twice"}
				return emitVagaro(cmd, flags, out)
			}
			defer db.Close()

			businessID, err := db.GetBusinessIDBySlug(ctx, slug)
			if err != nil {
				return apiErr(err)
			}
			out := menuDiffResult{Slug: slug, BusinessID: businessID,
				PriceChanges: []menuPriceChange{}, Added: []menuServiceRef{}, Removed: []menuServiceRef{}}
			if businessID == "" {
				out.Note = "business not synced yet — run 'vagaro-pp-cli sync --slug " + slug + "' at least twice"
				return emitVagaro(cmd, flags, out)
			}

			times, err := db.RecentSnapshotTimes(ctx, businessID, 2)
			if err != nil {
				return apiErr(err)
			}
			if len(times) < 2 {
				out.Note = fmt.Sprintf("need >=2 syncs to diff (have %d snapshot(s)) — run 'vagaro-pp-cli sync --slug %s' again", len(times), slug)
				return emitVagaro(cmd, flags, out)
			}
			// RecentSnapshotTimes returns newest-first.
			newerAt, olderAt := times[0], times[1]
			out.NewerSnapshot = newerAt
			out.OlderSnapshot = olderAt

			older, err := db.SnapshotServices(ctx, businessID, olderAt)
			if err != nil {
				return apiErr(err)
			}
			newer, err := db.SnapshotServices(ctx, businessID, newerAt)
			if err != nil {
				return apiErr(err)
			}
			diffSnapshots(&out, older, newer)
			out.ChangeCount = len(out.PriceChanges) + len(out.Added) + len(out.Removed)
			if out.ChangeCount == 0 {
				out.Note = "no menu changes between the two most recent snapshots"
			}
			return emitVagaro(cmd, flags, out)
		},
	}
	return cmd
}

// diffSnapshots computes added/removed services and price changes between two
// menu snapshots keyed by service ID.
func diffSnapshots(out *menuDiffResult, older, newer []store.SnapshotRow) {
	oldByID := map[string]store.SnapshotRow{}
	for _, r := range older {
		oldByID[r.ServiceID] = r
	}
	newByID := map[string]store.SnapshotRow{}
	for _, r := range newer {
		newByID[r.ServiceID] = r
	}
	for _, n := range newer {
		o, ok := oldByID[n.ServiceID]
		if !ok {
			out.Added = append(out.Added, menuServiceRef{ServiceID: n.ServiceID, Title: n.Title, Price: dollarsFromCents(n.PriceCents)})
			continue
		}
		if o.PriceCents != n.PriceCents {
			out.PriceChanges = append(out.PriceChanges, menuPriceChange{
				ServiceID: n.ServiceID,
				Title:     n.Title,
				OldPrice:  dollarsFromCents(o.PriceCents),
				NewPrice:  dollarsFromCents(n.PriceCents),
				DeltaText: deltaText(o.PriceCents, n.PriceCents),
			})
		}
	}
	for _, o := range older {
		if _, ok := newByID[o.ServiceID]; !ok {
			out.Removed = append(out.Removed, menuServiceRef{ServiceID: o.ServiceID, Title: o.Title, Price: dollarsFromCents(o.PriceCents)})
		}
	}
	sort.SliceStable(out.PriceChanges, func(i, j int) bool { return out.PriceChanges[i].ServiceID < out.PriceChanges[j].ServiceID })
	sort.SliceStable(out.Added, func(i, j int) bool { return out.Added[i].ServiceID < out.Added[j].ServiceID })
	sort.SliceStable(out.Removed, func(i, j int) bool { return out.Removed[i].ServiceID < out.Removed[j].ServiceID })
}

func deltaText(oldC, newC int) string {
	d := newC - oldC
	sign := "+"
	if d < 0 {
		sign = "-"
		d = -d
	}
	return fmt.Sprintf("%s$%d.%02d", sign, d/100, d%100)
}
