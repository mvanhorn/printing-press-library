// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

// Hand-built replacements for the generated `categories list` and
// `categories games` commands. The generated pair fetched apps.apple.com
// chart pages through the generated internal/client, outside the App Store
// governor and the capture log, and that client retries a 429 up to three
// times and then waits for its limiter: the one CLI path that kept hitting
// apps.apple.com after a throttle. By default these commands answer from the
// CLI's static genre table (internal/appstore/genres.go, the table `top
// --genre` resolves names against) and send no request. --live reads the
// category list from the robots-allowed chart page instead, through the
// governed App Store client and the capture log, and reports on stderr how
// the live list differs from the built-in table. The generated files stay
// untouched; the hook below swaps the commands in at startup, so
// regeneration keeps the replacement.

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, _, err := root.Find([]string{"categories"})
		if err != nil || parent == nil || parent.Name() != "categories" || parent.Parent() != root {
			return
		}
		replaceChildCommand(parent, newCategoriesListOfflineCmd(flags))
		replaceChildCommand(parent, newCategoriesGamesOfflineCmd(flags))
	})
}

// replaceChildCommand puts cmd under parent in place of any child with the
// same name.
func replaceChildCommand(parent, cmd *cobra.Command) {
	for _, existing := range parent.Commands() {
		if existing.Name() == cmd.Name() {
			parent.RemoveCommand(existing)
		}
	}
	parent.AddCommand(cmd)
}

// Category row sources.
const (
	categorySourceBuiltIn = "built-in"
	categorySourceLive    = "live"
)

// categoryRow is one category: an App Store genre, or a Kids age band of the
// overall chart (genre 36 plus ageBandId).
type categoryRow struct {
	GenreID   string `json:"genreId"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId,omitempty"`
	AgeBandID string `json:"ageBandId,omitempty"`
	// ChartURL is the web chart page `top` reads for this category (top-free,
	// iPhone, the --country storefront).
	ChartURL string `json:"chartUrl"`
	// Source is "built-in" (the CLI's genre table) or "live" (read from the
	// chart page with --live).
	Source string `json:"source"`
}

// appCategoryRows lists All Apps (36), the top-level iOS app genres and the
// Kids age bands, in genre id order.
func appCategoryRows(country string) []categoryRow {
	rows := make([]categoryRow, 0, 32)
	for _, g := range appstore.Genres("ios") {
		if g.ID != appstore.AllGenreID && g.ParentID != appstore.AllGenreID {
			continue
		}
		rows = append(rows, genreRow(country, g))
	}
	for _, b := range appstore.KidsAgeBands {
		name := "Kids " + b.Name
		if strings.HasPrefix(b.Name, "All Kids") {
			name = b.Name
		}
		rows = append(rows, categoryRow{
			GenreID:   appstore.AllGenreID,
			Name:      name,
			ParentID:  appstore.AllGenreID,
			AgeBandID: b.ID,
			ChartURL:  appstore.ChartsPageURL(country, "iphone", appstore.AllGenreID, "top-free", b.ID),
			Source:    categorySourceBuiltIn,
		})
	}
	return rows
}

// gameCategoryRows lists Games (6014) and its subgenres in id order.
func gameCategoryRows(country string) []categoryRow {
	rows := make([]categoryRow, 0, 20)
	for _, g := range appstore.Genres("ios") {
		if g.ID == appstore.GamesGenreID || g.ParentID == appstore.GamesGenreID {
			rows = append(rows, genreRow(country, g))
		}
	}
	return rows
}

func genreRow(country string, g appstore.Genre) categoryRow {
	return categoryRow{
		GenreID:  g.ID,
		Name:     g.Name,
		ParentID: g.ParentID,
		ChartURL: appstore.ChartsPageURL(country, "iphone", g.ID, "top-free", ""),
		Source:   categorySourceBuiltIn,
	}
}

// isAppCategory keeps All Apps, the top-level app genres and the Kids bands
// (the apps chart page also links game subgenres from some storefronts).
func isAppCategory(r categoryRow) bool {
	return r.GenreID == appstore.AllGenreID || !strings.HasPrefix(r.GenreID, "70")
}

// isGameCategory keeps Games and its subgenres (the games chart page also
// links the Kids bands of the overall chart).
func isGameCategory(r categoryRow) bool {
	return r.GenreID == appstore.GamesGenreID || strings.HasPrefix(r.GenreID, "70")
}

