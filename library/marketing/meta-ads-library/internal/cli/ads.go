// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func classifyAdLibraryErr(cmd *cobra.Command, flags *rootFlags, err error) error {
	if err == nil {
		return nil
	}
	var rateErr *cliutil.RateLimitError
	if errors.As(err, &rateErr) {
		classified := rateLimitErr(err)
		writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, classified, ExitCode(classified))
		return classified
	}
	return classifyAPIError(cmd.OutOrStdout(), err, flags)
}

func firstValidAdvertiser(advs []adlibrary.Advertiser) (adlibrary.Advertiser, bool) {
	for _, a := range advs {
		if strings.TrimSpace(a.PageID) != "" {
			return a, true
		}
	}
	return adlibrary.Advertiser{}, false
}

// isSnapshotEligiblePageScan returns true only for unfiltered active scans of a
// single Facebook Page, preventing narrowed searches from polluting diff snapshots.
func isSnapshotEligiblePageScan(params adlibrary.SearchParams) bool {
	if len(params.PageIDs) != 1 || strings.TrimSpace(params.PageIDs[0]) == "" {
		return false
	}
	if strings.TrimSpace(params.Query) != "" {
		return false
	}
	if s := strings.TrimSpace(params.ActiveStatus); s != "" && !strings.EqualFold(s, "ACTIVE") {
		return false
	}
	if m := strings.TrimSpace(params.MediaType); m != "" && !strings.EqualFold(m, "ALL") {
		return false
	}
	if t := strings.TrimSpace(params.AdType); t != "" && !strings.EqualFold(t, "ALL") {
		return false
	}
	if len(params.Platforms) > 0 || len(params.Languages) > 0 {
		return false
	}
	if strings.TrimSpace(params.Cursor) != "" || params.MinCollation > 0 {
		return false
	}
	return true
}

func newAdLibraryClient(flags *rootFlags) *adlibrary.Client {
	var opts []adlibrary.ClientOption
	if base := strings.TrimSpace(os.Getenv("META_AD_LIBRARY_GRAPHQL_BASE_URL")); base != "" {
		opts = append(opts, adlibrary.WithBaseURL(base))
	}
	if flags != nil && flags.rateLimit > 0 {
		opts = append(opts, adlibrary.WithRateLimit(flags.rateLimit))
	}
	return adlibrary.NewClient(opts...)
}

func newAdsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ads",
		Short: "Search global Meta ads, inspect ad details, and build Ad Library URLs (zero API key)",
		Long:  "Zero-auth public Meta Ad Library commands powered by pure HTTP GraphQL (no browser, no API key required).",
	}
	cmd.AddCommand(newAdsSearchCmd(flags))
	cmd.AddCommand(newAdsDetailsCmd(flags))
	cmd.AddCommand(newAdsURLCmd(flags))
	return cmd
}

