// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live — keyless Steam store services and storefront endpoints.
// PATCH(amend-2026-10-02: expose the keyless Steam store catalog as a command group)
//
// steam.go - the `steam` command group: the store-catalog surface over
// internal/source/steam. Steam was previously reachable only as incidental
// enrichment inside `ratings`; these commands make it a source in its own
// right (search, one app record, filtered browse).
//
// Everything here is KEYLESS. The catalog comes from Valve's store services
// (IStoreQueryService, IStoreBrowseService, IStoreService), which are not
// listed in https://partner.steamgames.com/doc/api. The documented catalog
// call there, IStoreService/GetAppList, needs a Steam Web API key and cannot
// filter by demo, tag, or price - so STEAM_API_KEY is deliberately NOT used.
// See the README section "Steam data sources".

package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"
)

// steamMeta is the provenance envelope shared by the steam commands.
type steamMeta struct {
	Source         string   `json:"source"`
	Country        string   `json:"country"`
	Language       string   `json:"language"`
	Term           string   `json:"term,omitempty"`
	AppID          int64    `json:"app_id,omitempty"`
	ResolvedBy     string   `json:"resolved_by,omitempty"`
	Types          []string `json:"types,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Title          string   `json:"title,omitempty"`
	FreeOnly       bool     `json:"free_only,omitempty"`
	Total          int      `json:"total,omitempty"`
	Page           int      `json:"page,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	NextPage       int      `json:"next_page,omitempty"`
	SourcesMissing []string `json:"sources_missing,omitempty"`
	// Truncated is the text-search view of "more matched than returned": the
	// SearchSuggestions endpoint ignores offsets, so it is the only honest
	// signal. A pointer keeps the field present (even when false) on the
	// --title path and absent on the paginated browse path.
	Truncated *bool `json:"truncated,omitempty"`
}

// steamSearchView is the search/browse envelope.
type steamSearchView struct {
	Meta    steamMeta         `json:"meta"`
	Results []steam.StoreItem `json:"results"`
}

// steamAppView is the single-app envelope: the typed store record plus the
// review rollup when the store has one.
type steamAppView struct {
	Meta    steamMeta     `json:"meta"`
	Results []steamAppRow `json:"results"`
}

type steamAppRow struct {
	steam.StoreItem
	// HasDemo is always present: true when the store record lists demos.
	HasDemo bool                 `json:"has_demo"`
	Reviews *steam.ReviewSummary `json:"reviews,omitempty"`
}

// steamClientHook lets tests point the per-request Steam client at an
// httptest server (BaseURL and APIBaseURL). It is nil in production, so this is
// a test seam only and leaves the runtime path unchanged.
var steamClientHook func(c *steam.Client)

// newSteamClient builds a store client for one request. Country and language
// come from the flag/env resolution below; the base URLs stay overridable so
// tests can point the commands at a local server.
func newSteamClient(country, lang string) *steam.Client {
	c := steam.New(&steam.Config{
		RateLimit: steam.DefaultRateLimit,
		Country:   country,
		Language:  lang,
	})
	if v := strings.TrimSpace(cliutil.EnvOverride("STEAM_STORE_BASE_URL")); v != "" {
		c.BaseURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("STEAM_STORE_API_BASE_URL")); v != "" {
		c.APIBaseURL = strings.TrimRight(v, "/")
	}
	if steamClientHook != nil {
		steamClientHook(c)
	}
	return c
}

// resolveSteamCountry applies the store-localisation precedence: explicit
// --country, then STEAM_COUNTRY, then ITAD_COUNTRY (so one setting localises
// both the Steam store and the IsThereAnyDeal price path), then US.
func resolveSteamCountry(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return normalizeITADCountry(flagValue)
	}
	for _, envVar := range []string{"STEAM_COUNTRY", "ITAD_COUNTRY"} {
		if env := strings.TrimSpace(cliutil.EnvOverride(envVar)); env != "" {
			if c, err := normalizeITADCountry(env); err == nil && c != "" {
				return c, nil
			}
		}
	}
	return steam.DefaultCountry, nil
}

// resolveSteamLanguage is the store locale: --lang, then STEAM_LANG, then the
// english default.
func resolveSteamLanguage(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("STEAM_LANG")); v != "" {
		return strings.ToLower(v)
	}
	return steam.DefaultLanguage
}

// classifySteamError maps Steam typed errors onto the CLI exit-code taxonomy:
// a missing app is a not-found (3) and an ambiguous title is a usage error (2),
// because resolving it needs a better argument from the caller.
func classifySteamError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, steam.ErrAppNotFound):
		return notFoundErr(err)
	case errors.Is(err, steam.ErrAmbiguousApp):
		return usageErr(err)
	}
	return err
}

func appTypeStrings(types []steam.AppType) []string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, string(t))
	}
	return out
}

