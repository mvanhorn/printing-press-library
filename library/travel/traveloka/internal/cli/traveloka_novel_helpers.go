// Novel-only local snapshot loading and bounded live-grid orchestration.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"time"
)

func novelInvalid(message string) error {
	return &traveloka.APIError{Code: "INVALID_INPUT", Message: message}
}
func novelNoArgs(args []string) error {
	if len(args) > 0 {
		return novelInvalid("this command accepts flags only; remove unexpected positional arguments")
	}
	return nil
}
func novelAnnotations(source, fixture string) map[string]string {
	a := travelokaAnnotations(source, fixture)
	if source == "local" {
		a["mcp:read-only"] = "true"
	}
	return a
}
func novelParseMaxAge(raw string) (time.Duration, error) {
	age, e := cliutil.ParseDurationLoose(raw)
	if e != nil || age < 0 {
		return 0, novelInvalid("--max-age requires a nonnegative duration such as 24h, 7d, 1w or 0")
	}
	return age, nil
}
func novelLocalOptions(limit, scan int, maxAge time.Duration) error {
	if limit < 1 || limit > 100 || scan < 1 || scan > 1000 || maxAge < 0 {
		return novelInvalid("--limit must be 1..100, --max-scan-records 1..1000 and --max-age nonnegative")
	}
	return nil
}
func novelEmpty(cmd *cobra.Command, f *rootFlags, collections []string, hint string) error {
	view := map[string]any{"status": "empty_local_cache", "hint": hint, "freshness": "saved_snapshot"}
	for _, key := range collections {
		view[key] = []any{}
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "no matching local snapshots;", hint)
	return f.printJSON(cmd, view)
}

