// Shared helpers for the hand-authored Hostex transcendence commands.
// Hand-authored: no generator header, so generate --force preserves this file.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/store"
)

// openLocalMirror resolves the SQLite mirror path and opens it read-only for a
// local transcendence command. Missing-mirror guard: if the DB file does not
// exist it prints a sync hint to stderr, emits an empty JSON array for
// --json/--agent callers, and returns done=true with a nil error so the caller
// can `return err` cleanly without treating an empty cache as a failure.
func openLocalMirror(cmd *cobra.Command, flags *rootFlags, dbPath string) (db *store.Store, done bool, err error) {
	if dbPath == "" {
		dbPath = defaultDBPath("hostex-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: hostex-pp-cli sync --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, true, nil
	}
	st, oerr := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if oerr != nil {
		return nil, true, fmt.Errorf("opening local mirror: %w", oerr)
	}
	return st, false, nil
}

// listObjs returns every synced object of resourceType from the local mirror,
// decoded into maps. store.List drains its result set before returning, so no
// rows stay open during decoding.
func listObjs(db *store.Store, resourceType string) ([]map[string]any, error) {
	raws, err := db.List(resourceType, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(raws))
	for _, r := range raws {
		obj, derr := store.DecodeJSONObject(r)
		if derr != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

func novStr(obj map[string]any, key string) string {
	v, ok := obj[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func novNum(obj map[string]any, key string) (float64, bool) {
	v, ok := obj[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func novMap(obj map[string]any, key string) map[string]any {
	if v, ok := obj[key].(map[string]any); ok {
		return v
	}
	return nil
}

// novTime parses the common Hostex timestamp shapes: RFC3339, "YYYY-MM-DD",
// and unix epoch (seconds or millis, as number or numeric string).
func novTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if tm, err := time.Parse(layout, s); err == nil {
				return tm.UTC(), true
			}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return epochToTime(n), true
		}
		return time.Time{}, false
	case float64:
		return epochToTime(int64(t)), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return epochToTime(n), true
		}
	}
	return time.Time{}, false
}

func epochToTime(n int64) time.Time {
	if n > 1_000_000_000_000 { // milliseconds
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

// novEmit writes v as JSON honoring --json/--select/--compact/--csv/--quiet.
func novEmit(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// rejectLiveDataSource enforces that a local-only command was not asked to run live.
func rejectLiveDataSource(flags *rootFlags) error {
	if strings.EqualFold(flags.dataSource, "live") {
		return usageErr(fmt.Errorf("this command has no live equivalent; it reads the local mirror (run sync first)"))
	}
	return nil
}

// rejectLocalDataSource enforces that a live-only command was not asked to run from local data.
func rejectLocalDataSource(flags *rootFlags) error {
	if strings.EqualFold(flags.dataSource, "local") {
		return usageErr(fmt.Errorf("this command has no local data source; it queries the Hostex API live"))
	}
	return nil
}

// nowUTC is overridable in tests; production uses the real clock.
var nowUTC = func() time.Time { return time.Now().UTC() }

// novUnwrapData extracts the `data` field from a Hostex response envelope.
// The client already maps non-zero error_code to an error, so on success the
// returned body is the full {request_id,error_code,error_msg,data} envelope.
func novUnwrapData(raw json.RawMessage) json.RawMessage {
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
		return env.Data
	}
	return raw
}

// idString formats a JSON-decoded id (commonly a float64 from encoding/json)
// as a plain string without scientific notation, so large integer ids like
// 12704864 render as "12704864" rather than "1.2704864e+07".
func idString(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// novToFloat coerces a JSON-decoded value (number or numeric string) to float64.
func novToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}
