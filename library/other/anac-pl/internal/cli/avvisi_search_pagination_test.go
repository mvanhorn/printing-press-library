package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAvvisiSearchAllFollowsContinuationTokens(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tokens = append(tokens, q.Get("tokenPaginazione"))
		if q.Get("keywords") != "test" || q.Get("atlasFuzzySearchEnabled") != "false" {
			t.Errorf("search filters lost between pages: %v", q)
		}
		if r.Header.Get("X-Test-Header") != "preserved" {
			t.Errorf("endpoint headers lost between pages: %v", r.Header)
		}
		if q.Get("tokenPaginazione") == "" {
			// A continuation token remains authoritative even on a short page.
			fmt.Fprint(w, `{"count":2,"content":[{"idAvviso":"first"}],"lastPaginationToken":"next"}`)
			return
		}
		if q.Get("tokenPaginazione") != "next" || q.Get("direzionePaginazione") != "AVANTI" {
			t.Errorf("missing ANAC continuation parameters: %v", q)
		}
		fmt.Fprint(w, `{"count":2,"content":[{"idAvviso":"second"}],"lastPaginationToken":""}`)
	}))
	defer srv.Close()
	data, prov, err := resolvePaginatedReadWithStrategy(t.Context(), clientVerso(srv.URL),
		&rootFlags{dataSource: "live"}, "auto", "avvisi", "/avvisi-full-text",
		map[string]string{"keywords": "test", "size": "2", "atlasFuzzySearchEnabled": "false"},
		map[string]string{"X-Test-Header": "preserved"}, true, "", "offset", "", "", "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		ID string `json:"idAvviso"`
	}
	if err := json.Unmarshal(data, &items); err != nil || len(items) != 2 || items[1].ID != "second" {
		t.Fatalf("incomplete --all result: %s, err=%v", data, err)
	}
	if fmt.Sprint(tokens) != "[ next]" || prov.Source != "live" {
		t.Fatalf("tokens=%v provenance=%+v", tokens, prov)
	}
}

func TestAvvisiSearchAllRejectsIncompleteContinuation(t *testing.T) {
	for _, response := range []string{
		`{"content":`,
		`{"error":"temporary failure"}`,
		`{"content":null}`,
		`{"content":[{"idAvviso":"second"}],"lastPaginationToken":"same"}`,
		`{"content":[],"lastPaginationToken":"next"}`,
	} {
		t.Run(response, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("tokenPaginazione") == "" {
					fmt.Fprint(w, `{"content":[{"idAvviso":"first"}],"lastPaginationToken":"same"}`)
					return
				}
				fmt.Fprint(w, response)
			}))
			defer srv.Close()
			data, err := paginatedGet(t.Context(), clientVerso(srv.URL), "/avvisi-full-text",
				map[string]string{"size": "1"}, nil, true, "", "offset", "", "", "")
			if err == nil || len(data) != 0 {
				t.Fatalf("partial results must not become success: data=%s err=%v", data, err)
			}
		})
	}
}

func TestAvvisiSearchAllContinuesAfterDuplicateOnlyPage(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("tokenPaginazione")
		tokens = append(tokens, token)
		switch token {
		case "":
			fmt.Fprint(w, `{"content":[{"idAvviso":"first"}],"lastPaginationToken":"next-1"}`)
		case "next-1":
			fmt.Fprint(w, `{"content":[{"idAvviso":"first"}],"lastPaginationToken":"next-2"}`)
		case "next-2":
			fmt.Fprint(w, `{"content":[{"idAvviso":"second"}],"lastPaginationToken":""}`)
		default:
			t.Errorf("unexpected continuation token %q", token)
		}
	}))
	defer srv.Close()
	data, err := paginatedGet(t.Context(), clientVerso(srv.URL), "/avvisi-full-text",
		map[string]string{"size": "1"}, nil, true, "", "offset", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		ID string `json:"idAvviso"`
	}
	if err := json.Unmarshal(data, &items); err != nil || len(items) != 2 || items[1].ID != "second" {
		t.Fatalf("duplicate-only page stopped token pagination: %s, err=%v", data, err)
	}
	if fmt.Sprint(tokens) != "[ next-1 next-2]" {
		t.Fatalf("unexpected tokens: %v", tokens)
	}
}

func TestAvvisiSearchAllAcceptsValidEmptyArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"content":[],"lastPaginationToken":""}`)
	}))
	defer srv.Close()
	data, err := paginatedGet(t.Context(), clientVerso(srv.URL), "/avvisi-full-text",
		map[string]string{"size": "1"}, nil, true, "", "offset", "", "", "")
	if err != nil || string(data) != "[]" {
		t.Fatalf("valid empty search was rejected: data=%s err=%v", data, err)
	}
}

func TestAvvisiSearchWithoutAllKeepsSinglePageEnvelope(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"count":2,"content":[{"idAvviso":"first"}],"lastPaginationToken":"next"}`)
	}))
	defer srv.Close()
	data, err := paginatedGet(t.Context(), clientVerso(srv.URL), "/avvisi-full-text",
		map[string]string{"size": "1"}, nil, false, "", "offset", "", "", "")
	if err != nil || requests != 1 || !strings.Contains(string(data), `"lastPaginationToken":"next"`) {
		t.Fatalf("single-page behavior changed: requests=%d data=%s err=%v", requests, data, err)
	}
}
