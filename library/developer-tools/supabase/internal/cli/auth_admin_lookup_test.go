// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestLookupAuthAdminUser_ExactMatchAfterFirstPageMixedCase(t *testing.T) {
	var mu sync.Mutex
	var pages []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("apikey"); got != "test-secret" {
			t.Errorf("apikey header = %q, want test secret", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization header = %q, want bearer test secret", got)
		}
		assertOnlyPaginationQuery(t, r.URL.Query())
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		mu.Lock()
		pages = append(pages, page)
		mu.Unlock()
		w.Header().Set("x-total-count", "3")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case 1:
			fmt.Fprint(w, `{"users":[{"id":"u-1","email":"other-one@example.test"},{"id":"u-2","email":"other-two@example.test"}]}`)
		case 2:
			fmt.Fprint(w, `{"users":[{"id":"target-id","email":"Target@Example.Test","created_at":"2026-07-12T00:00:00Z"}]}`)
		default:
			t.Errorf("unexpected page %d", page)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	user, err := lookupAuthAdminUser(context.Background(), testProjectSurface(server.URL), " target@example.test ", authAdminLookupOptions{PerPage: 2, MaxPages: 4})
	if err != nil {
		t.Fatalf("lookupAuthAdminUser() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(user, &got); err != nil {
		t.Fatalf("unmarshal matched user: %v", err)
	}
	if got["id"] != "target-id" || got["email"] != "Target@Example.Test" {
		t.Fatalf("matched user = %#v", got)
	}
	if fmt.Sprint(got) == "" || strings.Contains(fmt.Sprint(got), "other-one") {
		t.Fatalf("matched payload leaked an unrelated user: %#v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(pages) != "[1 2]" {
		t.Fatalf("requested pages = %v, want [1 2]", pages)
	}
}

func TestLookupAuthAdminUser_FailsClosed(t *testing.T) {
	const unrelated = "unrelated-user-sentinel@example.test"
	tests := []struct {
		name     string
		maxPages int
		handler  http.HandlerFunc
	}{
		{
			name: "zero exact matches",
			handler: pageHandler("1", map[int]string{
				1: `{"users":[{"id":"u-1","email":"` + unrelated + `"}]}`,
			}),
		},
		{
			name: "duplicate exact email across pages",
			handler: pageHandler("3", map[int]string{
				1: `{"users":[{"id":"u-1","email":"target@example.test"},{"id":"u-2","email":"` + unrelated + `"}]}`,
				2: `{"users":[{"id":"u-3","email":"TARGET@example.test"}]}`,
			}),
		},
		{
			name: "malformed envelope",
			handler: pageHandler("1", map[int]string{
				1: `{"unexpected":[{"id":"u-1","email":"` + unrelated + `"}]}`,
			}),
		},
		{
			name: "missing pagination metadata",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, `{"users":[{"id":"u-1","email":"`+unrelated+`"}]}`)
			},
		},
		{
			name: "invalid pagination metadata",
			handler: pageHandler("not-a-number", map[int]string{
				1: `{"users":[{"id":"u-1","email":"` + unrelated + `"}]}`,
			}),
		},
		{
			name: "malformed user",
			handler: pageHandler("1", map[int]string{
				1: `{"users":[{"id":"u-1","metadata":"` + unrelated + `"}]}`,
			}),
		},
		{
			name: "repeated page",
			handler: pageHandler("4", map[int]string{
				1: `{"users":[{"id":"u-1","email":"` + unrelated + `"},{"id":"u-2","email":"other@example.test"}]}`,
				2: `{"users":[{"id":"u-1","email":"` + unrelated + `"},{"id":"u-2","email":"other@example.test"}]}`,
			}),
		},
		{
			name: "truncated traversal",
			handler: pageHandler("3", map[int]string{
				1: `{"users":[{"id":"u-1","email":"` + unrelated + `"}]}`,
				2: `{"users":[]}`,
			}),
		},
		{
			name: "inconsistent total",
			handler: func(w http.ResponseWriter, r *http.Request) {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				if page == 1 {
					w.Header().Set("x-total-count", "3")
					fmt.Fprint(w, `{"users":[{"id":"u-1","email":"`+unrelated+`"},{"id":"u-2","email":"other@example.test"}]}`)
					return
				}
				w.Header().Set("x-total-count", "4")
				fmt.Fprint(w, `{"users":[{"id":"u-3","email":"target@example.test"}]}`)
			},
		},
		{
			name:     "page count limit",
			maxPages: 1,
			handler: pageHandler("3", map[int]string{
				1: `{"users":[{"id":"u-1","email":"` + unrelated + `"},{"id":"u-2","email":"other@example.test"}]}`,
			}),
		},
		{
			name: "rate limited provider response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"message":"rate limited","user":"`+unrelated+`"}`)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			t.Cleanup(server.Close)
			maxPages := tc.maxPages
			if maxPages == 0 {
				maxPages = 4
			}
			user, err := lookupAuthAdminUser(context.Background(), testProjectSurface(server.URL), "target@example.test", authAdminLookupOptions{PerPage: 2, MaxPages: maxPages})
			if err == nil {
				t.Fatalf("lookupAuthAdminUser() user = %s, want fail-closed error", user)
			}
			if len(user) != 0 {
				t.Fatalf("lookupAuthAdminUser() returned user data on error: %s", user)
			}
			if strings.Contains(err.Error(), unrelated) {
				t.Fatalf("error leaked unrelated user data: %v", err)
			}
		})
	}
}

