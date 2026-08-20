// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
package snipd

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
)

func TestMaskTokenNeverLeaksValue(t *testing.T) {
	secret := "supersecrettokenvalue1234567890"
	masked := MaskToken(secret)
	if strings.Contains(masked, secret) || strings.Contains(masked, "supersecret") {
		t.Fatalf("MaskToken leaked the value: %q", masked)
	}
	if masked != "present (31 chars)" {
		t.Errorf("MaskToken = %q, want length-only form", masked)
	}
	if MaskToken("") != "(not set)" {
		t.Errorf("MaskToken(empty) = %q", MaskToken(""))
	}
}

func TestClientSendsBearerAndParsesMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer testtoken" {
			t.Errorf("Authorization = %q, want Bearer testtoken", got)
		}
		if r.URL.Path != metadataPath {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"episode_batch_count":1,"episode_batches":[{"index":0,"episodes":[{"episode_id":"abc","total_snip_count":3}]}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "testtoken")
	m, err := c.FetchMetadata(context.Background(), "")
	if err != nil {
		t.Fatalf("FetchMetadata: %v", err)
	}
	if m.EpisodeBatchCount != 1 || m.TotalEpisodes() != 1 || m.TotalSnips() != 3 {
		t.Errorf("metadata parsed wrong: %+v", m)
	}
	if m.EpisodeBatches[0].Episodes[0].EpisodeID != "abc" {
		t.Errorf("episode id = %q", m.EpisodeBatches[0].Episodes[0].EpisodeID)
	}
}

func TestClientClassifiesAuthAndRateLimit(t *testing.T) {
	t.Run("401 is a clear auth error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"detail":"invalid token"}`))
		}))
		defer srv.Close()
		_, err := NewClient(srv.URL, "bad").FetchMetadata(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "SNIPD_TOKEN") {
			t.Fatalf("want auth error mentioning SNIPD_TOKEN, got %v", err)
		}
	})
	t.Run("429 is a typed RateLimitError", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer srv.Close()
		_, err := NewClient(srv.URL, "t").FetchMetadata(context.Background(), "")
		var rle *cliutil.RateLimitError
		if !errors.As(err, &rle) {
			t.Fatalf("want *cliutil.RateLimitError, got %v", err)
		}
	})
}

func TestParseZipRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	md := "episode_title=<<episode_title>>Ep One<</episode_title>>\n" +
		"show_title=<<show_title>>My Show<</show_title>>\n" +
		"@@SNIP@@\n" +
		"snip_title=<<snip_title>>A moment<</snip_title>>\n" +
		"snip_url=<<snip_url>>https://share.snipd.com/snip/11111111-2222-3333-4444-555555555555<</snip_url>>\n" +
		"snip_note=<<snip_note>>a note<</snip_note>>\n"
	f, _ := zw.Create("episodes/deadbeef-0000-0000-0000-000000000000_full_content.md")
	f.Write([]byte(md))
	// a non-markdown member that must be ignored
	mj, _ := zw.Create("batch0/metadata.json")
	mj.Write([]byte(`{"shows_data":{}}`))
	zw.Close()

	eps, snips, err := ParseZip(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseZip: %v", err)
	}
	if len(eps) != 1 || eps[0].EpisodeID != "deadbeef-0000-0000-0000-000000000000" {
		t.Fatalf("episodes = %+v", eps)
	}
	if eps[0].Title != "Ep One" || eps[0].Show != "My Show" {
		t.Errorf("episode fields wrong: %+v", eps[0])
	}
	if len(snips) != 1 || snips[0].Title != "A moment" {
		t.Fatalf("snips = %+v", snips)
	}
	if snips[0].SnipID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("snip id = %q", snips[0].SnipID)
	}
}
