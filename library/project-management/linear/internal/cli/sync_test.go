package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/config"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

func TestSyncProjectsUsesComplexitySafePageSize(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var seenFirst []int
	var seenAfter []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("path = %s, want /graphql", r.URL.Path)
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		first, ok := req.Variables["first"].(float64)
		if !ok {
			t.Errorf("first variable = %#v, want number", req.Variables["first"])
			http.Error(w, "bad first", http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()
		seenFirst = append(seenFirst, int(first))

		after, _ := req.Variables["after"].(string)
		seenAfter = append(seenAfter, after)

		hasNext := after == ""
		endCursor := ""
		if hasNext {
			endCursor = "cursor-1"
		}
		fmt.Fprintf(w, `{"data":{"projects":{"nodes":[{"id":"project-%d","name":"Project %d","state":"active","lead":{"id":"lead-1"},"targetDate":"2026-07-01"}],"pageInfo":{"hasNextPage":%t,"endCursor":%q}}}}`, len(seenFirst), len(seenFirst), hasNext, endCursor)
	}))
	t.Cleanup(srv.Close)

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	db, err := store.Open(filepath.Join(t.TempDir(), "linear.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	got, err := syncProjects(c, db, 0)
	if err != nil {
		t.Fatalf("syncProjects returned error: %v", err)
	}
	if got != 2 {
		t.Fatalf("synced projects = %d, want 2", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []int{linearProjectsSyncPageSize, linearProjectsSyncPageSize}; !slices.Equal(seenFirst, want) {
		t.Fatalf("first values = %v, want %v", seenFirst, want)
	}
	if want := []string{"", "cursor-1"}; !slices.Equal(seenAfter, want) {
		t.Fatalf("after values = %v, want %v", seenAfter, want)
	}
}

func TestSyncLabelsStoresTeamOwnership(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("path = %s, want /graphql", r.URL.Path)
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "team { id name key }") {
			t.Errorf("label sync query omitted team ownership: %s", req.Query)
			http.Error(w, "missing team", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-1","name":"pipeline-halt","color":"#f00","createdAt":"2026-06-10T00:00:00Z","team":{"id":"team-1","name":"Symphony","key":"SYMPH"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
	}))
	t.Cleanup(srv.Close)

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	db, err := store.Open(filepath.Join(t.TempDir(), "linear.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	got, err := syncLabels(c, db, 0)
	if err != nil {
		t.Fatalf("syncLabels returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("synced labels = %d, want 1", got)
	}
	raw, err := db.GetByID("issue_labels", "label-1")
	if err != nil {
		t.Fatalf("get issue label: %v", err)
	}
	var label struct {
		Team struct {
			Key string `json:"key"`
		} `json:"team"`
	}
	if err := json.Unmarshal(raw, &label); err != nil {
		t.Fatalf("decode label: %v", err)
	}
	if label.Team.Key != "SYMPH" {
		t.Fatalf("team key = %q, want SYMPH", label.Team.Key)
	}
}
