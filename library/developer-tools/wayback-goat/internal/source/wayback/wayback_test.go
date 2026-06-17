// Copyright 2026 Alex Bresler and contributors. Licensed under Apache-2.0. See LICENSE.

package wayback

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestChangesDetectsDigestFlip is the regression oracle for the core analytic.
// It mirrors the real pwcommunications.com history captured from the live CDX
// server: the page is unchanged across several 2001-2002 captures (digest
// K4AZ...) then changes once in 2003 (digest 4LDD...). Changes() must report
// exactly ONE change — not four — and must name the 2001 capture as first-seen.
func TestChangesDetectsDigestFlip(t *testing.T) {
	caps := []Capture{
		{Timestamp: "20010401190927", Digest: "K4AZ6YVPEZI4AZ2AWQZ7YSXMSYR3RQHY", Status: "200"},
		{Timestamp: "20010720193600", Digest: "K4AZ6YVPEZI4AZ2AWQZ7YSXMSYR3RQHY", Status: "200"},
		{Timestamp: "20020531170519", Digest: "K4AZ6YVPEZI4AZ2AWQZ7YSXMSYR3RQHY", Status: "200"},
		{Timestamp: "20030628062550", Digest: "4LDD6XUSC3DGEBP7M3UF35RWF5EEUV4I", Status: "200"},
	}
	firstSeen, changes := Changes(caps)
	if firstSeen == nil || firstSeen.Timestamp != "20010401190927" {
		t.Fatalf("first-seen wrong: %+v", firstSeen)
	}
	if len(changes) != 1 {
		t.Fatalf("expected exactly 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].NewDigest != "4LDD6XUSC3DGEBP7M3UF35RWF5EEUV4I" {
		t.Fatalf("change new-digest wrong: %s", changes[0].NewDigest)
	}
	if changes[0].PrevDigest != "K4AZ6YVPEZI4AZ2AWQZ7YSXMSYR3RQHY" {
		t.Fatalf("change prev-digest wrong: %s", changes[0].PrevDigest)
	}
}

// TestChangesIgnoresEmptyDigest ensures a capture with no digest cannot be
// counted as a change (revisit/redirect rows can lack a digest).
func TestChangesIgnoresEmptyDigest(t *testing.T) {
	caps := []Capture{
		{Timestamp: "20200101000000", Digest: "AAAA", Status: "200"},
		{Timestamp: "20200201000000", Digest: "", Status: "302"},
		{Timestamp: "20200301000000", Digest: "AAAA", Status: "200"},
	}
	_, changes := Changes(caps)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes (digest never truly flipped), got %d: %+v", len(changes), changes)
	}
}

// TestChangesEmpty: no captures yields nil/nil, not a spurious change.
func TestChangesEmpty(t *testing.T) {
	if fs, ch := Changes(nil); fs != nil || ch != nil {
		t.Fatalf("empty input should yield nil/nil, got %+v / %+v", fs, ch)
	}
}

// TestCapturesParsesCDXJSON drives Captures through a fake CDX server and
// asserts the header row is dropped and rows parse into the right fields. CI-safe:
// no live network.
func TestCapturesParsesCDXJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("url") == "" {
			t.Errorf("url param not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[["timestamp","original","statuscode","mimetype","digest"],
["20010401190927","http://www.example.com/","200","text/html","K4AZ"],
["20030628062550","http://www.example.com/","200","text/html","4LDD"]]`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), baseURL: srv.URL}
	caps, err := c.Captures(context.Background(), "example.com", CapturesOptions{})
	if err != nil {
		t.Fatalf("captures: %v", err)
	}
	if len(caps) != 2 {
		t.Fatalf("expected 2 captures (header dropped), got %d", len(caps))
	}
	if caps[0].Digest != "K4AZ" || caps[1].Digest != "4LDD" {
		t.Fatalf("digests parsed wrong: %+v", caps)
	}
}

// TestCapturesEmptyBody: an empty CDX body (no captures) returns nil, nil.
func TestCapturesEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(""))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), baseURL: srv.URL}
	caps, err := c.Captures(context.Background(), "never-archived.example", CapturesOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if caps != nil {
		t.Fatalf("expected nil captures, got %+v", caps)
	}
}
