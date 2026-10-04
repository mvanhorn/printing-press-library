// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live — keyless Steam store services (demos listing, full-game names and tags).
//
// steam_demos.go - the `steam demos` surface: a demos-only listing (app
// type = demo) that pairs each demo with the full game it belongs to.
// Free-to-play stays with `steam browse --free`; demos are a distinct app
// type here, not a price attribute.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"
)

// steamDemoRow pairs a demo's full store record with the display name of the
// full game it belongs to. The parent appid already rides on StoreItem.
type steamDemoRow struct {
	steam.StoreItem
	ParentName string   `json:"parent_name,omitempty"`
	ParentTags []string `json:"parent_tags,omitempty"`
}

// steamDemoView is the demos envelope, shaped like the `steam browse` one.
type steamDemoView struct {
	Meta    steamMeta      `json:"meta"`
	Results []steamDemoRow `json:"results"`
}

// steamDemosLong explains the demos-only scope, the every-tag contract, the
// title batch's truncation, and the request budget.
const steamDemosLong = `Demos only: this command fixes the app type to "demo" and never returns
full games. Free-to-play titles are not demos - use "steam browse --free" for
those. Each row carries the full game the demo belongs to: its parent_name and
parent_tags (the full game's tag names, in Steam's order). A demo's OWN tags
appear as "tags" only when Steam lists any - most demos have none. The human
table's "full game tags" column shows the first few parent_tags.

Every --tag must be present (the store service ANDs across tags). --title runs a
single text-search batch of 100 results by default (raise --limit up to 1000);
that endpoint ignores offsets, so the batch reports meta.truncated instead of a
next page, and --page is rejected with --title.

Request budget per invocation:
  - a browse page costs 3 requests: one catalog Query, one GetTagList lookup
    for the tag-name dictionary, and one GetItems lookup for the full-game
    names AND tags of every parent on the page;
  - the tag-name dictionary is fetched once and shared with --tag resolution
    and the parent lookup, so adding --tag does not add a request;
  - a --title batch costs 1 SearchSuggestions request, 1 GetTagList request for
    the tag-name dictionary, plus one name+tag lookup per 200 full games.

Both the browse and --title paths default to released demos ("available now"),
and --coming-soon swaps that for unreleased demos only. Either way the release
filter is applied server-side, so meta.total is the service's matching count,
not a filtered batch.

` + steamCatalogLong

