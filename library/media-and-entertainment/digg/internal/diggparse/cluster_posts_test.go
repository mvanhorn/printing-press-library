// Tests for the /ai/<clusterUrlId> posts parser. Two checked-in
// fixtures cover the load-bearing scenarios:
//
//   - cluster-buddhism-65idu2x5.html (9 posts, all types, multiple
//     authors, multiple images per post)
//   - cluster-nvidia-smsfvt1s.html (1 retweet with repost-context
//     chip pair, multi-paragraph body)
//
// Plus a synthetic-malformed-RSC test that confirms the parser
// tolerates a bad chunk without dropping the valid neighbors.

package diggparse

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadClusterFixture(t *testing.T, name string) []byte {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "testdata", name),
		filepath.Join("testdata", name),
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Fatalf("%s not found; tried: %v", name, candidates)
	return nil
}

func TestParseClusterPosts_BuddhismFixtureReturns9Posts(t *testing.T) {
	html := loadClusterFixture(t, "cluster-buddhism-65idu2x5.html")
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	if got := len(posts); got != 9 {
		t.Fatalf("got %d posts, want 9", got)
	}
	// Every post has non-empty post_x_id and author_username.
	for _, p := range posts {
		if p.PostXID == "" {
			t.Errorf("post with empty PostXID: %+v", p)
		}
		if p.Author.Username == "" {
			t.Errorf("post %s with empty author username", p.PostXID)
		}
	}
}

func TestParseClusterPosts_BuddhismFirstPostHasBodyAndMedia(t *testing.T) {
	html := loadClusterFixture(t, "cluster-buddhism-65idu2x5.html")
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	var first *ClusterPost
	for i := range posts {
		if posts[i].PostXID == "2053022747765457005" {
			first = &posts[i]
			break
		}
	}
	if first == nil {
		t.Fatal("expected to find post_x_id 2053022747765457005 in fixture")
	}
	if first.Author.Username != "tszzl" {
		t.Errorf("author.username = %q, want tszzl", first.Author.Username)
	}
	if first.PostType != "tweet" {
		t.Errorf("post_type = %q, want tweet", first.PostType)
	}
	if !first.BodyLoaded {
		t.Errorf("body_loaded = false; expected the top tweet's body to render in default HTML")
	}
	if first.Body == nil || *first.Body != "hmm" {
		got := "<nil>"
		if first.Body != nil {
			got = *first.Body
		}
		t.Errorf("body = %q, want %q", got, "hmm")
	}
	wantXURL := "https://x.com/tszzl/status/2053022747765457005"
	if first.XURL != wantXURL {
		t.Errorf("xUrl = %q, want %q", first.XURL, wantXURL)
	}
	// Two HH3NT* images.
	matches := 0
	for _, u := range first.MediaURLs {
		if strings.Contains(u, "HH3NTnv") || strings.Contains(u, "HH3NTns") {
			matches++
		}
	}
	if matches < 2 {
		t.Errorf("expected >=2 HH3NTn* media URLs, got %v", first.MediaURLs)
	}
	// Repost context only on retweet/quote.
	if first.Repost != nil {
		t.Errorf("first post is type=tweet; expected no repost_context, got %+v", first.Repost)
	}
}

func TestParseClusterPosts_BuddhismParsesAllPostTypes(t *testing.T) {
	html := loadClusterFixture(t, "cluster-buddhism-65idu2x5.html")
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	// Confirm the fixture exposes at least both tweet and reply types,
	// so the type-filter test in cli/posts_test.go has signal to assert
	// against. (quote/retweet are exercised in the NVIDIA fixture.)
	types := make(map[string]bool)
	for _, p := range posts {
		types[p.PostType] = true
	}
	for _, want := range []string{"tweet", "reply", "quote"} {
		if !types[want] {
			t.Errorf("expected fixture to expose post_type=%q at least once; got types=%v", want, types)
		}
	}
}