// Generated sync hints do not exist for this custom snapshot table.
// These equivalents query public records and write only to stderr.
func hintIfUnsynced(cmd *cobra.Command, db *store.Store, kind string) bool {
	var at sql.NullString
	err := db.DB().QueryRowContext(cmd.Context(), "SELECT MAX(retrieved_at) FROM traveloka_snapshots WHERE kind=?", kind).Scan(&at)
	if err != nil || !at.Valid {
		fmt.Fprintln(cmd.ErrOrStderr(), "no matching local source snapshots; run flights search or hotels rooms to populate public history")
		return false
	}
	return true
}
func hintIfStale(cmd *cobra.Command, snapshot *traveloka.Snapshot, maxAge time.Duration) {
	if maxAge == 0 || snapshot == nil {
		return
	}
	t, e := time.Parse(time.RFC3339Nano, snapshot.RetrievedAt)
	if e == nil && time.Since(t) > maxAge {
		fmt.Fprintf(cmd.ErrOrStderr(), "local source snapshot %s retrieved at %s is stale; run the original flight/hotel search to refresh it; --max-age 0 disables this hint\n", snapshot.ID, snapshot.RetrievedAt)
	}
}
func novelOpenHistory(ctx context.Context, path string) (*store.Store, bool, error) {
	if path == "" {
		path = defaultDBPath("traveloka-pp-cli")
	}
	if _, e := os.Stat(path); os.IsNotExist(e) {
		return nil, false, nil
	} else if e != nil {
		return nil, false, e
	}
	db, e := store.OpenReadOnlyContext(ctx, path)
	if e != nil {
		return nil, false, e
	}
	var found int
	e = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='traveloka_snapshots'").Scan(&found)
	if e != nil {
		_ = db.Close() // Read-only cleanup; preserve the preceding query error or absence result.
		return nil, false, e
	}
	if found == 0 {
		_ = db.Close() // Read-only cleanup; preserve the preceding query error or absence result.
		return nil, false, nil
	}
	return db, true, nil
}
func novelLoad(ctx context.Context, cmd *cobra.Command, dbPath, file, id, kind string, maxAge time.Duration) (*traveloka.Snapshot, error) {
	if file != "" && id != "" {
		return nil, novelInvalid("--snapshot and --snapshot-id are mutually exclusive")
	}
	s, e := travelokaLoad(ctx, dbPath, file, id, kind)
	if e != nil {
		return s, e
	}
	hintIfStale(cmd, s, maxAge)
	if file != "" {
		return s, nil
	}
	db, found, e := novelOpenHistory(ctx, dbPath)
	if e != nil || !found {
		return s, e
	}
	defer db.Close()
	// No parent rows remain open when these scalar hint queries run.
	if kind == "" && s != nil {
		kind = s.Kind
	}
	hintIfUnsynced(cmd, db, kind)
	return s, nil
}
func novelLatestPair(ctx context.Context, cmd *cobra.Command, path, kind string, maxAge time.Duration) (*traveloka.Snapshot, *traveloka.Snapshot, error) {
	db, found, e := novelOpenHistory(ctx, path)
	if e != nil || !found {
		return nil, nil, e
	}
	defer db.Close()
	filter := "kind=(SELECT kind FROM traveloka_snapshots ORDER BY julianday(retrieved_at) DESC,rowid DESC LIMIT 1)"
	args := []any{}
	if kind != "" {
		filter = "kind=?"
		args = append(args, kind)
	}
	rows, e := db.DB().QueryContext(ctx, "SELECT snapshot FROM traveloka_snapshots WHERE "+filter+" ORDER BY julianday(retrieved_at) DESC,rowid DESC LIMIT 2", args...)
	if e != nil {
		return nil, nil, e
	}
	raw := []string{}
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			_ = rows.Close() // Preserve the preceding row decoding error.
			return nil, nil, e
		}
		raw = append(raw, b)
	}
	e = rows.Err()
	closeErr := rows.Close()
	if e != nil {
		return nil, nil, e
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	// Drain-first: decoding and subsequent hint lookups happen after rows.Close.
	if len(raw) < 2 {
		return nil, nil, nil
	}
	decoded := []*traveloka.Snapshot{}
	for _, b := range raw {
		var s traveloka.Snapshot
		dec := json.NewDecoder(strings.NewReader(b))
		dec.UseNumber()
		if e = dec.Decode(&s); e != nil {
			return nil, nil, e
		}
		if s.ID == "" || s.Kind == "" {
			return nil, nil, novelInvalid("stored snapshot requires an original source retrieval identity")
		}
		if _, e = time.Parse(time.RFC3339Nano, s.RetrievedAt); e != nil {
			return nil, nil, novelInvalid("stored snapshot requires an original RFC3339 retrieval timestamp")
		}
		s.Freshness = "saved_snapshot"
		decoded = append(decoded, &s)
	}
	hintIfUnsynced(cmd, db, decoded[0].Kind)
	for _, snapshot := range decoded {
		hintIfStale(cmd, snapshot, maxAge)
	}
	return decoded[1], decoded[0], nil
}
func novelGridContext(cmd *cobra.Command, f *rootFlags) (context.Context, context.CancelFunc) {
	ctx, cancel := boundCtx(cmd.Context(), f)
	if cliutil.IsDogfoodEnv() {
		limited, end := context.WithTimeout(ctx, 25*time.Second)
		return limited, func() { end(); cancel() }
	}
	return ctx, cancel
}
func novelGridAttempts(count int) int {
	if cliutil.IsDogfoodEnv() && count > 1 {
		return 1
	}
	return count
}
func novelGridOutput(cmd *cobra.Command, f *rootFlags, view travelokacompare.GridResult) error {
	if view.FetchFailureCount > 0 || view.SaveFailureCount > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d attempted fetches failed; comparisons use %d successful retrievals; save_failures=%d; inspect structured failure entries\n", view.FetchFailureCount, view.Attempted, view.Attempted-view.FetchFailureCount, view.SaveFailureCount)
	}
	if e := f.printJSON(cmd, view); e != nil {
		return e
	}
	if view.Requested > 0 && view.Attempted == 0 && view.ContextErr != nil {
		return apiErr(fmt.Errorf("grid retrieval ended before any requested cell could start: %w", view.ContextErr))
	}
	if view.Attempted > 0 && view.FetchFailureCount+view.SaveFailureCount == view.Attempted {
		allRateLimited := view.FetchFailureCount == view.Attempted
		for _, cell := range view.Cells {
			if cell.Attempted && (cell.Error == nil || cell.Error.Code != "RATE_LIMITED") {
				allRateLimited = false
			}
		}
		if allRateLimited {
			for _, cell := range view.Cells {
				var rate *cliutil.RateLimitError
				if errors.As(cell.Cause, &rate) {
					return rateLimitErr(cell.Cause)
				}
			}
			for _, cell := range view.Cells {
				if cell.Error != nil {
					return rateLimitErr(cell.Error)
				}
			}
		}
		for _, cell := range view.Cells {
			if cell.Error != nil && (cell.Error.Code == "AUTH_REQUIRED" || cell.Error.Code == "ACCESS_BLOCKED") {
				return authErr(cell.Error)
			}
		}
		return apiErr(fmt.Errorf("all attempted grid cells failed; inspect the structured cell errors"))
	}
	return nil
}
