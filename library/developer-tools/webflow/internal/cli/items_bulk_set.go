// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type bulkSetChange struct {
	ItemID string            `json:"itemId"`
	Name   string            `json:"name"`
	Fields map[string]string `json:"fields"`
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
}

type bulkSetView struct {
	CollectionID string            `json:"collectionId"`
	Match        map[string]string `json:"match"`
	Set          map[string]string `json:"set"`
	ItemsScanned int               `json:"itemsScanned"`
	Matched      int               `json:"matched"`
	Applied      int               `json:"applied"`
	Failed       int               `json:"failed"`
	DryRun       bool              `json:"dryRun"`
	Source       string            `json:"selectionSource"`
	Changes      []bulkSetChange   `json:"changes"`
	Note         string            `json:"note,omitempty"`
}

func newNovelItemsBulkSetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var matchPairs []string
	var setPairs []string
	var apply bool
	var limit int
	var live bool
	var maxScanPages int

	cmd := &cobra.Command{
		Use:   "bulk-set [collection-id]",
		Short: "Set the same field value on many CMS items selected from the local mirror (previews the change set; writes only with --apply)",
		Long: strings.Trim(`
Use this command to apply the same field value to many CMS items selected by a
condition from the local mirror.

Do NOT use this command to check what a publish would change before running it;
use 'publish preview' instead. Do NOT use it to push the edited items live; use
the generated 'collections items publish' command instead.

Selection reads the local mirror when it holds this collection, and otherwise
fetches the collection's items from the API. Note that generated sync cannot
populate items on its own: 'sync --resources items' needs WEBFLOW_COLLECTION_ID
and covers one collection per run. Matching is exact string equality on fieldData keys, AND-combined across
every --match pair. There is no predicate language and no partial matching.

This command previews by default and changes nothing until you pass --apply.
Writes are paced against the API's own X-RateLimit-Remaining and Retry-After
headers, so a large batch survives the 60-request-per-minute floor.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match category=news --set category=updates
  webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --match status=draft --set author=editorial --apply
  webflow-pp-cli items bulk-set 580e63fc8c9a982ac9b8b745 --set featured=false --json
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--set=category=updates",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would select matching CMS items and preview field changes")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("items bulk-set selects from the local mirror and has no live selection path. Run 'webflow-pp-cli sync --resources items' then retry"))
			}
			collectionID := resolveCollectionID(args)
			if collectionID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a collection id is required (positionally or via WEBFLOW_COLLECTION_ID)"))
			}
			setValues, err := parseKeyValuePairs("set", setPairs)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if len(setValues) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--set is required (for example --set category=updates)"))
			}
			matchValues, err := parseKeyValuePairs("match", matchPairs)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "items")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}
			rows, err := lq.selectRaw(
				`SELECT "id", "collections_id", "data" FROM "items" WHERE "collections_id" = ?`, collectionID)
			if err != nil {
				return err
			}
			liveSelection := false
			if len(rows) == 0 && flags.dataSource != "local" {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				rows, err = lq.fetchPaged(c, "/collections/"+escapeSeg(collectionID)+"/items",
					"items", collectionID, maxScanPages)
				if err != nil {
					if note, degrade := liveFallbackErr(err, "this collection's items"); degrade {
						return emitEmptyLocal(cmd, flags, dbPath, "items", bulkSetView{
							CollectionID: collectionID, Match: matchValues, Set: setValues,
							Changes: make([]bulkSetChange, 0), DryRun: !apply,
							Source: "unavailable", Note: note,
						})
					}
					return fmt.Errorf("fetching collection items: %w", err)
				}
				liveSelection = true
			}

			view := selectBulkTargets(rows, collectionID, matchValues, setValues, limit)
			view.DryRun = !apply
			view.Source = "local-mirror"
			if liveSelection {
				view.Source = "api"
			}

			if !apply {
				if view.Matched > 0 {
					view.Note = fmt.Sprintf("preview only; re-run with --apply to write %d change(s) to the API", len(view.Changes))
				}
				return emitLocalResult(cmd, flags, view, func() { renderBulkSet(cmd, view) })
			}

			// Apply path: the local mirror chose the targets, the API is the
			// write sink. Every request goes through the generated client so
			// the adaptive limiter honors X-RateLimit-Remaining / Retry-After.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			base := "/collections/" + escapeSeg(collectionID) + "/items/"
			var firstErr error
			for i := range view.Changes {
				ch := &view.Changes[i]
				// Per-item live path is /items/{item_id}/live. The plural
				// /items/live is the *bulk* endpoint and takes an items array,
				// so suffixing the collection path would 404 every write.
				path := base + escapeSeg(ch.ItemID)
				if live {
					path += "/live"
				}
				body := map[string]any{"fieldData": stringMapToAny(ch.Fields)}
				if _, _, writeErr := c.Patch(ctx, path, body); writeErr != nil {
					ch.Status = "failed"
					ch.Error = writeErr.Error()
					view.Failed++
					if firstErr == nil {
						firstErr = writeErr
					}
					// The generated client surfaces exhausted 429s as
					// *client.APIError, not cliutil.RateLimitError (which is
					// declared but never constructed), so match on status.
					if isRateLimited(writeErr) {
						view.Note = "stopped early: the API rate limit was exhausted and retries did not recover. Re-run to continue where this left off."
						break
					}
					continue
				}
				ch.Status = "applied"
				view.Applied++
			}
			if view.Failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d writes failed; %d applied\n",
					view.Failed, len(view.Changes), view.Applied)
			}

			if err := emitLocalResult(cmd, flags, view, func() { renderBulkSet(cmd, view) }); err != nil {
				return err
			}
			if view.Failed > 0 {
				return classifyWriteError(fmt.Errorf("%d of %d item writes failed: %w",
					view.Failed, len(view.Changes), firstErr))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	cmd.Flags().StringArrayVar(&matchPairs, "match", nil, "Select items whose fieldData key equals this value; repeatable and AND-combined (for example --match category=news)")
	cmd.Flags().StringArrayVar(&setPairs, "set", nil, "Field value to write on every matched item; repeatable (for example --set category=updates)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the changes; without this the command only previews")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum items to change in one run")
	cmd.Flags().BoolVar(&live, "live", false, "Write to the published (live) item instead of the staged item")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "When selecting from the API rather than the mirror, maximum 100-item pages to scan")
	return cmd
}

// parseKeyValuePairs turns repeated key=value flags into a map, rejecting
// malformed input by naming the offending token.
func parseKeyValuePairs(flagName string, pairs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, p := range pairs {
		k, v, found := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		if !found || k == "" {
			return nil, fmt.Errorf("--%s expects key=value, got %q", flagName, p)
		}
		out[k] = v
	}
	return out, nil
}

func stringMapToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch strings.ToLower(v) {
		case "true":
			out[k] = true
		case "false":
			out[k] = false
		default:
			out[k] = v
		}
	}
	return out
}

// selectBulkTargets picks the items to change. Split out so it is testable
// without a store or an API.
func selectBulkTargets(rows []rawRow, collectionID string, match, set map[string]string, limit int) bulkSetView {
	view := bulkSetView{
		CollectionID: collectionID,
		Match:        match,
		Set:          set,
		Changes:      make([]bulkSetChange, 0, 16),
	}
	matched := make([]bulkSetChange, 0, 16)

	for _, r := range rows {
		decoded := decodeRows[wfItem]([]rawRow{r})
		if len(decoded) == 0 {
			continue
		}
		it := decoded[0]
		view.ItemsScanned++
		if it.archived() {
			continue
		}
		if !itemMatches(it, match) {
			continue
		}
		view.Matched++
		fields := make(map[string]string, len(set))
		for k, v := range set {
			fields[k] = v
		}
		matched = append(matched, bulkSetChange{
			ItemID: it.ID,
			Name:   it.name(),
			Fields: fields,
			Status: "pending",
		})
	}

	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })
	if limit > 0 && len(matched) > limit {
		view.Changes = matched[:limit]
		view.Note = fmt.Sprintf("%d items matched; this run is capped at %d by --limit", len(matched), limit)
	} else {
		view.Changes = matched
	}
	if view.Matched == 0 {
		view.Note = "no items matched; check the --match keys against the collection's field slugs, or run 'webflow-pp-cli sync --resources items' if the mirror is stale"
	}
	return view
}

// itemMatches applies every --match pair as exact string equality, AND-combined.
func itemMatches(it wfItem, match map[string]string) bool {
	for k, want := range match {
		raw, present := it.FieldData[k]
		if !present {
			return false
		}
		if fmt.Sprintf("%v", raw) != want {
			return false
		}
	}
	return true
}

func renderBulkSet(cmd *cobra.Command, view bulkSetView) {
	verb := "would change"
	if !view.DryRun {
		verb = "changed"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %d of %d scanned items in collection %s\n",
		verb, len(view.Changes), view.ItemsScanned, view.CollectionID)
	if view.Note != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
	}
	for _, ch := range view.Changes {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-40s %v\n", ch.Status, truncate(ch.Name, 40), ch.Fields)
	}
}