func TestParseClusterPosts_NVIDIAFixtureRetweetWithRepostContext(t *testing.T) {
	html := loadClusterFixture(t, "cluster-nvidia-smsfvt1s.html")
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	if len(posts) == 0 {
		t.Fatal("NVIDIA fixture should yield at least one post")
	}
	// The single post in the RSC array is the retweet by DanielleFong of
	// himanshustwts's tweet.
	var rt *ClusterPost
	for i := range posts {
		if posts[i].PostType == "retweet" {
			rt = &posts[i]
			break
		}
	}
	if rt == nil {
		t.Fatal("expected a post with post_type=retweet in NVIDIA fixture")
	}
	if rt.Author.Username != "DanielleFong" {
		t.Errorf("retweet author = %q, want DanielleFong (case-preserved from RSC)", rt.Author.Username)
	}
	// The body the page renders is the original tweet's text. The
	// strict-class match returns it verbatim.
	if rt.Body == nil {
		t.Fatal("expected retweet body to be populated from the strict-class wrap-anywhere paragraph")
	}
	if !strings.HasPrefix(*rt.Body, "the harness of claude code is very interesting") {
		t.Errorf("body should start with 'the harness of claude code is very interesting'; got %q",
			truncForTest(*rt.Body, 80))
	}
	if rt.Repost == nil {
		t.Fatal("expected repost_context to be populated for retweet")
	}
	if rt.Repost.RepostingHandle != "DanielleFong" {
		t.Errorf("repost_context.reposting_handle = %q, want DanielleFong", rt.Repost.RepostingHandle)
	}
	if rt.Repost.OriginalHandle != "himanshustwts" {
		t.Errorf("repost_context.original_handle = %q, want himanshustwts", rt.Repost.OriginalHandle)
	}
}

func TestParseClusterPosts_LazyLoadedBodyIsNullNotDropped(t *testing.T) {
	// Synthesize a fixture-like RSC payload with two posts: one whose
	// X anchor is rendered in the DOM (so body extraction can run) and
	// one whose post_x_id appears only in the RSC array (no anchor in
	// the DOM, simulating the "EXPAND DATA" lazy-load case). The lazy
	// post must come back with body=nil/body_loaded=false rather than
	// being filtered out.
	html := []byte(`<html><body>` +
		`<a href="https://x.com/foo/status/1111" target="_blank">link</a>` +
		`<p class="wrap-anywhere whitespace-pre-wrap font-sans text-base leading-6 text-foreground">eager body</p>` +
		`<script>self.__next_f.push([1,"\"posts\":[{\"post_x_id\":\"1111\",\"posted_at\":\"2026-05-09T08:02:21+00:00\",\"post_type\":\"tweet\",\"author_username\":\"foo\",\"author_display_name\":\"Foo\",\"author_category\":\"Researcher\",\"author_profile_image_url\":\"\",\"author_rank\":1},{\"post_x_id\":\"2222\",\"posted_at\":\"2026-05-09T08:03:21+00:00\",\"post_type\":\"reply\",\"author_username\":\"bar\",\"author_display_name\":\"Bar\",\"author_category\":\"Creator\",\"author_profile_image_url\":\"\",\"author_rank\":2}]"])</script>` +
		`</body></html>`)
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts (eager + lazy); got %d", len(posts))
	}
	var eager, lazy *ClusterPost
	for i := range posts {
		switch posts[i].PostXID {
		case "1111":
			eager = &posts[i]
		case "2222":
			lazy = &posts[i]
		}
	}
	if eager == nil || lazy == nil {
		t.Fatalf("lost a post in promotion; eager=%v lazy=%v", eager, lazy)
	}
	if !eager.BodyLoaded || eager.Body == nil || *eager.Body != "eager body" {
		got := "<nil>"
		if eager.Body != nil {
			got = *eager.Body
		}
		t.Errorf("eager post body=%q (loaded=%v), want %q (loaded=true)", got, eager.BodyLoaded, "eager body")
	}
	if lazy.BodyLoaded {
		t.Errorf("lazy post body_loaded=true; expected false (no DOM render)")
	}
	if lazy.Body != nil {
		t.Errorf("lazy post body=%q; expected nil", *lazy.Body)
	}
}

func TestParseClusterPosts_BodyHTMLEntitiesDecoded(t *testing.T) {
	// Next.js SSR HTML-encodes &, <, >, ", and ' inside the rendered
	// body paragraph. Without entity decoding, callers see &amp; /
	// &lt; / &gt; / &quot; / &#39; instead of the original characters.
	// Synthesize a fixture-shaped page whose strict-class body contains
	// every HTML-entity flavor we expect upstream to emit, then assert
	// the parsed body comes back with those entities decoded.
	html := []byte(`<html><body>` +
		`<a href="https://x.com/foo/status/1111" target="_blank">link</a>` +
		`<p class="wrap-anywhere whitespace-pre-wrap font-sans text-base leading-6 text-foreground">a &amp; b &lt; c &gt; d &quot;e&quot; &#39;f&#39;</p>` +
		`<script>self.__next_f.push([1,"\"posts\":[{\"post_x_id\":\"1111\",\"posted_at\":\"2026-05-09T08:02:21+00:00\",\"post_type\":\"tweet\",\"author_username\":\"foo\",\"author_display_name\":\"Foo\",\"author_category\":\"Researcher\",\"author_profile_image_url\":\"\",\"author_rank\":1}]"])</script>` +
		`</body></html>`)
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post; got %d", len(posts))
	}
	p := posts[0]
	if !p.BodyLoaded || p.Body == nil {
		t.Fatalf("expected body to be loaded; got loaded=%v body=%v", p.BodyLoaded, p.Body)
	}
	want := `a & b < c > d "e" 'f'`
	if *p.Body != want {
		t.Errorf("body = %q, want %q (HTML entities should be decoded)", *p.Body, want)
	}
}

