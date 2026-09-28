// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/store"
)

func TestLocalFallbackRefusesRowsFromAnotherQuery(t *testing.T) {
	testenv.Isolate(t, cliutil.DataDir)
	setDefaultDBScopeCredential("")

	dbPath := defaultDBPath("serply-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "from-query-alpha"
	if err := db.Upsert("news", "row-1", json.RawMessage(`{"id":"row-1","title":"`+marker+`","link":"https://alpha.example/a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	flags := &rootFlags{dataSource: "auto"}
	params := map[string]string{"q": "beta query"}
	data, _, err := resolveLocal(context.Background(), flags, io.Discard, "news", true, "/v1/news", params, networkFallbackReason)
	if !errors.Is(err, errLocalQueryUnfiltered) {
		t.Fatalf("resolveLocal err = %v, want query refusal", err)
	}
	if strings.Contains(string(data), marker) {
		t.Fatalf("local read returned another query's row: %s", data)
	}

	t.Setenv(cliutil.DogfoodEnvVar, "1")
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1:1"}, time.Second, 0)
	c.NoCache = true
	data, _, err = resolveRead(context.Background(), c, flags, "news", true, "/v1/news", params, nil, io.Discard)
	if err == nil || !errors.Is(err, errLocalQueryUnfiltered) {
		t.Fatalf("network fallback err = %v, want query refusal", err)
	}
	if strings.Contains(string(data), marker) {
		t.Fatalf("network fallback returned another query's row: %s", data)
	}

	unfiltered, _, err := resolveLocal(context.Background(), flags, io.Discard, "news", true, "/v1/news", nil, "user_requested")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unfiltered), marker) {
		t.Fatalf("local read without a query should still see stored rows: %s", unfiltered)
	}
}
