// Copyright 2026 stanrails and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/visit-detroit-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/visit-detroit-blog/internal/store"
	"github.com/spf13/cobra"
)

const blogResource = "blogs"
const siteBaseURL = "https://visitdetroit.com"
const noSyncHint = "no blog data in the local store — run 'visit-detroit-blog-pp-cli sync' first"

// blogRecord is the stored Algolia hit shape we care about.
type blogRecord struct {
	IDNum      int64    `json:"id"`
	ObjectID   string   `json:"objectID"`
	Title      string   `json:"title"`
	URI        string   `json:"uri"`
	Snippet    string   `json:"snippet"`
	Content    string   `json:"content"`
	Categories []string `json:"blogCategories"`
	Regions    []string `json:"partnerRegions"`
	PostDate   int64    `json:"postDate"`
	Updated    int64    `json:"dateUpdated"`
	Image      string   `json:"primaryImageUrl"`
	Sponsored  bool     `json:"sponsoredContent"`
}

func (b blogRecord) id() string {
	if b.ObjectID != "" {
		return b.ObjectID
	}
	if b.IDNum != 0 {
		return fmt.Sprintf("%d", b.IDNum)
	}
	return blogSlug(b.URI)
}

// blogOut is the agent-facing output shape. Lowercase JSON keys keep
// --select paths predictable (the filter lowercases path segments).
type blogOut struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Slug       string   `json:"slug"`
	URL        string   `json:"url"`
	Snippet    string   `json:"snippet,omitempty"`
	Categories []string `json:"categories"`
	Regions    []string `json:"regions"`
	PostedAt   string   `json:"posted_at,omitempty"`
	Sponsored  bool     `json:"sponsored"`
	Image      string   `json:"image,omitempty"`
	Content    string   `json:"content,omitempty"`
}

func (b blogRecord) out(withContent bool) blogOut {
	o := blogOut{
		ID:         b.id(),
		Title:      b.Title,
		Slug:       blogSlug(b.URI),
		URL:        blogURL(b.URI),
		Snippet:    b.Snippet,
		Categories: b.Categories,
		Regions:    b.Regions,
		PostedAt:   unixDate(b.PostDate),
		Sponsored:  b.Sponsored,
		Image:      b.Image,
	}
	if o.Categories == nil {
		o.Categories = []string{}
	}
	if o.Regions == nil {
		o.Regions = []string{}
	}
	if withContent {
		o.Content = b.Content
	}
	return o
}

func blogSlug(uri string) string {
	s := strings.Trim(uri, "/")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	return parts[len(parts)-1]
}