func TestParseClusterPosts_BodyCompactClassEntitiesDecoded(t *testing.T) {
	// Same coverage for the compact-class fallback used on the
	// expand-on-hover row list. Two posts: the first with an X anchor
	// the strict-class body matches against, the second whose body is
	// rendered in the compact-class paragraph and contains entities.
	html := []byte(`<html><body>` +
		`<a href="https://x.com/foo/status/1111" target="_blank">link</a>` +
		`<p class="wrap-anywhere whitespace-pre-wrap font-sans text-base leading-6 text-foreground">eager body</p>` +
		`<a href="https://x.com/bar/status/2222" target="_blank">link</a>` +
		`<p class="wrap-anywhere text-foreground">x &amp; y &lt; z &quot;ok&quot;</p>` +
		`<script>self.__next_f.push([1,"\"posts\":[{\"post_x_id\":\"1111\",\"posted_at\":\"2026-05-09T08:02:21+00:00\",\"post_type\":\"tweet\",\"author_username\":\"foo\",\"author_display_name\":\"Foo\",\"author_category\":\"Researcher\",\"author_profile_image_url\":\"\",\"author_rank\":1},{\"post_x_id\":\"2222\",\"posted_at\":\"2026-05-09T08:03:21+00:00\",\"post_type\":\"reply\",\"author_username\":\"bar\",\"author_display_name\":\"Bar\",\"author_category\":\"Creator\",\"author_profile_image_url\":\"\",\"author_rank\":2}]"])</script>` +
		`</body></html>`)
	posts, err := ParseClusterPosts(html)
	if err != nil {
		t.Fatalf("ParseClusterPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts; got %d", len(posts))
	}
	var compact *ClusterPost
	for i := range posts {
		if posts[i].PostXID == "2222" {
			compact = &posts[i]
		}
	}
	if compact == nil {
		t.Fatal("expected post 2222 in result")
	}
	if !compact.BodyLoaded || compact.Body == nil {
		t.Fatalf("compact body not loaded; loaded=%v body=%v", compact.BodyLoaded, compact.Body)
	}
	want := `x & y < z "ok"`
	if *compact.Body != want {
		t.Errorf("compact body = %q, want %q", *compact.Body, want)
	}
}

func TestExtractClusterPosts_MalformedChunkSurfacedNotPanic(t *testing.T) {
	// Same shape as the roster_1000 test: keep braces balanced so the
	// scanObjectsContaining helper hands the chunk to the JSON decoder,
	// then corrupt the inner JSON. The malformed record drops; valid
	// records survive; the error reports the bad index.
	decoded := `{"post_x_id":"1","author_username":"alice","post_type":"tweet"}` +
		`{"post_x_id":"2","author_username":"bob","author_rank":not_a_number}` +
		`{"post_x_id":"3","author_username":"carol","post_type":"reply"}`
	posts, err := ExtractClusterPosts(decoded)
	if err == nil {
		t.Error("expected an error wrapping the malformed chunk index")
	}
	gotIDs := make(map[string]bool)
	for _, p := range posts {
		gotIDs[p.PostXID] = true
	}
	if !gotIDs["1"] || !gotIDs["3"] {
		t.Errorf("valid records dropped: got %v", gotIDs)
	}
	if gotIDs["2"] {
		t.Errorf("malformed bob record should not have decoded")
	}
}

func TestParseClusterPosts_LiveURL(t *testing.T) {
	if os.Getenv("DIGG_LIVE_TESTS") != "1" {
		t.Skip("set DIGG_LIVE_TESTS=1 to run live /ai/<id> fetch")
	}
	resp, err := http.Get("https://di.gg/ai/65idu2x5")
	if err != nil {
		t.Fatalf("live fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("live fetch status %d", resp.StatusCode)
	}
	buf := make([]byte, 4*1024*1024)
	n, _ := resp.Body.Read(buf)
	body := buf[:n]
	for {
		more := make([]byte, 4*1024*1024)
		m, _ := resp.Body.Read(more)
		if m == 0 {
			break
		}
		body = append(body, more[:m]...)
	}
	posts, err := ParseClusterPosts(body)
	if err != nil && len(posts) == 0 {
		t.Fatalf("live parse: %v", err)
	}
	if len(posts) == 0 {
		t.Errorf("live parse got 0 posts (cluster may have churned; rerun)")
	}
}

func truncForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