func newAdsSearchCmd(flags *rootFlags) *cobra.Command {
	var query string
	var pageIDs []string
	var pageName string
	var rawURL string
	var country string
	var status string
	var adType string
	var mediaType string
	var searchType string
	var platforms []string
	var languages []string
	var sortBy string
	var limit int
	var maxPages int
	var cursor string
	var minCollation int
	var noSave bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search public Meta ads globally by keyword, page ID, brand name, or Ad Library URL",
		Long: `Search active and inactive commercial or political ads across Facebook, Instagram, Threads, Messenger, and Audience Network in any country with zero API key and zero browser.

Automatically unwraps l.facebook.com redirect links into clean landing URLs and persists results plus advertiser snapshots into local SQLite for winners, diff, compare, hooks, funnels, and offline FTS5 search.`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli ads search --query "running shoes" --country US --limit 10 --agent
  meta-ads-library-pp-cli ads search --page-id 15087023444 --country US --limit 25 --agent
  meta-ads-library-pp-cli ads search --url "https://www.facebook.com/ads/library/?active_status=active&ad_type=all&country=MX&q=cafe" --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--query=nike;--country=US;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && strings.TrimSpace(query) == "" {
				query = strings.Join(args, " ")
			}
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ads search")
			}

			params := adlibrary.SearchParams{
				Query:        strings.TrimSpace(query),
				Country:      country,
				AdType:       adType,
				ActiveStatus: status,
				MediaType:    mediaType,
				SearchType:   searchType,
				PageIDs:      pageIDs,
				Platforms:    platforms,
				Languages:    languages,
				SortBy:       sortBy,
				Limit:        limit,
				MaxPages:     maxPages,
				Cursor:       cursor,
				MinCollation: minCollation,
			}

			if strings.TrimSpace(rawURL) != "" {
				parsed, parsedAdID, err := adlibrary.ParseAdLibraryURL(rawURL)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --url %q: %w", rawURL, err))
				}
				if params.Query == "" {
					params.Query = parsed.Query
				}
				if !cmd.Flags().Changed("country") && parsed.Country != "" {
					params.Country = parsed.Country
				}
				if !cmd.Flags().Changed("status") && parsed.ActiveStatus != "" {
					params.ActiveStatus = parsed.ActiveStatus
				}
				if !cmd.Flags().Changed("ad-type") && parsed.AdType != "" {
					params.AdType = parsed.AdType
				}
				if !cmd.Flags().Changed("media-type") && parsed.MediaType != "" {
					params.MediaType = parsed.MediaType
				}
				if !cmd.Flags().Changed("search-type") && parsed.SearchType != "" {
					params.SearchType = parsed.SearchType
				}
				if !cmd.Flags().Changed("sort-by") && parsed.SortBy != "" {
					params.SortBy = parsed.SortBy
				}
				if len(params.PageIDs) == 0 && len(parsed.PageIDs) > 0 {
					params.PageIDs = parsed.PageIDs
				}
				if len(params.Platforms) == 0 && len(parsed.Platforms) > 0 {
					params.Platforms = parsed.Platforms
				}
				if parsedAdID != "" && params.Query == "" && len(params.PageIDs) == 0 && strings.TrimSpace(pageName) == "" {
					return usageErr(fmt.Errorf("single-ad URL (?id=%s) cannot be queried with 'ads search'; use 'meta-ads-library-pp-cli ads details %s --page-id <page_id>' instead", parsedAdID, parsedAdID))
				}
			}

			if params.Query == "" && len(params.PageIDs) == 0 && strings.TrimSpace(pageName) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a keyword (--query), --page-id, --page-name, or --url"))
			}

			if cliutil.IsVerifyEnv() {
				return writeAdsSearchResult(cmd, adlibrary.SearchResult{
					Ads:        make([]adlibrary.Ad, 0),
					IsComplete: true,
				}, flags)
			}
			if cliutil.IsDogfoodEnv() {
				if params.MaxPages <= 0 || params.MaxPages > 1 {
					params.MaxPages = 1
				}
				if params.Limit <= 0 || params.Limit > 5 {
					params.Limit = 5
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := newAdLibraryClient(flags)

			// Resolve --page-name via typeahead if supplied.
			resolvedPageName := ""
			if strings.TrimSpace(pageName) != "" && len(params.PageIDs) == 0 {
				advs, err := client.SearchAdvertisers(ctx, pageName, params.Country)
				if err != nil {
					return classifyAdLibraryErr(cmd, flags, err)
				}
				top, ok := firstValidAdvertiser(advs)
				if !ok {
					return notFoundErr(fmt.Errorf("no advertiser page found matching %q in country %s", pageName, params.Country))
				}
				params.PageIDs = []string{top.PageID}
				resolvedPageName = top.PageName
				if params.SearchType == "" {
					params.SearchType = "PAGE"
				}
			}

			scanStartedAt := time.Now().UTC()
			result, err := client.SearchAds(ctx, params)
			if err != nil {
				return classifyAdLibraryErr(cmd, flags, err)
			}

			eligibleSnapshot := isSnapshotEligiblePageScan(params)
			if !noSave && !localDataDisabled() && (len(result.Ads) > 0 || eligibleSnapshot) {
				resolvedDB := dbPath
				if resolvedDB == "" {
					resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
				}
				db, openErr := store.OpenWithContext(ctx, resolvedDB)
				if openErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to open local database %s: %v\n", resolvedDB, openErr)
				} else {
					capturedAt := time.Now().UTC()
					saveFailed := false
					if len(result.Ads) > 0 {
						if saveErr := adlibrary.SaveAdsBatch(ctx, db, result.Ads); saveErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist ads locally: %v\n", saveErr)
							saveFailed = true
						}
					}
					if !saveFailed && eligibleSnapshot {
						pageTitle := resolvedPageName
						for _, a := range result.Ads {
							if a.PageName != "" {
								pageTitle = a.PageName
								break
							}
						}
						if pageTitle == "" {
							if _, existingName, _, resErr := adlibrary.ResolvePageID(ctx, db, params.PageIDs[0]); resErr == nil && existingName != "" {
								pageTitle = existingName
							}
						}
						if _, snapErr := adlibrary.RecordSnapshot(ctx, db, params.PageIDs[0], pageTitle, params.Country, params.Query, result.Ads, result.IsComplete, capturedAt); snapErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to record snapshot: %v\n", snapErr)
						}
						if result.IsComplete {
							if _, reconErr := adlibrary.ReconcilePageActiveAds(ctx, db, params.PageIDs[0], params.Country, result.Ads, scanStartedAt); reconErr != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to reconcile active ads: %v\n", reconErr)
							}
						}
					}
					_ = db.Close()
				}
			}

			return writeAdsSearchResult(cmd, result, flags)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Keyword search string for ad copy, headlines, and CTAs")
	cmd.Flags().StringSliceVar(&pageIDs, "page-id", nil, "Filter to one or more numeric Facebook Page IDs (repeatable)")
	cmd.Flags().StringVar(&pageName, "page-name", "", "Look up a brand name via typeahead and fetch ads for the top matching Facebook Page")
	cmd.Flags().StringVar(&rawURL, "url", "", "Meta Ad Library web URL (https://www.facebook.com/ads/library/?...) to parse and execute")
	cmd.Flags().StringVarP(&country, "country", "c", "US", "ISO 3166-1 alpha-2 country code (e.g. US, MX, GB, DE) or ALL")
	cmd.Flags().StringVar(&status, "status", "ACTIVE", "Ad active status filter: ACTIVE, INACTIVE, or ALL")
	cmd.Flags().StringVar(&adType, "ad-type", "ALL", "Ad category: ALL, POLITICAL_AND_ISSUE_ADS, HOUSING_ADS, EMPLOYMENT_ADS, CREDIT_ADS")
	cmd.Flags().StringVar(&mediaType, "media-type", "ALL", "Creative media type filter: ALL, IMAGE, VIDEO, MEME, NONE")
	cmd.Flags().StringVar(&searchType, "search-type", "", "Search match mode: KEYWORD_UNORDERED, KEYWORD_EXACT_PHRASE, or PAGE")
	cmd.Flags().StringSliceVar(&platforms, "platform", nil, "Filter publisher platforms (FACEBOOK, INSTAGRAM, MESSENGER, THREADS, AUDIENCE_NETWORK)")
	cmd.Flags().StringSliceVar(&languages, "language", nil, "Filter ISO 639-1 content languages (e.g. en, es)")
	cmd.Flags().StringVar(&sortBy, "sort-by", "relevancy", "Sort order: relevancy or impressions")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of ads to return")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Maximum GraphQL pages to fetch before returning")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a prior search response")
	cmd.Flags().IntVar(&minCollation, "min-collation", 0, "Client-side minimum collation count (scale filter)")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Do not persist fetched ads to local SQLite")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database (defaults to standard mirror path)")
	return cmd
}

