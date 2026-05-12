// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// latest-news fans out across three Kickstarter surfaces in parallel —
// Discover newest in Technology, Discover most-funded overall, and the
// Magazine index page — then merges/dedupes the streams into a single
// agent-friendly feed of fresh signal.
//
// Output is JSONL by default (one record per line) so Scout can stream
// the result without materializing the whole slice; pass --json for a
// wrapped envelope.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/internal/store"
)

// latestNewsItem is the unified row shape across discover and magazine
// sources. discover items use name + blurb + goal/pledged/etc; magazine
// items use title and leave the funding fields zero.
type latestNewsItem struct {
	Source       string  `json:"source"`
	ID           int64   `json:"id,omitempty"`
	Slug         string  `json:"slug,omitempty"`
	Name         string  `json:"name"`
	Blurb        string  `json:"blurb,omitempty"`
	URL          string  `json:"url,omitempty"`
	LaunchedAt   int64   `json:"launched_at,omitempty"`
	State        string  `json:"state,omitempty"`
	Goal         float64 `json:"goal,omitempty"`
	Pledged      float64 `json:"pledged,omitempty"`
	BackersCount int     `json:"backers_count,omitempty"`
	Category     string  `json:"category,omitempty"`
	CreatorName  string  `json:"creator_name,omitempty"`
}

func newLatestNewsCmd(flags *rootFlags) *cobra.Command {
	var vertical string
	var limit int
	var since string
	var includeMagazine bool
	var jsonl bool

	cmd := &cobra.Command{
		Use:   "latest-news",
		Short: "Unified roll-up across Discover newest, Discover most-funded, and Magazine",
		Long: `Fan out across three Kickstarter surfaces in parallel and emit a single
ranked, deduplicated stream of fresh signal:

  1. Discover newest in Technology  (category-id 16, sort=newest)
  2. Discover most-funded overall   (sort=most_funded)
  3. Kickstarter Magazine index     (HTML, link-extracted; opt-out with
                                     --include-magazine=false)

Filter the result with --vertical <slug> to keep only projects whose
name+blurb contain at least one keyword for that vertical. Filter with
--since 7d to keep only projects launched within the window.

Default output is JSONL (one object per line) for agent streaming. Pass
--json for a single wrapped envelope.`,
		Example: `  kickstarter-pp-cli latest-news --vertical ai-agents --limit 25
  kickstarter-pp-cli latest-news --since 7d --json
  kickstarter-pp-cli latest-news --no-include-magazine --jsonl`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if vertical != "" {
				if _, ok := verticalKeywords[vertical]; !ok {
					return usageErr(fmt.Errorf("unknown vertical %q; available: %s", vertical, strings.Join(knownVerticals(), ", ")))
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sinceCutoff := int64(0)
			if since != "" {
				if secs, ok := parseSinceFlagSeconds(since); ok {
					sinceCutoff = time.Now().Unix() - secs
				} else {
					return usageErr(fmt.Errorf("invalid --since value %q: expected like 7d, 24h, 30m, 1w", since))
				}
			}
			items, fetched := fanoutLatestNewsAndProjects(cmd.Context(), c, includeMagazine)
			// Write-through cache: every project we saw lands as a
			// discover_project row so subsequent local-only commands
			// (vertical, funding-rank, category velocity) have data
			// without requiring a separate sync pass.
			ingestProjectsIfPossible(cmd.Context(), fetched)

			// Apply filters
			filtered := items[:0]
			seen := map[string]bool{}
			for _, it := range items {
				key := it.Slug
				if key == "" {
					key = it.URL
				}
				if key == "" {
					key = fmt.Sprintf("%s:%d", it.Source, it.ID)
				}
				if seen[key] {
					continue
				}
				if vertical != "" {
					hay := it.Name + " " + it.Blurb
					if !matchAnyVertical(vertical, hay) {
						continue
					}
				}
				if sinceCutoff > 0 && it.LaunchedAt > 0 && it.LaunchedAt < sinceCutoff {
					continue
				}
				seen[key] = true
				filtered = append(filtered, it)
			}
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}

			// JSONL path: one record per line on stdout
			if jsonl {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for _, it := range filtered {
					if err := enc.Encode(it); err != nil {
						return err
					}
				}
				return nil
			}
			// Wrapped JSON envelope
			env := map[string]any{
				"items":           filtered,
				"count":           len(filtered),
				"sources_queried": sourcesQueriedCount(includeMagazine),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&vertical, "vertical", "", "Filter by vertical slug (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max items to emit after filtering")
	cmd.Flags().StringVar(&since, "since", "", "Only items launched within window (e.g. 7d, 24h)")
	cmd.Flags().BoolVar(&includeMagazine, "include-magazine", true, "Include Kickstarter Magazine articles in the roll-up")
	cmd.Flags().BoolVar(&jsonl, "jsonl", false, "Emit JSONL (one record per line) instead of a wrapped envelope")
	return cmd
}

// sourcesQueriedCount returns how many fan-out sources the run consulted.
// Used by the --json envelope so callers can audit coverage without
// re-deriving the fan-out shape.
func sourcesQueriedCount(includeMagazine bool) int {
	if includeMagazine {
		return 3
	}
	return 2
}

// fanoutLatestNewsAndProjects runs the three sub-fetches in parallel and
// returns the merged result list plus the raw DiscoverProject payloads
// (so the caller can write-through into the local store). Errors from
// individual fan-out sources surface as stderr warnings rather than
// failing the whole command — partial data is more useful to an agent
// than nothing.
func fanoutLatestNewsAndProjects(ctx context.Context, c *client.Client, includeMagazine bool) ([]latestNewsItem, []store.DiscoverProject) {
	var (
		mu       sync.Mutex
		out      []latestNewsItem
		projects []store.DiscoverProject
		wg       sync.WaitGroup
	)

	append2 := func(items []latestNewsItem, projs []store.DiscoverProject) {
		mu.Lock()
		defer mu.Unlock()
		out = append(out, items...)
		projects = append(projects, projs...)
	}

	// Sub 1: Discover newest in Technology (category 16)
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, projs, err := fetchDiscoverAsItems(ctx, c, map[string]string{
			"format":      "json",
			"category_id": "16",
			"sort":        "newest",
			"page":        "1",
		}, "discover-newest")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: discover newest tech: %v\n", err)
			return
		}
		append2(items, projs)
	}()

	// Sub 2: Discover most-funded overall
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, projs, err := fetchDiscoverAsItems(ctx, c, map[string]string{
			"format": "json",
			"sort":   "most_funded",
			"page":   "1",
		}, "discover-funded")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: discover most-funded: %v\n", err)
			return
		}
		append2(items, projs)
	}()

	// Sub 3: Magazine (optional)
	if includeMagazine {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := fetchMagazineAsItems(ctx, c)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: magazine list: %v\n", err)
				return
			}
			append2(items, nil)
		}()
	}

	wg.Wait()
	return out, projects
}

