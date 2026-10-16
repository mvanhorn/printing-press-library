// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package seohead

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSEO(t *testing.T) {
	tests := []struct {
		name, input string
		check       func(*testing.T, Head)
	}{
		{"full head", `<html><head><title> Article </title><meta name="description" content="Desc"><link rel="canonical" href="https://x.test/a"><meta name="robots" content="noindex,follow"><meta property="og:title" content="OG"><meta property="og:description" content="OG desc"><script type="application/ld+json">{"@type":"WebPage","@graph":[{"@type":["Article","BreadcrumbList"]}]}</script></head><body></body></html>`, func(t *testing.T, h Head) {
			if h.Title != "Article" || h.Description != "Desc" || h.Canonical != "https://x.test/a" || !h.Noindex || h.OGTitle != "OG" || h.OGDescription != "OG desc" || strings.Join(h.SchemaTypes, ",") != "Article,BreadcrumbList,WebPage" {
				t.Fatalf("head: %+v", h)
			}
		}},
		{"missing description", `<head><title>X</title></head>`, func(t *testing.T, h Head) {
			if h.Description != "" {
				t.Fatal(h.Description)
			}
		}},
		{"body title fallback", `<html><head></head><body><title>Fallback</title></body></html>`, func(t *testing.T, h Head) {
			if h.Title != "Fallback" {
				t.Fatalf("%+v", h)
			}
		}},
		{"entities", `<head><title>A &amp; B &#39;C&#39;</title><meta name="description" content="A &amp; B &#39;C&#39;"></head>`, func(t *testing.T, h Head) {
			if h.Title != "A & B 'C'" || h.Description != "A & B 'C'" {
				t.Fatalf("%+v", h)
			}
		}},
		{"body ignored", `<html><head><title>Head</title></head><body><title>Body wins?</title><meta name="description" content="body"></body></html>`, func(t *testing.T, h Head) {
			if h.Title != "Head" || h.Description != "" {
				t.Fatalf("%+v", h)
			}
		}},
		{"body JSON-LD collected", `<html><head><title>Head</title></head><body><script type="application/ld+json">{"@type":"Article"}</script></body></html>`, func(t *testing.T, h Head) {
			if strings.Join(h.SchemaTypes, ",") != "Article" {
				t.Fatalf("%+v", h)
			}
		}},
		{"foreign title ignored", `<head><meta name=description content=D></head><body><svg><title>Menu icon</title></svg></body>`, func(t *testing.T, h Head) {
			if h.Title != "" || h.Description != "D" {
				t.Fatalf("%+v", h)
			}
		}},
		{"head tags recovered into body", `<head><title>T</title><img src=p.gif><meta name=description content=D><script type="application/ld+json">{"@type":"Article"}</script></head><body>..`, func(t *testing.T, h Head) {
			if h.Title != "T" || h.Description != "D" || strings.Join(h.SchemaTypes, ",") != "Article" {
				t.Fatalf("%+v", h)
			}
		}},
		{"malformed JSON-LD", `<head><script type="application/ld+json">{bad</script></head>`, func(t *testing.T, h Head) {
			if len(h.SchemaTypes) != 0 || h.SchemaTypes == nil {
				t.Fatalf("%+v", h)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.check(t, Parse("https://x.test/", []byte(tt.input))) })
	}
}

func TestSEOFetcher(t *testing.T) {
	var srv *httptest.Server
	func() {
		defer func() {
			if recover() != nil {
				t.Skip("local listener unavailable in this sandbox")
			}
		}()
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.UserAgent() != "rankmath-pp-cli" {
				t.Errorf("UA %q", r.UserAgent())
			}
			w.Write([]byte(`<head><title>Fetched</title></head>`))
		}))
	}()
	defer srv.Close()
	f := New(time.Second, 0)
	h, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if h.Title != "Fetched" || h.Status != 200 || h.FinalURL != srv.URL || h.FetchedAt.IsZero() {
		t.Fatalf("%+v", h)
	}
}
