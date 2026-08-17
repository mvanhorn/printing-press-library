// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH (tracks-resolve-setlist-to-uris):
// tracks resolve — turn "artist + title" into exactly one Spotify track URI.
//
// The Web API only offers /search, which answers with a ranked page of
// candidates. Every workflow that ends in "add these songs to a playlist"
// (a setlist, a tracklist, a list of recommendations) therefore has to run one
// search per title and then re-rank the page by hand, because Spotify's own
// relevance order routinely puts a live version, a remaster, or a cover above
// the studio recording the caller asked for. This command owns that ranking
// (exact title beats normalized-exact beats prefix; a track credited to the
// requested artist always beats one that is not) and emits a single URI, so
// the caller pipes straight into `playlists items add-to-playlist`.
//
// Reads titles from stdin (one per line, or "artist<TAB>title") so a whole
// setlist resolves in one invocation instead of N shell-loop iterations.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

// resolvedTrack is one row of `tracks resolve` output.
type resolvedTrack struct {
	Query      string   `json:"query"`
	QueryTitle string   `json:"query_title"`
	Artist     string   `json:"query_artist,omitempty"`
	URI        string   `json:"uri,omitempty"`
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	Artists    []string `json:"artists,omitempty"`
	Album      string   `json:"album,omitempty"`
	MatchKind  string   `json:"match_kind"`
	Candidates int      `json:"candidates"`
}

// resolveMatchRank scores a candidate against the requested title/artist.
// Lower is better; -1 means "reject". The ranking is the whole point of the
// command: Spotify's own relevance order is not title-exactness order.
func resolveMatchRank(candName string, candArtists []string, wantTitle, wantArtist string) int {
	name := strings.TrimSpace(candName)
	lowerName := strings.ToLower(name)
	lowerWant := strings.ToLower(strings.TrimSpace(wantTitle))

	artistOK := wantArtist == ""
	if !artistOK {
		lowerArtist := strings.ToLower(strings.TrimSpace(wantArtist))
		for _, a := range candArtists {
			if strings.Contains(strings.ToLower(a), lowerArtist) {
				artistOK = true
				break
			}
		}
	}
	// A wrong-artist hit is never preferable to a right-artist one, but it is
	// still better than no answer at all, so it is ranked, not rejected.
	artistPenalty := 0
	if !artistOK {
		artistPenalty = 100
	}

	switch {
	case lowerName == lowerWant:
		return artistPenalty + 0
	case normalizeTrackTitle(lowerName) == normalizeTrackTitle(lowerWant):
		// "Quantum Flux - Remastered", "4D (Live)" — same song, decorated.
		return artistPenalty + 1
	case strings.HasPrefix(lowerName, lowerWant):
		return artistPenalty + 2
	case strings.Contains(lowerName, lowerWant):
		return artistPenalty + 3
	}
	return -1
}

