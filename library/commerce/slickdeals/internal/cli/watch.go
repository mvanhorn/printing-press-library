// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// watch.go implements `slickdeals-pp-cli watch <deal-id>`. v0.2 only
// supports `--once`: a single frontpage RSS fetch, filtered by deal ID.
// The standalone-deal RSS pattern (newsearch.php?threadid=<id>&rss=1) is
// unreliable per the v0.2 handoff, so we filter the live frontpage feed
// client-side. If the deal isn't on the current frontpage we return
// notFoundErr — deal-page scraping is deferred to v0.3.
//
// `--persist` writes a DealSnapshot to the local SQLite store via raw SQL
// (the Snapshots-Analytics engineer's `store.InsertSnapshot` API isn't
// available yet; the integration pass will swap us over).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"
	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"

	"github.com/spf13/cobra"
)

// fetchWatchItem pulls the frontpage RSS feed and returns the item matching
// dealID, or notFoundErr when the deal isn't on the current frontpage. It's
// split out from RunE so tests can drive it against an httptest.NewServer
// without going through cobra or persistence.
func fetchWatchItem(ctx context.Context, hc *http.Client, feedURL, dealID string) (*rss.Item, error) {
	var items []rss.Item
	var err error
	if feedURL == "" {
		items, err = rss.LiveFrontpage(ctx, hc)
	} else {
		items, err = rss.FetchURL(ctx, feedURL, hc)
	}
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].DealID == dealID {
			return &items[i], nil
		}
	}
	return nil, notFoundErr(fmt.Errorf("deal %s not on current frontpage; v0.3 will support deal-page scraping", dealID))
}

// persistWatchSnapshot writes one DealSnapshot row to the local store via
// the canonical store.InsertSnapshot API (which owns schema management).
func persistWatchSnapshot(db *store.Store, item *rss.Item, capturedAt time.Time) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshaling rss item: %w", err)
	}
	snap := &store.DealSnapshot{
		DealID:     item.DealID,
		CapturedAt: capturedAt,
		Thumbs:     item.Thumbs,
		Merchant:   item.Merchant,
		Category:   item.Category,
		Title:      item.Title,
		Link:       item.Link,
		Raw:        string(raw),
	}
	if err := db.InsertSnapshot(snap); err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}
	return nil
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var once bool
	var persist bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "watch <deal-id>",
		Short:       "Fetch a single Slickdeals deal by ID from the live frontpage RSS feed",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Watch a Slickdeals deal: fetch the current frontpage RSS and report the
matching item. v0.2 supports --once only (single fetch + exit); background
polling is v0.3.

The standalone-deal RSS pattern (newsearch.php?threadid=<id>&rss=1) is
unreliable, so the deal must currently appear on the frontpage feed.
Use --persist to write the snapshot to the local SQLite store.`,
		Example: `  # Print one frontpage snapshot for deal 19510173
  slickdeals-pp-cli watch 19510173 --once --json

  # Same, but also persist the snapshot for later 'digest' queries
  slickdeals-pp-cli watch 19510173 --once --persist --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			dealID := args[0]
			if _, err := strconv.Atoi(dealID); err != nil {
				return usageErr(fmt.Errorf("deal-id must be numeric, got %q", dealID))
			}

			_ = once // currently always single-shot; v0.3 adds background polling

			item, err := fetchWatchItem(cmd.Context(), nil, "", dealID)
			if err != nil {
				return err
			}

			capturedAt := time.Now().UTC()
			if persist {
				if dbPath == "" {
					dbPath = defaultDBPath("slickdeals-pp-cli")
				}
				db, oerr := store.OpenWithContext(cmd.Context(), dbPath)
				if oerr != nil {
					return fmt.Errorf("opening local database: %w", oerr)
				}
				defer db.Close()
				if perr := persistWatchSnapshot(db, item, capturedAt); perr != nil {
					return perr
				}
			}

			raw, merr := json.Marshal(item)
			if merr != nil {
				return merr
			}
			envelope, werr := wrapWithProvenance(raw, DataProvenance{
				Source:       "live",
				SyncedAt:     &capturedAt,
				ResourceType: "watch",
			})
			if werr != nil {
				return werr
			}
			printProvenance(cmd, 1, DataProvenance{Source: "live", ResourceType: "watch"})
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(envelope), flags)
		},
	}

	cmd.Flags().BoolVar(&once, "once", true, "Fetch once and exit (v0.2 default; --once=false is v0.3)")
	cmd.Flags().BoolVar(&persist, "persist", false, "Persist the snapshot to the local SQLite store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slickdeals-pp-cli/data.db)")

	return cmd
}