// ingestProjectsIfPossible best-effort writes the projects into the
// local store. Never fails the caller — the data is still in `out`.
func ingestProjectsIfPossible(ctx context.Context, projects []store.DiscoverProject) {
	if len(projects) == 0 {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("kickstarter-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	if err := db.EnsureV2Schema(ctx); err != nil {
		return
	}
	_ = db.IngestDiscoverProjects(projects)
}

// fetchDiscoverAsItems calls /discover/advanced with the given params and
// returns (a) latestNewsItem rows for the agent stream and (b) the raw
// DiscoverProject payloads so the caller can write-through the local
// store. Both slices stay in the same order so callers correlating
// items[i] with projects[i] don't drift.
func fetchDiscoverAsItems(ctx context.Context, c *client.Client, params map[string]string, source string) ([]latestNewsItem, []store.DiscoverProject, error) {
	_ = ctx // c.Get has no ctx parameter; cancellation is enforced via Surf timeouts
	data, err := c.Get("/discover/advanced", params)
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		Projects []store.DiscoverProject `json:"projects"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, nil, fmt.Errorf("decoding discover response: %w", err)
	}
	out := make([]latestNewsItem, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		out = append(out, latestNewsItem{
			Source:       source,
			ID:           p.ID,
			Slug:         p.Slug,
			Name:         p.Name,
			Blurb:        p.Blurb,
			URL:          p.URLs.Web.Project,
			LaunchedAt:   p.LaunchedAt,
			State:        p.State,
			Goal:         p.Goal,
			Pledged:      p.Pledged,
			BackersCount: p.BackersCount,
			Category:     p.Category.Name,
			CreatorName:  p.Creator.Name,
		})
	}
	return out, resp.Projects, nil
}

// magazineLinkRE picks up Kickstarter Magazine article URLs in the
// rendered index HTML. The page is largely JS-driven, so this is a
// best-effort scrape — when no links match, fetchMagazineAsItems returns
// an empty slice rather than erroring.
var magazineLinkRE = regexp.MustCompile(`https?://www\.kickstarter\.com/(?:magazine|stories)/[a-z0-9][a-z0-9-]+`)
var magazineTitleRE = regexp.MustCompile(`<title[^>]*>(.*?)</title>`)

// fetchMagazineAsItems pulls the rendered /magazine HTML and extracts any
// article URLs it can spot. When the index has no inline articles (KS
// renders many cards client-side) it falls back to emitting one item
// pointing at the index itself so the source is still represented in the
// fan-out.
func fetchMagazineAsItems(ctx context.Context, c *client.Client) ([]latestNewsItem, error) {
	_ = ctx
	data, err := c.Get("/magazine", nil)
	if err != nil {
		return nil, err
	}
	// Response is HTML — Get returns it wrapped or raw depending on
	// resolver path. Try string first; fall back to JSON-string unwrap.
	html := string(data)
	if len(html) > 0 && html[0] == '"' {
		var s string
		if json.Unmarshal(data, &s) == nil {
			html = s
		}
	}
	urls := uniqueStrings(magazineLinkRE.FindAllString(html, -1))
	if len(urls) == 0 {
		// Surface the index itself as the magazine source so the source
		// count in the envelope remains honest.
		title := ""
		if m := magazineTitleRE.FindStringSubmatch(html); len(m) > 1 {
			title = strings.ReplaceAll(strings.TrimSpace(m[1]), "&mdash;", "—")
		}
		if title == "" {
			title = "Kickstarter Magazine"
		}
		return []latestNewsItem{{
			Source: "magazine",
			Name:   title,
			URL:    "https://www.kickstarter.com/magazine",
		}}, nil
	}
	out := make([]latestNewsItem, 0, len(urls))
	for _, u := range urls {
		slug := slugFromURL(u)
		out = append(out, latestNewsItem{
			Source: "magazine",
			Slug:   slug,
			Name:   slug,
			URL:    u,
		})
	}
	return out, nil
}

// uniqueStrings keeps the first occurrence of each value, preserving order.
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// slugFromURL returns the last path segment of a URL — used as a stable
// dedupe key for magazine items.
func slugFromURL(u string) string {
	idx := strings.LastIndex(u, "/")
	if idx < 0 || idx == len(u)-1 {
		return u
	}
	return u[idx+1:]
}
