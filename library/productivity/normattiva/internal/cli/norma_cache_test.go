// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"
)

func TestDatedCacheReusedOnlyAfterWindowCloses(t *testing.T) {
	testenv.Isolate(t)
	var hits int
	var fine, text string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fineRaw := "99999999"
		if fine != "" {
			fineRaw = strings.ReplaceAll(fine, "-", "")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200, "message": "ok",
			"data": map[string]any{"atto": map[string]any{
				"titolo":                    "Legge 241",
				"articoloHtml":              "<div>" + text + "</div>",
				"articoloDataInizioVigenza": "19900807",
				"articoloDataFineVigenza":   fineRaw,
			}},
		})
	}))
	defer srv.Close()
	t.Setenv("NORMATTIVA_BASE_URL", srv.URL)
	flags := &rootFlags{dataSource: "auto", rateLimit: 0, noCache: true}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	past, err := norma.ParseURN("urn:nir:stato:legge:1990-08-07;241~art3!vig=2010-01-01")
	if err != nil {
		t.Fatal(err)
	}
	cacheArticle(ctx, articleView{URN: past.String(), Testo: "stale open", FineVigenza: ""})
	text, fine = "closed text", "2011-05-01"
	got, err := fetchArticle(ctx, c, flags, past)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || !strings.Contains(got.Testo, "closed text") || got.FineVigenza != "2011-05-01" {
		t.Fatalf("open past pin: hits=%d view=%+v", hits, got)
	}
	text = "should not be fetched"
	if _, err := fetchArticle(ctx, c, flags, past); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("closed window was fetched again (hits=%d)", hits)
	}

	future, err := norma.ParseURN("urn:nir:stato:legge:1990-08-07;241~art3!vig=2099-01-01")
	if err != nil {
		t.Fatal(err)
	}
	cacheArticle(ctx, articleView{URN: future.String(), Testo: "stale future", FineVigenza: ""})
	text, fine = "future text", "2099-06-01"
	got, err = fetchArticle(ctx, c, flags, future)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 || !strings.Contains(got.Testo, "future text") {
		t.Fatalf("future pin reused an open cache: hits=%d view=%+v", hits, got)
	}
	if _, err := fetchArticle(ctx, c, flags, future); err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Fatalf("future FineVigenza was treated as settled (hits=%d)", hits)
	}

	orig, err := norma.ParseURN("urn:nir:stato:legge:1990-08-07;241~art3@originale")
	if err != nil {
		t.Fatal(err)
	}
	cacheArticle(ctx, articleView{URN: orig.String(), Testo: "original settled", FineVigenza: "1991-01-01"})
	text = "replaced original"
	got, err = fetchArticle(ctx, c, flags, orig)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 || !strings.Contains(got.Testo, "original settled") {
		t.Fatalf("@originale with a passed FineVigenza was not reused: hits=%d view=%+v", hits, got)
	}
}

func TestNoLocalDataSkipsCustomCaches(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv(noLocalDataEnvVar, "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200, "message": "ok",
			"data": map[string]any{"atto": map[string]any{
				"titolo":                    "Legge 241",
				"articoloHtml":              "<div>live text</div>",
				"articoloDataInizioVigenza": "19900807",
				"articoloDataFineVigenza":   "99999999",
			}},
		})
	}))
	defer srv.Close()
	t.Setenv("NORMATTIVA_BASE_URL", srv.URL)
	flags := &rootFlags{dataSource: "live", rateLimit: 0, noCache: true}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	u, err := norma.ParseURN("urn:nir:stato:legge:1990-08-07;241~art3")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := fetchArticle(ctx, c, flags, u); err != nil {
		t.Fatal(err)
	}
	cacheAttoRows(ctx, []attoRow{{CodiceRedazionale: "090G0294", Titolo: "Legge 241"}})
	dbPath := defaultDBPath(normaCLIName)
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("local store %s exists with %s=1 (stat err=%v)", dbPath, noLocalDataEnvVar, err)
	}
}
