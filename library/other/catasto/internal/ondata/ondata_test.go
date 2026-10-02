// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package ondata

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/parquet-go/parquet-go"
)

func indexParquetBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := parquet.Write(&buf, []IndexEntry{{Comune: "H501", File: "12_Lazio.parquet", CODISTAT: "058091", DenominazioneIT: "ROMA"}}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func incompatibleIndexParquetBytes(t *testing.T) []byte {
	t.Helper()
	type incompatibleIndex struct {
		Comune          int64  `parquet:"comune"`
		File            string `parquet:"file"`
		CODISTAT        string `parquet:"CODISTAT"`
		DenominazioneIT string `parquet:"DENOMINAZIONE_IT"`
	}
	var buf bytes.Buffer
	if err := parquet.Write(&buf, []incompatibleIndex{{Comune: 123, File: "12_Lazio.parquet"}}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNormalizeNumericForms(t *testing.T) {
	cases := []struct {
		in   string
		pad  int
		want []string
	}{
		{"508", 4, []string{"508", "0508"}},
		{"0508", 4, []string{"0508", "508"}},
		{"B", 0, []string{"B"}},
		{"", 4, []string{""}},
		{"  12  ", 0, []string{"12"}},
	}
	for _, c := range cases {
		got := normalizeNumericForms(c.in, c.pad)
		if len(got) != len(c.want) {
			t.Errorf("normalizeNumericForms(%q,%d) = %v; want %v", c.in, c.pad, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("normalizeNumericForms(%q,%d)[%d] = %q; want %q", c.in, c.pad, i, got[i], c.want[i])
			}
		}
	}
}

func TestFetchRefreshesStaleCacheAtomically(t *testing.T) {
	fresh := indexParquetBytes(t)
	dir := t.TempDir()
	local := filepath.Join(dir, "index.parquet")
	if err := os.WriteFile(local, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(fresh)
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	path, err := c.fetch(context.Background(), "index.parquet")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fresh) || requests != 1 {
		t.Fatalf("cache size=%d requests=%d, want validated fresh data after one refresh", len(got), requests)
	}
}

func TestFetchKeepsStaleCacheWhenDownloadIsInvalid(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, IndexFileName)
	good := indexParquetBytes(t)
	if err := os.WriteFile(local, good, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "nonempty but invalid Parquet")
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	gotPath, err := c.fetch(context.Background(), IndexFileName)
	if err != nil || gotPath != local {
		t.Fatalf("invalid refresh lost cached file: path=%q err=%v", gotPath, err)
	}
	got, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("invalid refresh changed cached data: len=%d err=%v", len(got), err)
	}
}

func TestFetchKeepsStaleCacheWhenParquetTypesAreIncompatible(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, IndexFileName)
	good := indexParquetBytes(t)
	if err := os.WriteFile(local, good, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	wrong := incompatibleIndexParquetBytes(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(wrong)
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	gotPath, err := c.fetch(context.Background(), IndexFileName)
	if err != nil || gotPath != local {
		t.Fatalf("incompatible refresh lost cached file: path=%q err=%v", gotPath, err)
	}
	got, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("incompatible refresh changed cached data: len=%d err=%v", len(got), err)
	}
}

func TestFetchRejectsOversizeResponseBeforeReplacingCache(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, IndexFileName)
	good := indexParquetBytes(t)
	if err := os.WriteFile(local, good, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "16777217")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	gotPath, err := c.fetch(context.Background(), IndexFileName)
	if err != nil || gotPath != local {
		t.Fatalf("oversize refresh lost cached file: path=%q err=%v", gotPath, err)
	}
	got, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("oversize refresh changed cached data: len=%d err=%v", len(got), err)
	}
}

func TestFetchCapsChunkedResponseAndRemovesPartialFile(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, IndexFileName)
	good := indexParquetBytes(t)
	if err := os.WriteFile(local, good, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush() // force chunked encoding without Content-Length
		_, _ = w.Write(bytes.Repeat([]byte("x"), indexSizeLimit+1))
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	gotPath, err := c.fetch(context.Background(), IndexFileName)
	if err != nil || gotPath != local {
		t.Fatalf("oversize chunked refresh lost cache: path=%q err=%v", gotPath, err)
	}
	got, err := os.ReadFile(local)
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("oversize chunked refresh changed cache: len=%d err=%v", len(got), err)
	}
	partials, err := filepath.Glob(filepath.Join(dir, IndexFileName+".part-*"))
	if err != nil || len(partials) != 0 {
		t.Fatalf("partial files remain after oversized response: %v, %v", partials, err)
	}
}

func TestFetchKeepsStaleCacheWhenRefreshFails(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "index.parquet")
	if err := os.WriteFile(local, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	gotPath, err := c.fetch(context.Background(), "index.parquet")
	if err != nil || gotPath != local {
		t.Fatalf("failed refresh lost cached file: path=%q err=%v", gotPath, err)
	}
	got, err := os.ReadFile(local)
	if err != nil || string(got) != "cached" {
		t.Fatalf("cached data changed after failed refresh: %q, %v", got, err)
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if _, err := c.fetch(context.Background(), "index.parquet"); err == nil {
		t.Fatal("refresh failure without a cache returned success")
	}
}

func TestFetchRejectsEmptyRefreshAndUnsafeFilename(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "index.parquet")
	if err := os.WriteFile(local, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(local, old, old); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), CacheDir: dir, SourceURL: server.URL, CacheTTL: time.Hour}
	if _, err := c.fetch(context.Background(), "index.parquet"); err != nil {
		t.Fatalf("empty refresh should keep stale cache: %v", err)
	}
	got, err := os.ReadFile(local)
	if err != nil || string(got) != "cached" {
		t.Fatalf("empty response replaced cached data: %q, %v", got, err)
	}
	for _, name := range []string{"../other.parquet", `..\other.parquet`, "subdir/other.parquet"} {
		if _, err := c.fetch(context.Background(), name); err == nil {
			t.Errorf("unsafe dataset filename %q accepted", name)
		}
	}
}

func TestAnyEqual(t *testing.T) {
	if !anyEqual("0508", []string{"508", "0508"}) {
		t.Error("expected 0508 to match")
	}
	if !anyEqual("b", []string{"B"}) {
		t.Error("expected case-insensitive match")
	}
	if anyEqual("999", []string{"508", "0508"}) {
		t.Error("expected no match")
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"a", "b"}, "a")
	if len(got) != 2 {
		t.Errorf("appendUnique should dedupe; got %v", got)
	}
	got = appendUnique([]string{"a"}, "b")
	if len(got) != 2 || got[1] != "b" {
		t.Errorf("appendUnique should append new; got %v", got)
	}
}

func TestParcelRowCoords(t *testing.T) {
	r := ParcelRow{X: 12_492_405, Y: 41_890_252}
	if r.Lon() < 12.49 || r.Lon() > 12.50 {
		t.Errorf("Lon() = %v; want ~12.4924", r.Lon())
	}
	if r.Lat() < 41.88 || r.Lat() > 41.90 {
		t.Errorf("Lat() = %v; want ~41.8902", r.Lat())
	}
}

func TestMissingGeometryMessageDistinguishesRecordsFromMap(t *testing.T) {
	got := missingGeometryMessage("G273", "35", "1900", 1530, "19_Sicilia.parquet", []string{"190", "191"})
	for _, want := range []string{
		"not represented in this geometry dataset",
		"may still exist in Catasto Fabbricati or in official records",
		"nearest represented particelle: 190, 191",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missingGeometryMessage() = %q, want substring %q", got, want)
		}
	}
}
