// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Local Suno transcendence commands share tolerant clip JSON extraction.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
)

func openDefaultStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("suno-pp-cli"))
}

func openExistingStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("suno-pp-cli")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	return store.OpenWithContext(ctx, dbPath)
}

func readClipRaw(ctx context.Context, id string) (json.RawMessage, error) {
	s, err := openExistingStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if s != nil {
		defer s.Close()
		for _, typ := range []string{"clip", "clips"} {
			if raw, err := s.Get(typ, id); err == nil {
				return raw, nil
			} else if err != sql.ErrNoRows {
				return nil, fmt.Errorf("reading %s/%s: %w", typ, id, err)
			}
		}
	}
	return nil, sql.ErrNoRows
}

func unmarshalObject(raw json.RawMessage) map[string]any {
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return obj
}

func valueAt(obj map[string]any, path ...string) any {
	var cur any = obj
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func stringAtAny(obj map[string]any, paths ...[]string) string {
	for _, path := range paths {
		switch v := valueAt(obj, path...).(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			if v != 0 {
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	return ""
}

func numberAtAny(obj map[string]any, paths ...[]string) float64 {
	for _, path := range paths {
		switch v := valueAt(obj, path...).(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			n, _ := strconv.ParseFloat(v, 64)
			if n != 0 {
				return n
			}
		}
	}
	return 0
}

func boolAtAny(obj map[string]any, paths ...[]string) bool {
	for _, path := range paths {
		if v, ok := valueAt(obj, path...).(bool); ok {
			return v
		}
	}
	return false
}

func timeAtAny(obj map[string]any, paths ...[]string) time.Time {
	for _, path := range paths {
		if s := stringAtAny(obj, path); s != "" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		}
	}
	return time.Time{}
}

func clipCreatedAt(obj map[string]any) time.Time {
	return timeAtAny(obj, []string{"created_at"}, []string{"createdAt"}, []string{"metadata", "created_at"}, []string{"metadata", "createdAt"})
}

func clipPersonaID(obj map[string]any) string {
	return stringAtAny(obj, []string{"persona_id"}, []string{"personaId"}, []string{"metadata", "persona_id"}, []string{"metadata", "personaId"})
}

func clipModel(obj map[string]any) string {
	return stringAtAny(obj, []string{"model_name"}, []string{"mv"}, []string{"major_model_version"}, []string{"metadata", "model_name"}, []string{"metadata", "mv"})
}

func clipParentID(obj map[string]any) string {
	return stringAtAny(obj, []string{"parent_clip_id"}, []string{"parent_id"}, []string{"metadata", "parent_clip_id"}, []string{"metadata", "parent_id"})
}

func clipDuration(obj map[string]any) float64 {
	return numberAtAny(obj, []string{"duration"}, []string{"duration_s"}, []string{"metadata", "duration"}, []string{"metadata", "duration_s"})
}

func clipTitle(obj map[string]any) string {
	if s := stringAtAny(obj, []string{"title"}, []string{"name"}, []string{"metadata", "title"}); s != "" {
		return s
	}
	return stringAtAny(obj, []string{"id"})
}

func clipTags(obj map[string]any) []string {
	for _, path := range [][]string{{"tags"}, {"metadata", "tags"}} {
		switch v := valueAt(obj, path...).(type) {
		case string:
			return splitList(v)
		case []any:
			var out []string
			for _, item := range v {
				if s := strings.TrimSpace(fmt.Sprintf("%v", item)); s != "" {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "clip"
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	s = strings.Trim(re.ReplaceAllString(s, "-"), "-.")
	if s == "" {
		return "clip"
	}
	return s
}

func ensureExt(path, ext string) string {
	if strings.EqualFold(filepath.Ext(path), ext) {
		return path
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

func parseDurationRange(spec string) (float64, float64, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, 0, nil
	}
	if lo, hi, ok := strings.Cut(spec, "-"); ok {
		a, err := strconv.ParseFloat(strings.TrimSpace(lo), 64)
		if err != nil {
			return 0, 0, err
		}
		b, err := strconv.ParseFloat(strings.TrimSpace(hi), 64)
		if err != nil {
			return 0, 0, err
		}
		if a > b {
			a, b = b, a
		}
		return a, b, nil
	}
	n, err := strconv.ParseFloat(spec, 64)
	return n, n, err
}

func durationDistance(d, lo, hi float64) float64 {
	if lo == 0 && hi == 0 {
		return 0
	}
	if lo == hi {
		return math.Abs(d - lo)
	}
	if d >= lo && d <= hi {
		return 0
	}
	if d < lo {
		return lo - d
	}
	return d - hi
}

func extractVariantObjects(data json.RawMessage) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			if stringAtAny(x, []string{"id"}) != "" || stringAtAny(x, []string{"clip_id"}) != "" {
				out = append(out, x)
			}
			for _, key := range []string{"clips", "data", "items", "results"} {
				if child, ok := x[key]; ok {
					walk(child)
				}
			}
		}
	}
	var parsed any
	if json.Unmarshal(data, &parsed) == nil {
		walk(parsed)
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
