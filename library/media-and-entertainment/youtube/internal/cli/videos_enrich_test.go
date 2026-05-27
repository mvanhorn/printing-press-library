// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// fakeAPIGetter satisfies apiGetter with a canned response, so the metadata
// path that videos-enrich and playlist-enrich share can be tested without a
// real client or network.
type fakeAPIGetter struct {
	data json.RawMessage
	err  error
}

func (f fakeAPIGetter) GetWithHeaders(string, map[string]string, map[string]string) (json.RawMessage, error) {
	return f.data, f.err
}

func TestFetchVideoMetadata(t *testing.T) {
	resp := `{"items":[{
		"id":"vid12345678",
		"snippet":{
			"title":"Bread &amp; Butter",
			"description":"see &lt;link&gt; below",
			"channelTitle":"Baker &amp; Co",
			"publishedAt":"2020-01-02T03:04:05Z",
			"thumbnails":{"default":{"url":"https://i/d.jpg"},"high":{"url":"https://i/h.jpg"}}
		},
		"contentDetails":{"duration":"PT3M34S"},
		"statistics":{"viewCount":"1775537066","likeCount":"42"}
	}]}`

	meta, warns := fetchVideoMetadata(fakeAPIGetter{data: json.RawMessage(resp)},
		[]playlistEntry{{videoID: "vid12345678"}})
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	m, ok := meta["vid12345678"]
	if !ok {
		t.Fatal("expected vid12345678 in metadata map")
	}
	// HTML entities must be decoded (matches the existing novel commands).
	if m.title != "Bread & Butter" {
		t.Errorf("title = %q, want %q", m.title, "Bread & Butter")
	}
	if m.description != "see <link> below" {
		t.Errorf("description = %q, want %q", m.description, "see <link> below")
	}
	if m.channelTitle != "Baker & Co" {
		t.Errorf("channelTitle = %q, want %q", m.channelTitle, "Baker & Co")
	}
	// Thumbnail priority is high > medium > default.
	if m.thumbnailURL != "https://i/h.jpg" {
		t.Errorf("thumbnailURL = %q, want the high thumbnail", m.thumbnailURL)
	}
	if m.duration != "PT3M34S" || m.viewCount != "1775537066" || m.likeCount != "42" {
		t.Errorf("scalar fields wrong: dur=%q views=%q likes=%q", m.duration, m.viewCount, m.likeCount)
	}
}

func TestFetchVideoMetadataThumbnailFallback(t *testing.T) {
	// Only a default thumbnail present — must fall back to it.
	resp := `{"items":[{"id":"vidABCDEFGH","snippet":{"title":"x","thumbnails":{"default":{"url":"https://i/d.jpg"}}}}]}`
	meta, _ := fetchVideoMetadata(fakeAPIGetter{data: json.RawMessage(resp)},
		[]playlistEntry{{videoID: "vidABCDEFGH"}})
	if got := meta["vidABCDEFGH"].thumbnailURL; got != "https://i/d.jpg" {
		t.Errorf("thumbnailURL = %q, want default fallback", got)
	}
}

func TestFetchVideoMetadataMissingVideo(t *testing.T) {
	// videos.list returns no items (private/deleted/bad ID): the video must be
	// absent from the map so the caller can set metadataError.
	meta, warns := fetchVideoMetadata(fakeAPIGetter{data: json.RawMessage(`{"items":[]}`)},
		[]playlistEntry{{videoID: "missing1234"}})
	if _, ok := meta["missing1234"]; ok {
		t.Error("expected missing video to be absent from metadata map")
	}
	if len(warns) != 0 {
		t.Errorf("empty items is not a warning condition, got: %v", warns)
	}
}
