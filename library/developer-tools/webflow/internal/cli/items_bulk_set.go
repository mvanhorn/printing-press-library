// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

			// A prior run that stopped on an exhausted rate limit leaves a resume
			// file recording which item IDs already got written under this exact
			// collection+match+set signature. Excluding them here - before the
			// --limit window is chosen - means a re-run's window is made of
			// genuinely untouched items instead of reselecting the same already-
			// applied head every time.
			signature := bulkSetResumeSignature(collectionID, matchValues, setValues, live)
			skip := loadBulkSetResume(signature)

			view := selectBulkTargets(rows, collectionID, matchValues, setValues, limit, skip)
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
			fieldTypes, ferr := collectionFieldTypes(lq, c, collectionID)
			if ferr != nil {
				// A schema fetch failure only matters when it leaves a --set
				// value ambiguous between "the literal string true/false" and
				// "a Switch field's boolean". Refuse to guess in that case
				// instead of risking a silent wrong-type write; any other
				// --set value is unaffected by not knowing the field's type.
				if amb, found := ambiguousBooleanSetField(setValues); found {
					return apiErr(fmt.Errorf(
						"could not fetch the collection schema to confirm whether %q is a Switch (boolean) field: %w; refusing to guess since --set %s=%s means something different for a Switch field than a text field. Re-run once the schema is reachable",
						amb, ferr, amb, setValues[amb]))
				}
			}

			// Already-applied IDs (excluded from view.Changes above) still belong
			// in the resume file if it needs saving again below, so this run's
			// save doesn't forget everything a previous run already recorded.
			appliedIDs := make([]string, 0, len(skip)+len(view.Changes))
			for id := range skip {
				appliedIDs = append(appliedIDs, id)
			}

			base := "/collections/" + escapeSeg(collectionID) + "/items/"
			var firstErr error
			rateLimited := false
			for i := range view.Changes {
				ch := &view.Changes[i]
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

			// view.Matched counts every matched item; view.Skipped + len(view.Changes)
			// is what this run actually accounted for (already-applied plus this
			// run's window). When that is less than Matched, --limit truncated the
			// window and untouched matches remain beyond it - the resume file must
			// survive so the next run's window advances onto them, even though
			// this run itself never hit the rate limit.
			moreMatchesRemain := view.Skipped+len(view.Changes) < view.Matched
			if rateLimited || moreMatchesRemain {
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
// schema lookup collections_completeness uses. It returns a non-nil error
// only when neither source produced a schema and the live fetch itself
// failed (as opposed to legitimately returning zero fields), so a caller can
// tell "the schema says this field isn't a Switch" apart from "the field's
// type could not be determined at all".
func collectionFieldTypes(lq *localQuery, c liveFetcher, collectionID string) (map[string]string, error) {
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
	var fetchErr error
	if len(schemaRows) == 0 {
		schemaRows, fetchErr = lq.fetchOne(c, "/collections/"+escapeSeg(collectionID), collectionID)
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
	if len(types) == 0 && fetchErr != nil {
		return types, fetchErr
	}
	return types, nil
}

// ambiguousBooleanSetField reports the first (in sorted key order, for a
// deterministic error message) --set field whose value is literally "true"
// or "false" - the only values whose written JSON type actually depends on
// whether the field is a Switch.
func ambiguousBooleanSetField(set map[string]string) (string, bool) {
	for _, k := range sortedMapKeys(set) {
		switch strings.ToLower(set[k]) {
		case "true", "false":
			return k, true
		}
	}
	return "", false
}

// bulkSetResumeFileName is the sidecar state file that lets a rate-limited
// apply run resume without resending already-applied writes.
const bulkSetResumeFileName = "bulk-set-resume.json"

// Lock timings for the shared resume file. An update is a small read, edit,
// and rename, so a healthy holder releases in well under a millisecond; the
// wait only has to outlast a burst of concurrent runs, and the stale age only
// has to outlast a slow one before a waiter reclaims a lock nobody owns.
const (
	bulkSetResumeLockWait  = 2 * time.Second
	bulkSetResumeLockPoll  = 5 * time.Millisecond
	bulkSetResumeLockStale = 10 * time.Second
)

// bulkSetResumeFile records applied item IDs per batch signature. Every
// in-flight bulk-set batch (whatever its --match/--set) shares this one file,
// so it must be keyed by signature rather than holding a single record -
// otherwise saving one batch's progress overwrites every other batch's.
type bulkSetResumeFile struct {
	Batches map[string][]string `json:"batches"`
}

// bulkSetResumeSignature scopes resume state to one exact collection+match+
// set+live combination, so a different --match, --set, or --live on the same
// collection starts its own slate instead of inheriting an unrelated skip
// list. live must be included: it selects the write endpoint (staged item vs
// /live), so an ID written through one is not "already applied" for the
// other even when collection+match+set are identical.
func bulkSetResumeSignature(collectionID string, match, set map[string]string, live bool) string {
	h := sha256.New()
	fmt.Fprintf(h, "collection=%s\n", collectionID)
	for _, k := range sortedMapKeys(match) {
		fmt.Fprintf(h, "match:%s=%s\n", k, match[k])
	}
	for _, k := range sortedMapKeys(set) {
		fmt.Fprintf(h, "set:%s=%s\n", k, set[k])
	}
	fmt.Fprintf(h, "live=%v\n", live)
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

// readBulkSetResumeFile loads every batch's resume state. A missing file, an
// unreadable file, or a corrupt one all resolve to "no batches recorded"
// rather than blocking the command - this state is a convenience, not a
// source of truth the command depends on to be safe.
func readBulkSetResumeFile() bulkSetResumeFile {
	empty := bulkSetResumeFile{Batches: map[string][]string{}}
	p, err := bulkSetResumePath()
	if err != nil {
		return empty
	}
	data, err := os.ReadFile(p) // #nosec G304 -- fixed filename under the CLI's own state dir.
	if err != nil {
		return empty
	}
	var file bulkSetResumeFile
	if jsonErr := json.Unmarshal(data, &file); jsonErr != nil || file.Batches == nil {
		return empty
	}
	return file
}

// writeBulkSetResumeFile is best-effort: a write failure only means the next
// run redoes some already-applied writes, which wastes rate-limit budget but
// does not corrupt data (Webflow field sets are idempotent). The write goes to
// a temp file in the same directory and is renamed into place, so a concurrent
// run reading the file never sees a half-written one and mistakes a live batch
// for "nothing recorded".
func writeBulkSetResumeFile(file bulkSetResumeFile) {
	p, err := bulkSetResumePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(file)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), bulkSetResumeFileName+".*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name() // os.CreateTemp already opens it 0600, matching the old write.
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
	}
}

// lockBulkSetResume serializes the read-modify-write of the shared resume file
// across concurrent bulk-set runs. Without it, two runs with different
// signatures each read the file, edit their own entry, and write the whole
// thing back - the later writer silently drops the other batch's freshly
// recorded entry, so that batch's retry resends updates it already applied and
// burns rate-limit budget before reaching untouched items.
//
// The lock is a plain O_EXCL sidecar file rather than an OS advisory lock so
// the same code works on every platform this CLI ships to. It returns false
// when the lock cannot be taken; callers then skip their update, which loses
// only their own bookkeeping instead of destroying another batch's.
func lockBulkSetResume() (release func(), ok bool) {
	p, err := bulkSetResumePath()
	if err != nil {
		return nil, false
	}
	lockPath := p + ".lock"
	deadline := time.Now().Add(bulkSetResumeLockWait)
	for {
		f, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- fixed filename under the CLI's own state dir.
		if openErr == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, true
		}
		if !errors.Is(openErr, os.ErrExist) {
			return nil, false
		}
		// A run killed between acquiring and releasing would otherwise wedge
		// every later run's bookkeeping forever, so an untouched lock past the
		// stale age is taken over rather than waited on.
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > bulkSetResumeLockStale {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, false
		}
		time.Sleep(bulkSetResumeLockPoll)
	}
}

// loadBulkSetResume returns the item IDs already applied under this exact
// signature by a previous run that stopped early on rate limiting.
func loadBulkSetResume(signature string) map[string]bool {
	ids, ok := readBulkSetResumeFile().Batches[signature]
	if !ok {
		return nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// saveBulkSetResume persists which items this signature's batch has applied
// so a follow-up run of the identical command skips them instead of
// resending. Read-modify-write against the shared file so saving this
// batch's progress never overwrites a different signature's entry - held
// under the lock so a concurrent run's entry cannot be lost between this
// read and this write.
func saveBulkSetResume(signature string, appliedIDs []string) {
	release, ok := lockBulkSetResume()
	if !ok {
		return
	}
	defer release()

	file := readBulkSetResumeFile()
	file.Batches[signature] = appliedIDs
	writeBulkSetResumeFile(file)
}

// clearBulkSetResume removes one finished signature's entry so a later,
// unrelated bulk-set on the same collection does not inherit a stale skip
// list, while leaving every other signature's still-in-progress entry alone.
func clearBulkSetResume(signature string) {
	release, ok := lockBulkSetResume()
	if !ok {
		return
	}
	defer release()

	file := readBulkSetResumeFile()
	if _, ok := file.Batches[signature]; !ok {
		return
	}
	delete(file.Batches, signature)
	writeBulkSetResumeFile(file)
}

// selectBulkTargets picks the items to change. Split out so it is testable
// without a store or an API. skip holds item IDs a previous --apply run
// already wrote for this exact collection+match+set signature; they are
// excluded before the --limit window is chosen (not just marked within it),
// so a re-run's window is made of the next genuinely untouched matches
// instead of reselecting the same already-applied head forever.
func selectBulkTargets(rows []rawRow, collectionID string, match, set map[string]string, limit int, skip map[string]bool) bulkSetView {
	view := bulkSetView{
		CollectionID: collectionID,
		Match:        match,
		Set:          set,
		Changes:      make([]bulkSetChange, 0, 16),
	}
	remaining := make([]bulkSetChange, 0, 16)

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
		if skip[it.ID] {
			view.Skipped++
			continue
		}
		fields := make(map[string]string, len(set))
		for k, v := range set {
			fields[k] = v
		}
		remaining = append(remaining, bulkSetChange{
			ItemID: it.ID,
			Name:   it.name(),
			Fields: fields,
			Status: "pending",
		})
	}

	sort.SliceStable(remaining, func(i, j int) bool { return remaining[i].Name < remaining[j].Name })
	if limit > 0 && len(remaining) > limit {
		view.Changes = remaining[:limit]
		view.Note = fmt.Sprintf("%d items still need this change; this run is capped at %d by --limit", len(remaining), limit)
	} else {
		view.Changes = remaining
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
