// Hand-written Etsy selective local-data cleanup; preserved across reprints.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

// Family pruning deliberately uses the same buyer-family index as MCP privacy,
// captured BEFORE the resource learnings disappear. Missing optional tables are OK.
func etsyCleanupLearn(tx *sql.Tx, ids map[string]bool) error {
	if len(ids) == 0 {
		return nil
	}
	idx, err := etsyLoadBuyerIndexForErasure(context.Background(), tx, true)
	if err != nil {
		return err
	}
	for _, spec := range [][2]string{{"search_patterns", "query_template"}, {"learning_playbooks", "query_family"}, {"learn_candidates", "query_family"}} {
		var exists int
		if err = tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, spec[0]).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		q := `SELECT id,COALESCE(` + spec[1] + `,'') FROM ` + spec[0]
		rows, e := tx.Query(q)
		if e != nil {
			return e
		}
		var remove []int64
		for rows.Next() {
			var id int64
			var family string
			if e = rows.Scan(&id, &family); e != nil {
				rows.Close()
				return e
			}
			if idx.familyBuyerBearing(family) {
				remove = append(remove, id)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, id := range remove {
			if _, err = tx.Exec(`DELETE FROM `+spec[0]+` WHERE id=?`, id); err != nil {
				return err
			}
		}
		if spec[0] == "search_patterns" {
			for _, kind := range store.EtsyBuyerTypes() {
				if _, err = tx.Exec(`DELETE FROM search_patterns WHERE resource_type=?`, kind); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func etsyRewriteTeachLog(ids map[string]bool) error {
	state, err := cliutil.StateDir()
	if err != nil {
		return err
	}
	data, err := cliutil.DataDir()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	paths := []string{}
	for _, dir := range []string{state, data} {
		for _, name := range []string{"teach.log", "teach.log.1", "learnings.jsonl", "learnings.jsonl.1"} {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	// etsy-ext: use the learning package's fixed legacy path even with DataDir overridden.
	legacy, e := learn.LegacyTeachLogPath()
	if e != nil {
		return e
	}
	paths = append(paths, legacy, legacy+".1")

	seen := map[string]bool{}
	var failures []error
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
			continue
		}
		lockPath := strings.TrimSuffix(path, ".1")
		e := cliutil.WithFileLock(lockPath, func() error {
			info, e := os.Lstat(path)
			if errors.Is(e, os.ErrNotExist) {
				return nil
			}
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("refusing non-regular learn log")
			}
			body, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			var kept bytes.Buffer
			for _, line := range bytes.SplitAfter(body, []byte("\n")) {
				if !etsyLogMatches(line, ids) {
					kept.Write(line)
				}
			}
			if bytes.Equal(body, kept.Bytes()) {
				return nil
			}
			return etsyReplaceTeachLog(path, kept.Bytes())
		})
		if e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}

func etsyLogMatches(line []byte, ids map[string]bool) bool {
	// etsy-ext: tokenize once per text; work is independent of the erased-id count.
	match := func(text string) bool {
		for _, token := range strings.FieldsFunc(text, func(r rune) bool { return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') }) {
			if ids[token] {
				return true
			}
		}
		return false
	}

	var value any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return match(string(line))
	}
	var walk func(any) bool
	walk = func(v any) bool {
		switch x := v.(type) {
		case string:
			return match(x)
		case json.Number:
			return ids[x.String()]
		case []any:
			for _, child := range x {
				if walk(child) {
					return true
				}
			}
		case map[string]any:
			for _, child := range x {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}

func etsyCountsText(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}

func etsySelectiveStores(explicit string) ([]string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return nil, err
	}
	files, err := matchingLocalFiles(localDataKindStore, dir, localStoreFilePattern)
	if err != nil {
		return nil, err
	}
	var paths []string
	seen := map[string]bool{}
	add := func(path string) {
		path, _ = filepath.Abs(path)
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	if explicit != "" {
		add(explicit)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".db") {
			add(file.Path)
		}
	}
	if len(paths) == 0 {
		add(defaultDBPath("etsy-pp-cli"))
	}
	return paths, nil
}

// etsy-ext: injectable post-commit pass proves the pre-commit scrub independently.
var etsyPostCommitRewriteTeachLog = etsyRewriteTeachLog

func etsyRunSelective(cmd *cobra.Command, flags *rootFlags, dbPath string, o store.EtsyPruneOptions, action string, extra map[string]any) error {
	paths, err := etsySelectiveStores(dbPath)
	if err != nil {
		return err
	}
	o.Context = cmd.Context()
	o.Apply = flags.yes && !flags.dryRun && !cliutil.IsAnyHarness()
	o.CleanupLearn = etsyCleanupLearn
	counts := map[string]int{}
	floor := map[string]int64{}
	var reports []map[string]any
	ids := map[string]bool{}
	missing, unshipped, unlinkable, unsuppressed := 0, 0, 0, 0
	unknownShipped, lostLinks, readmitted := 0, 0, 0
	suppressedBuyerReceipt := false
	vacuum := ""
	// Preflight every store, including owner checks, before changing any copy.
	resolvedBuyers := map[string]bool{}
	selectedReviews := map[string]map[string][]string{}
	for _, path := range paths {
		db, e := store.OpenReadOnlyContext(cmd.Context(), path)
		if e != nil {
			return e
		}
		preview := o
		preview.Apply = false
		result, selectionErr := db.EtsyPrune(preview)
		e = selectionErr
		selectedReviews[path] = result.ReviewIDs
		for id := range result.ErasedIDs {
			ids[id] = true
		}
		for _, id := range result.BuyerIDs {
			resolvedBuyers[id] = true
		}
		db.Close()
		if e != nil {
			return usageErr(fmt.Errorf("selective cleanup refused: %w", e))
		}
	}
	if o.Erase != nil {
		copied := *o.Erase
		copied.BuyerIDs = append([]string{}, o.Erase.BuyerIDs...)
		for id := range resolvedBuyers {
			copied.BuyerIDs = append(copied.BuyerIDs, id)
		}
		o.Erase = &copied
		// A buyer resolved in one credential store may be the owner in another.
		for _, path := range paths {
			db, e := store.OpenReadOnlyContext(cmd.Context(), path)
			if e != nil {
				return e
			}
			preview := o
			preview.Apply = false
			result, previewErr := db.EtsyPrune(preview)
			e = previewErr
			selectedReviews[path] = result.ReviewIDs
			for id := range result.ErasedIDs {
				ids[id] = true
			}
			db.Close()
			if e != nil {
				return usageErr(e)
			}
		}
	}
	if o.Apply && len(ids) > 0 {
		if err = etsyRewriteTeachLog(ids); err != nil {
			return err
		}
	}
	var applyErrors []error
	for _, path := range paths {
		open := store.OpenReadOnlyContext
		if o.Apply {
			open = store.OpenWithContext
		}
		db, e := open(cmd.Context(), path)
		if e != nil {
			applyErrors = append(applyErrors, e)
			continue
		}
		perStore := o
		if o.Erase != nil {
			erase := *o.Erase
			erase.PreScrubReviews = selectedReviews[path]
			perStore.Erase = &erase
		}
		result, e := db.EtsyPrune(perStore)
		closeErr := db.Close()
		if e != nil {
			applyErrors = append(applyErrors, e)
			if !result.Committed {
				continue
			}
		}
		if closeErr != nil {
			applyErrors = append(applyErrors, closeErr)
		}
		for kind, n := range result.Counts {
			counts[kind] += n
		}
		for scope, cutoff := range result.Floor {
			floor[scope] = cutoff
		}
		for id := range result.ErasedIDs {
			ids[id] = true
		}
		unknownShipped += result.KeptUnknownShipped
		lostLinks += result.ReviewsUnlinkableAfterPrune
		readmitted += result.ReadmittedReceipts
		suppressedBuyerReceipt = suppressedBuyerReceipt || result.SuppressedBuyerReceipt
		unsuppressed += result.UnsuppressedReceipts
		missing += result.KeptNoTimestamp
		unshipped += result.KeptUnshipped
		unlinkable += result.KeptUnlinkableReviews
		if etsyVacuumRank(result.Vacuum) > etsyVacuumRank(vacuum) {
			vacuum = result.Vacuum
		}
		reports = append(reports, map[string]any{"store": filepath.Base(path), "counts": result.Counts, "vacuum": result.Vacuum, "vacuum_steps": result.VacuumSteps})
	}
	removed := false
	if o.Apply && len(ids) > 0 {
		if e := etsyPostCommitRewriteTeachLog(ids); e != nil {
			applyErrors = append(applyErrors, e)
		}
		cache, e := cliutil.CacheDir()
		if e != nil {
			applyErrors = append(applyErrors, e)
		} else {
			httpDir := filepath.Join(cache, "http")
			if _, e = os.Lstat(httpDir); e == nil {
				if e = os.RemoveAll(httpDir); e != nil {
					applyErrors = append(applyErrors, e)
				} else {
					removed = true
				}
			} else if !errors.Is(e, os.ErrNotExist) {
				applyErrors = append(applyErrors, e)
			}
		}
	}
	if len(applyErrors) > 0 {
		if !flags.asJSON && vacuum != "" && vacuum != "completed" {
			fmt.Fprintf(cmd.ErrOrStderr(), "purged; vacuum incomplete (%s) — rerun with --vacuum when the store is idle\n", strings.ReplaceAll(vacuum, "_", " "))
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Selective cleanup partial failure: committed_stores=%d counts=%s\n", len(reports), etsyCountsText(counts))
		for _, report := range reports {
			fmt.Fprintf(cmd.ErrOrStderr(), "Committed store %s: %s\n", report["store"], etsyCountsText(report["counts"].(map[string]int)))
		}
		return errors.Join(applyErrors...)
	}
	out := map[string]any{"action": action, "dry_run": !o.Apply, "counts": counts, "unsuppressed_receipts": unsuppressed, "kept_unlinkable_reviews": unlinkable, "kept_no_timestamp": missing, "kept_unshipped": unshipped, "floor": floor, "stores": reports, "http_cache_removed": removed}
	// etsy-ext: expose irreversible link loss and reversible retention metadata.
	out["kept_unknown_shipped"] = unknownShipped
	out["reviews_unlinkable_after_prune"] = lostLinks
	if lostLinks > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d reviews will no longer be erasable by buyer; add --reviews to prune them too\n", lostLinks)
	}
	if o.ClearFloor && o.Receipts {
		out["readmitted_receipts"] = readmitted
		out["message"] = fmt.Sprintf("clearing re-admits previously pruned receipts on the next sync (%d remembered ids), when no suppression applies", readmitted)
		if !flags.asJSON {
			fmt.Fprintln(cmd.OutOrStdout(), out["message"])
		}
	}
	if o.Erase != nil {
		out["owner_refusal_unavailable"] = o.Erase.OwnerRefusalUnavailable
		if o.Erase.OwnerRefusalUnavailable && !flags.asJSON {
			fmt.Fprintln(cmd.ErrOrStderr(), "Own-id refusal unavailable: no owner id found locally")
		}
		if len(ids) == 0 {
			out["message"] = "no records matched"
		}
	}
	if suppressedBuyerReceipt {
		out["message"] = "receipt belongs to a suppressed buyer; use --unsuppress <buyer>"
		if !flags.asJSON {
			fmt.Fprintln(cmd.OutOrStdout(), out["message"])
		}
	}
	if vacuum != "" {
		out["vacuum"] = vacuum
	}
	for key, value := range extra {
		out[key] = value
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}
	if o.Erase != nil && len(ids) == 0 && len(o.Erase.Unsuppress) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No records matched")
	}
	verb := "Would purge"
	if o.Apply {
		verb = "Purged"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %v; kept: no timestamp=%d, unshipped=%d, kept_unknown_shipped=%d, kept_unlinkable_reviews=%d; floor=%s\n", verb, etsyCountsText(counts), missing, unshipped, unknownShipped, unlinkable, etsyFloorText(floor))
	for _, report := range reports {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", report["store"], etsyCountsText(report["counts"].(map[string]int)))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "HTTP cache removed: %t\n", removed)
	if vacuum != "" && vacuum != "completed" {
		fmt.Fprintf(cmd.OutOrStdout(), "purged; vacuum incomplete (%s) — rerun with --vacuum when the store is idle\n", strings.ReplaceAll(vacuum, "_", " "))
	}
	return nil
}

func etsyReplaceTeachLog(path string, body []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".teach-erasure-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func etsyFloorText(floor map[string]int64) string {
	counts := map[string]int{}
	for k, v := range floor {
		counts[k] = int(v)
	}
	return etsyCountsText(counts)
}

// etsy-ext: cross-store reclamation reports the least complete store.
func etsyVacuumRank(status string) int {
	return map[string]int{"": 0, "completed": 1, "optimize_busy": 2, "checkpoint_busy": 3, "skipped_busy": 4, "error": 5}[status]
}
