// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored `sync` command: populates the local mirror (wantlist,
// collection, inventory) and captures marketplace price snapshots — the
// price-history substrate the Discogs API does not provide.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/store"

	"github.com/spf13/cobra"
)

type discogsPagination struct {
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

// syncPaginate walks a paginated Discogs list endpoint and returns the raw
// items under itemsKey across pages (bounded by maxPages).
func syncPaginate(ctx context.Context, c *client.Client, path, itemsKey string, maxPages int) ([]json.RawMessage, error) {
	var out []json.RawMessage
	for page := 1; page <= maxPages; page++ {
		params := map[string]string{"page": strconv.Itoa(page), "per_page": "100"}
		data, err := c.Get(ctx, path, params)
		if err != nil {
			return out, err
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(data, &env); err != nil {
			return out, fmt.Errorf("parsing %s: %w", path, err)
		}
		var items []json.RawMessage
		if raw, ok := env[itemsKey]; ok {
			_ = json.Unmarshal(raw, &items)
		}
		out = append(out, items...)
		var pag struct {
			Pagination discogsPagination `json:"pagination"`
		}
		_ = json.Unmarshal(data, &pag)
		if len(items) == 0 || page >= pag.Pagination.Pages {
			break
		}
	}
	return out, nil
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var resources string
	var userFlag string
	var maxPages int
	var snapshot string
	var currency string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync your wantlist, collection, and inventory to the local mirror and capture price snapshots.",
		Long: strings.Trim(`
Populate the local SQLite mirror from Discogs and record marketplace price
snapshots. The Discogs API keeps no price history, so repeated syncs are what
make 'fills', 'undervalued', 'portfolio', and 'comps' work over time.

--snapshot controls how many marketplace stat calls are made (each is one
request against the 60/min limit):
  limits     (default) snapshot only releases with a limit set (for 'fills')
  collection snapshot every release in your collection (for 'portfolio')
  all        limits + collection + wantlist
  none       populate the mirror only, no price calls
`, "\n"),
		Example: strings.Trim(`
  discogs-pp-cli sync --resources wantlist,collection
  discogs-pp-cli sync --snapshot collection
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would sync resources [%s] and snapshot prices [%s]\n", resources, snapshot)
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, err := openDiscogsStore(ctx, flags)
			if err != nil {
				return err
			}
			defer st.Close()

			username, err := resolveUsername(ctx, c, st, userFlag)
			if err != nil {
				return err
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			want := map[string]bool{}
			for _, r := range strings.Split(resources, ",") {
				want[strings.TrimSpace(strings.ToLower(r))] = true
			}

			type resSummary struct {
				Resource string `json:"resource"`
				Count    int    `json:"count"`
			}
			summary := []resSummary{}

			if want["wantlist"] {
				items, err := syncPaginate(ctx, c, fmt.Sprintf("/users/%s/wants", username), "wants", maxPages)
				if err != nil {
					return fmt.Errorf("syncing wantlist: %w", err)
				}
				n := upsertReleases(ctx, st, "wantlist", items)
				summary = append(summary, resSummary{"wantlist", n})
			}
			if want["collection"] {
				items, err := syncPaginate(ctx, c, fmt.Sprintf("/users/%s/collection/folders/0/releases", username), "releases", maxPages)
				if err != nil {
					return fmt.Errorf("syncing collection: %w", err)
				}
				n := upsertCollection(ctx, st, items)
				summary = append(summary, resSummary{"collection", n})
			}
			if want["inventory"] {
				items, err := syncPaginate(ctx, c, fmt.Sprintf("/users/%s/inventory", username), "listings", maxPages)
				if err != nil {
					// Inventory is seller-only; a 404/403 here is not fatal to the rest of the sync.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not sync inventory (%v)\n", err)
				} else {
					n := upsertListings(ctx, st, items)
					summary = append(summary, resSummary{"inventory", n})
				}
			}

			snapped := captureSyncSnapshots(ctx, cmd, c, st, snapshot, currency)

			view := struct {
				Username  string       `json:"username"`
				Synced    []resSummary `json:"synced"`
				Snapshots int          `json:"snapshots_captured"`
			}{Username: username, Synced: summary, Snapshots: snapped}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, s := range summary {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %d\n", s.Resource, s.Count)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshots    %d\n", snapped)
			return nil
		},
	}
	cmd.Flags().StringVar(&resources, "resources", "wantlist,collection,inventory", "Comma-separated resources to sync: wantlist, collection, inventory")
	cmd.Flags().StringVar(&userFlag, "user", "", "Discogs username (defaults to the authenticated user)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages to fetch per resource")
	cmd.Flags().StringVar(&snapshot, "snapshot", "limits", "Price-snapshot scope: limits, collection, all, none")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for price snapshots (e.g. USD)")
	return cmd
}

// upsertReleases upserts wantlist-style items keyed by release id.
func upsertReleases(ctx context.Context, st *store.Store, resourceType string, items []json.RawMessage) int {
	n := 0
	for _, raw := range items {
		var w wantItem
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		rid := w.ID
		if rid == 0 {
			rid = w.BasicInfo.ID
		}
		if rid == 0 {
			continue
		}
		if err := st.Upsert(resourceType, strconv.FormatInt(rid, 10), raw); err == nil {
			n++
		}
	}
	return n
}

// upsertCollection upserts collection items keyed by instance id (a release
// can appear as multiple instances/copies).
func upsertCollection(ctx context.Context, st *store.Store, items []json.RawMessage) int {
	n := 0
	for _, raw := range items {
		var it collectionItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		key := it.InstanceID
		if key == 0 {
			key = it.BasicInfo.ID
		}
		if key == 0 {
			continue
		}
		if err := st.Upsert("collection", strconv.FormatInt(key, 10), raw); err == nil {
			n++
		}
	}
	return n
}

// upsertListings upserts marketplace inventory listings keyed by listing id.
func upsertListings(ctx context.Context, st *store.Store, items []json.RawMessage) int {
	n := 0
	for _, raw := range items {
		var l struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &l); err != nil || l.ID == 0 {
			continue
		}
		if err := st.Upsert("inventory", strconv.FormatInt(l.ID, 10), raw); err == nil {
			n++
		}
	}
	return n
}

// captureSyncSnapshots records marketplace stats snapshots for the requested
// scope, bounded and dogfood-curtailed.
func captureSyncSnapshots(ctx context.Context, cmd *cobra.Command, c *client.Client, st *store.Store, scope, currency string) int {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "none" || scope == "" {
		return 0
	}
	ids := map[int64]bool{}

	addFromResource := func(resourceType string) {
		rows, err := st.DB().QueryContext(ctx, `SELECT id FROM resources WHERE resource_type = ?`, resourceType)
		if err != nil {
			return
		}
		var got []int64
		for rows.Next() {
			var idStr string
			if err := rows.Scan(&idStr); err != nil {
				continue
			}
			if v, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				got = append(got, v)
			}
		}
		_ = rows.Err()
		_ = rows.Close()
		for _, v := range got {
			ids[v] = true
		}
	}

	// limits are always release-ids; collection resource keys are instance-ids,
	// so pull release ids from the stored basic_information instead.
	addLimits := func() {
		rows, err := st.DB().QueryContext(ctx, `SELECT release_id FROM wantlist_limits`)
		if err != nil {
			return
		}
		var got []int64
		for rows.Next() {
			var v int64
			if err := rows.Scan(&v); err == nil {
				got = append(got, v)
			}
		}
		_ = rows.Err()
		_ = rows.Close()
		for _, v := range got {
			ids[v] = true
		}
	}
	addCollectionReleaseIDs := func() {
		rows, err := st.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'collection'`)
		if err != nil {
			return
		}
		var raws [][]byte
		for rows.Next() {
			var d []byte
			if err := rows.Scan(&d); err == nil {
				raws = append(raws, d)
			}
		}
		_ = rows.Err()
		_ = rows.Close()
		for _, d := range raws {
			var it collectionItem
			if err := json.Unmarshal(d, &it); err == nil && it.BasicInfo.ID != 0 {
				ids[it.BasicInfo.ID] = true
			}
		}
	}

	switch scope {
	case "limits":
		addLimits()
	case "collection":
		addCollectionReleaseIDs()
	case "all":
		addLimits()
		addCollectionReleaseIDs()
		addFromResource("wantlist")
	default:
		addLimits()
	}

	max := len(ids)
	if cliutil.IsDogfoodEnv() && max > 3 {
		max = 3
	}
	n := 0
	for rid := range ids {
		if n >= max {
			break
		}
		if _, err := captureSnapshot(ctx, c, st, rid, currency); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: snapshot failed for release %d: %v\n", rid, err)
			continue
		}
		n++
	}
	return n
}
