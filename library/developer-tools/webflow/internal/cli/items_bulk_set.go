// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/internal/cliutil"
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
	Skipped      int               `json:"skipped"`
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

			// Field types decide how a --set value is coerced. Without consulting
			// the schema, a plain-text field whose value happens to be the literal
			// string "true" or "false" would be sent as a JSON boolean and rejected
			// (or silently mistyped) by Webflow; only a Switch field should coerce.
			fieldTypes := collectionFieldTypes(lq, c, collectionID)

			// A prior run that stopped on an exhausted rate limit leaves a resume
			// file recording which item IDs already got written. Re-running the
			// identical command (same collection, --match, --set) skips those
			// instead of resending them, so forward progress reaches the
			// untouched tail instead of redoing the same head of the sorted list.
			signature := bulkSetResumeSignature(collectionID, matchValues, setValues)
			skip := loadBulkSetResume(signature)
			appliedIDs := make([]string, 0, len(view.Changes))

			base := "/collections/" + escapeSeg(collectionID) + "/items/"
			var firstErr error
			rateLimited := false
			for i := range view.Changes {
				ch := &view.Changes[i]
				if skip[ch.ItemID] {
					ch.Status = "skipped"
					view.Skipped++
					appliedIDs = append(appliedIDs, ch.ItemID)
					continue
				}
				// Per-item live path is /items/{item_id}/live. The plural
				// /items/live is the *bulk* endpoint and takes an items array,
				// so suffixing the collection path would 404 every write.
				path := base + escapeSeg(ch.ItemID)
				if live {
					path += "/live"
				}
				body := map[string]any{"fieldData": stringMapToAny(ch.Fields, fieldTypes)}
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
						rateLimited = true
						break
					}
					continue
				}
				ch.Status = "applied"
				view.Applied++
				appliedIDs = append(appliedIDs, ch.ItemID)
			}

			if rateLimited {
				saveBulkSetResume(signature, appliedIDs)
			} else {
				clearBulkSetResume(signature)
			}

			var noteParts []string
			if view.Note != "" {
				noteParts = append(noteParts, view.Note)
			}
			if view.Skipped > 0 {
				noteParts = append(noteParts, fmt.Sprintf(
					"skipped %d item(s) already applied by a previous run", view.Skipped))
			}
			if rateLimited {
				noteParts = append(noteParts,
					"stopped early: the API rate limit was exhausted and retries did not recover. Re-run the identical command to continue where this left off.")
			}
			view.Note = strings.Join(noteParts, "; ")

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

// stringMapToAny converts each --set value to the JSON type its field
// actually needs. Only a field whose collection schema type is "Switch"
// (Webflow's boolean field) coerces "true"/"false" to a JSON boolean; every
// other field, including one with no known type, keeps the literal string so
// a plain-text field valued "true" or "false" is never silently retyped.
func stringMapToAny(m map[string]string, fieldTypes map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if fieldTypes[k] == "Switch" {
			switch strings.ToLower(v) {
			case "true":
				out[k] = true
				continue
			case "false":
				out[k] = false
				continue
			}
		}
		out[k] = v
	}
	return out
}

// collectionFieldTypes returns slug -> Webflow field type for a collection,
// preferring the local mirror and falling back to one live GET, mirroring the
// schema lookup collections_completeness uses. A schema miss returns an empty
// map rather than an error: stringMapToAny then leaves every value as a
// string instead of guessing a type from the value.
func collectionFieldTypes(lq *localQuery, c liveFetcher, collectionID string) map[string]string {
	var schemaRows []rawRow
	for _, table := range []string{"sites_collections", "collections"} {
		if !lq.hasTable(table) {
			continue
		}
		found, qerr := lq.selectRaw(
			fmt.Sprintf(`SELECT "id", "id", "data" FROM %q WHERE "id" = ?`, table), collectionID)
		if qerr == nil && len(found) > 0 {
			schemaRows = found
			break
		}
	}
	if len(schemaRows) == 0 {
		schemaRows, _ = lq.fetchOne(c, "/collections/"+escapeSeg(collectionID), collectionID)
	}
	types := map[string]string{}
	colls := decodeRows[wfCollection](schemaRows)
	if len(colls) > 0 {
		for _, f := range colls[0].Fields {
			if f.Slug != "" {
				types[f.Slug] = f.Type
			}
		}
	}
	return types
}

// bulkSetResumeFileName is the sidecar state file that lets a rate-limited
// apply run resume without resending already-applied writes.
const bulkSetResumeFileName = "bulk-set-resume.json"

// bulkSetResumeState records which item IDs a previous apply run for this
// exact signature already wrote, so a re-run can skip them.
type bulkSetResumeState struct {
	Signature  string   `json:"signature"`
	AppliedIDs []string `json:"appliedItemIds"`
}

// bulkSetResumeSignature scopes resume state to one exact collection+match+set
// combination, so a different --match or --set on the same collection starts
// its own slate instead of inheriting an unrelated skip list.
func bulkSetResumeSignature(collectionID string, match, set map[string]string) string {
	h := sha256.New()
	fmt.Fprintf(h, "collection=%s\n", collectionID)
	for _, k := range sortedMapKeys(match) {
		fmt.Fprintf(h, "match:%s=%s\n", k, match[k])
	}
	for _, k := range sortedMapKeys(set) {
		fmt.Fprintf(h, "set:%s=%s\n", k, set[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func bulkSetResumePath() (string, error) {
	dir, err := cliutil.StateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, bulkSetResumeFileName), nil
}

// loadBulkSetResume returns the item IDs already applied under this exact
// signature by a previous run that stopped early on rate limiting. A missing
// file, a different signature, or any read error all resolve to "nothing to
// skip" rather than blocking the command.
func loadBulkSetResume(signature string) map[string]bool {
	p, err := bulkSetResumePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p) // #nosec G304 -- fixed filename under the CLI's own state dir.
	if err != nil {
		return nil
	}
	var state bulkSetResumeState
	if jsonErr := json.Unmarshal(data, &state); jsonErr != nil || state.Signature != signature {
		return nil
	}
	out := make(map[string]bool, len(state.AppliedIDs))
	for _, id := range state.AppliedIDs {
		out[id] = true
	}
	return out
}

// saveBulkSetResume persists which items were applied so a follow-up run of
// the identical command skips them instead of resending. Best-effort: a write
// failure only means the next run redoes some already-applied writes, which
// wastes rate-limit budget but does not corrupt data (Webflow field sets are
// idempotent).
func saveBulkSetResume(signature string, appliedIDs []string) {
	p, err := bulkSetResumePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(bulkSetResumeState{Signature: signature, AppliedIDs: appliedIDs})
	if err != nil {
		return
	}
	_ = os.WriteFile(p, data, 0o600)
}

// clearBulkSetResume removes a finished signature's resume state so a later,
// unrelated bulk-set on the same collection does not inherit a stale skip
// list. Matches on signature so clearing one in-flight batch's file never
// discards a different --match/--set batch's resume state.
func clearBulkSetResume(signature string) {
	p, err := bulkSetResumePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(p) // #nosec G304 -- fixed filename under the CLI's own state dir.
	if err != nil {
		return
	}
	var state bulkSetResumeState
	if jsonErr := json.Unmarshal(data, &state); jsonErr != nil || state.Signature != signature {
		return
	}
	_ = os.Remove(p)
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