// steamTypeFlagHelp is shared by every --type flag so the taxonomy is described
// once.
const steamTypeFlagHelp = "Comma-separated app types: game, demo, dlc, soundtrack, software, video, mod, hardware"

// steamCatalogLong explains the keyless store-service tradeoff (and why no
// Steam API key is involved) in every command help body.
const steamCatalogLong = `Keyless: no Steam API key is used or needed. The catalog comes from Valve's
public store services (IStoreQueryService, IStoreBrowseService, IStoreService),
which are not part of the documented partner API at
https://partner.steamgames.com/doc/api. That documented catalog call,
IStoreService/GetAppList, requires a Steam Web API key and cannot filter by
demo, tag, or price - which is why this CLI talks to the store services
instead. The sibling steam-web CLI covers the key-gated Web API (players,
achievements, stats).`

func newSteamCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "steam",
		Short: "Keyless Steam store catalog: search, app records, and filtered browse",
		Long: `Browse Valve's Steam store directly: plural search across app types,
full typed app records (with demo links, tags, platforms, and price), and a
paginated catalog browse with type/free/tag/release filters.

` + steamCatalogLong,
	}
	cmd.AddCommand(newSteamSearchCmd(flags))
	cmd.AddCommand(newSteamAppCmd(flags))
	cmd.AddCommand(newSteamBrowseCmd(flags))
	cmd.AddCommand(newSteamDemosCmd(flags))
	return cmd
}

func newSteamSearchCmd(flags *rootFlags) *cobra.Command {
	var typesCSV, country, lang string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Search the Steam store catalog for games, demos, DLC, soundtracks, and more",
		Long: `Search the Steam store for a term and return typed catalog records.

Unlike "steam browse", text search is capped at 100 results and has no paging:
the store search service ignores an offset. Use browse with --type/--free/--tag
when you need to walk the catalog page by page.

` + steamCatalogLong,
		Example: strings.Trim(`
  game-goat-pp-cli steam search "Hollow Knight"
  game-goat-pp-cli steam search doom --type game,demo --limit 25 --json
  game-goat-pp-cli steam search "Elden Ring" --country DE --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "term=Hollow Knight;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "steam search")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <term> [--type <list>] [--limit <1-100>]", "a search term is required")
			}
			types, terr := steam.ParseAppTypes(typesCSV)
			if terr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <term> --type <list>", terr.Error())
			}
			if limit < 1 || limit > steam.MaxPageSize {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <term> --limit <1-100>", "--limit must be between 1 and 100")
			}
			resolvedCountry, cerr := resolveSteamCountry(country)
			if cerr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <term> --country <iso>", cerr.Error())
			}
			resolvedLang := resolveSteamLanguage(lang)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			term := strings.Join(args, " ")
			items, serr := newSteamClient(resolvedCountry, resolvedLang).Search(ctx, term, steam.SearchOptions{Types: types, Limit: limit})
			if serr != nil {
				return classifySteamError(serr)
			}
			view := steamSearchView{
				Meta: steamMeta{
					Source:   "live",
					Country:  resolvedCountry,
					Language: resolvedLang,
					Term:     term,
					Types:    appTypeStrings(types),
					Limit:    limit,
					Total:    len(items),
				},
				Results: items,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderSteamItems(cmd, fmt.Sprintf("Steam store search: %s (%d results, %s)", term, len(items), resolvedCountry), items)
		},
	}

	cmd.Flags().StringVar(&typesCSV, "type", "game", steamTypeFlagHelp)
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return (1-100)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region (default STEAM_COUNTRY, ITAD_COUNTRY, or US)")
	cmd.Flags().StringVar(&lang, "lang", "", "Store locale for store text (default english)")
	return cmd
}

func newSteamAppCmd(flags *rootFlags) *cobra.Command {
	var year, country, lang string

	cmd := &cobra.Command{
		Use:   "app <appid|title>",
		Short: "One full typed Steam store record, including demo links and the review summary",
		Long: `Fetch the full store record for one app: name, type, release date,
developers/publishers, platforms, store tags, price, the app's demo links, and
the keyless review rollup.

A bare number is an appid. Otherwise the title is resolved to an appid and a
trailing "(YYYY)" (as RAWG writes remake years) is honoured: "DOOM (2016)"
resolves to the 2016 reboot. When remakes share a name the resolver reports the
candidates instead of guessing, so pin with --year or pass an appid. A review
failure (for example an unreleased app) degrades to "steam_reviews" in
sources_missing rather than failing the command.

` + steamCatalogLong,
		Example: strings.Trim(`
  game-goat-pp-cli steam app 379720
  game-goat-pp-cli steam app "DOOM (2016)" --agent
  game-goat-pp-cli steam app "Hollow Knight" --country GB --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "appid=379720;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "steam app")
			}
			if len(args) == 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <appid|title> [--year <yyyy>]", "an appid or a game title is required")
			}
			if year != "" && !isYearValue(year) {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <title> --year <yyyy>", "--year must be a 4-digit release year, e.g. 2016")
			}
			resolvedCountry, cerr := resolveSteamCountry(country)
			if cerr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" <appid|title> --country <iso>", cerr.Error())
			}
			resolvedLang := resolveSteamLanguage(lang)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newSteamClient(resolvedCountry, resolvedLang)
			arg := strings.Join(args, " ")
			appid := int64(0)
			resolvedBy := "appid"
			if id, perr := strconv.ParseInt(strings.TrimSpace(arg), 10, 64); perr == nil && id > 0 {
				appid = id
			} else {
				pinnedYear := 0
				if year != "" {
					pinnedYear, _ = strconv.Atoi(year)
				}
				id, rerr := c.ResolveAppIDWithHint(ctx, arg, pinnedYear)
				if rerr != nil {
					return classifySteamError(rerr)
				}
				appid = id
				resolvedBy = "title"
			}

			item, ierr := c.Item(ctx, appid)
			if ierr != nil {
				return classifySteamError(ierr)
			}
			row := steamAppRow{StoreItem: *item, HasDemo: len(item.DemoAppIDs) > 0}
			meta := steamMeta{
				Source:     "live",
				Country:    resolvedCountry,
				Language:   resolvedLang,
				AppID:      appid,
				ResolvedBy: resolvedBy,
			}
			if review, rerr := c.ReviewSummary(ctx, appid); rerr == nil {
				row.Reviews = &review
			} else {
				meta.SourcesMissing = []string{"steam_reviews"}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam reviews unavailable for appid %d: %v\n", appid, rerr)
			}
			view := steamAppView{Meta: meta, Results: []steamAppRow{row}}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if err := renderSteamItems(cmd, fmt.Sprintf("Steam app %d: %s", row.AppID, row.Name), []steam.StoreItem{row.StoreItem}); err != nil {
				return err
			}
			renderSteamReviewLine(cmd.OutOrStdout(), row.Reviews)
			return nil
		},
	}

	cmd.Flags().StringVar(&year, "year", "", "Pin a title to a release year when remakes share the name (e.g. 2016)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region (default STEAM_COUNTRY, ITAD_COUNTRY, or US)")
	cmd.Flags().StringVar(&lang, "lang", "", "Store locale for store text (default english)")
	return cmd
}