// shownOnAppsPage reports whether the apps chart page lists the built-in row:
// it never lists Games (6014), whose charts live on the games page, so the
// drift note must not report Games as missing.
func shownOnAppsPage(r categoryRow) bool { return r.GenreID != appstore.GamesGenreID }

func shownOnGamesPage(categoryRow) bool { return true }

func newCategoriesListOfflineCmd(flags *rootFlags) *cobra.Command {
	return newCategoriesTableCmd(flags, categoriesTableSpec{
		use:   "list",
		short: "List App Store app categories (genre ids such as 6014 Games, 6026 Developer Tools, Kids age bands)",
		long: `
List the App Store app categories with the genre ids the chart commands take:
All Apps (36), every top-level app genre (6000-6027, e.g. 6014 Games, 6026
Developer Tools, 6027 Graphics & Design) and the Kids age bands of the
overall chart (genre 36 with ageBandId 0, 10000, 10001 or 10002). Each row
carries the web chart URL 'top' reads for it in the --country storefront.

By default this answers from the CLI's built-in genre table (Apple's genre
tree, checked against the live chart pages of 2026-10-23), the same table
'top --genre' resolves names against, and sends no request to Apple. --live
reads the list from the public chart page instead (one paced request to
apps.apple.com) and notes on stderr any category the built-in table lacks or
the page no longer lists. Game subgenres are listed by 'categories games'.`,
		example: `
  app-store-pp-cli categories list --json
  app-store-pp-cli categories list --live --country gb --select genreId,name,ageBandId`,
		pageGenre: appstore.AllGenreID,
		rows:      appCategoryRows,
		keep:      isAppCategory,
		shown:     shownOnAppsPage,
	})
}

func newCategoriesGamesOfflineCmd(flags *rootFlags) *cobra.Command {
	return newCategoriesTableCmd(flags, categoriesTableSpec{
		use:   "games",
		short: "List App Store game subcategories (genre ids 7001-7019 such as 7012 Puzzle)",
		long: `
List Games (6014) and its subgenres with their genre ids (7001 Action through
7019 Word; 7003 is Casual, 7012 Puzzle, 7014 Role-Playing). Pass an id or a
name (puzzle, rpg, "games:sports") to 'top --genre'. Each row carries the web
chart URL 'top' reads for it in the --country storefront.

By default this answers from the CLI's built-in genre table and sends no
request to Apple. --live reads the list from the public games chart page
(one paced request) and notes on stderr how it differs from the table.`,
		example: `
  app-store-pp-cli categories games --json
  app-store-pp-cli categories games --live --country pk --select genreId,name`,
		pageGenre: appstore.GamesGenreID,
		rows:      gameCategoryRows,
		keep:      isGameCategory,
		shown:     shownOnGamesPage,
	})
}

type categoriesTableSpec struct {
	use, short, long, example string
	// pageGenre is the chart page --live reads (36 for apps, 6014 for games).
	pageGenre string
	rows      func(country string) []categoryRow
	// keep filters the live page's categories to this command's scope.
	keep func(categoryRow) bool
	// shown filters the built-in rows to those the live page is expected to
	// list, for the drift note.
	shown func(categoryRow) bool
}

func newCategoriesTableCmd(flags *rootFlags, spec categoriesTableSpec) *cobra.Command {
	var live bool
	var dbPath string
	cmd := &cobra.Command{
		Use:     spec.use,
		Short:   spec.short,
		Long:    strings.Trim(spec.long, "\n"),
		Example: strings.Trim(spec.example, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "computed",
			"pp:typed-exit-codes": "0,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "categories "+spec.use)
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("categories %s takes no arguments (got %q)", spec.use, strings.Join(args, " ")))
			}
			cc, err := appStoreCountry(flags)
			if err != nil {
				return err
			}
			builtIn := spec.rows(cc)
			if !live {
				return printRows(cmd, flags, builtIn)
			}
			rows, err := liveCategoryRows(cmd, flags, spec, cc)
			if err != nil {
				return err
			}
			expected := make([]categoryRow, 0, len(builtIn))
			for _, r := range builtIn {
				if spec.shown(r) {
					expected = append(expected, r)
				}
			}
			writeCategoryDrift(cmd.ErrOrStderr(), expected, rows)
			return printRows(cmd, flags, rows)
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Read the category list from the public chart page (one paced request) instead of the built-in table")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local snapshot store path for the capture log of --live (default: the CLI data directory)")
	return cmd
}

