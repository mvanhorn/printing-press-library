package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/myanimelist/internal/cliutil/testenv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// adaptationAnimeFixture is a title page with a known announced total (25) that
// is still airing, and a related manga entry marked "Adaptation".
const adaptationAnimeFixture = `<html><head><title>Airing Show - MyAnimeList.net</title></head><body>
<h1 class="h1"><strong>Airing Show</strong></h1>
<div class="leftside">
<div class="spaceit_pad"><span class="dark_text">Type:</span> TV</div>
<div class="spaceit_pad"><span class="dark_text">Episodes:</span> 25</div>
<div class="spaceit_pad"><span class="dark_text">Status:</span> Currently Airing</div>
</div>
<h2 id="related_entries">Related Entries</h2></div></div><div class="related-entries">
<div class="entries-tile">
<div class="entry borderClass "><div class="image"><a href="https://myanimelist.net/manga/2/Airing_Show"><img src="x"></a></div><div class="information"><div class="title"><a href="https://myanimelist.net/manga/2/Airing_Show">Airing Show</a></div><div class="spaceit_pad"><span>Adaptation</span></div></div></div>
</div></div></body></html>`

const adaptationMangaFixture = `<html><head><title>Airing Show - MyAnimeList.net</title></head><body>
<h1 class="h1"><strong>Airing Show</strong></h1>
<div class="leftside">
<div class="spaceit_pad"><span class="dark_text">Type:</span> Manga</div>
<div class="spaceit_pad"><span class="dark_text">Volumes:</span> 12</div>
<div class="spaceit_pad"><span class="dark_text">Chapters:</span> 100</div>
<div class="spaceit_pad"><span class="dark_text">Status:</span> Finished</div>
</div></body></html>`

// adaptationEpisodeFixture renders an aired-episode table with n rows.
func adaptationEpisodeFixture(n int) string {
	var b strings.Builder
	b.WriteString(`<html><head><title>Airing Show - Episodes - MyAnimeList.net</title></head><body><table>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b,
			`<tr class="episode-aired"><td><a href="https://myanimelist.net/anime/52991/Airing_Show/episode/%d">%d</a> Episode %d · average 4.%d</td></tr>`,
			i, i, i, i)
	}
	b.WriteString(`</table></body></html>`)
	return b.String()
}

// TestAdaptationUsesAiredEpisodeCountNotAnnouncedTotal drives the real command
// against fixture pages. The fix is the episode-table read: a still-airing show
// announced at 25 episodes but five episodes into its run must not report the
// whole 100-chapter source as covered. Without that read the coverage band is
// computed from the announced total and lands at 80-100 chapters again.
func TestAdaptationUsesAiredEpisodeCountNotAnnouncedTotal(t *testing.T) {
	testenv.Isolate(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/anime/52991":
			fmt.Fprint(w, adaptationAnimeFixture)
		case "/manga/2":
			fmt.Fprint(w, adaptationMangaFixture)
		case "/anime/52991/_/episode":
			fmt.Fprint(w, adaptationEpisodeFixture(5))
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `<html><body>not found</body></html>`)
		}
	}))
	defer srv.Close()
	t.Setenv("MYANIMELIST_BASE_URL", srv.URL)

	stdout, stderr, err := runRootArgs(t, "adaptation", "52991", "--json")
	if err != nil {
		t.Fatalf("adaptation 52991: %v (stderr=%s)", err, stderr)
	}

	var view adaptationView
	if err := json.Unmarshal([]byte(stdout), &view); err != nil {
		t.Fatalf("decoding adaptation output %q: %v", stdout, err)
	}
	if view.AnimeEpisodes != 25 {
		t.Fatalf("anime_episodes = %d, want the announced 25", view.AnimeEpisodes)
	}
	if view.ReachedEpisodes != 5 {
		t.Fatalf("reached_episodes = %d, want 5 from the episode table (not the announced 25)", view.ReachedEpisodes)
	}
	if view.MangaChapters != 100 {
		t.Fatalf("manga_chapters = %d, want 100", view.MangaChapters)
	}
	// 100 chapters / 25 announced episodes = 4 per episode; five aired episodes
	// is a mid of 20, so the band is 16-24 and nowhere near the source total.
	if view.ChaptersPerEpisode != 4 {
		t.Fatalf("chapters_per_episode = %v, want 4", view.ChaptersPerEpisode)
	}
	if view.CoveredLow != 16 || view.CoveredHigh != 24 {
		t.Fatalf("coverage band = %d-%d, want 16-24", view.CoveredLow, view.CoveredHigh)
	}
	if view.CoveredHigh >= view.MangaChapters {
		t.Fatalf("coverage band %d-%d assumes the %d-chapter source was fully covered", view.CoveredLow, view.CoveredHigh, view.MangaChapters)
	}
	if !strings.Contains(view.Note, "still airing") {
		t.Fatalf("note = %q, want the still-airing caveat", view.Note)
	}
}