func TestLookupAuthAdminUser_ProviderOutageDoesNotLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	ps := testProjectSurface(server.URL)
	server.Close()
	user, err := lookupAuthAdminUser(context.Background(), ps, "target@example.test", authAdminLookupOptions{PerPage: 2, MaxPages: 2})
	if err == nil || len(user) != 0 {
		t.Fatalf("outage result user=%s err=%v, want empty user and error", user, err)
	}
	if strings.Contains(err.Error(), "target@example.test") {
		t.Fatalf("outage error leaked lookup email: %v", err)
	}
}

func TestLookupAuthAdminUser_ConcurrentReadOnlyLookups(t *testing.T) {
	server := httptest.NewServer(pageHandler("1", map[int]string{
		1: `{"users":[{"id":"target-id","email":"target@example.test"}]}`,
	}))
	t.Cleanup(server.Close)
	ps := testProjectSurface(server.URL)

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user, err := lookupAuthAdminUser(context.Background(), ps, "TARGET@example.test", authAdminLookupOptions{PerPage: 2, MaxPages: 2})
			if err != nil {
				errs <- err
				return
			}
			if !bytes.Contains(user, []byte(`"id":"target-id"`)) {
				errs <- fmt.Errorf("unexpected user payload: %s", user)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestAuthAdminLookupCommandOutputsOnlyExactMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertOnlyPaginationQuery(t, r.URL.Query())
		w.Header().Set("x-total-count", "2")
		fmt.Fprint(w, `{"users":[{"id":"unrelated-id","email":"unrelated@example.test"},{"id":"target-id","email":"target@example.test"}]}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "test-secret")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"auth-admin", "lookup", "TARGET@example.test", "--json"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("auth-admin lookup command error = %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "target-id") || strings.Contains(got, "unrelated-id") || strings.Contains(got, "unrelated@example.test") {
		t.Fatalf("command output did not stay exact and non-disclosing: %s", got)
	}
}

func TestAuthAdminLookupCommandProviderErrorDoesNotLeak(t *testing.T) {
	const unrelated = "unrelated-provider-sentinel@example.test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"message":"provider unavailable","user":"`+unrelated+`"}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("SUPABASE_URL", server.URL)
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "test-secret")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"auth-admin", "lookup", "target@example.test", "--json"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("auth-admin lookup command succeeded on provider error")
	}
	if stdout.Len() != 0 || strings.Contains(err.Error(), unrelated) || strings.Contains(err.Error(), "target@example.test") {
		t.Fatalf("provider failure leaked lookup data: stdout=%q err=%v", stdout.String(), err)
	}
}

func testProjectSurface(baseURL string) *projectSurface {
	return &projectSurface{
		BaseURL:    baseURL,
		SecretKey:  "test-secret",
		ProjectRef: "test-ref",
		httpClient: http.DefaultClient,
	}
}

func pageHandler(total string, pages map[int]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		body, ok := pages[page]
		if !ok {
			body = `{"users":[]}`
		}
		w.Header().Set("x-total-count", total)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}
}

func assertOnlyPaginationQuery(t *testing.T, query url.Values) {
	t.Helper()
	if len(query) != 2 || query.Get("page") == "" || query.Get("per_page") == "" {
		t.Errorf("query = %v, want only page and per_page", query)
	}
	if query.Has("email") {
		t.Errorf("query unexpectedly contains unsupported email parameter: %v", query)
	}
}