func blogURL(uri string) string {
	if uri == "" {
		return ""
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	return siteBaseURL + uri
}

func unixDate(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

// blogFilter is the shared cross-axis filter used by list and reading-list.
type blogFilter struct {
	category  string
	region    string
	since     int64
	until     int64
	sponsored string // "", "no", "only"
}

func (f blogFilter) match(b blogRecord) bool {
	if f.category != "" && !containsFold(b.Categories, f.category) {
		return false
	}
	if f.region != "" && !regionMatch(b.Regions, f.region) {
		return false
	}
	if f.since > 0 && b.PostDate < f.since {
		return false
	}
	if f.until > 0 && b.PostDate > f.until {
		return false
	}
	switch f.sponsored {
	case "no":
		if b.Sponsored {
			return false
		}
	case "only":
		if !b.Sponsored {
			return false
		}
	}
	return true
}

// containsFold reports whether any element equals q case-insensitively.
func containsFold(list []string, q string) bool {
	for _, v := range list {
		if strings.EqualFold(v, q) {
			return true
		}
	}
	return false
}

// regionMatch is forgiving: it matches a region exactly OR as a substring, so
// "Greektown" matches the stored "Downtown Detroit » Greektown".
func regionMatch(list []string, q string) bool {
	lq := strings.ToLower(strings.TrimSpace(q))
	for _, v := range list {
		if strings.Contains(strings.ToLower(v), lq) {
			return true
		}
	}
	return false
}

// parseDateFlag accepts YYYY-MM-DD, RFC3339, or a relative window like 30d/2w.
func parseDateFlag(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if t, err := parseSinceDuration(s); err == nil {
		return t.Unix(), nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("invalid date %q: use YYYY-MM-DD or a relative window like 30d, 2w", s)
}

// resolveSponsored maps the --no-sponsored / --sponsored-only flag pair to a
// filter mode, rejecting the contradictory combination.
func resolveSponsored(noSponsored, sponsoredOnly bool) (string, error) {
	if noSponsored && sponsoredOnly {
		return "", usageErr(fmt.Errorf("--no-sponsored and --sponsored-only are mutually exclusive"))
	}
	if noSponsored {
		return "no", nil
	}
	if sponsoredOnly {
		return "only", nil
	}
	return "", nil
}

// loadAllBlogs reads every stored article. It queries the store directly
// rather than db.List, whose limit<=0 default of 200 would silently truncate a
// 748-row corpus and undercount every cross-axis aggregation below.
func loadAllBlogs(db *store.Store) ([]blogRecord, error) {
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = ?`, blogResource)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []blogRecord
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var b blogRecord
		if json.Unmarshal([]byte(data), &b) != nil {
			continue
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func openStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("visit-detroit-blog-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'visit-detroit-blog-pp-cli sync' first.", err)
	}
	return db, nil
}

func sortByDateDesc(b []blogRecord) {
	sort.SliceStable(b, func(i, j int) bool { return b[i].PostDate > b[j].PostDate })
}

func toOutList(recs []blogRecord) []blogOut {
	out := make([]blogOut, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.out(false))
	}
	return out
}

// emit routes a Go value to JSON (machine/piped) or a human renderer.
func emitBlogs(cmd *cobra.Command, flags *rootFlags, recs []blogRecord) error {
	out := cmd.OutOrStdout()
	if wantsHumanTable(out, flags) {
		renderBlogTable(out, recs)
		return nil
	}
	return printJSONFiltered(out, toOutList(recs), flags)
}

func renderBlogTable(w interface{ Write([]byte) (int, error) }, recs []blogRecord) {
	if len(recs) == 0 {
		fmt.Fprintln(w, "(no matching articles)")
		return
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "DATE\tCATEGORIES\tTITLE\tSLUG")
	fmt.Fprintln(tw, "----\t----------\t-----\t----")
	for _, b := range recs {
		cats := strings.Join(b.Categories, ", ")
		if len(cats) > 28 {
			cats = cats[:27] + "…"
		}
		title := b.Title
		if len(title) > 52 {
			title = title[:51] + "…"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", unixDate(b.PostDate), cats, title, blogSlug(b.URI))
	}
	tw.Flush()
}

// newBlogsCmd is the hand-built replacement for the generated raw-POST blogs
// command. It is a group: blogs list / get / related / coverage / reading-list.
func newBlogsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "blogs",
		Short:       "Browse, read, and analyze Inside the D blog articles",
		Long:        "Browse, filter, read, and analyze the Inside the D editorial blog from the local store.\nRun 'visit-detroit-blog-pp-cli sync' once to populate the store, then use the subcommands below.",
		Example:     "  visit-detroit-blog-pp-cli blogs list --category Dining --region Corktown\n  visit-detroit-blog-pp-cli blogs get donuts\n  visit-detroit-blog-pp-cli blogs related donuts --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newBlogsListCmd(flags))
	cmd.AddCommand(newBlogsGetCmd(flags))
	cmd.AddCommand(newBlogsRelatedCmd(flags))
	cmd.AddCommand(newBlogsCoverageCmd(flags))
	cmd.AddCommand(newBlogsReadingListCmd(flags))
	return cmd
}

func newBlogsListCmd(flags *rootFlags) *cobra.Command {
	var category, region, since, until, dbPath string
	var noSponsored, sponsoredOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List and filter Inside the D articles across category, neighborhood, and date",
		Long:  "Filter the local article store by category AND neighborhood AND date window in one query — the cross-axis slice the website's single-facet search box cannot express. Reads the local store; run 'sync' first.",
		Example: strings.Trim(`
  visit-detroit-blog-pp-cli blogs list --category Dining --region Corktown
  visit-detroit-blog-pp-cli blogs list --category Outdoors --since 2025-01-01 --limit 10
  visit-detroit-blog-pp-cli blogs list --region "Eastern Market" --no-sponsored --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sp, err := resolveSponsored(noSponsored, sponsoredOnly)
			if err != nil {
				return err
			}
			sinceTS, err := parseDateFlag(since)
			if err != nil {
				return usageErr(err)
			}
			untilTS, err := parseDateFlag(until)
			if err != nil {
				return usageErr(err)
			}
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			f := blogFilter{category: category, region: region, since: sinceTS, until: untilTS, sponsored: sp}
			var matched []blogRecord
			for _, b := range all {
				if f.match(b) {
					matched = append(matched, b)
				}
			}
			sortByDateDesc(matched)
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			return emitBlogs(cmd, flags, matched)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by blog category (e.g. Dining, Culture, Outdoors). See 'categories'.")
	cmd.Flags().StringVar(&region, "region", "", "Filter by Detroit neighborhood/region (e.g. Corktown, Greektown). See 'regions'.")
	cmd.Flags().StringVar(&since, "since", "", "Only articles posted on/after this date (YYYY-MM-DD or relative like 30d)")
	cmd.Flags().StringVar(&until, "until", "", "Only articles posted on/before this date (YYYY-MM-DD or relative like 30d)")
	cmd.Flags().BoolVar(&noSponsored, "no-sponsored", false, "Exclude sponsored content (editorial only)")
	cmd.Flags().BoolVar(&sponsoredOnly, "sponsored-only", false, "Show only sponsored content")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum articles to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newBlogsGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "get [slug-or-id]",
		Short:       "Read a full Inside the D article by slug, URI, or id",
		Long:        "Print one article including its full body text from the local store — no browser. Identify it by slug (the last path segment, e.g. 'donuts'), full URI, or numeric id.",
		Example:     "  visit-detroit-blog-pp-cli blogs get donuts\n  visit-detroit-blog-pp-cli blogs get ethiopian --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			key := args[0]
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			if len(all) == 0 {
				return notFoundErr(fmt.Errorf("%s", noSyncHint))
			}
			b, ok := findBlog(all, key)
			if !ok {
				return notFoundErr(fmt.Errorf("no article matching %q (try 'visit-detroit-blog-pp-cli blogs list' or 'search %s')", key, key))
			}
			out := cmd.OutOrStdout()
			if wantsHumanTable(out, flags) {
				renderArticle(out, b)
				return nil
			}
			return printJSONFiltered(out, b.out(true), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func findBlog(blogs []blogRecord, key string) (blogRecord, bool) {
	key = strings.TrimSpace(key)
	for _, b := range blogs {
		if b.id() == key || b.URI == key || blogSlug(b.URI) == key {
			return b, true
		}
	}
	lk := strings.ToLower(key)
	for _, b := range blogs {
		if strings.ToLower(blogSlug(b.URI)) == lk {
			return b, true
		}
	}
	return blogRecord{}, false
}

func renderArticle(w interface{ Write([]byte) (int, error) }, b blogRecord) {
	fmt.Fprintf(w, "%s\n", b.Title)
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", minInt(len(b.Title), 72)))
	if d := unixDate(b.PostDate); d != "" {
		fmt.Fprintf(w, "Posted: %s", d)
	}
	if len(b.Categories) > 0 {
		fmt.Fprintf(w, "  ·  %s", strings.Join(b.Categories, ", "))
	}
	if len(b.Regions) > 0 {
		fmt.Fprintf(w, "  ·  %s", strings.Join(b.Regions, ", "))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s\n", blogURL(b.URI))
	if b.Sponsored {
		fmt.Fprintln(w, "[sponsored content]")
	}
	fmt.Fprintln(w)
	body := b.Content
	if body == "" {
		body = b.Snippet
	}
	fmt.Fprintln(w, body)
}

// relatedBlog wraps an article with its relevance score against a target.
type relatedBlog struct {
	blogOut
	Score            int      `json:"score"`
	SharedCategories []string `json:"shared_categories"`
	SharedRegions    []string `json:"shared_regions"`
}

func newBlogsRelatedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "related [slug-or-id]",
		Short:       "Find articles sharing the most categories and neighborhoods with one post",
		Long:        "Rank other articles by how many blog categories and neighborhoods they share with the target post — the cross-axis 'related reads' join the website has no surface for. Reads the local store.",
		Example:     "  visit-detroit-blog-pp-cli blogs related donuts --limit 5\n  visit-detroit-blog-pp-cli blogs related ikea --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			if len(all) == 0 {
				return notFoundErr(fmt.Errorf("%s", noSyncHint))
			}
			target, ok := findBlog(all, args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("no article matching %q", args[0]))
			}
			tc := lowerSet(target.Categories)
			tr := lowerSet(target.Regions)
			var scored []relatedBlog
			for _, b := range all {
				if b.id() == target.id() {
					continue
				}
				sharedCat := intersectIn(b.Categories, tc)
				sharedReg := intersectIn(b.Regions, tr)
				score := 2*len(sharedCat) + len(sharedReg)
				if score == 0 {
					continue
				}
				rb := relatedBlog{blogOut: b.out(false), Score: score, SharedCategories: sharedCat, SharedRegions: sharedReg}
				scored = append(scored, rb)
			}
			sort.SliceStable(scored, func(i, j int) bool {
				if scored[i].Score != scored[j].Score {
					return scored[i].Score > scored[j].Score
				}
				return scored[i].PostedAt > scored[j].PostedAt
			})
			if limit > 0 && len(scored) > limit {
				scored = scored[:limit]
			}
			out := cmd.OutOrStdout()
			if wantsHumanTable(out, flags) {
				if len(scored) == 0 {
					fmt.Fprintln(out, "(no related articles found)")
					return nil
				}
				tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "SCORE\tSHARED\tTITLE\tSLUG")
				fmt.Fprintln(tw, "-----\t------\t-----\t----")
				for _, b := range scored {
					shared := strings.Join(append(append([]string{}, b.SharedCategories...), b.SharedRegions...), ", ")
					if len(shared) > 32 {
						shared = shared[:31] + "…"
					}
					title := b.Title
					if len(title) > 48 {
						title = title[:47] + "…"
					}
					fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", b.Score, shared, title, b.Slug)
				}
				tw.Flush()
				return nil
			}
			return printJSONFiltered(out, scored, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum related articles to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func lowerSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, v := range list {
		m[strings.ToLower(v)] = true
	}
	return m
}

func intersectIn(list []string, set map[string]bool) []string {
	var out []string
	for _, v := range list {
		if set[strings.ToLower(v)] {
			out = append(out, v)
		}
	}
	return out
}

// coverage cross-tab output types.
type facetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type categoryCoverage struct {
	Category string       `json:"category"`
	Count    int          `json:"count"`
	Regions  []facetCount `json:"regions"`
}

type regionCoverage struct {
	Region     string       `json:"region"`
	Count      int          `json:"count"`
	Categories []facetCount `json:"categories"`
}

func newBlogsCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath, category, region string
	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Cross-tabulate article counts across categories and neighborhoods",
		Long:        "Show where the blog is dense or thin by cross-tabulating article counts across categories and neighborhoods — the two-dimensional view Algolia's one-dimensional facet API can't return. Use --category or --region to focus one axis.",
		Example:     "  visit-detroit-blog-pp-cli blogs coverage\n  visit-detroit-blog-pp-cli blogs coverage --category Outdoors\n  visit-detroit-blog-pp-cli blogs coverage --region Corktown --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			out := cmd.OutOrStdout()

			switch {
			case category != "":
				regions := map[string]int{}
				total := 0
				for _, b := range all {
					if !containsFold(b.Categories, category) {
						continue
					}
					total++
					for _, r := range b.Regions {
						regions[r]++
					}
				}
				cov := categoryCoverage{Category: category, Count: total, Regions: sortFacets(regions)}
				if wantsHumanTable(out, flags) {
					fmt.Fprintf(out, "%s: %d articles\n", category, total)
					renderFacetTable(out, "NEIGHBORHOOD", cov.Regions)
					return nil
				}
				return printJSONFiltered(out, cov, flags)
			case region != "":
				cats := map[string]int{}
				total := 0
				for _, b := range all {
					if !regionMatch(b.Regions, region) {
						continue
					}
					total++
					for _, c := range b.Categories {
						cats[c]++
					}
				}
				cov := regionCoverage{Region: region, Count: total, Categories: sortFacets(cats)}
				if wantsHumanTable(out, flags) {
					fmt.Fprintf(out, "%s: %d articles\n", region, total)
					renderFacetTable(out, "CATEGORY", cov.Categories)
					return nil
				}
				return printJSONFiltered(out, cov, flags)
			default:
				byCat := map[string]map[string]int{}
				catTotals := map[string]int{}
				for _, b := range all {
					for _, c := range b.Categories {
						catTotals[c]++
						if byCat[c] == nil {
							byCat[c] = map[string]int{}
						}
						for _, r := range b.Regions {
							byCat[c][r]++
						}
					}
				}
				var result []categoryCoverage
				for c, total := range catTotals {
					result = append(result, categoryCoverage{Category: c, Count: total, Regions: sortFacets(byCat[c])})
				}
				sort.SliceStable(result, func(i, j int) bool { return result[i].Count > result[j].Count })
				if wantsHumanTable(out, flags) {
					tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
					fmt.Fprintln(tw, "CATEGORY\tCOUNT\tTOP NEIGHBORHOODS")
					fmt.Fprintln(tw, "--------\t-----\t----------------")
					for _, c := range result {
						top := []string{}
						for i, r := range c.Regions {
							if i >= 3 {
								break
							}
							top = append(top, fmt.Sprintf("%s (%d)", r.Value, r.Count))
						}
						fmt.Fprintf(tw, "%s\t%d\t%s\n", c.Category, c.Count, strings.Join(top, ", "))
					}
					tw.Flush()
					return nil
				}
				return printJSONFiltered(out, result, flags)
			}
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Focus on one category: show its neighborhood breakdown")
	cmd.Flags().StringVar(&region, "region", "", "Focus on one neighborhood: show its category breakdown")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func sortFacets(m map[string]int) []facetCount {
	out := make([]facetCount, 0, len(m))
	for k, v := range m {
		out = append(out, facetCount{Value: k, Count: v})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func renderFacetTable(w interface{ Write([]byte) (int, error) }, header string, facets []facetCount) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\tCOUNT\n", header)
	fmt.Fprintf(tw, "%s\t-----\n", strings.Repeat("-", len(header)))
	for _, f := range facets {
		fmt.Fprintf(tw, "%s\t%d\n", f.Value, f.Count)
	}
	tw.Flush()
}

func newBlogsReadingListCmd(flags *rootFlags) *cobra.Command {
	var category, region, since, until, dbPath, output, format string
	var noSponsored, sponsoredOnly bool
	var limit int
	cmd := &cobra.Command{
		Use:   "reading-list",
		Short: "Build an ordered, deduped reading list (markdown/json/csv), optionally to a file",
		Long:  "Materialize a stable reading list from any filter. Defaults to stdout; pass --output to write a file. Use --no-sponsored for a neutral, editorial-only handout. Reads the local store.",
		Example: strings.Trim(`
  visit-detroit-blog-pp-cli blogs reading-list --region "Downtown Detroit" --category Culture --no-sponsored --output detroit-culture.md
  visit-detroit-blog-pp-cli blogs reading-list --category Dining --format csv`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sp, err := resolveSponsored(noSponsored, sponsoredOnly)
			if err != nil {
				return err
			}
			sinceTS, err := parseDateFlag(since)
			if err != nil {
				return usageErr(err)
			}
			untilTS, err := parseDateFlag(until)
			if err != nil {
				return usageErr(err)
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = formatFromOutput(output)
			}
			switch format {
			case "md", "markdown", "json", "csv":
			default:
				return usageErr(fmt.Errorf("invalid --format %q: use md, json, or csv", format))
			}
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			f := blogFilter{category: category, region: region, since: sinceTS, until: untilTS, sponsored: sp}
			var matched []blogRecord
			for _, b := range all {
				if f.match(b) {
					matched = append(matched, b)
				}
			}
			sortByDateDesc(matched)
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			// --json / --agent always means JSON on stdout (the universal
			// machine-output contract), bypassing the md/csv file-export path.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), toOutList(matched), flags)
			}
			rendered, err := renderReadingList(format, matched, describeFilter(f))
			if err != nil {
				return err
			}
			// Verify floor: never write files to the user's cwd during a
			// verify pass, even with --output set.
			if output == "" || cliutil.IsVerifyEnv() {
				if output != "" {
					fmt.Fprintf(os.Stderr, "would write %d articles to %s\n", len(matched), output)
				}
				fmt.Fprint(cmd.OutOrStdout(), rendered)
				return nil
			}
			if err := os.WriteFile(output, []byte(rendered), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", output, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %d articles to %s\n", len(matched), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by blog category")
	cmd.Flags().StringVar(&region, "region", "", "Filter by neighborhood/region")
	cmd.Flags().StringVar(&since, "since", "", "Only articles posted on/after this date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&until, "until", "", "Only articles posted on/before this date (YYYY-MM-DD or relative)")
	cmd.Flags().BoolVar(&noSponsored, "no-sponsored", false, "Exclude sponsored content (editorial only)")
	cmd.Flags().BoolVar(&sponsoredOnly, "sponsored-only", false, "Show only sponsored content")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum articles (0 = all matching)")
	cmd.Flags().StringVar(&output, "output", "", "Write to this file instead of stdout (format inferred from extension)")
	cmd.Flags().StringVar(&format, "format", "", "Output format: md (default), json, or csv")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func formatFromOutput(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return "json"
	case strings.HasSuffix(lower, ".csv"):
		return "csv"
	default:
		return "md"
	}
}

func describeFilter(f blogFilter) string {
	var parts []string
	if f.category != "" {
		parts = append(parts, "category="+f.category)
	}
	if f.region != "" {
		parts = append(parts, "region="+f.region)
	}
	if f.sponsored == "no" {
		parts = append(parts, "editorial only")
	}
	if f.sponsored == "only" {
		parts = append(parts, "sponsored only")
	}
	if len(parts) == 0 {
		return "all articles"
	}
	return strings.Join(parts, " · ")
}

func renderReadingList(format string, recs []blogRecord, filterDesc string) (string, error) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(toOutList(recs), "", "  ")
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case "csv":
		var sb strings.Builder
		sb.WriteString("id,title,url,categories,regions,posted_at,sponsored\n")
		for _, b := range recs {
			sb.WriteString(csvField(b.id()) + ",")
			sb.WriteString(csvField(b.Title) + ",")
			sb.WriteString(csvField(blogURL(b.URI)) + ",")
			sb.WriteString(csvField(strings.Join(b.Categories, "; ")) + ",")
			sb.WriteString(csvField(strings.Join(b.Regions, "; ")) + ",")
			sb.WriteString(csvField(unixDate(b.PostDate)) + ",")
			sb.WriteString(fmt.Sprintf("%t\n", b.Sponsored))
		}
		return sb.String(), nil
	default: // md
		var sb strings.Builder
		sb.WriteString("# Inside the D — Reading List\n\n")
		fmt.Fprintf(&sb, "_%d article(s) · %s_\n\n", len(recs), filterDesc)
		for i, b := range recs {
			fmt.Fprintf(&sb, "%d. **%s** — %s\n", i+1, b.Title, blogURL(b.URI))
			meta := []string{}
			if len(b.Categories) > 0 {
				meta = append(meta, strings.Join(b.Categories, ", "))
			}
			if len(b.Regions) > 0 {
				meta = append(meta, strings.Join(b.Regions, ", "))
			}
			if d := unixDate(b.PostDate); d != "" {
				meta = append(meta, d)
			}
			if len(meta) > 0 {
				fmt.Fprintf(&sb, "   _%s_\n", strings.Join(meta, " · "))
			}
			if b.Snippet != "" {
				fmt.Fprintf(&sb, "   %s\n", b.Snippet)
			}
			sb.WriteString("\n")
		}
		return sb.String(), nil
	}
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// --- Top-level commands: search, categories, regions, recent ---

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var noSponsored, sponsoredOnly bool
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Full-text search across every Inside the D article (offline, ranked)",
		Long:        "Ranked full-text search over article titles, summaries, and full bodies in the local store. Run 'sync' first to populate it.",
		Example:     "  visit-detroit-blog-pp-cli search \"ethiopian food\"\n  visit-detroit-blog-pp-cli search \"patio season\" --limit 5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			sp, err := resolveSponsored(noSponsored, sponsoredOnly)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			// Over-fetch so sponsored filtering doesn't starve the result.
			fetch := limit
			if sp != "" {
				fetch = limit * 4
			}
			raws, err := db.Search(query, fetch)
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}
			var recs []blogRecord
			for _, r := range raws {
				var b blogRecord
				if json.Unmarshal(r, &b) != nil {
					continue
				}
				if sp == "no" && b.Sponsored {
					continue
				}
				if sp == "only" && !b.Sponsored {
					continue
				}
				recs = append(recs, b)
			}
			if limit > 0 && len(recs) > limit {
				recs = recs[:limit]
			}
			return emitBlogs(cmd, flags, recs)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	cmd.Flags().BoolVar(&noSponsored, "no-sponsored", false, "Exclude sponsored content")
	cmd.Flags().BoolVar(&sponsoredOnly, "sponsored-only", false, "Show only sponsored content")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "categories",
		Short:       "List blog categories with article counts",
		Long:        "List every blog category and how many articles carry it, from the local store. Use these exact spellings with 'blogs list --category'.",
		Example:     "  visit-detroit-blog-pp-cli categories\n  visit-detroit-blog-pp-cli categories --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return facetCommand(cmd, flags, dbPath, func(b blogRecord) []string { return b.Categories }, "CATEGORY")
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newRegionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "regions",
		Short:       "List Detroit neighborhoods/regions with article counts",
		Long:        "List every neighborhood/region tag and how many articles carry it, from the local store. Use these spellings with 'blogs list --region'.",
		Example:     "  visit-detroit-blog-pp-cli regions\n  visit-detroit-blog-pp-cli regions --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return facetCommand(cmd, flags, dbPath, func(b blogRecord) []string { return b.Regions }, "REGION")
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func facetCommand(cmd *cobra.Command, flags *rootFlags, dbPath string, pick func(blogRecord) []string, header string) error {
	db, err := openStore(cmd, dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	all, err := loadAllBlogs(db)
	if err != nil {
		return fmt.Errorf("loading articles: %w", err)
	}
	counts := map[string]int{}
	for _, b := range all {
		for _, v := range pick(b) {
			counts[v]++
		}
	}
	facets := sortFacets(counts)
	out := cmd.OutOrStdout()
	if wantsHumanTable(out, flags) {
		renderFacetTable(out, header, facets)
		return nil
	}
	return printJSONFiltered(out, facets, flags)
}

func newRecentCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, until string
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Show the newest Inside the D articles by post date",
		Long:        "List the most recently posted articles from the local store, newest first. Narrow with --since/--until.",
		Example:     "  visit-detroit-blog-pp-cli recent --limit 10\n  visit-detroit-blog-pp-cli recent --since 30d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sinceTS, err := parseDateFlag(since)
			if err != nil {
				return usageErr(err)
			}
			untilTS, err := parseDateFlag(until)
			if err != nil {
				return usageErr(err)
			}
			db, err := openStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			all, err := loadAllBlogs(db)
			if err != nil {
				return fmt.Errorf("loading articles: %w", err)
			}
			f := blogFilter{since: sinceTS, until: untilTS}
			var matched []blogRecord
			for _, b := range all {
				if f.match(b) {
					matched = append(matched, b)
				}
			}
			sortByDateDesc(matched)
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			return emitBlogs(cmd, flags, matched)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum articles to return (0 = all)")
	cmd.Flags().StringVar(&since, "since", "", "Only articles posted on/after this date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&until, "until", "", "Only articles posted on/before this date (YYYY-MM-DD or relative)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
