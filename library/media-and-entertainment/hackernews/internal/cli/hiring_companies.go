package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// `hiring companies` walks the last N "Who is hiring?" threads, tokenizes
// each post for a company name (using the same `Acme Corp | Engineer | ...`
// header regex as hiring_stats), and aggregates per-company appearances
// across months. The output answers persona Marco's "which companies
// posted in M of the last N months" question — impossible from any
// single API call.

type companyMonthHit struct {
	Name        string `json:"name"`
	MonthsCount int    `json:"months_posted"`
	FirstSeen   string `json:"first_seen"` // YYYY-MM
	LastSeen    string `json:"last_seen"`  // YYYY-MM
}

type companyTrackerOutput struct {
	MonthsScanned   int               `json:"months_scanned"`
	ThreadsScanned  int               `json:"threads_scanned"`
	PostsScanned    int               `json:"posts_scanned"`
	UniqueCompanies int               `json:"unique_companies"`
	MinPosts        int               `json:"min_posts_filter"`
	Companies       []companyMonthHit `json:"companies"`
}

func newHiringCompaniesCmd(flags *rootFlags) *cobra.Command {
	var months int
	var minPosts int
	cmd := &cobra.Command{
		Use:   "companies",
		Short: "Companies posting across the last N hiring threads — first-seen, last-seen, months count",
		Long: `Walk the most recent N 'Who is hiring?' threads on HN and join post-level
company mentions across months.

Output rows are companies that appear in at least --min-posts of the scanned
months. Each row reports months_posted (how many of the scanned months had a
post from this company), first_seen (earliest YYYY-MM), and last_seen (latest
YYYY-MM). Useful for sourcing — persistent hirers vs one-off posters.

Heuristic: same Acme Corp | Engineer | ... header regex as 'hiring stats'.
We deduplicate per month to avoid one company posting twice in the same
thread inflating its count.`,
		Example: strings.Trim(`
  hackernews-pp-cli hiring companies --months 6
  hackernews-pp-cli hiring companies --months 6 --min-posts 3 --json
  hackernews-pp-cli hiring companies --months 12 --min-posts 6 --agent
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if months <= 0 {
				months = 3
			}
			if months > 12 {
				months = 12
			}
			if minPosts < 1 {
				minPosts = 1
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

			// company → set of YYYY-MM strings
			seen := map[string]map[string]struct{}{}
			out := companyTrackerOutput{
				MonthsScanned: months,
				MinPosts:      minPosts,
			}

			for _, t := range threads {
				out.ThreadsScanned++
				month := time.Unix(t.CreatedAtI, 0).UTC().Format("2006-01")
				node, err := ac.Item(t.ObjectID)
				if err != nil {
					continue
				}
				for _, c := range node.Children {
					out.PostsScanned++
					text := stripHTML(c.Text)
					if m := companyRE.FindStringSubmatch(text); len(m) == 2 {
						company := strings.TrimSpace(m[1])
						if company == "" {
							continue
						}
						if seen[company] == nil {
							seen[company] = map[string]struct{}{}
						}
						seen[company][month] = struct{}{}
					}
				}
			}

			for company, monthsSet := range seen {
				if len(monthsSet) < minPosts {
					continue
				}
				monthsList := make([]string, 0, len(monthsSet))
				for m := range monthsSet {
					monthsList = append(monthsList, m)
				}
				sort.Strings(monthsList)
				out.Companies = append(out.Companies, companyMonthHit{
					Name:        company,
					MonthsCount: len(monthsList),
					FirstSeen:   monthsList[0],
					LastSeen:    monthsList[len(monthsList)-1],
				})
			}
			out.UniqueCompanies = len(out.Companies)

			// Sort by months_count desc, then name asc.
			sort.Slice(out.Companies, func(i, j int) bool {
				if out.Companies[i].MonthsCount != out.Companies[j].MonthsCount {
					return out.Companies[i].MonthsCount > out.Companies[j].MonthsCount
				}
				return out.Companies[i].Name < out.Companies[j].Name
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d thread(s), %d posts. %d companies posted in >= %d month(s).\n\n",
				out.ThreadsScanned, out.PostsScanned, out.UniqueCompanies, minPosts)
			for _, c := range out.Companies {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d months  %s → %s\n", c.Name, c.MonthsCount, c.FirstSeen, c.LastSeen)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&months, "months", 3, "Number of recent monthly hiring threads to scan (1-12)")
	cmd.Flags().IntVar(&minPosts, "min-posts", 1, "Only show companies that posted in at least this many of the scanned months")
	return cmd
}
