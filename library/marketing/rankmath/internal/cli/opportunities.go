// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
	"github.com/spf13/cobra"
)

type keywordRow struct {
	Query       string  `json:"query"`
	Page        string  `json:"page"`
	Position    float64 `json:"position"`
	Impressions float64 `json:"impressions"`
	Clicks      float64 `json:"clicks"`
	CTR         float64 `json:"ctr"`
}

func decodeKeywordRows(raw json.RawMessage) ([]keywordRow, error) {
	out := make([]keywordRow, 0)
	if isNoDataResponse(raw) {
		return out, nil
	}
	var rows []json.RawMessage
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		if err := json.Unmarshal(raw, &keyed); err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(keyed))
		for k := range keyed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(keyed[k], &obj); err != nil {
				return nil, err
			}
			if obj["query"] == nil {
				obj["query"], _ = json.Marshal(k)
			}
			if obj["page"] == nil {
				obj["page"], _ = json.Marshal(k)
			}
			b, _ := json.Marshal(obj)
			rows = append(rows, b)
		}
	}
	for _, rawRow := range rows {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(rawRow, &obj); err != nil {
			return nil, err
		}
		var r keywordRow
		if b := obj["query"]; b != nil {
			if err := json.Unmarshal(b, &r.Query); err != nil {
				return nil, err
			}
		}
		if b := obj["page"]; b != nil {
			if err := json.Unmarshal(b, &r.Page); err != nil {
				return nil, err
			}
		}
		for name, dst := range map[string]*float64{"position": &r.Position, "impressions": &r.Impressions, "clicks": &r.Clicks, "ctr": &r.CTR} {
			b := obj[name]
			if b == nil {
				continue
			}
			var metric struct {
				Total float64 `json:"total"`
			}
			if err := json.Unmarshal(b, dst); err != nil {
				if err := json.Unmarshal(b, &metric); err != nil {
					return nil, fmt.Errorf("%s: %w", name, err)
				}
				*dst = metric.Total
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func tokenCovered(query, text string) bool {
	split := func(s string) []string {
		return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	}
	tokens := split(query)
	if len(tokens) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, t := range split(text) {
		have[t] = true
	}
	for _, t := range tokens {
		if !have[t] {
			return false
		}
	}
	return true
}

type opportunity struct {
	Query       string   `json:"query"`
	Position    float64  `json:"position"`
	Impressions float64  `json:"impressions"`
	Clicks      float64  `json:"clicks"`
	Page        string   `json:"page"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	MatchScope  string   `json:"match_scope"`
	MissingIn   []string `json:"missing_in"`
}
type opportunitiesResult struct {
	ScannedKeywordPages int           `json:"scanned_keyword_pages"`
	KeywordsConsidered  int           `json:"keywords_considered"`
	UnmatchedPages      int           `json:"unmatched_pages"`
	PageLookup          string        `json:"page_lookup"`
	Opportunities       []opportunity `json:"opportunities"`
	Note                string        `json:"note,omitempty"`
}

type opportunitySnapshot struct {
	head        seohead.Head
	titleTokens map[string]bool
	descTokens  map[string]bool
}

func pageKey(page string) string {
	host, path, _ := normalizePagePath(page)
	return host + "|" + path
}

func pathKey(page string) string {
	_, path, _ := normalizePagePath(page)
	return path
}

func tokenSet(text string) map[string]bool {
	set := make(map[string]bool)
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		set[token] = true
	}
	return set
}

func coversTokens(query string, have map[string]bool) bool {
	tokens := tokenSet(query)
	if len(tokens) == 0 {
		return false
	}
	for token := range tokens {
		if !have[token] {
			return false
		}
	}
	return true
}

func buildOpportunity(row, page keywordRow, snapshots, pathSnapshots map[string]opportunitySnapshot) (opportunity, bool) {
	o := opportunity{Query: row.Query, Position: row.Position, Impressions: row.Impressions, Clicks: row.Clicks, Page: page.Page, MatchScope: "site", MissingIn: make([]string, 0)}
	titleOK, descOK, matched := false, false, false
	if page.Page != "" {
		o.MatchScope = "page"
		snapshot, ok := snapshots[pageKey(page.Page)]
		if !ok && !strings.Contains(page.Page, "://") {
			snapshot, ok = pathSnapshots[pathKey(page.Page)]
		}
		if ok {
			matched = true
			o.Title, o.Description = snapshot.head.Title, snapshot.head.Description
			titleOK = coversTokens(row.Query, snapshot.titleTokens)
			descOK = coversTokens(row.Query, snapshot.descTokens)
		}
	} else {
		queryTokens := tokenSet(row.Query)
		if len(queryTokens) > 0 {
			for _, snapshot := range snapshots {
				if coversSet(queryTokens, snapshot.titleTokens) {
					titleOK = true
				}
				if coversSet(queryTokens, snapshot.descTokens) {
					descOK = true
				}
				if titleOK && descOK {
					break
				}
			}
		}
	}
	if !titleOK {
		o.MissingIn = append(o.MissingIn, "title")
	}
	if !descOK {
		o.MissingIn = append(o.MissingIn, "description")
	}
	return o, matched
}

func coversSet(tokens, have map[string]bool) bool {
	for token := range tokens {
		if !have[token] {
			return false
		}
	}
	return len(tokens) > 0
}

func newNovelOpportunitiesCmd(flags *rootFlags) *cobra.Command {
	var minPos, maxPos, impressions float64
	var capPages, limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "opportunities",
		Short:       "Find ranking queries missing from SEO titles and descriptions.",
		Example:     "  rankmath-pp-cli opportunities --min-impressions 100 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "opportunities")
		}
		if len(args) > 0 || minPos < 0 || maxPos < minPos || impressions < 0 || capPages < 1 || limit < 1 {
			return usageErr(fmt.Errorf("require valid position range, nonnegative impressions, and positive scan pages and limit"))
		}
		result := opportunitiesResult{PageLookup: "unavailable", Opportunities: make([]opportunity, 0)}
		emit := func() error {
			if len(result.Opportunities) == 0 && result.Note == "" {
				result.Note = fmt.Sprintf("No missing metadata matched position %g–%g and at least %g impressions within %d scanned keyword pages.", minPos, maxPos, impressions, result.ScannedKeywordPages)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(result.Opportunities) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				return nil
			}
			rows := make([][]string, 0)
			for _, r := range result.Opportunities {
				rows = append(rows, []string{r.Query, fmt.Sprint(r.Impressions), r.Page, strings.Join(r.MissingIn, ", ")})
			}
			return flags.printTable(cmd, []string{"Query", "Impressions", "Page", "Missing"}, rows)
		}
		db, found, err := openMirrorOrHint(cmd, flags, dbPath)
		if err != nil {
			return err
		}
		if !found {
			result.Note = "No local SEO snapshots available."
			return emit()
		}
		defer db.Close()
		if err := db.EnsureSEOSnapshots(cmd.Context()); err != nil {
			return err
		}
		inventory, err := loadPublishedInventory(db)
		if err != nil {
			return err
		}
		inventoryPages := make(map[string]bool, len(inventory))
		for _, item := range inventory {
			inventoryPages[pageKey(item.URL)] = true
		}
		latest, err := db.LatestSEOSnapshots(cmd.Context())
		if err != nil {
			return err
		}
		snapshots := make(map[string]opportunitySnapshot, len(latest))
		pathSnapshots := make(map[string]opportunitySnapshot, len(latest))
		for _, head := range latest {
			key := pageKey(head.URL)
			if inventoryPages[key] {
				snapshot := opportunitySnapshot{head: head, titleTokens: tokenSet(head.Title), descTokens: tokenSet(head.Description)}
				snapshots[key] = snapshot
				pathSnapshots[pathKey(head.URL)] = snapshot
			}
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		capPages = scanCap(capPages, 1)
		unavailable := false
		for p := 1; p <= capPages; p++ {
			raw, err := c.Get(ctx, "/rankmath/v1/an/keywordsRows", map[string]string{"page": strconv.Itoa(p), "perPage": "25", "orderBy": "impressions", "order": "desc"})
			if err != nil {
				return err
			}
			result.ScannedKeywordPages++
			rows, err := decodeKeywordRows(raw)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				break
			}
			result.KeywordsConsidered += len(rows)
			for _, r := range rows {
				if r.Position < minPos || r.Position > maxPos || r.Impressions < impressions {
					continue
				}
				pages := make([]keywordRow, 0)
				raw, e := c.Get(ctx, "/rankmath/v1/an/getKeywordPages", map[string]string{"query": r.Query})
				if e == nil {
					pages, e = decodeKeywordRows(raw)
				}
				if e != nil {
					unavailable = true
				} else {
					result.PageLookup = "available"
				}
				if len(pages) == 0 {
					pages = append(pages, keywordRow{})
				}
				for _, page := range pages {
					o, matchedSnapshot := buildOpportunity(r, page, snapshots, pathSnapshots)
					if o.MatchScope == "page" && !matchedSnapshot {
						result.UnmatchedPages++
						continue
					}
					if len(o.MissingIn) > 0 {
						result.Opportunities = append(result.Opportunities, o)
					}
				}
			}
		}
		if unavailable {
			result.PageLookup = "unavailable"
		}
		sort.SliceStable(result.Opportunities, func(i, j int) bool { return result.Opportunities[i].Impressions > result.Opportunities[j].Impressions })
		if len(result.Opportunities) > limit {
			result.Opportunities = result.Opportunities[:limit]
		}
		return emit()
	}
	cmd.Flags().Float64Var(&minPos, "min-position", 4, "Minimum ranking position")
	cmd.Flags().Float64Var(&maxPos, "max-position", 20, "Maximum ranking position")
	cmd.Flags().Float64Var(&impressions, "min-impressions", 50, "Minimum impressions")
	cmd.Flags().IntVar(&capPages, "max-scan-pages", 4, "Maximum keyword pages to scan")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum opportunities")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror database")
	return cmd
}
