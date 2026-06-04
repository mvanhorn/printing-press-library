// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Suno clip JSON nests several high-gravity fields (tags, prompt, duration,
// avg_bpm, has_stem, is_remix, make_instrumental) under a `metadata` object
// rather than at the top level. sunoClipField resolves those by checking the
// top level first, then falling back to metadata.<key>, so the typed clips
// columns that power top/analytics/grep/lineage are populated correctly.

package store

// sunoClipField returns obj[key] if present and non-nil, otherwise
// obj["metadata"][key]. Returns nil when neither is set.
func sunoClipField(obj map[string]any, key string) any {
	if v, ok := obj[key]; ok && v != nil {
		return v
	}
	if md, ok := obj["metadata"].(map[string]any); ok {
		if v, ok := md[key]; ok && v != nil {
			return v
		}
	}
	return nil
}
