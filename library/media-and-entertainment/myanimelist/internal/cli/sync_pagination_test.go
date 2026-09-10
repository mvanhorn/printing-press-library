package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil/testenv"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestRankingPaginationUsesRankOffset is the regression guard for the ranking
// offset misread. topanime.php and topmanga.php address the ranking by rank
// offset published in `limit` (limit=50 starts at rank 51), so the first
// request must send offset 0 and each later request must advance it by the page
// stride — never hold it fixed while advancing some other parameter.
func TestRankingPaginationUsesRankOffset(t *testing.T) {
	t.Parallel()

	pageSize := determinePaginationDefaults("ranking")
	if pageSize.cursorParam != "limit" {
		t.Fatalf("ranking cursorParam = %q, want %q (the rank offset lives in limit)", pageSize.cursorParam, "limit")
	}
	if pageSize.cursorType != "offset" {
		t.Fatalf("ranking cursorType = %q, want %q", pageSize.cursorType, "offset")
	}
	if pageSize.limitParam != "" {
		t.Fatalf("ranking limitParam = %q, want empty (limit is the offset, not a page size)", pageSize.limitParam)
	}
	if pageSize.limit != 50 {
		t.Fatalf("ranking stride = %d, want 50 ranks per request", pageSize.limit)
	}

	// The first page must not skip ranks 1-50.
	first := syncPaginationParams(pageSize, "")
	if len(first) != 1 || first["limit"] != "0" {
		t.Fatalf("first-page params = %v, want map[limit:0]", first)
	}

	// Offsets advance by the stride, so no window is repeated or skipped.
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 4; page++ {
		params := syncPaginationParams(pageSize, cursor)
		offset, ok := params["limit"]
		if !ok {
			t.Fatalf("page %d params = %v, want a limit offset", page, params)
		}
		if seen[offset] {
			t.Fatalf("page %d repeated offset %s; windows must advance", page, offset)
		}
		seen[offset] = true
		cursor = syncAdvanceOffset(cursor, pageSize.limit)
	}
	for _, want := range []string{"0", "50", "100", "150"} {
		if !seen[want] {
			t.Fatalf("offset %s never requested; got %v", want, seen)
		}
	}

	if topManga := determinePaginationDefaults("ranking-topmanga-php"); topManga != pageSize {
		t.Fatalf("topmanga pagination = %+v, want the same rank-offset paginator as ranking", topManga)
	}
}

// TestDefaultPaginationParamsUnchanged pins the ordinary shape for every other
// resource: page size in limit, position in page.
func TestDefaultPaginationParamsUnchanged(t *testing.T) {
	t.Parallel()

	pageSize := determinePaginationDefaults("anime")
	if pageSize.limitParam != "limit" || pageSize.limit != 100 {
		t.Fatalf("anime pagination = %+v, want limitParam=limit limit=100", pageSize)
	}
	if pageSize.cursorParam != "page" || pageSize.cursorType != "" {
		t.Fatalf("anime pagination = %+v, want cursorParam=page cursorType=\"\"", pageSize)
	}

	first := syncPaginationParams(pageSize, "")
	if len(first) != 1 || first["limit"] != "100" {
		t.Fatalf("first-page params = %v, want map[limit:100]", first)
	}
	second := syncPaginationParams(pageSize, "2")
	if len(second) != 2 || second["limit"] != "100" || second["page"] != "2" {
		t.Fatalf("second-page params = %v, want map[limit:100 page:2]", second)
	}
}

// rankingStubPage renders a page the extraction stage turns into enough items to
// make the sync loop advance a page.
func rankingStubPage(links int) string {
	var b strings.Builder
	b.WriteString(`<html><head><title>Top Anime - MyAnimeList.net</title></head><body><div class="ranking">`)
	for i := 1; i <= links; i++ {
		fmt.Fprintf(&b, `<a href="https://myanimelist.net/anime/%d/Top_%d">Top %d</a>`, i, i, i)
	}
	b.WriteString(`</div></body></html>`)
	return b.String()
}

func requestLimitOffset(t *testing.T, rawQuery string) int {
	t.Helper()
	for _, part := range strings.Split(rawQuery, "&") {
		if !strings.HasPrefix(part, "limit=") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(part, "limit="))
		if err != nil {
			t.Fatalf("limit parameter %q is not an integer: %v", part, err)
		}
		return n
	}
	t.Fatalf("ranking request %q carries no limit (rank offset) parameter", rawQuery)
	return 0
}

// TestRankingSyncSendsRankOffsetZero drives the real sync command against a
// local server and asserts the request the loop actually builds. The original
// defect lived in that loop, not in the helpers: ranking's `limit` is the rank
// offset, so the first page must ask for offset 0 — sending the 50-rank stride
// (or omitting the parameter, since the spec default is 50) skips ranks 1-50.
func TestRankingSyncSendsRankOffsetZero(t *testing.T) {
	testenv.Isolate(t)

	var mu sync.Mutex
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, rankingStubPage(60))
	}))
	defer srv.Close()
	t.Setenv("MYANIMELIST_BASE_URL", srv.URL)

	dbPath := filepath.Join(t.TempDir(), "sync.db")
	if _, stderr, err := runRootArgs(t, "sync", "--resources", "ranking", "--max-pages", "2", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("sync ranking: %v (stderr=%s)", err, stderr)
	}

	mu.Lock()
	got := append([]string(nil), queries...)
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("sync made no request for the ranking resource")
	}

	if first := requestLimitOffset(t, got[0]); first != 0 {
		t.Fatalf("first ranking request offset = %d (query %q), want 0 so ranks 1-50 are not skipped", first, got[0])
	}
	// Offsets must strictly increase: a repeated window means the paginator is
	// holding the offset while advancing something else.
	previous := -1
	for i, q := range got {
		offset := requestLimitOffset(t, q)
		if offset <= previous {
			t.Fatalf("ranking request %d (query %q) repeats or regresses the rank offset (previous %d)", i, q, previous)
		}
		previous = offset
	}
	if len(got) > 1 {
		if second := requestLimitOffset(t, got[1]); second != 50 {
			t.Fatalf("second ranking request offset = %d (query %q), want the 50-rank stride", second, got[1])
		}
	}
}
