// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
// PATCH: Local Suno transcendence commands share tolerant clip JSON extraction.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