func writeAdsSearchResult(cmd *cobra.Command, result adlibrary.SearchResult, flags *rootFlags) error {
	ads := result.Ads
	if ads == nil {
		ads = make([]adlibrary.Ad, 0)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if flags != nil && (flags.csv || flags.plain || flags.quiet) {
			if err := printJSONFiltered(cmd.OutOrStdout(), ads, flags); err != nil {
				return err
			}
			printAdsSearchContinuationHint(cmd, result)
			return nil
		}
		if err := validatePlatformAnalytics(flags); err != nil {
			return err
		}
		itemsRaw, err := json.Marshal(ads)
		if err != nil {
			return err
		}
		var selectErr error
		if flags != nil && flags.selectFields != "" {
			itemsRaw, selectErr = filterFieldsChecked(json.RawMessage(itemsRaw), flags.selectFields)
			selectErr = selectErrorForDryRun(selectErr, flags, json.RawMessage(itemsRaw))
		} else if flags != nil && flags.compact {
			itemsRaw = compactFieldsKeep(json.RawMessage(itemsRaw), nil)
		}
		meta := map[string]any{
			"source":        "live",
			"count":         len(ads),
			"total_count":   result.TotalCount,
			"pages_read":    result.PagesRead,
			"has_next_page": result.HasNextPage,
			"is_complete":   result.IsComplete,
		}
		if result.NextCursor != "" {
			meta["next_cursor"] = result.NextCursor
		}
		envelope, err := json.Marshal(map[string]any{
			"meta":    meta,
			"results": json.RawMessage(itemsRaw),
		})
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(json.RawMessage(envelope), flags, "results", true)
		if err != nil {
			return err
		}
		if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
			return err
		}
		return selectErr
	}
	if len(ads) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matching ads found.")
		return nil
	}
	rows := make([][]string, 0, len(ads))
	for _, ad := range ads {
		hook, _ := adlibrary.ExtractOpeningHook(ad.BodyText)
		hook = truncate(hook, 55)
		rows = append(rows, []string{
			ad.AdArchiveID,
			ad.PageName,
			ad.DisplayFormat,
			fmt.Sprintf("%dx", ad.CollationCount),
			fmt.Sprintf("%dd", ad.DaysRunning),
			ad.LandingDomain,
			hook,
		})
	}
	if err := flags.printTable(cmd, []string{"AD ID", "PAGE", "FORMAT", "COLL", "AGE", "DOMAIN", "HOOK"}, rows); err != nil {
		return err
	}
	printAdsSearchContinuationHint(cmd, result)
	return nil
}

