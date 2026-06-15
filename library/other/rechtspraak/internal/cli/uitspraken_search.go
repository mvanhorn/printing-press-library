// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newUitsprakenSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagMax                 int
		flagFromOff             int
		flagFromDate            string
		flagToDate              string
		flagModifiedFrom        string
		flagModifiedTo          string
		flagSubjects            []string
		flagCourts              []string
		flagType                string
		flagReplaces            []string
		flagReturn              string
		flagSort                string
		flagKeyword             []string
		flagExclude             []string
		flagPhrase              []string
		flagRegex               []string
		flagProcedure           []string
		flagAnnotateCount       bool
		flagIncludePredecessors bool
		flagScanBody            bool
		flagMaxPages            int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search the Dutch ECLI index with rich local narrowing on top",
		Long: `Search /uitspraken/zoeken with the upstream filter set (date range, modified
range, multi-court UNION, multi-subject UNION, type, replaces) plus local
narrowing flags. By default, --keyword/--exclude/--phrase/--regex match
against the Atom feed's title + summary only (cheap, no extra fetches).
Pass --scan-body to also match against each decision's body — required
when party names, company names, or substantive terms live in the body
text but not the headnote.

  --keyword KW       must contain KW (AND, repeatable, case-insensitive)
  --exclude KW       must NOT contain KW (NOT, repeatable, case-insensitive)
  --phrase "P"       must contain exact phrase
  --regex /R/        Go-syntax regex must match (repeatable; AND)
  --procedure NAME   match against procedure metadata (the upstream API ignores procedure=)
  --scan-body        fetch each entry's body for matching (one extra HTTP call per entry)
  --max-pages N      sweep N pages of upstream results (default 1; warn-on-truncation always)
  --include-predecessors   also query predecessor courts for the named --court(s)

Per IVO 1.15: same-type params are OR-unioned; cross-type params are
AND-combined. The upstream API has no free-text search, so keyword
narrowing happens locally — efficiently on title+summary by default,
across the full body with --scan-body.`,
		Example: `  rechtspraak-pp-cli uitspraken search --from 2024-01-01 --to 2024-01-31 --court HR
  rechtspraak-pp-cli uitspraken search --court HR --subject belastingrecht --keyword "omkering bewijslast"
  rechtspraak-pp-cli uitspraken search --court RBNNE --include-predecessors --from 2010-01-01 --to 2014-12-31
  # Find a company/party name that lives in the body, not the headnote:
  rechtspraak-pp-cli uitspraken search --court RBDHA --from 2026-04-17 --to 2026-04-17 --keyword "Transvision" --scan-body
  # Sweep a wider window when total > 1 page:
  rechtspraak-pp-cli uitspraken search --court HR --from 2024-01-01 --to 2024-12-31 --max-pages 10`,
		Annotations: map[string]string{"pp:endpoint": "uitspraken.search", "pp:method": "GET", "pp:path": "/uitspraken/zoeken", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			courtIdx, err := getCourtIndex(ctx)
			if err != nil {
				return err
			}
			subjIdx, err := getSubjectIndex(ctx)
			if err != nil {
				return err
			}

			params := rechtspraak.SearchParams{
				Type:     flagType,
				Replaces: flagReplaces,
				Return:   flagReturn,
				Sort:     flagSort,
				Max:      flagMax,
				From:     flagFromOff,
			}
			if flagFromDate != "" || flagToDate != "" {
				if flagFromDate != "" {
					params.Dates = append(params.Dates, flagFromDate)
				}
				if flagToDate != "" {
					params.Dates = append(params.Dates, flagToDate)
				}
			}
			if flagModifiedFrom != "" || flagModifiedTo != "" {
				if flagModifiedFrom != "" {
					params.Modified = append(params.Modified, flagModifiedFrom)
				}
				if flagModifiedTo != "" {
					params.Modified = append(params.Modified, flagModifiedTo)
				}
			}
			for _, s := range flagSubjects {
				if uri := subjIdx.URI(s); uri != "" {
					params.Subjects = append(params.Subjects, uri)
				} else if s != "" {
					return fmt.Errorf("unknown subject %q (try `rechtspraak-pp-cli subjects`)", s)
				}
			}
			for _, c := range flagCourts {
				court, ok := courtIdx.Resolve(c)
				if !ok {
					return fmt.Errorf("unknown court %q (try `rechtspraak-pp-cli code %s`)", c, c)
				}
				params.Creators = append(params.Creators, court.Identifier)
				if flagIncludePredecessors {
					for _, p := range courtIdx.Predecessors(court) {
						params.Creators = append(params.Creators, p.Identifier)
					}
				}
			}

			http := mustHTTP()

			// Pagination: --max-pages controls how many pages of upstream
			// results we fetch. Default 1 preserves prior behavior. The
			// truncation warning fires whenever upstream total > fetched
			// — regardless of --max-pages — so users see when a window
			// query is partial.
			if flagMaxPages < 1 {
				flagMaxPages = 1
			}
			var entries []rechtspraak.SearchEntry
			var total int
			for page := 0; page < flagMaxPages; page++ {
				p := params
				if page > 0 {
					p.From = flagFromOff + page*params.Max
				}
				pageEntries, pageTotal, err := http.Search(ctx, p)
				if err != nil {
					return err
				}
				entries = append(entries, pageEntries...)
				total = pageTotal
				if len(pageEntries) < params.Max {
					break
				}
				if total > 0 && len(entries) >= total {
					break
				}
			}
			// Truncation warning: emit on stderr whenever upstream reports
			// more total matches than we fetched. Always emit (not just
			// under --annotate-count) so users can never accidentally
			// trust a partial sweep.
			if total > len(entries) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"search: fetched %d of %d total upstream matches (%d page(s) at --max=%d). Re-run with --max-pages N to sweep more.\n",
					len(entries), total, flagMaxPages, params.Max)
			}

			// Local narrowing. Without --scan-body, match against title +
			// summary only (cheap, no extra fetches). With --scan-body,
			// fetch each entry's body via /uitspraken/content and match
			// against title + summary + body — required when party or
			// company names live in the body but not the headnote.
			fetched := len(entries)
			if len(flagKeyword) > 0 || len(flagExclude) > 0 || len(flagPhrase) > 0 || len(flagRegex) > 0 || len(flagProcedure) > 0 {
				regexes := make([]*regexp.Regexp, 0, len(flagRegex))
				for _, r := range flagRegex {
					re, err := regexp.Compile(r)
					if err != nil {
						return fmt.Errorf("bad --regex %q: %w", r, err)
					}
					regexes = append(regexes, re)
				}
				var procIdx *rechtspraak.ProcedureIndex
				if len(flagProcedure) > 0 {
					procIdx, _ = getProcedureIndex(ctx)
				}
				if flagScanBody {
					entries = narrowEntriesWithBody(ctx, http, procIdx, entries, flagKeyword, flagExclude, flagPhrase, regexes, flagProcedure)
				} else {
					entries = narrowEntriesLocal(ctx, http, procIdx, entries, flagKeyword, flagExclude, flagPhrase, regexes, flagProcedure)
				}
			}
			if flagAnnotateCount {
				fmt.Fprintf(cmd.ErrOrStderr(), "search: total=%d fetched=%d after_narrow=%d scan_body=%v\n",
					total, fetched, len(entries), flagScanBody)
			}

			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), map[string]any{
					"total":   total,
					"count":   len(entries),
					"entries": entries,
				})
			}
			if flags.quiet {
				for _, e := range entries {
					fmt.Fprintln(cmd.OutOrStdout(), e.ECLI)
				}
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", e.ECLI, e.Title)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagMax, "max", 100, "Maximum results per page (server caps at 1000)")
	cmd.Flags().IntVar(&flagFromOff, "from-offset", 0, "Pagination offset (server's from= param)")
	cmd.Flags().StringVar(&flagFromDate, "from", "", "Decision-date floor (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagToDate, "to", "", "Decision-date ceiling (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagModifiedFrom, "modified-from", "", "Modification-date floor (ISO8601)")
	cmd.Flags().StringVar(&flagModifiedTo, "modified-to", "", "Modification-date ceiling (ISO8601)")
	cmd.Flags().StringSliceVar(&flagSubjects, "subject", nil, "Subject area (name, slug, or PSI URI; repeat for OR-union)")
	cmd.Flags().StringSliceVar(&flagCourts, "court", nil, "Court (afkorting, name, or PSI URI; repeat for OR-union)")
	cmd.Flags().StringVar(&flagType, "type", "", "Document type: Uitspraak | Conclusie")
	cmd.Flags().StringSliceVar(&flagReplaces, "replaces", nil, "Old LJN code(s) - returns the ECLI that superseded each LJN")
	cmd.Flags().StringVar(&flagReturn, "return", "", "DOC = only ECLIs with a document body")
	cmd.Flags().StringVar(&flagSort, "sort", "", "Sort by modification date: ASC | DESC (default ASC)")
	cmd.Flags().StringSliceVar(&flagKeyword, "keyword", nil, "Local keyword filter (AND, repeatable, case-insensitive)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Local exclude filter (NOT, repeatable, case-insensitive)")
	cmd.Flags().StringSliceVar(&flagPhrase, "phrase", nil, "Required exact phrase (repeatable)")
	cmd.Flags().StringSliceVar(&flagRegex, "regex", nil, "Go-syntax regex that must match against title+summary (repeatable)")
	cmd.Flags().StringSliceVar(&flagProcedure, "procedure", nil, "Procedure type (name or slug; upstream ignores procedure=, filtered locally)")
	cmd.Flags().BoolVar(&flagAnnotateCount, "annotate-count", false, "Print total/fetched/post-narrow counts to stderr")
	cmd.Flags().BoolVar(&flagIncludePredecessors, "include-predecessors", false, "Also query the named court's predecessors (Wet Herziening Gerechtelijke Kaart)")
	cmd.Flags().BoolVar(&flagScanBody, "scan-body", false, "Fetch each entry's body and match --keyword/--exclude/--phrase/--regex against title+summary+body (one extra HTTP call per entry; rate-limited)")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 1, "Maximum pages of upstream results to fetch (each page = --max entries). Default 1; truncation warning fires when upstream total exceeds fetched")
	// Validate enum-style values manually so errors are clear.
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if flagType != "" && flagType != "Uitspraak" && flagType != "Conclusie" {
			return fmt.Errorf("--type must be Uitspraak or Conclusie, got %q", flagType)
		}
		if flagReturn != "" && flagReturn != "DOC" {
			return fmt.Errorf("--return must be DOC, got %q", flagReturn)
		}
		if flagSort != "" && flagSort != "ASC" && flagSort != "DESC" {
			return fmt.Errorf("--sort must be ASC or DESC, got %q", flagSort)
		}
		return nil
	}
	return cmd
}