func newSteamBrowseCmd(flags *rootFlags) *cobra.Command {
	var typesCSV, country, lang string
	var tags []string
	var freeOnly, comingSoon, releasedOnly bool
	var limit, page int

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Paginated Steam catalog browse with type, free, tag, and release filters",
		Long: `Walk the Steam catalog page by page with real pagination.

Filters: --type selects app types, --free keeps free items only, --tag keeps
items carrying a store tag (name or tagid, repeatable or comma-separated), and
--coming-soon / --released restrict to unreleased or released items. --page/--limit page through
the result set; meta carries total, page, limit, and next_page.

"Every free demo in a region" is therefore:
  steam browse --type demo --free --country DE

Note: bundles cannot be enumerated. The store services expose no bundle filter,
so --type bundle is rejected rather than silently returning nothing.

` + steamCatalogLong,
		Example: strings.Trim(`
  game-goat-pp-cli steam browse --type demo --free --limit 20 --agent
  game-goat-pp-cli steam browse --type dlc --tag Roguelike --page 2 --json
  game-goat-pp-cli steam browse --type game --coming-soon --country DE
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--type=demo;--free;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "steam browse")
			}
			types, terr := steam.ParseAppTypes(typesCSV)
			if terr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --type <list>", terr.Error())
			}
			if comingSoon && releasedOnly {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" [--coming-soon | --released]", "--coming-soon and --released are mutually exclusive")
			}
			if limit < 1 || limit > steam.MaxPageSize {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <1-100>", "--limit must be between 1 and 100")
			}
			if page < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --page <n>", "--page must be 1 or greater")
			}
			resolvedCountry, cerr := resolveSteamCountry(country)
			if cerr != nil {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --country <iso>", cerr.Error())
			}
			resolvedLang := resolveSteamLanguage(lang)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newSteamClient(resolvedCountry, resolvedLang)
			tagIDs := make([]int, 0, len(tags))
			for _, t := range tags {
				id, rerr := c.ResolveTag(ctx, t)
				if rerr != nil {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --tag <name-or-id>", rerr.Error())
				}
				tagIDs = append(tagIDs, id)
			}

			result, berr := c.Browse(ctx, steam.BrowseOptions{
				Types:        types,
				FreeOnly:     freeOnly,
				TagIDs:       tagIDs,
				ComingSoon:   comingSoon,
				ReleasedOnly: releasedOnly,
				Start:        (page - 1) * limit,
				Count:        limit,
			})
			if berr != nil {
				return classifySteamError(berr)
			}
			meta := steamMeta{
				Source:   "live",
				Country:  resolvedCountry,
				Language: resolvedLang,
				Types:    appTypeStrings(types),
				Tags:     tags,
				FreeOnly: freeOnly,
				Total:    result.Total,
				Page:     page,
				Limit:    limit,
			}
			if result.HasMore() {
				meta.NextPage = page + 1
			}
			view := steamSearchView{Meta: meta, Results: result.Items}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderSteamItems(cmd, fmt.Sprintf("Steam catalog browse (%d of %d, page %d, %s)", len(result.Items), result.Total, page, resolvedCountry), result.Items)
		},
	}

	cmd.Flags().StringVar(&typesCSV, "type", "game", steamTypeFlagHelp)
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Store tag name or tagid to require (repeatable or comma-separated, e.g. --tag Roguelike,Metroidvania)")
	cmd.Flags().BoolVar(&freeOnly, "free", false, "Only free items (free to play, free demos)")
	cmd.Flags().BoolVar(&comingSoon, "coming-soon", false, "Only unreleased items")
	cmd.Flags().BoolVar(&releasedOnly, "released", false, "Only released items")
	cmd.Flags().IntVar(&limit, "limit", 20, "Results per page (1-100)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number, 1-based")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region (default STEAM_COUNTRY, ITAD_COUNTRY, or US)")
	cmd.Flags().StringVar(&lang, "lang", "", "Store locale for store text (default english)")
	return cmd
}

// renderSteamReviewLine is the human review rollup for the steam app
// command, including the unavailable marker when the review source was
// dropped.
func renderSteamReviewLine(w io.Writer, review *steam.ReviewSummary) {
	if review == nil {
		fmt.Fprintln(w, "Reviews: unavailable (sources_missing: steam_reviews)")
		return
	}
	total := review.Total
	if total == 0 {
		total = review.Positive + review.Negative
	}
	desc := review.Desc
	if desc == "" {
		desc = "-"
	}
	if total > 0 {
		fmt.Fprintf(w, "Reviews: %s (%d%% positive of %d)\n", desc, review.Positive*100/total, total)
		return
	}
	fmt.Fprintf(w, "Reviews: %s (no reviews yet)\n", desc)
}

// renderSteamItems is the human table for search, browse, and app.
func renderSteamItems(cmd *cobra.Command, heading string, items []steam.StoreItem) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, heading)
	if len(items) == 0 {
		fmt.Fprintln(w, "no Steam store items matched")
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"appid":    item.AppID,
			"type":     string(item.Type),
			"name":     item.Name,
			"released": orDash(item.ReleaseDate),
			"price":    steamPriceLabel(item),
			"flags":    steamFlagLabel(item),
		})
	}
	return printAutoTable(w, rows)
}

// steamPriceLabel renders a price cell, preferring the store's own formatted
// string because it already carries the region's currency and separators.
func steamPriceLabel(item steam.StoreItem) string {
	if item.Price == nil {
		if item.IsFree {
			return "free"
		}
		return "-"
	}
	label := item.Price.Formatted
	if label == "" {
		label = strconv.Itoa(item.Price.FinalCents)
	}
	if item.Price.DiscountPercent > 0 {
		label += fmt.Sprintf(" (-%d%%)", item.Price.DiscountPercent)
	}
	return label
}

// steamFlagLabel marks the attributes the taxonomy keeps out of the type field:
// Steam models free-to-play and early access as attributes, not app types.
func steamFlagLabel(item steam.StoreItem) string {
	var flags []string
	if item.IsFree {
		flags = append(flags, "free")
	}
	if item.EarlyAccess {
		flags = append(flags, "early-access")
	}
	if item.ComingSoon {
		flags = append(flags, "coming-soon")
	}
	if len(item.DemoAppIDs) > 0 {
		flags = append(flags, "has-demo")
	}
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSteamCmd(flags))
	})
	whichIndex = append(whichIndex,
		whichEntry{Command: "steam search", Description: "Search the keyless Steam store catalog by term with an app-type filter (game, demo, dlc, soundtrack, software, video, mod, hardware). No Steam API key needed."},
		whichEntry{Command: "steam app", Description: "Full typed Steam store record for one appid or title: type, release, platforms, tags, price, demo links, and the keyless review summary; resolves a trailing (YYYY) remake suffix."},
		whichEntry{Command: "steam browse", Description: "Paginated Steam catalog browse filtered by app type, free-only, store tag, and coming-soon/released, localised with --country."},
		whichEntry{Command: "steam demos", Description: "Find free demos available on Steam: demos only, filter by title and by tags (every tag required), each with the full game it belongs to."},
	)
}