// normalizeTrackTitle strips the decorations Spotify appends to catalog titles
// ("- Remastered 2011", "(Live)", "feat. X") plus all punctuation, so that a
// setlist's bare title still matches the catalog entry.
func normalizeTrackTitle(s string) string {
	s = strings.ToLower(s)
	for _, cut := range []string{" - ", " (", " [", " feat.", " ft.", " featuring "} {
		if idx := strings.Index(s, cut); idx > 0 {
			s = s[:idx]
		}
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func newTracksResolveCmd(flags *rootFlags) *cobra.Command {
	var artist string
	var market string
	var limit int
	var failOnMiss bool

	cmd := &cobra.Command{
		Use:   "resolve [title]",
		Short: "Resolve artist + title to exactly one track URI (batch via stdin)",
		Long: `Resolve a song to a single Spotify track URI.

/search returns a ranked page, and its ranking is relevance, not title
exactness — a live cut, a remaster, or a cover regularly outranks the studio
recording you asked for. resolve re-ranks that page: exact title beats
normalized-exact (decorations like "- Remastered 2011" or "(Live)" removed)
beats prefix beats substring, and a track credited to --artist always beats
one that is not.

Reads titles from stdin when no positional title is given, one per line.
A line may also be "artist<TAB>title" to vary the artist per row, which
overrides --artist for that line. Blank lines and lines starting with # are
skipped, so a pasted setlist works as-is.

Default output is one URI per line, ready to pipe. --json emits the full
match record per row, including how the match was made and how many
candidates were considered.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2", "mcp:read-only": "true"},
		Example: `  spotify-pp-cli tracks resolve "4D" --artist Northlane
  spotify-pp-cli tracks resolve --artist Northlane --json <<'EOF'
  4D
  Quantum Flux
  EOF
  pbpaste | spotify-pp-cli tracks resolve --artist Northlane | \
    xargs -I{} echo {} > uris.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			type queryRow struct{ artist, title string }
			var rows []queryRow

			if len(args) > 0 {
				title := strings.TrimSpace(strings.Join(args, " "))
				if title == "" {
					return usageErr(fmt.Errorf("a title is required (positional arg or stdin lines)"))
				}
				rows = append(rows, queryRow{artist: artist, title: title})
			} else {
				scanner := bufio.NewScanner(cmd.InOrStdin())
				scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					rowArtist := artist
					title := line
					if a, t, ok := strings.Cut(line, "\t"); ok {
						rowArtist = strings.TrimSpace(a)
						title = strings.TrimSpace(t)
					}
					if title == "" {
						continue
					}
					rows = append(rows, queryRow{artist: rowArtist, title: title})
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("read titles from stdin: %w", err)
				}
				if len(rows) == 0 {
					return usageErr(fmt.Errorf("no titles given: pass a title argument or pipe one title per line on stdin"))
				}
			}

			if dryRunOK(flags) {
				out := make([]resolvedTrack, 0, len(rows))
				for _, r := range rows {
					out = append(out, resolvedTrack{
						Query:      searchQueryFor(r.artist, r.title),
						QueryTitle: r.title,
						Artist:     r.artist,
						MatchKind:  "dry_run",
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "results": out}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results := make([]resolvedTrack, 0, len(rows))
			missed := 0
			// One search per row, paced by the client's default 3 req/s, so a
			// full tracklist sits here for seconds with nothing on screen.
			// Gate the counter the way printProvenance gates provenance: a
			// human watching a TTY wants it, an agent reading a pipe does not.
			showProgress := len(rows) > 1 && isTerminal(cmd.OutOrStdout())
			for i, r := range rows {
				if showProgress {
					fmt.Fprintf(cmd.ErrOrStderr(), "\rresolving %d/%d: %-40.40s", i+1, len(rows), r.title)
				}
				q := searchQueryFor(r.artist, r.title)
				params := map[string]string{
					"q":     q,
					"type":  "track",
					"limit": fmt.Sprintf("%d", limit),
				}
				if market != "" {
					params["market"] = market
				}
				data, err := c.Get(cmd.Context(), "/search?"+encodeSearchParams(params), nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var resp struct {
					Tracks struct {
						Items []struct {
							ID      string `json:"id"`
							URI     string `json:"uri"`
							Name    string `json:"name"`
							Artists []struct {
								Name string `json:"name"`
							} `json:"artists"`
							Album struct {
								Name string `json:"name"`
							} `json:"album"`
						} `json:"items"`
					} `json:"tracks"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					return fmt.Errorf("decode search response for %q: %w", r.title, err)
				}

				row := resolvedTrack{
					Query:      q,
					QueryTitle: r.title,
					Artist:     r.artist,
					MatchKind:  "none",
					Candidates: len(resp.Tracks.Items),
				}
				bestRank := -1
				for _, item := range resp.Tracks.Items {
					names := make([]string, 0, len(item.Artists))
					for _, a := range item.Artists {
						names = append(names, a.Name)
					}
					rank := resolveMatchRank(item.Name, names, r.title, r.artist)
					if rank < 0 {
						continue
					}
					if bestRank >= 0 && rank >= bestRank {
						continue
					}
					bestRank = rank
					row.URI = item.URI
					row.ID = item.ID
					row.Name = item.Name
					row.Artists = names
					row.Album = item.Album.Name
				}
				row.MatchKind = resolveMatchKindLabel(bestRank)
				if row.URI == "" {
					missed++
				}
				results = append(results, row)
			}
			if showProgress {
				// Retire the counter line so the first row of output does not
				// land on top of it.
				fmt.Fprintf(cmd.ErrOrStderr(), "\r%-60s\r", "")
			}

			if flags.asJSON || flags.compact || flags.agent {
				// Emit the rows first: --fail-on-miss changes the exit code,
				// not the output, and a caller that asked for JSON still wants
				// to see which titles resolved before acting on the failure.
				if err := printJSONFilteredMeta(cmd.OutOrStdout(), map[string]any{
					"results":  results,
					"resolved": len(results) - missed,
					"missed":   missed,
				}, flags, map[string]any{"source": "live"}); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					if r.URI == "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "no match: %s\n", r.QueryTitle)
						continue
					}
					fmt.Fprintln(cmd.OutOrStdout(), r.URI)
				}
			}
			if missed > 0 && failOnMiss {
				return usageErr(fmt.Errorf("%d of %d titles did not resolve", missed, len(results)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&artist, "artist", "", "Artist to constrain and rank by (applies to every stdin line unless the line carries its own)")
	cmd.Flags().StringVar(&market, "market", "", "Market (ISO 3166-1 alpha-2) to resolve availability against")
	cmd.Flags().IntVar(&limit, "limit", 10, "Candidates to fetch per title before re-ranking")
	cmd.Flags().BoolVar(&failOnMiss, "fail-on-miss", false, "Exit 2 when any title fails to resolve")

	return cmd
}

// searchQueryFor builds the Spotify search query string for a title/artist pair.
func searchQueryFor(artist, title string) string {
	q := "track:" + title
	if strings.TrimSpace(artist) != "" {
		q += " artist:" + artist
	}
	return q
}

func encodeSearchParams(params map[string]string) string {
	values := url.Values{}
	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}
	return values.Encode()
}

func resolveMatchKindLabel(rank int) string {
	switch {
	case rank < 0:
		return "none"
	case rank == 0:
		return "exact"
	case rank == 1:
		return "normalized"
	case rank == 2:
		return "prefix"
	case rank == 3:
		return "substring"
	case rank == 100:
		return "exact_other_artist"
	case rank == 101:
		return "normalized_other_artist"
	case rank == 102:
		return "prefix_other_artist"
	default:
		return "substring_other_artist"
	}
}