func printAdsSearchContinuationHint(cmd *cobra.Command, result adlibrary.SearchResult) {
	if result.IsComplete {
		return
	}
	if result.HasNextPage && result.NextCursor != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "More results available (has_next_page=true). Continue with: --cursor %q\n", result.NextCursor)
		return
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Results capped by --limit before page exhaustion. Increase --limit or --max-pages for a complete scan.")
}

func newAdsDetailsCmd(flags *rootFlags) *cobra.Command {
	var adID string
	var pageID string
	var country string
	var noSave bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "details [ad-archive-id]",
		Short: "Fetch full ad details and EU/UK/BR transparency disclosures for a single ad (zero API key)",
		Long: `Calls Meta's public AdLibraryV3AdDetailsQuery for a single ad archive ID and Facebook Page ID to inspect EU/UK/BR total reach, beneficiary/payer disclosures, age-gender breakdowns, advertiser metadata, and policy/AI disclosure flags.

Use this command to inspect one specific ad's full transparency payload. Do NOT use it for an advertiser-wide transparency rollup; use 'transparency' instead.`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli ads details 1550534860016399 --page-id 115939511520844 --country US --agent
  meta-ads-library-pp-cli ads details --ad-id 1550534860016399 --page-id 115939511520844 --country DE --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "ad=1550534860016399;--page-id=115939511520844;--country=US",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && strings.TrimSpace(adID) == "" {
				adID = strings.TrimSpace(args[0])
			}
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ads details")
			}
			if strings.TrimSpace(adID) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("ad archive ID is required (pass positional [ad-archive-id] or --ad-id)"))
			}

			// If pageID was omitted, attempt to resolve it from local SQLite before requiring --page-id.
			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if strings.TrimSpace(pageID) == "" {
				if _, statErr := os.Stat(resolvedDB); statErr == nil {
					if db, openErr := store.OpenReadOnlyContext(cmd.Context(), resolvedDB); openErr == nil {
						var pid string
						_ = db.DB().QueryRowContext(cmd.Context(), `SELECT page_id FROM meta_ads WHERE ad_archive_id = ? LIMIT 1`, adID).Scan(&pid)
						pageID = strings.TrimSpace(pid)
						_ = db.Close()
					}
				}
			}
			if strings.TrimSpace(pageID) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--page-id is required when ad %s is not yet stored locally", adID))
			}

			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), adlibrary.AdDetails{AdArchiveID: adID, PageID: pageID, Country: country}, flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := newAdLibraryClient(flags)
			details, err := client.GetAdDetails(ctx, adID, pageID, country)
			if err != nil {
				return classifyAdLibraryErr(cmd, flags, err)
			}

			if !noSave && !localDataDisabled() {
				db, openErr := store.OpenWithContext(ctx, resolvedDB)
				if openErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to open local database %s: %v\n", resolvedDB, openErr)
				} else {
					if saveErr := adlibrary.SaveAdDetails(ctx, db, details); saveErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist ad details locally: %v\n", saveErr)
					}
					_ = db.Close()
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), details, flags)
			}
			rows := [][]string{
				{"Ad Archive ID", details.AdArchiveID},
				{"Page ID", details.PageID},
				{"Page Name", details.PageName},
				{"Country", details.Country},
				{"EU Total Reach", fmt.Sprintf("%d", details.EUTotalReach)},
				{"UK Total Reach", fmt.Sprintf("%d", details.UKTotalReach)},
				{"BR Total Reach", fmt.Sprintf("%d", details.BRTotalReach)},
				{"Target Gender", details.TargetGender},
				{"Beneficiary/Payer Count", fmt.Sprintf("%d", len(details.BeneficiaryPayers))},
				{"Violating EU SIEP", fmt.Sprintf("%t", details.IsViolatingEUSIEP)},
			}
			return flags.printTable(cmd, []string{"FIELD", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&adID, "ad-id", "", "Numeric Meta Ad Library archive ID")
	cmd.Flags().StringVar(&pageID, "page-id", "", "Numeric Facebook Page ID of the advertiser (auto-resolved from SQLite if previously synced)")
	cmd.Flags().StringVarP(&country, "country", "c", "ALL", "ISO country code or ALL for transparency lookup")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Do not persist fetched ad details to local SQLite")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

func newAdsURLCmd(flags *rootFlags) *cobra.Command {
	var adID string
	var pageID string
	var query string
	var country string
	var status string
	var adType string

	cmd := &cobra.Command{
		Use:   "url",
		Short: "Build a clickable Meta Ad Library deep link for an ad ID, page ID, or keyword query",
		Example: strings.Trim(`
  meta-ads-library-pp-cli ads url --ad-id 1550534860016399 --json
  meta-ads-library-pp-cli ads url --page-id 15087023444 --country US --json
  meta-ads-library-pp-cli ads url --query "running shoes" --country MX --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "computed",
			"pp:happy-args":  "--page-id=15087023444;--country=US",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "ads url")
			}
			if strings.TrimSpace(adID) == "" && strings.TrimSpace(pageID) == "" && strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least one of --ad-id, --page-id, or --query"))
			}
			built := adlibrary.BuildAdLibraryURL(adID, pageID, query, country, status, adType)
			res := map[string]any{
				"url":     built,
				"ad_id":   adID,
				"page_id": pageID,
				"query":   query,
				"country": strings.ToUpper(strings.TrimSpace(country)),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), built)
			return nil
		},
	}

	cmd.Flags().StringVar(&adID, "ad-id", "", "Numeric Meta Ad Library archive ID")
	cmd.Flags().StringVar(&pageID, "page-id", "", "Numeric Facebook Page ID to view all ads for")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Keyword search term for the deep link")
	cmd.Flags().StringVarP(&country, "country", "c", "US", "ISO country code (e.g. US, MX, GB, DE) or ALL")
	cmd.Flags().StringVar(&status, "status", "active", "Ad active status: active, inactive, or all")
	cmd.Flags().StringVar(&adType, "ad-type", "all", "Ad category: all, political, housing, employment, credit")
	return cmd
}
