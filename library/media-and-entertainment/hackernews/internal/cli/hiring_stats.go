package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// hiring-stats fetches the latest N "Who is hiring?" threads, walks
// every top-level post, and aggregates language mentions, remote
// percentage, and top-mentioned company names.
//
// We deliberately use simple keyword-counting heuristics rather than
// LLM extraction — the goal is a deterministic, reproducible structured
// output, not a polished prose summary.

type hiringStats struct {
	Months         int            `json:"months_scanned"`
	ThreadsScanned int            `json:"threads_scanned"`
	PostsScanned   int            `json:"posts_scanned"`
	Languages      map[string]int `json:"languages"`
	RemotePercent  float64        `json:"remote_percent"`
	OnSitePercent  float64        `json:"onsite_percent"`
	TopCompanies   []companyHit   `json:"top_companies"`
	HasH1B         int            `json:"posts_offering_visa_or_h1b"`
}

type companyHit struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var languageKeywords = []string{
	"Go", "Golang", "Rust", "Python", "TypeScript", "JavaScript",
	"Ruby", "Elixir", "Java", "Kotlin", "Swift", "Scala", "Clojure",
	"Haskell", "OCaml", "C++", "C#", "Erlang", "PHP", "Zig",
}

var companyRE = regexp.MustCompile(`(?m)^([A-Z][A-Za-z0-9&\.\- ]{2,30})\s*\|`) // catches "Acme Corp | Engineer | ..." style headers
var visaRE = regexp.MustCompile(`(?i)(VISA|H[\-]?1B|sponsorship)`)

func newHiringStatsCmd(flags *rootFlags) *cobra.Command {
	var months int
	cmd := &cobra.Command{
		Use:   "hiring-stats",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Aggregate Who's Hiring threads across N months — top languages, remote ratio, top companies",
		Long: `Walk the most recent N 'Who is hiring?' threads on HN and compute aggregate stats.

Heuristics: we look for whole-word language names, count posts mentioning
'remote' (or 'on-site'), and try to extract the company name from the
common '| Engineer | …' header format hiring posts use. This is
a best-effort summary, not a hiring-data product.`,
		Example: strings.Trim(`
  hackernews-pp-cli hiring-stats --months 1
  hackernews-pp-cli hiring-stats --months 6 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if months <= 0 {
				months = 1
			}
			if months > 12 {
				months = 12
			}
			ac := algolia.New(flags.timeout)
			resp, err := ac.Search("", algolia.SearchOpts{
				Tags:        "story,author_whoishiring",
				ByDate:      true,
				HitsPerPage: 30,
			})
			if err != nil {
				return apiErr(err)
			}
			// Pick threads whose title starts with "Ask HN: Who is hiring".
			threads := []algolia.SearchHit{}
			for _, h := range resp.Hits {
				if strings.HasPrefix(strings.ToLower(h.Title), "ask hn: who is hiring") {
					threads = append(threads, h)
					if len(threads) >= months {
						break
					}
				}
			}
			if len(threads) == 0 {
				return apiErr(fmt.Errorf("no whoishiring threads found"))
			}

			out := hiringStats{
				Months:    months,
				Languages: map[string]int{},
			}
			companyCounts := map[string]int{}
			remote := 0
			onsite := 0

			for _, t := range threads {
				out.ThreadsScanned++
				node, err := ac.Item(t.ObjectID)
				if err != nil {
					continue
				}
				for _, c := range node.Children {
					out.PostsScanned++
					text := stripHTML(c.Text)
					lower := strings.ToLower(text)
					for _, lang := range languageKeywords {
						if mentionsWord(lower, strings.ToLower(lang)) {
							out.Languages[lang]++
						}
					}
					if strings.Contains(lower, "remote") {
						remote++
					}
					if strings.Contains(lower, "on-site") || strings.Contains(lower, "onsite") || strings.Contains(lower, "in office") {
						onsite++
					}
					if visaRE.MatchString(text) {
						out.HasH1B++
					}
					if m := companyRE.FindStringSubmatch(text); len(m) == 2 {
						company := strings.TrimSpace(m[1])
						if company != "" {
							companyCounts[company]++
						}
					}
				}
			}

			if out.PostsScanned > 0 {
				out.RemotePercent = roundPct(remote, out.PostsScanned)
				out.OnSitePercent = roundPct(onsite, out.PostsScanned)
			}

			// Top 10 companies by mention count.
			type kv struct {
				k string
				v int
			}
			pairs := make([]kv, 0, len(companyCounts))
			for k, v := range companyCounts {
				pairs = append(pairs, kv{k, v})
			}
			sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
			limit := 10
			if len(pairs) < limit {
				limit = len(pairs)
			}
			for _, p := range pairs[:limit] {
				out.TopCompanies = append(out.TopCompanies, companyHit{Name: p.k, Count: p.v})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d thread(s), %d posts.\n\n", out.ThreadsScanned, out.PostsScanned)
			fmt.Fprintln(cmd.OutOrStdout(), "Languages mentioned:")
			lpairs := make([]kv, 0, len(out.Languages))
			for k, v := range out.Languages {
				lpairs = append(lpairs, kv{k, v})
			}
			sort.Slice(lpairs, func(i, j int) bool { return lpairs[i].v > lpairs[j].v })
			for _, p := range lpairs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %d\n", p.k, p.v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nRemote: %.1f%% — On-site: %.1f%%\n", out.RemotePercent, out.OnSitePercent)
			fmt.Fprintf(cmd.OutOrStdout(), "Posts mentioning visa/H-1B: %d\n", out.HasH1B)
			if len(out.TopCompanies) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop companies:")
				for _, c := range out.TopCompanies {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", c.Name, c.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&months, "months", 1, "Number of recent monthly threads to scan (1-12)")
	return cmd
}

func mentionsWord(haystack, needle string) bool {
	// Quick whole-word check that handles language names with no
	// internal punctuation. Falls back to a substring check when the
	// needle has '+' or '#' (C++, C#) since those break Go regexp word
	// boundaries.
	if strings.ContainsAny(needle, "+#") {
		return strings.Contains(haystack, needle)
	}
	pattern := `\b` + regexp.QuoteMeta(needle) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(haystack)
}

func roundPct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n*1000/total) / 10
}