// liveCategoryRows reads the category list from the chart page through the
// governed client, logging the capture to the local store.
func liveCategoryRows(cmd *cobra.Command, flags *rootFlags, spec categoriesTableSpec, country string) ([]categoryRow, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	st, err := openAppStoreStore(cmd, flags)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()
	c, err := newAppStoreClient(flags)
	if err != nil {
		return nil, err
	}
	attachCaptureLog(c, st)
	page, err := c.ChartsPage(ctx, appstore.ChartsPageQuery{
		Country:  country,
		Platform: "iphone",
		GenreID:  spec.pageGenre,
		Chart:    "top-free",
	})
	if err != nil {
		return nil, classifyAppStoreErr(err)
	}
	var flat []categoryRow
	flattenLiveCategories(country, spec.pageGenre, page.Categories, &flat)
	rows := make([]categoryRow, 0, len(flat))
	seen := make(map[string]bool, len(flat))
	for _, r := range flat {
		key := categoryKey(r)
		if seen[key] || !spec.keep(r) {
			continue
		}
		seen[key] = true
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil, apiErr(fmt.Errorf("the %s chart page listed no categories (the page layout may have changed); 'categories %s' without --live still answers from the built-in table", country, spec.use))
	}
	return rows, nil
}

// flattenLiveCategories turns the chart page's category tree into rows.
// Top-level genres hang under the page's genre; Kids bands under All Apps.
func flattenLiveCategories(country, pageGenre string, cats []appstore.ChartCategory, out *[]categoryRow) {
	for _, c := range cats {
		parent := pageGenre
		switch {
		case c.AgeBandID != "":
			parent = appstore.AllGenreID
		case c.GenreID == pageGenre || c.GenreID == appstore.AllGenreID:
			parent = ""
		}
		*out = append(*out, categoryRow{
			GenreID:   c.GenreID,
			Name:      c.Name,
			ParentID:  parent,
			AgeBandID: c.AgeBandID,
			ChartURL:  appstore.ChartsPageURL(country, "iphone", c.GenreID, "top-free", c.AgeBandID),
			Source:    categorySourceLive,
		})
		if len(c.Children) > 0 {
			flattenLiveCategories(country, pageGenre, c.Children, out)
		}
	}
}

func categoryKey(r categoryRow) string { return r.GenreID + "|" + r.AgeBandID }

// writeCategoryDrift notes on stderr how the live list differs from the
// built-in table (JSON on stdout is unaffected).
func writeCategoryDrift(w io.Writer, builtIn, live []categoryRow) {
	inLive := make(map[string]categoryRow, len(live))
	for _, r := range live {
		inLive[categoryKey(r)] = r
	}
	inTable := make(map[string]categoryRow, len(builtIn))
	for _, r := range builtIn {
		inTable[categoryKey(r)] = r
	}
	var onlyLive, onlyTable []string
	for k, r := range inLive {
		if _, ok := inTable[k]; !ok {
			onlyLive = append(onlyLive, describeCategory(r))
		}
	}
	for k, r := range inTable {
		if _, ok := inLive[k]; !ok {
			onlyTable = append(onlyTable, describeCategory(r))
		}
	}
	sort.Strings(onlyLive)
	sort.Strings(onlyTable)
	if len(onlyLive) == 0 && len(onlyTable) == 0 {
		_, _ = fmt.Fprintf(w, "note: the live chart page lists the same %d categories as the built-in table\n", len(live))
		return
	}
	if len(onlyLive) > 0 {
		_, _ = fmt.Fprintf(w, "note: on the live chart page but not in the built-in table: %s\n", strings.Join(onlyLive, ", "))
	}
	if len(onlyTable) > 0 {
		_, _ = fmt.Fprintf(w, "note: in the built-in table but not on the live chart page: %s\n", strings.Join(onlyTable, ", "))
	}
}

func describeCategory(r categoryRow) string {
	if r.AgeBandID != "" {
		return fmt.Sprintf("%s (%s, age band %s)", r.Name, r.GenreID, r.AgeBandID)
	}
	return fmt.Sprintf("%s (%s)", r.Name, r.GenreID)
}