// narrowEntriesWithBody is the --scan-body variant: it fetches each entry's
// full content (metadata + inhoudsindicatie + uitspraak body) via
// /uitspraken/content and applies the filter set against the full text.
// One HTTP call per entry, paced via the shared rate limiter. Required when
// company/party names live in the body but not the headnote — the most
// common cause of "I know this term is in the decision but search returns
// 0" complaints.
func narrowEntriesWithBody(ctx context.Context, http *rechtspraak.HTTP, procIdx *rechtspraak.ProcedureIndex, entries []rechtspraak.SearchEntry, keywords, excludes, phrases []string, regexes []*regexp.Regexp, procedures []string) []rechtspraak.SearchEntry {
	out := make([]rechtspraak.SearchEntry, 0, len(entries))
	for _, e := range entries {
		d, err := http.Get(ctx, e.ECLI, false)
		if err != nil {
			continue
		}
		body := d.Title + "\n" + d.Summary + "\n" + d.Body
		corpus := strings.ToLower(body)
		skip := false
		for _, kw := range keywords {
			if !strings.Contains(corpus, strings.ToLower(kw)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, ex := range excludes {
			if strings.Contains(corpus, strings.ToLower(ex)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, p := range phrases {
			if !strings.Contains(body, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, re := range regexes {
			if !re.MatchString(body) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if len(procedures) > 0 && procIdx != nil {
			match := false
			for _, p := range procedures {
				if procIdx.Matches(p, d.ProcedureURI) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// narrowEntriesLocal applies keyword / exclude / phrase / regex / procedure
// filters to a SearchEntry slice. Procedure narrowing requires fetching each
// decision's metadata (the search Atom feed doesn't include procedure), so
// it short-circuits the inexpensive title+summary checks first.
func narrowEntriesLocal(ctx context.Context, http *rechtspraak.HTTP, procIdx *rechtspraak.ProcedureIndex, entries []rechtspraak.SearchEntry, keywords, excludes, phrases []string, regexes []*regexp.Regexp, procedures []string) []rechtspraak.SearchEntry {
	out := make([]rechtspraak.SearchEntry, 0, len(entries))
	for _, e := range entries {
		body := e.Title + "\n" + e.Summary
		corpus := strings.ToLower(body)
		skip := false
		for _, kw := range keywords {
			if !strings.Contains(corpus, strings.ToLower(kw)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, ex := range excludes {
			if strings.Contains(corpus, strings.ToLower(ex)) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, p := range phrases {
			if !strings.Contains(body, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, re := range regexes {
			if !re.MatchString(body) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if len(procedures) > 0 && procIdx != nil {
			d, err := http.Get(ctx, e.ECLI, true)
			if err != nil {
				continue
			}
			match := false
			for _, p := range procedures {
				if procIdx.Matches(p, d.ProcedureURI) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}
