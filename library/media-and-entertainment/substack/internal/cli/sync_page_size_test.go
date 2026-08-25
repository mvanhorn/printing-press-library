// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// substackRejectsLimitAbove is the highest limit Substack's list endpoints
// accept. Measured live against www.thefp.com and astralcodexten.substack.com
// while fixing https://github.com/mvanhorn/printing-press-library/issues/1524:
// limit=50 returns 200, limit=51 returns HTTP 400 with a query-validation body.
const substackRejectsLimitAbove = 50

func TestClampSyncPageSizeLowersGeneratedDefault(t *testing.T) {
	t.Parallel()
	if got := clampSyncPageSize("posts", 100); got != substackMaxPageSize {
		t.Fatalf("clampSyncPageSize(\"posts\", 100) = %d, want %d", got, substackMaxPageSize)
	}
}

// /post_management/published caps below the shared limit, so it needs its own
// page size rather than the one the other resources use.
func TestClampSyncPageSizeUsesPublishedCap(t *testing.T) {
	t.Parallel()
	if got := clampSyncPageSize("posts-published", 100); got != publishedPostsPageSize {
		t.Fatalf("clampSyncPageSize(\"posts-published\", 100) = %d, want %d", got, publishedPostsPageSize)
	}
}

func TestClampSyncPageSizeKeepsSmallerLimit(t *testing.T) {
	t.Parallel()
	if got := clampSyncPageSize("posts", 5); got != 5 {
		t.Fatalf("clampSyncPageSize(\"posts\", 5) = %d, want 5: a page size already below the cap must be left alone", got)
	}
}

// Every resource that paginates must end up at or below the limit Substack
// validates, otherwise sync fails that resource outright.
func TestEveryPaginatedResourceClearsSubstackLimitCap(t *testing.T) {
	t.Parallel()
	generated := determinePaginationDefaults().limit

	for _, resource := range knownSyncResourceNames() {
		if !resourceSupportsPagination(resource) {
			continue
		}
		effective := clampSyncPageSize(resource, generated)
		if effective > substackRejectsLimitAbove {
			t.Errorf("resource %q syncs at page size %d; Substack rejects anything above %d with HTTP 400",
				resource, effective, substackRejectsLimitAbove)
		}
		if effective <= 0 {
			t.Errorf("resource %q syncs at non-positive page size %d", resource, effective)
		}
	}
}

// newFakeSubstackList reproduces Substack's limit validation: it rejects any
// limit above the cap the way the live API does, and otherwise serves an
// offset-addressed window of posts.
func newFakeSubstackList(totalPosts, rejectAbove int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		if limit > rejectAbove {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"errors":[{"location":"query","param":"limit","value":%d,"msg":"Invalid value"}]}`, limit)
			return
		}

		posts := []map[string]int{}
		for id := offset; id < offset+limit && id < totalPosts; id++ {
			posts = append(posts, map[string]int{"id": id})
		}
		_ = json.NewEncoder(w).Encode(posts)
	}))
}

// fetchFirstPage issues the request syncResource issues for a resource's first
// page and reports how many records came back.
func fetchFirstPage(serverURL string, pageSize int) (int, error) {
	resp, err := http.Get(fmt.Sprintf("%s/list?offset=0&limit=%d", serverURL, pageSize))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var posts []struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return 0, err
	}
	return len(posts), nil
}

// The page size the generator emits is what issue #1524 reported: the request
// is rejected and the resource yields nothing.
func TestGeneratedPageSizeIsRejectedBySubstack(t *testing.T) {
	t.Parallel()
	server := newFakeSubstackList(60, substackRejectsLimitAbove)
	defer server.Close()

	if _, err := fetchFirstPage(server.URL, determinePaginationDefaults().limit); err == nil {
		t.Fatal("expected the unclamped generated page size to be rejected with HTTP 400")
	}
}

// The clamped page size is accepted and returns a full page of records.
func TestClampedPageSizeIsAcceptedAndReturnsRecords(t *testing.T) {
	t.Parallel()
	server := newFakeSubstackList(60, substackRejectsLimitAbove)
	defer server.Close()

	for _, resource := range []string{"posts", "drafts", "posts-published"} {
		pageSize := clampSyncPageSize(resource, determinePaginationDefaults().limit)

		got, err := fetchFirstPage(server.URL, pageSize)
		if err != nil {
			t.Fatalf("resource %q at page size %d was rejected: %v", resource, pageSize, err)
		}
		if got != pageSize {
			t.Errorf("resource %q collected %d records at page size %d, want a full page", resource, got, pageSize)
		}
	}
}

// posts-published caps lower than the other resources, so a page size that
// works elsewhere must not be reused for it.
func TestPublishedRejectsTheSharedPageSize(t *testing.T) {
	t.Parallel()
	server := newFakeSubstackList(60, publishedPostsPageSize)
	defer server.Close()

	if _, err := fetchFirstPage(server.URL, substackMaxPageSize); err == nil {
		t.Fatalf("page size %d should be rejected by an endpoint capped at %d",
			substackMaxPageSize, publishedPostsPageSize)
	}
	if _, err := fetchFirstPage(server.URL, clampSyncPageSize("posts-published", 100)); err != nil {
		t.Fatalf("clamped posts-published page size was rejected: %v", err)
	}
}

func TestReconcileSyncPageSizeAfterUserOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		resource   string
		userParams *syncUserParams
		want       int
	}{
		{
			name:       "--param smaller than cap",
			resource:   "posts",
			userParams: &syncUserParams{flatGlobal: map[string]string{"limit": "12"}},
			want:       12,
		},
		{
			name:       "--global-param above cap",
			resource:   "posts",
			userParams: &syncUserParams{trueGlobal: map[string]string{"limit": "1000"}},
			want:       substackMaxPageSize,
		},
		{
			name:     "--resource-param uses published cap",
			resource: "posts-published",
			userParams: &syncUserParams{perResource: map[string]map[string]string{
				"posts-published": {"limit": "1000"},
			}},
			want: publishedPostsPageSize,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pageSize := determinePaginationDefaults()
			params := map[string]string{
				pageSize.limitParam: strconv.Itoa(pageSize.limit),
			}
			tt.userParams.applyTo(tt.resource, params, false)

			if err := reconcileSyncPageSize(tt.resource, &pageSize, params); err != nil {
				t.Fatalf("reconcileSyncPageSize() error = %v", err)
			}
			if pageSize.limit != tt.want {
				t.Errorf("paginator limit = %d, want %d", pageSize.limit, tt.want)
			}
			if got := params[pageSize.limitParam]; got != strconv.Itoa(tt.want) {
				t.Errorf("request limit = %q, want %q", got, strconv.Itoa(tt.want))
			}
		})
	}
}

func TestReconcileSyncPageSizeRejectsInvalidOverride(t *testing.T) {
	t.Parallel()

	for _, limit := range []string{"", "0", "-1", "not-a-number"} {
		limit := limit
		t.Run("limit="+limit, func(t *testing.T) {
			t.Parallel()

			pageSize := determinePaginationDefaults()
			params := map[string]string{pageSize.limitParam: limit}
			if err := reconcileSyncPageSize("posts", &pageSize, params); err == nil {
				t.Fatalf("reconcileSyncPageSize() accepted invalid limit %q", limit)
			}
		})
	}
}