func newSteamDemosCmd(flags *rootFlags) *cobra.Command {
	var title, country, lang string
	var tags []string
	var comingSoon bool
	var limit, page int

	cmd := &cobra.Command{
		Use:   "demos",
		Short: "Find free demos available on Steam (demos only), with the full game each belongs to",
		Long:  steamDemosLong,
		Example: strings.Trim(`
  game-goat-pp-cli steam demos --limit 20 --agent
  game-goat-pp-cli steam demos --tag Roguelike --tag Metroidvania --country DE
  game-goat-pp-cli steam demos --title portal --limit 1000 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--limit=20;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "steam demos")
			}
			title = strings.TrimSpace(title)
			if title != "" && !cmd.Flags().Changed("limit") {
				limit = 100
			}
			if page < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --page <n>", "--page must be 1 or greater")
			}
			if title != "" {
				if page != 1 {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --title <term>",
						"--page cannot be used with --title: the Steam text-search endpoint ignores offsets, so a title batch is always one page (raise --limit instead)")
				}
				if limit < 1 || limit > steam.MaxSearchBatch {
					return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --title <term> --limit <1-1000>", "--limit must be between 1 and 1000 with --title")
				}
			} else if limit < 1 || limit > steam.MaxPageSize {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <1-100>", "--limit must be between 1 and 100")
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

			meta := steamMeta{
				Source:   "live",
				Country:  resolvedCountry,
				Language: resolvedLang,
				Types:    []string{string(steam.AppTypeDemo)},
				Tags:     tags,
				Limit:    limit,
			}
			var items []steam.StoreItem

			if title != "" {
				p, serr := c.SearchPage(ctx, title, steam.SearchPageOptions{
					Types:        []steam.AppType{steam.AppTypeDemo},
					TagIDs:       tagIDs,
					ComingSoon:   comingSoon,
					ReleasedOnly: !comingSoon,
					Limit:        limit,
				})
				if serr != nil {
					return classifySteamError(serr)
				}
				items = p.Items
				c.AttachTagNames(ctx, items)
				meta.Title = title
				meta.Total = p.Total
				truncated := p.Truncated()
				meta.Truncated = &truncated
			} else {
				result, berr := c.Browse(ctx, steam.BrowseOptions{
					Types:        []steam.AppType{steam.AppTypeDemo},
					TagIDs:       tagIDs,
					ComingSoon:   comingSoon,
					ReleasedOnly: !comingSoon,
					Start:        (page - 1) * limit,
					Count:        limit,
				})
				if berr != nil {
					return classifySteamError(berr)
				}
				items = result.Items
				meta.Total = result.Total
				meta.Page = page
				if result.HasMore() {
					meta.NextPage = page + 1
				}
			}

			rows := demoRows(items)
			if _, perr := attachDemoParentSummaries(ctx, c, rows); perr != nil {
				// A plain error means every chunk failed; ErrPartialLookup means
				// some chunks resolved. Either way the full-game source is
				// incomplete. ErrTagNamesUnavailable is tracked separately.
				if errors.Is(perr, steam.ErrPartialLookup) ||
					(!errors.Is(perr, steam.ErrTagNamesUnavailable)) {
					meta.SourcesMissing = appendUniqueSource(meta.SourcesMissing, "steam_parent")
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam full-game names unavailable (sources_missing: steam_parent): %v\n", perr)
				}
				if errors.Is(perr, steam.ErrTagNamesUnavailable) {
					meta.SourcesMissing = appendUniqueSource(meta.SourcesMissing, "steam_tags")
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam tag names unavailable (sources_missing: steam_tags): %v\n", perr)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), steamDemoView{Meta: meta, Results: rows}, flags)
			}
			return renderSteamDemoItems(cmd, steamDemoHeading(title, meta, len(rows), page), rows)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Only demos whose title matches this term (one batch, up to 1000; the text-search endpoint ignores offsets, so --page is rejected with --title)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Store tag name or tagid that every result must carry (repeatable or comma-separated, e.g. --tag Roguelike,Metroidvania)")
	cmd.Flags().BoolVar(&comingSoon, "coming-soon", false, "Only unreleased demos; without it both paths return available-now demos (server-side filter)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Results per page (default 20; 100 with --title)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number, 1-based (browse only; rejected with --title)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 storefront region (default STEAM_COUNTRY, ITAD_COUNTRY, or US)")
	cmd.Flags().StringVar(&lang, "lang", "", "Store locale for store text (default english)")
	return cmd
}

func demoRows(items []steam.StoreItem) []steamDemoRow {
	rows := make([]steamDemoRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, steamDemoRow{StoreItem: item})
	}
	return rows
}

// attachDemoParentSummaries fills ParentName and ParentTags from one
// AppSummaries lookup over the unique parent appids on the page. The full
// game's tags arrive in the same GetItems request as its name, so the page
// stays at three requests. A failure is reported to the caller (so it can
// degrade to sources_missing) but never drops the demo rows: partial failures
// still fill the rows whose chunks resolved because AppSummaries returns the
// successful summaries alongside the error.
func attachDemoParentSummaries(ctx context.Context, c *steam.Client, rows []steamDemoRow) ([]int64, error) {
	seen := map[int64]bool{}
	var ids []int64
	for _, row := range rows {
		if row.ParentAppID > 0 && !seen[row.ParentAppID] {
			seen[row.ParentAppID] = true
			ids = append(ids, row.ParentAppID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	summaries, err := c.AppSummaries(ctx, ids)
	for i := range rows {
		summary, ok := summaries[rows[i].ParentAppID]
		if !ok {
			continue
		}
		rows[i].ParentName = summary.Name
		rows[i].ParentTags = namedTagNames(summary.Tags)
	}
	return ids, err
}

// appendUniqueSource adds src to list unless it is already present, so a
// degradation can be recorded once even when several code paths report it.
func appendUniqueSource(list []string, src string) []string {
	for _, s := range list {
		if s == src {
			return list
		}
	}
	return append(list, src)
}

// namedTagNames keeps the named tags in order, skipping entries the dictionary
// could not resolve.
func namedTagNames(tags []steam.Tag) []string {
	var out []string
	for _, t := range tags {
		if strings.TrimSpace(t.Name) != "" {
			out = append(out, t.Name)
		}
	}
	return out
}

func steamDemoHeading(title string, meta steamMeta, count, page int) string {
	if title != "" {
		suffix := ""
		if meta.Truncated != nil && *meta.Truncated {
			suffix = ", truncated"
		}
		return fmt.Sprintf("Steam demos matching %q (%d of %d%s)", title, count, meta.Total, suffix)
	}
	return fmt.Sprintf("Steam demos (%d of %d, page %d, %s)", count, meta.Total, page, meta.Country)
}

// renderSteamDemoItems is the human table for demos: the same columns as the
// other steam commands plus the full game each demo belongs to.
func renderSteamDemoItems(cmd *cobra.Command, heading string, rows []steamDemoRow) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, heading)
	if len(rows) == 0 {
		fmt.Fprintln(w, "no Steam demos matched")
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		parent := row.ParentName
		if parent == "" {
			parent = "-"
		}
		out = append(out, map[string]any{
			"appid":          row.AppID,
			"name":           row.Name,
			"full game":      parent,
			"released":       orDash(row.ReleaseDate),
			"platforms":      steamPlatformLabel(row.StoreItem),
			"flags":          steamFlagLabel(row.StoreItem),
			"full game tags": steamDemoParentTagLabel(row),
		})
	}
	return printAutoTable(w, out)
}

// steamDemoParentTagLabel renders the first three of the full game's tag names
// for the demos table, comma-separated, or "-" when the full game carries no
// named tags.
func steamDemoParentTagLabel(row steamDemoRow) string {
	names := row.ParentTags
	if len(names) > 3 {
		names = names[:3]
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ",")
}

// steamPlatformLabel lists the desktop platforms the store marks available.
func steamPlatformLabel(item steam.StoreItem) string {
	var oses []string
	if item.Platforms.Windows {
		oses = append(oses, "windows")
	}
	if item.Platforms.Mac {
		oses = append(oses, "mac")
	}
	if item.Platforms.Linux {
		oses = append(oses, "linux")
	}
	if len(oses) == 0 {
		return "-"
	}
	return strings.Join(oses, ",")
}
