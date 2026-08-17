// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/context-dev/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/context-dev/internal/cliutil"
	"github.com/spf13/cobra"
)

// sensitiveIdentifierLikeRE is shared by generic public entity-discovery validation.
// It is not tied to the removed doctor-discover workflow; it keeps free-form
// person-identifying input out of entity-discover's fielded name/location args.
var sensitiveIdentifierLikeRE = regexp.MustCompile(`\b\d{3}[- ]?\d{2}[- ]?\d{4}\b`)

type workflowProvenance struct {
	Step      string         `json:"step"`
	Method    string         `json:"method"`
	Endpoint  string         `json:"endpoint"`
	Input     map[string]any `json:"input,omitempty"`
	SourceURL string         `json:"source_url,omitempty"`
	Status    string         `json:"status"`
	Error     string         `json:"error,omitempty"`
}

type workflowEstimate struct {
	Command          string               `json:"command"`
	DryRun           bool                 `json:"dry_run,omitempty"`
	EstimatedCredits int                  `json:"estimated_credits"`
	PlannedRequests  []workflowProvenance `json:"planned_requests"`
	Warnings         []string             `json:"warnings,omitempty"`
}

type operatorWorkflow struct {
	flags *rootFlags
	cmd   *cobra.Command
	c     *client.Client
}

type brandBrief struct {
	Domain          string               `json:"domain"`
	Website         string               `json:"website,omitempty"`
	Title           string               `json:"title,omitempty"`
	Description     string               `json:"description,omitempty"`
	Logo            string               `json:"logo,omitempty"`
	Colors          any                  `json:"colors,omitempty"`
	Fonts           any                  `json:"fonts,omitempty"`
	Socials         map[string]any       `json:"socials,omitempty"`
	ContactSurfaces []string             `json:"contact_surfaces,omitempty"`
	Screenshot      any                  `json:"screenshot,omitempty"`
	Summary         string               `json:"summary,omitempty"`
	Provenance      []workflowProvenance `json:"provenance"`
}

type entityWorkflowCandidate struct {
	EntityType  string               `json:"entity_type"`
	Name        string               `json:"name,omitempty"`
	Description string               `json:"description,omitempty"`
	Location    string               `json:"location,omitempty"`
	Address     string               `json:"address,omitempty"`
	Website     string               `json:"website,omitempty"`
	Socials     map[string]any       `json:"socials,omitempty"`
	Logo        string               `json:"logo,omitempty"`
	SourceURL   string               `json:"source_url,omitempty"`
	Score       int                  `json:"score"`
	Provenance  []workflowProvenance `json:"provenance"`
}

type competitorMapOutput struct {
	Input       map[string]any                `json:"input"`
	Clusters    []competitorCluster           `json:"clusters"`
	Competitors []competitorWorkflowCandidate `json:"competitors"`
	Provenance  []workflowProvenance          `json:"provenance"`
}

type competitorCluster struct {
	Category string   `json:"category"`
	Market   string   `json:"market,omitempty"`
	Domains  []string `json:"domains"`
}

type competitorWorkflowCandidate struct {
	Rank           int                  `json:"rank"`
	Name           string               `json:"name,omitempty"`
	Domain         string               `json:"domain,omitempty"`
	Website        string               `json:"website,omitempty"`
	Description    string               `json:"description,omitempty"`
	Category       string               `json:"category,omitempty"`
	Market         string               `json:"market,omitempty"`
	WhyRanked      string               `json:"why_ranked"`
	OverlapSignals []string             `json:"overlap_signals"`
	Score          int                  `json:"score"`
	Provenance     []workflowProvenance `json:"provenance"`
}

type sourcePackOutput struct {
	Query      string               `json:"query"`
	Status     string               `json:"status"`
	Sources    []sourcePackSource   `json:"sources"`
	Claims     []sourcePackClaim    `json:"claims"`
	Markdown   string               `json:"markdown,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type sourcePackSource struct {
	Rank       int                  `json:"rank"`
	Title      string               `json:"title,omitempty"`
	URL        string               `json:"url"`
	Snippet    string               `json:"snippet,omitempty"`
	Summary    string               `json:"summary,omitempty"`
	Extracted  any                  `json:"extracted,omitempty"`
	Error      string               `json:"error,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type sourcePackClaim struct {
	Text       string               `json:"text"`
	SourceURL  string               `json:"source_url"`
	Provenance []workflowProvenance `json:"provenance"`
}

type crawlBudgetPlan struct {
	Seed               string   `json:"seed"`
	Domain             string   `json:"domain"`
	URLRegex           string   `json:"urlRegex"`
	MaxPages           int      `json:"max_pages"`
	EstimatedCredits   int      `json:"estimated_credits"`
	SameDomainOnly     bool     `json:"same_domain_only"`
	LikelyCoverage     string   `json:"likely_coverage"`
	RiskWarnings       []string `json:"risk_warnings"`
	RecommendedCommand string   `json:"recommended_command"`
}

type websiteChangeDigest struct {
	Domain                string               `json:"domain"`
	CurrentSnapshot       string               `json:"current_snapshot"`
	PreviousSnapshot      string               `json:"previous_snapshot,omitempty"`
	CurrentTimestamp      string               `json:"current_timestamp"`
	PreviousTimestamp     string               `json:"previous_timestamp,omitempty"`
	ChangedCopy           []string             `json:"changed_copy"`
	ChangedLinksFacts     []string             `json:"changed_links_facts"`
	ChangedVisualIdentity []string             `json:"changed_visual_identity"`
	ScreenshotReferences  []string             `json:"screenshot_references"`
	Provenance            []workflowProvenance `json:"provenance"`
}

type websiteSnapshot struct {
	Domain     string               `json:"domain"`
	Timestamp  string               `json:"timestamp"`
	Scrape     map[string]any       `json:"scrape,omitempty"`
	Styleguide map[string]any       `json:"styleguide,omitempty"`
	Screenshot map[string]any       `json:"screenshot,omitempty"`
	Summary    string               `json:"summary,omitempty"`
	Links      []string             `json:"links,omitempty"`
	Colors     []string             `json:"colors,omitempty"`
	Fonts      []string             `json:"fonts,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type schemaLabOutput struct {
	SchemaFile     string                    `json:"schema_file"`
	Instructions   string                    `json:"instructions,omitempty"`
	FieldFillRates map[string]schemaFillRate `json:"field_fill_rates"`
	ParseFailures  int                       `json:"parse_failures"`
	ExampleMisses  []string                  `json:"example_misses"`
	Results        []schemaLabResult         `json:"results"`
	Provenance     []workflowProvenance      `json:"provenance"`
}

type schemaFillRate struct {
	Filled int     `json:"filled"`
	Total  int     `json:"total"`
	Rate   float64 `json:"rate"`
}

type schemaLabResult struct {
	URL        string               `json:"url"`
	Status     string               `json:"status"`
	Filled     []string             `json:"filled_fields,omitempty"`
	Missing    []string             `json:"missing_fields,omitempty"`
	Error      string               `json:"error,omitempty"`
	Raw        any                  `json:"raw,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type assetPackOutput struct {
	Domain     string               `json:"domain"`
	Website    string               `json:"website,omitempty"`
	Title      string               `json:"title,omitempty"`
	Logo       string               `json:"logo,omitempty"`
	Palette    []string             `json:"palette,omitempty"`
	Fonts      []string             `json:"fonts,omitempty"`
	Styleguide any                  `json:"styleguide,omitempty"`
	Screenshot any                  `json:"screenshot,omitempty"`
	Favicon    string               `json:"favicon,omitempty"`
	Socials    map[string]any       `json:"socials,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type trustCheckOutput struct {
	Domain          string               `json:"domain"`
	RiskLevel       string               `json:"risk_level"`
	Signals         []string             `json:"signals"`
	Inconsistencies []string             `json:"inconsistencies"`
	MissingEvidence []string             `json:"missing_evidence"`
	Provenance      []workflowProvenance `json:"provenance"`
}

type brandQAOutput struct {
	Domain       string               `json:"domain"`
	Question     string               `json:"question"`
	Answer       string               `json:"answer,omitempty"`
	URLsAnalyzed []string             `json:"urls_analyzed,omitempty"`
	Provenance   []workflowProvenance `json:"provenance"`
}

type emailEnrichOutput struct {
	Email      string               `json:"email"`
	Domain     string               `json:"domain,omitempty"`
	Company    map[string]any       `json:"company,omitempty"`
	Prefill    map[string]any       `json:"prefill,omitempty"`
	Provenance []workflowProvenance `json:"provenance"`
}

type tickerEnrichOutput struct {
	Identifier     string               `json:"identifier"`
	IdentifierType string               `json:"identifier_type"`
	Domain         string               `json:"domain,omitempty"`
	Company        map[string]any       `json:"company,omitempty"`
	NAICS          any                  `json:"naics,omitempty"`
	SIC            any                  `json:"sic,omitempty"`
	Provenance     []workflowProvenance `json:"provenance"`
}

type leadBatchOutput struct {
	InputCSV string           `json:"input_csv"`
	Rows     []leadBatchRow   `json:"rows"`
	Summary  leadBatchSummary `json:"summary"`
}

type leadBatchSummary struct {
	Processed int `json:"processed"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type leadBatchRow struct {
	RowNumber     int                  `json:"row_number"`
	Success       bool                 `json:"success"`
	Domain        string               `json:"domain,omitempty"`
	Name          string               `json:"name,omitempty"`
	Location      string               `json:"location,omitempty"`
	Record        any                  `json:"record,omitempty"`
	Error         string               `json:"error,omitempty"`
	FailureReason string               `json:"failure_reason,omitempty"`
	Provenance    []workflowProvenance `json:"provenance"`
}

var publicWorkflowFieldRE = regexp.MustCompile(`^[\pL\pN][\pL\pN .,&'()/+-]{0,119}$`)

func newEntityDiscoverCmd(flags *rootFlags) *cobra.Command {
	var entityType, name, location, includeDomains string
	var maxCandidates int
	var estimate bool
	cmd := &cobra.Command{
		Use:   "entity-discover --type company|venue|provider|school|agency|other --name <name> --location <place>",
		Short: "Discover and rank public entity candidates with search, brand, and scrape enrichment",
		Args:  noArgsUnlessDryRun(flags, "entity-discover"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxCandidates < 1 {
				maxCandidates = 1
			}
			if err := validateEntityDiscoverInput(entityType, name, location); err != nil && !flags.dryRun {
				return usageErr(err)
			}
			query := strings.TrimSpace(name + " " + location + " " + entityType)
			plan := workflowEstimate{
				Command:          "entity-discover",
				DryRun:           flags.dryRun,
				EstimatedCredits: 1 + maxCandidates*11,
				PlannedRequests: []workflowProvenance{
					plannedPost("/web/search", map[string]any{"query": query, "includeDomains": splitCommaValues(includeDomains)}),
					plannedGet("/brand/retrieve", map[string]any{"per_candidate": true}),
					plannedGet("/web/scrape/markdown", map[string]any{"per_candidate": true}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			body := map[string]any{"query": query}
			if includeDomains != "" {
				body["includeDomains"] = splitCommaValues(includeDomains)
			}
			searchData, searchProv, err := w.post("/web/search", body, "search")
			if err != nil {
				return classifyAPIError(fmt.Errorf("entity-discover search failed: %w", err), flags)
			}
			results := extractWorkflowSearchResults(searchData)
			if len(results) == 0 {
				return writeJSONPayload(cmd, flags, []entityWorkflowCandidate{})
			}
			if maxCandidates > len(results) {
				maxCandidates = len(results)
			}
			candidates := make([]entityWorkflowCandidate, 0, maxCandidates)
			for i := 0; i < maxCandidates; i++ {
				candidate := entityCandidateFromSearch(results[i], entityType, name, location, searchProv)
				if domain := domainFromURL(results[i].URL); domain != "" {
					brand, prov := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand_enrichment")
					mergeEntityBrand(&candidate, brand, prov)
					scrape, scrapeProv := w.getSoft("/web/scrape/markdown", map[string]string{"url": candidateSourceURL(candidate)}, "scrape_enrichment")
					mergeEntityScrape(&candidate, scrape, scrapeProv)
				}
				candidates = append(candidates, candidate)
			}
			sort.SliceStable(candidates, func(i, j int) bool {
				return candidates[i].Score > candidates[j].Score
			})
			return writeJSONPayload(cmd, flags, candidates)
		},
	}
	cmd.Flags().StringVar(&entityType, "type", "", "Entity type: company, venue, provider, school, agency, or other")
	cmd.Flags().StringVar(&name, "name", "", "Public entity name")
	cmd.Flags().StringVar(&location, "location", "", "Public location field")
	cmd.Flags().StringVar(&includeDomains, "include-domains", "", "Optional comma-separated search domain allowlist")
	cmd.Flags().IntVar(&maxCandidates, "max-candidates", 5, "Maximum search candidates to enrich")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newBrandBriefCmd(flags *rootFlags) *cobra.Command {
	var estimate bool
	cmd := &cobra.Command{
		Use:     "brand-brief <domain|url>",
		Short:   "Build a normalized brand profile from brand, styleguide, screenshot, and scrape signals",
		Example: "  context-dev-pp-cli brand-brief example.com --json\n  context-dev-pp-cli brand-brief https://example.com --estimate",
		Args:    oneArgUnlessDryRun(flags, "brand-brief"),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				var err error
				domain, err = normalizeDomainArg(args[0])
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			plan := workflowEstimate{
				Command:          "brand-brief",
				DryRun:           flags.dryRun,
				EstimatedCredits: 26,
				PlannedRequests: []workflowProvenance{
					plannedGet("/brand/retrieve", map[string]any{"domain": domain}),
					plannedGet("/web/styleguide", map[string]any{"domain": domain}),
					plannedGet("/web/screenshot", map[string]any{"domain": domain}),
					plannedGet("/web/scrape/markdown", map[string]any{"url": websiteURL(domain)}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildBrandBrief(w, domain)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newCompetitorMapCmd(flags *rootFlags) *cobra.Command {
	var domain, query, market string
	var max int
	var estimate bool
	cmd := &cobra.Command{
		Use:   "competitor-map (--domain <domain|url> | --query <query>)",
		Short: "Find adjacent entities, enrich them, and cluster competitors by category and market",
		Args:  noArgsUnlessDryRun(flags, "competitor-map"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if max < 1 {
				max = 1
			}
			if domain == "" && query == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("either --domain or --query is required"))
			}
			if domain != "" {
				var err error
				domain, err = normalizeDomainArg(domain)
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			if query != "" {
				if err := validatePublicSearchQuery("query", query); err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			plan := workflowEstimate{
				Command:          "competitor-map",
				DryRun:           flags.dryRun,
				EstimatedCredits: 1 + max*10,
				PlannedRequests:  []workflowProvenance{plannedGet("/web/competitors", map[string]any{"domain": domain, "numCompetitors": max}), plannedPost("/web/search", map[string]any{"query": query}), plannedGet("/brand/retrieve", map[string]any{"per_competitor": true})},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			var results []searchResult
			var rootProv []workflowProvenance
			if domain != "" {
				data, prov, err := w.get("/web/competitors", map[string]string{"domain": domain, "numCompetitors": strconv.Itoa(max)}, "competitor_search")
				if err != nil {
					return classifyAPIError(fmt.Errorf("competitor-map failed: %w", err), flags)
				}
				rootProv = append(rootProv, prov...)
				results = extractCompetitorResults(data)
			} else {
				data, prov, err := w.post("/web/search", map[string]any{"query": query}, "competitor_search")
				if err != nil {
					return classifyAPIError(fmt.Errorf("competitor-map search failed: %w", err), flags)
				}
				rootProv = append(rootProv, prov...)
				results = extractWorkflowSearchResults(data)
			}
			out := buildCompetitorMap(w, results, domain, query, market, max, rootProv)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "Seed domain or URL")
	cmd.Flags().StringVar(&query, "query", "", "Search query when no seed domain is available")
	cmd.Flags().StringVar(&market, "market", "", "Optional market label to include in clustering")
	cmd.Flags().IntVar(&max, "max", 5, "Maximum competitors to return")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newCrawlBudgetPlanCmd(flags *rootFlags) *cobra.Command {
	var maxPages int
	cmd := &cobra.Command{
		Use:     "crawl-budget-plan <seed>",
		Short:   "Plan same-domain crawl scope and likely credit cost without calling crawl endpoints",
		Example: "  context-dev-pp-cli crawl-budget-plan https://example.com --max-pages 25 --json",
		Args:    oneArgUnlessDryRun(flags, "crawl-budget-plan"),
		RunE: func(cmd *cobra.Command, args []string) error {
			seed := ""
			if len(args) > 0 {
				seed = args[0]
			}
			if maxPages < 1 {
				maxPages = 1
			}
			parsed, err := normalizeSeedURL(seed)
			if err != nil && !flags.dryRun {
				return usageErr(err)
			}
			domain := ""
			regex := ""
			if parsed != nil {
				domain = parsed.Hostname()
				regex = exactHostRegex(domain)
			}
			warnings := crawlRiskWarnings(parsed, maxPages)
			out := crawlBudgetPlan{
				Seed:               seed,
				Domain:             domain,
				URLRegex:           regex,
				MaxPages:           maxPages,
				EstimatedCredits:   maxPages,
				SameDomainOnly:     true,
				LikelyCoverage:     likelyCoverage(maxPages),
				RiskWarnings:       warnings,
				RecommendedCommand: fmt.Sprintf("context-dev-pp-cli crawl %s --max-pages %d --estimate", shellQuote(seed), maxPages),
			}
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 25, "Maximum pages to plan for")
	return cmd
}

func newSourcePackCmd(flags *rootFlags) *cobra.Command {
	var query, schemaPath string
	var maxSources int
	var estimate bool
	cmd := &cobra.Command{
		Use:     "source-pack --query <query>",
		Short:   "Search, scrape, and optionally extract cited source packs",
		Example: "  context-dev-pp-cli source-pack --query \"Context.dev brand API\" --max-sources 5 --json\n  context-dev-pp-cli source-pack --query \"Context.dev brand API\" --schema schema.json --estimate",
		Args:    noArgsUnlessDryRun(flags, "source-pack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxSources < 1 {
				maxSources = 1
			}
			if err := validatePublicSearchQuery("query", query); err != nil && !flags.dryRun {
				return usageErr(err)
			}
			var schema any
			if schemaPath != "" && !flags.dryRun && !estimate {
				var err error
				schema, err = readJSONFile(schemaPath)
				if err != nil {
					return err
				}
			}
			credits := 1 + maxSources
			if schemaPath != "" {
				credits += maxSources * 10
			}
			plan := workflowEstimate{
				Command:          "source-pack",
				DryRun:           flags.dryRun,
				EstimatedCredits: credits,
				PlannedRequests: []workflowProvenance{
					plannedPost("/web/search", map[string]any{"query": query}),
					plannedGet("/web/scrape/markdown", map[string]any{"per_source": true}),
					plannedPost("/web/extract", map[string]any{"per_source": schemaPath != ""}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out, err := buildSourcePack(w, query, maxSources, schema)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().IntVar(&maxSources, "max-sources", 5, "Maximum sources to scrape")
	cmd.Flags().StringVar(&schemaPath, "schema", "", "Optional JSON Schema file for per-source extraction")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newWebsiteChangeDigestCmd(flags *rootFlags) *cobra.Command {
	var estimate bool
	cmd := &cobra.Command{
		Use:   "website-change-digest <domain|url>",
		Short: "Snapshot website scrape, styleguide, and screenshot signals and diff against local state",
		Args:  oneArgUnlessDryRun(flags, "website-change-digest"),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				var err error
				domain, err = normalizeDomainArg(args[0])
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			plan := workflowEstimate{
				Command:          "website-change-digest",
				DryRun:           flags.dryRun,
				EstimatedCredits: 16,
				PlannedRequests: []workflowProvenance{
					plannedGet("/web/scrape/markdown", map[string]any{"url": websiteURL(domain)}),
					plannedGet("/web/styleguide", map[string]any{"domain": domain}),
					plannedGet("/web/screenshot", map[string]any{"domain": domain}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out, err := buildWebsiteChangeDigest(w, domain)
			if err != nil {
				return err
			}
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newSchemaLabCmd(flags *rootFlags) *cobra.Command {
	var urls []string
	var schemaPath, instructions string
	var estimate bool
	cmd := &cobra.Command{
		Use:   "schema-lab --url <url> --schema <file.json>",
		Short: "Run an extraction schema across sample pages and report fill rates and failures",
		Args:  noArgsUnlessDryRun(flags, "schema-lab"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(urls) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("at least one --url is required"))
			}
			for _, u := range urls {
				if _, err := normalizeURLArg(u); err != nil && !flags.dryRun {
					return usageErr(fmt.Errorf("--url %q: %w", u, err))
				}
			}
			if schemaPath == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--schema is required"))
			}
			plan := workflowEstimate{
				Command:          "schema-lab",
				DryRun:           flags.dryRun,
				EstimatedCredits: len(urls) * 10,
				PlannedRequests:  []workflowProvenance{plannedPost("/web/extract", map[string]any{"per_url": true, "url_count": len(urls)})},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			schema, err := readJSONFile(schemaPath)
			if err != nil {
				return err
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildSchemaLab(w, urls, schemaPath, schema, instructions)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().StringArrayVar(&urls, "url", nil, "Sample URL to extract; repeat for multiple pages")
	cmd.Flags().StringVar(&schemaPath, "schema", "", "JSON Schema file")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Optional extraction instructions")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newBrandKitCmd(flags *rootFlags) *cobra.Command {
	var estimate bool
	cmd := &cobra.Command{
		Use:     "brand-kit <domain|url>",
		Aliases: []string{"asset-pack"},
		Short:   "Generate an on-demand brand kit: logo, palette, fonts, screenshot, favicon, and socials in one bundle",
		Example: "  context-dev-pp-cli brand-kit example.com --json\n  context-dev-pp-cli brand-kit https://example.com --estimate",
		Args:    oneArgUnlessDryRun(flags, "brand-kit"),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				var err error
				domain, err = normalizeDomainArg(args[0])
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			plan := workflowEstimate{
				Command:          "brand-kit",
				DryRun:           flags.dryRun,
				EstimatedCredits: 25,
				PlannedRequests: []workflowProvenance{
					plannedGet("/brand/retrieve", map[string]any{"domain": domain}),
					plannedGet("/web/styleguide", map[string]any{"domain": domain}),
					plannedGet("/web/screenshot", map[string]any{"domain": domain}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildAssetPack(w, domain)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newBrandQACmd(flags *rootFlags) *cobra.Command {
	var question string
	var estimate bool
	cmd := &cobra.Command{
		Use:     "brand-qa <domain|url>",
		Short:   "Ask a natural-language question about a brand's website and get a grounded answer",
		Example: "  context-dev-pp-cli brand-qa example.com --question \"What is the return policy?\" --json",
		Args:    oneArgUnlessDryRun(flags, "brand-qa"),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				var err error
				domain, err = normalizeDomainArg(args[0])
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			if strings.TrimSpace(question) == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--question is required"))
			}
			plan := workflowEstimate{
				Command:          "brand-qa",
				DryRun:           flags.dryRun,
				EstimatedCredits: 10,
				PlannedRequests:  []workflowProvenance{plannedPost("/brand/ai/query", brandQABody(domain, question))},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildBrandQA(w, domain, question)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&question, "question", "", "Natural-language question to answer from the brand's website")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newEmailEnrichCmd(flags *rootFlags) *cobra.Command {
	var estimate bool
	cmd := &cobra.Command{
		Use:     "email-enrich <email>",
		Short:   "Turn a work email into a company profile for signup-form autofill",
		Example: "  context-dev-pp-cli email-enrich founders@example.com --json",
		Args:    oneArgUnlessDryRun(flags, "email-enrich"),
		RunE: func(cmd *cobra.Command, args []string) error {
			email := ""
			if len(args) > 0 {
				email = strings.TrimSpace(args[0])
			}
			if email != "" && !strings.Contains(email, "@") && !flags.dryRun {
				return usageErr(fmt.Errorf("argument must be an email address"))
			}
			plan := workflowEstimate{
				Command:          "email-enrich",
				DryRun:           flags.dryRun,
				EstimatedCredits: 1,
				PlannedRequests:  []workflowProvenance{plannedGet("/brand/retrieve-by-email", map[string]any{"email": email})},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildEmailEnrich(w, email)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newTickerEnrichCmd(flags *rootFlags) *cobra.Command {
	var exchange string
	var estimate bool
	cmd := &cobra.Command{
		Use:     "ticker-enrich <ticker|isin>",
		Short:   "Resolve a public company from a stock ticker or ISIN to a brand profile with industry codes",
		Example: "  context-dev-pp-cli ticker-enrich AAPL --json\n  context-dev-pp-cli ticker-enrich US0378331005 --json",
		Args:    oneArgUnlessDryRun(flags, "ticker-enrich"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = strings.TrimSpace(args[0])
			}
			idType := identifierType(id)
			endpoint := "/brand/retrieve-by-ticker"
			if idType == "isin" {
				endpoint = "/brand/retrieve-by-isin"
			}
			plan := workflowEstimate{
				Command:          "ticker-enrich",
				DryRun:           flags.dryRun,
				EstimatedCredits: 21,
				PlannedRequests: []workflowProvenance{
					plannedGet(endpoint, map[string]any{idType: id}),
					plannedGet("/web/naics", map[string]any{"input": "<resolved domain>"}),
					plannedGet("/web/sic", map[string]any{"input": "<resolved domain>"}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildTickerEnrich(w, id, idType, exchange)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&exchange, "exchange", "", "Optional stock exchange for the ticker (defaults to NASDAQ)")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newTrustCheckCmd(flags *rootFlags) *cobra.Command {
	var estimate bool
	cmd := &cobra.Command{
		Use:   "trust-check <domain|url>",
		Short: "Compare website, brand, social, and web signals for consistency risk",
		Args:  oneArgUnlessDryRun(flags, "trust-check"),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := ""
			if len(args) > 0 {
				var err error
				domain, err = normalizeDomainArg(args[0])
				if err != nil && !flags.dryRun {
					return usageErr(err)
				}
			}
			plan := workflowEstimate{
				Command:          "trust-check",
				DryRun:           flags.dryRun,
				EstimatedCredits: 12,
				PlannedRequests: []workflowProvenance{
					plannedGet("/brand/retrieve", map[string]any{"domain": domain}),
					plannedPost("/web/search", map[string]any{"query": domain}),
					plannedGet("/web/screenshot", map[string]any{"domain": domain}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildTrustCheck(w, domain)
			return writeJSONPayload(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newLeadEnrichBatchCmd(flags *rootFlags) *cobra.Command {
	var domainColumn, nameColumn, locationColumn, outputPath string
	var maxRows int
	var resume, strict, estimate bool
	cmd := &cobra.Command{
		Use:     "lead-enrich-batch <csv>",
		Short:   "Enrich CSV rows of domains and names without letting one bad row stop the batch",
		Example: "  context-dev-pp-cli lead-enrich-batch leads.csv --domain-column domain --output enriched.json --json\n  context-dev-pp-cli lead-enrich-batch leads.csv --name-column company --location-column city --resume --output enriched.json --json",
		Args:    oneArgUnlessDryRun(flags, "lead-enrich-batch"),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := ""
			if len(args) > 0 {
				inputPath = args[0]
			}
			if domainColumn == "" && nameColumn == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--domain-column or --name-column is required"))
			}
			if resume && outputPath == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--resume requires --output: resumption skips rows already present in the output file, so without it every row is reprocessed and re-billed"))
			}
			plan := workflowEstimate{
				Command:          "lead-enrich-batch",
				DryRun:           flags.dryRun,
				EstimatedCredits: estimateLeadBatchCredits(maxRows, domainColumn, nameColumn),
				PlannedRequests: []workflowProvenance{
					plannedGet("/brand/retrieve", map[string]any{"per_domain_row": domainColumn != ""}),
					plannedPost("/web/search", map[string]any{"per_name_row_without_domain": nameColumn != ""}),
				},
			}
			if estimate || flags.dryRun {
				return writeJSONPayload(cmd, flags, plan)
			}
			rows, err := readCSVRecords(inputPath)
			if err != nil {
				return err
			}
			resumeRows := map[int]struct{}{}
			if resume && outputPath != "" {
				resumeRows = readCompletedLeadRows(outputPath)
			}
			w, err := newOperatorWorkflow(cmd, flags)
			if err != nil {
				return err
			}
			out := buildLeadBatch(w, inputPath, rows, domainColumn, nameColumn, locationColumn, maxRows, resumeRows, strict)
			if outputPath != "" {
				data, err := json.MarshalIndent(out, "", "  ")
				if err != nil {
					return err
				}
				if err := cliutil.AtomicWritePrivateFile(outputPath, append(data, '\n'), 0o600, 0o700); err != nil {
					return err
				}
			}
			if err := writeJSONPayload(cmd, flags, out); err != nil {
				return err
			}
			if strict && out.Summary.Failed > 0 {
				return partialFailureErr(fmt.Errorf("lead-enrich-batch failed %d row(s) in strict mode", out.Summary.Failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&domainColumn, "domain-column", "", "CSV column containing domains")
	cmd.Flags().StringVar(&nameColumn, "name-column", "", "CSV column containing names")
	cmd.Flags().StringVar(&locationColumn, "location-column", "", "CSV column containing locations")
	cmd.Flags().StringVar(&outputPath, "output", "", "Optional JSON output path")
	cmd.Flags().IntVar(&maxRows, "max-rows", 0, "Maximum data rows to process; 0 means all")
	cmd.Flags().BoolVar(&resume, "resume", false, "Skip row numbers already present in --output")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail the batch on the first row error")
	cmd.Flags().BoolVar(&estimate, "estimate", false, "Estimate credits without spending them")
	return cmd
}

func newOperatorWorkflow(cmd *cobra.Command, flags *rootFlags) (*operatorWorkflow, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return &operatorWorkflow{flags: flags, cmd: cmd, c: c}, nil
}

func (w *operatorWorkflow) get(path string, params map[string]string, step string) (json.RawMessage, []workflowProvenance, error) {
	data, err := w.c.GetNoCache(w.cmd.Context(), path, params)
	prov := workflowProvenance{Step: step, Method: "GET", Endpoint: path, Input: stringifyMap(params), Status: "ok"}
	if err != nil {
		prov.Status = "error"
		prov.Error = err.Error()
		return nil, []workflowProvenance{prov}, err
	}
	return data, []workflowProvenance{prov}, nil
}

func (w *operatorWorkflow) getSoft(path string, params map[string]string, step string) (map[string]any, []workflowProvenance) {
	data, prov, err := w.get(path, params, step)
	if err != nil {
		return nil, prov
	}
	obj := rawObject(data)
	return obj, prov
}

func (w *operatorWorkflow) post(path string, body map[string]any, step string) (json.RawMessage, []workflowProvenance, error) {
	data, _, err := w.c.PostWithParams(w.cmd.Context(), path, nil, body)
	prov := workflowProvenance{Step: step, Method: "POST", Endpoint: path, Input: body, Status: "ok"}
	if err != nil {
		prov.Status = "error"
		prov.Error = err.Error()
		return nil, []workflowProvenance{prov}, err
	}
	return data, []workflowProvenance{prov}, nil
}

func (w *operatorWorkflow) postSoft(path string, body map[string]any, step string) (map[string]any, []workflowProvenance) {
	data, prov, err := w.post(path, body, step)
	if err != nil {
		return nil, prov
	}
	return rawObject(data), prov
}

func plannedGet(endpoint string, input map[string]any) workflowProvenance {
	return workflowProvenance{Step: "planned", Method: "GET", Endpoint: endpoint, Input: cleanAnyMap(input), Status: "planned"}
}

func plannedPost(endpoint string, input map[string]any) workflowProvenance {
	return workflowProvenance{Step: "planned", Method: "POST", Endpoint: endpoint, Input: cleanAnyMap(input), Status: "planned"}
}

func noArgsUnlessDryRun(flags *rootFlags, name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if flags.dryRun {
			return nil
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("%s does not accept positional arguments", name))
		}
		return nil
	}
}

func oneArgUnlessDryRun(flags *rootFlags, name string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if flags.dryRun {
			return nil
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("%s requires exactly one argument", name))
		}
		return nil
	}
}

func validateEntityDiscoverInput(entityType, name, location string) error {
	switch entityType {
	case "company", "venue", "provider", "school", "agency", "other":
	default:
		return fmt.Errorf("--type must be one of company, venue, provider, school, agency, other")
	}
	if err := validatePublicWorkflowField("name", name); err != nil {
		return err
	}
	return validatePublicWorkflowField("location", location)
}

func validatePublicWorkflowField(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > 120 {
		return fmt.Errorf("%s must be 120 characters or fewer", label)
	}
	lower := strings.ToLower(value)
	blocked := []string{"patient", "dob", "date of birth", "mrn", "medical record", "ssn", "passport", "driver license", "social security", "private notes"}
	for _, term := range blocked {
		if strings.Contains(lower, term) {
			return fmt.Errorf("%s must not contain sensitive or person-identifying context", label)
		}
	}
	if sensitiveIdentifierLikeRE.MatchString(value) || strings.ContainsAny(value, "\n\r;{}[]") {
		return fmt.Errorf("%s must be a field value, not a free-form note", label)
	}
	if !publicWorkflowFieldRE.MatchString(value) || len(strings.Fields(value)) > 16 {
		return fmt.Errorf("%s must be a concise field value", label)
	}
	return nil
}

func validatePublicSearchQuery(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if len(value) > 240 {
		return fmt.Errorf("%s must be 240 characters or fewer", label)
	}
	lower := strings.ToLower(value)
	blocked := []string{"patient", "dob", "date of birth", "mrn", "medical record", "ssn", "passport", "driver license", "social security", "private notes"}
	for _, term := range blocked {
		if strings.Contains(lower, term) {
			return fmt.Errorf("%s must not contain sensitive or person-identifying context", label)
		}
	}
	if sensitiveIdentifierLikeRE.MatchString(value) || strings.ContainsAny(value, "\n\r;{}[]") {
		return fmt.Errorf("%s must be a public search query, not a free-form private note", label)
	}
	if len(strings.Fields(value)) > 32 {
		return fmt.Errorf("%s must be a concise public search query", label)
	}
	return nil
}

func extractWorkflowSearchResults(data json.RawMessage) []searchResult {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil
	}
	rows := firstArray(v, "results", "data", "items", "sources")
	results := make([]searchResult, 0, len(rows))
	for i, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		result := searchResult{
			Title:   firstString(obj, "title", "name", "company", "domain"),
			URL:     firstString(obj, "url", "link", "source_url", "website", "domain"),
			Snippet: firstString(obj, "snippet", "description", "summary", "markdown", "content"),
			Rank:    i + 1,
		}
		if result.URL == "" && strings.Contains(result.Title, ".") {
			result.URL = result.Title
		}
		if result.URL != "" && !strings.Contains(result.URL, "://") && strings.Contains(result.URL, ".") {
			result.URL = websiteURL(result.URL)
		}
		if result.URL == "" {
			continue
		}
		results = append(results, result)
	}
	return results
}

func extractCompetitorResults(data json.RawMessage) []searchResult {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil
	}
	rows := firstArray(v, "competitors", "results", "data", "items")
	results := make([]searchResult, 0, len(rows))
	for i, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		result := searchResult{
			Title:   firstString(obj, "name", "title", "domain"),
			URL:     firstString(obj, "website", "url", "domain", "source_url"),
			Snippet: firstString(obj, "description", "snippet", "summary", "why"),
			Rank:    i + 1,
		}
		if result.URL != "" && !strings.Contains(result.URL, "://") && strings.Contains(result.URL, ".") {
			result.URL = websiteURL(result.URL)
		}
		if result.URL != "" {
			results = append(results, result)
		}
	}
	return results
}

func entityCandidateFromSearch(result searchResult, entityType, name, location string, prov []workflowProvenance) entityWorkflowCandidate {
	score := 100 - result.Rank*5
	text := strings.ToLower(result.Title + " " + result.Snippet + " " + result.URL)
	for _, token := range strings.Fields(strings.ToLower(name)) {
		token = strings.Trim(token, ".,")
		if len(token) > 2 && strings.Contains(text, token) {
			score += 15
		}
	}
	for _, token := range strings.Fields(strings.ToLower(location)) {
		token = strings.Trim(token, ".,")
		if len(token) > 2 && strings.Contains(text, token) {
			score += 8
		}
	}
	p := append([]workflowProvenance{}, prov...)
	p = append(p, workflowProvenance{Step: "search_result", Method: "POST", Endpoint: "/web/search", SourceURL: result.URL, Status: "ok"})
	return entityWorkflowCandidate{
		EntityType:  entityType,
		Name:        firstNonEmpty(result.Title, name),
		Description: result.Snippet,
		Location:    location,
		Website:     result.URL,
		SourceURL:   result.URL,
		Score:       score,
		Provenance:  p,
	}
}

func mergeEntityBrand(candidate *entityWorkflowCandidate, brand map[string]any, prov []workflowProvenance) {
	candidate.Provenance = append(candidate.Provenance, prov...)
	if brand == nil {
		return
	}
	brand = brandData(brand)
	if name := firstString(brand, "title", "name", "legalName"); name != "" {
		candidate.Name = name
		candidate.Score += 20
	}
	if description := firstString(brand, "description", "summary", "tagline", "slogan"); description != "" {
		candidate.Description = description
	}
	if website := firstString(brand, "website", "url", "domain"); website != "" {
		candidate.Website = normalizeWebsiteString(website)
	}
	if address := brandAddress(brand); address != "" {
		candidate.Address = address
		candidate.Location = firstNonEmpty(candidate.Location, address)
		candidate.Score += 8
	}
	if socials := normalizeSocials(brand); len(socials) > 0 {
		candidate.Socials = socials
		candidate.Score += 5
	}
	if logo := firstLogo(brand); logo != "" {
		candidate.Logo = logo
		candidate.Score += 5
	}
}

func mergeEntityScrape(candidate *entityWorkflowCandidate, scrape map[string]any, prov []workflowProvenance) {
	candidate.Provenance = append(candidate.Provenance, prov...)
	if scrape == nil {
		return
	}
	summary := summarizeRawMap(scrape)
	if candidate.Description == "" && summary != "" {
		candidate.Description = summary
	}
	if summary != "" {
		candidate.Score += 3
	}
}

func candidateSourceURL(candidate entityWorkflowCandidate) string {
	if candidate.SourceURL != "" {
		return candidate.SourceURL
	}
	return candidate.Website
}

func buildBrandBrief(w *operatorWorkflow, domain string) brandBrief {
	brand, brandProv := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand")
	style, styleProv := w.getSoft("/web/styleguide", map[string]string{"domain": domain}, "styleguide")
	screenshot, screenshotProv := w.getSoft("/web/screenshot", map[string]string{"domain": domain}, "screenshot")
	scrape, scrapeProv := w.getSoft("/web/scrape/markdown", map[string]string{"url": websiteURL(domain)}, "scrape_summary")
	brand = brandData(brand)
	style = styleguideData(style)
	out := brandBrief{
		Domain:      domain,
		Website:     firstNonEmpty(firstString(brand, "website", "url"), websiteURL(domain)),
		Title:       firstString(brand, "title", "name", "legalName"),
		Description: firstString(brand, "description", "summary", "tagline", "slogan"),
		Logo:        firstLogo(brand),
		Socials:     normalizeSocials(brand),
		Colors:      firstNonNil(firstAny(style, "colors", "palette"), firstAny(brand, "colors", "palette")),
		Fonts:       firstNonNil(firstAny(style, "fonts", "typography"), firstAny(brand, "fonts")),
		Screenshot:  firstNonNil(firstAny(screenshot, "screenshot", "url", "image"), screenshot),
		Summary:     firstNonEmpty(firstString(brand, "description", "slogan", "summary", "tagline"), summarizeRawMap(scrape)),
		Provenance:  joinProvenance(brandProv, styleProv, screenshotProv, scrapeProv),
	}
	out.ContactSurfaces = contactSurfaces(brand, scrape)
	return out
}

func buildCompetitorMap(w *operatorWorkflow, results []searchResult, seedDomain, query, market string, max int, rootProv []workflowProvenance) competitorMapOutput {
	if len(results) > max {
		results = results[:max]
	}
	out := competitorMapOutput{
		Input:      cleanAnyMap(map[string]any{"domain": seedDomain, "query": query, "market": market, "max": max}),
		Provenance: rootProv,
	}
	clusterDomains := map[string][]string{}
	for i, result := range results {
		domain := domainFromURL(result.URL)
		brand, prov := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand_enrichment")
		brand = brandData(brand)
		category := firstNonEmpty(brandCategory(brand), "unknown")
		description := firstNonEmpty(firstString(brand, "description", "summary"), result.Snippet)
		name := firstNonEmpty(firstString(brand, "title", "name", "legalName"), result.Title, domain)
		signals := overlapSignals(seedDomain, query, market, result, brand)
		score := 100 - result.Rank*5 + len(signals)*8
		out.Competitors = append(out.Competitors, competitorWorkflowCandidate{
			Rank:           i + 1,
			Name:           name,
			Domain:         domain,
			Website:        firstNonEmpty(firstString(brand, "website", "url"), result.URL),
			Description:    description,
			Category:       category,
			Market:         market,
			WhyRanked:      whyRanked(result, signals),
			OverlapSignals: signals,
			Score:          score,
			Provenance:     append(append([]workflowProvenance{}, rootProv...), prov...),
		})
		clusterKey := category + "|" + market
		clusterDomains[clusterKey] = append(clusterDomains[clusterKey], domain)
	}
	sort.SliceStable(out.Competitors, func(i, j int) bool {
		return out.Competitors[i].Score > out.Competitors[j].Score
	})
	for i := range out.Competitors {
		out.Competitors[i].Rank = i + 1
	}
	keys := make([]string, 0, len(clusterDomains))
	for key := range clusterDomains {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		category, marketPart, _ := strings.Cut(key, "|")
		out.Clusters = append(out.Clusters, competitorCluster{Category: category, Market: marketPart, Domains: clusterDomains[key]})
	}
	return out
}

func buildSourcePack(w *operatorWorkflow, query string, maxSources int, schema any) (sourcePackOutput, error) {
	data, prov, err := w.post("/web/search", map[string]any{"query": query}, "search")
	if err != nil {
		return sourcePackOutput{}, fmt.Errorf("source-pack search failed: %w", err)
	}
	results := extractWorkflowSearchResults(data)
	out := sourcePackOutput{Query: query, Status: "ok", Provenance: prov}
	if len(results) == 0 {
		out.Status = "no_results"
		return out, nil
	}
	if len(results) > maxSources {
		results = results[:maxSources]
	}
	var markdown bytes.Buffer
	for i, result := range results {
		source := sourcePackSource{Rank: i + 1, Title: result.Title, URL: result.URL, Snippet: result.Snippet, Provenance: append([]workflowProvenance{}, prov...)}
		scrape, scrapeProv := w.getSoft("/web/scrape/markdown", map[string]string{"url": result.URL}, "scrape_source")
		source.Provenance = append(source.Provenance, scrapeProv...)
		if scrape == nil {
			source.Error = provenanceError(scrapeProv)
		} else {
			source.Summary = summarizeRawMap(scrape)
			fmt.Fprintf(&markdown, "### Source %d: %s\n%s\n\nSource: %s\n\n", i+1, firstNonEmpty(result.Title, result.URL), source.Summary, result.URL)
			if source.Summary != "" {
				out.Claims = append(out.Claims, sourcePackClaim{Text: source.Summary, SourceURL: result.URL, Provenance: scrapeProv})
			}
		}
		if schema != nil {
			extracted, extractProv := w.postSoft("/web/extract", map[string]any{"url": result.URL, "schema": schema}, "extract_source")
			source.Provenance = append(source.Provenance, extractProv...)
			if extracted != nil {
				source.Extracted = extractData(extracted)
			} else if source.Error == "" {
				source.Error = provenanceError(extractProv)
			}
		}
		out.Sources = append(out.Sources, source)
	}
	out.Markdown = strings.TrimSpace(markdown.String())
	return out, nil
}

func buildWebsiteChangeDigest(w *operatorWorkflow, domain string) (websiteChangeDigest, error) {
	snapshot := websiteSnapshot{Domain: domain, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	scrape, scrapeProv := w.getSoft("/web/scrape/markdown", map[string]string{"url": websiteURL(domain)}, "scrape")
	style, styleProv := w.getSoft("/web/styleguide", map[string]string{"domain": domain}, "styleguide")
	screenshot, screenshotProv := w.getSoft("/web/screenshot", map[string]string{"domain": domain}, "screenshot")
	style = styleguideData(style)
	snapshot.Scrape = scrape
	snapshot.Styleguide = style
	snapshot.Screenshot = screenshot
	snapshot.Summary = summarizeRawMap(scrape)
	snapshot.Links = extractLinksFromText(snapshot.Summary)
	snapshot.Colors = collectStrings(firstAny(style, "colors", "palette"))
	snapshot.Fonts = collectFontFamilies(style)
	snapshot.Provenance = joinProvenance(scrapeProv, styleProv, screenshotProv)

	dir, err := workflowSnapshotDir(domain)
	if err != nil {
		return websiteChangeDigest{}, err
	}
	previous, previousPath := readLatestSnapshot(dir)
	currentPath := filepath.Join(dir, strings.ReplaceAll(snapshot.Timestamp, ":", "-")+".json")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return websiteChangeDigest{}, err
	}
	if err := cliutil.AtomicWritePrivateFile(currentPath, append(data, '\n'), 0o600, 0o700); err != nil {
		return websiteChangeDigest{}, err
	}
	if err := cliutil.AtomicWritePrivateFile(filepath.Join(dir, "latest.json"), append(data, '\n'), 0o600, 0o700); err != nil {
		return websiteChangeDigest{}, err
	}
	out := websiteChangeDigest{
		Domain:               domain,
		CurrentSnapshot:      currentPath,
		PreviousSnapshot:     previousPath,
		CurrentTimestamp:     snapshot.Timestamp,
		ScreenshotReferences: screenshotReferences(screenshot),
		Provenance:           snapshot.Provenance,
	}
	if previous != nil {
		out.PreviousTimestamp = previous.Timestamp
		out.ChangedCopy = diffSummary(previous.Summary, snapshot.Summary)
		out.ChangedLinksFacts = diffStringSets("links", previous.Links, snapshot.Links)
		out.ChangedVisualIdentity = append(diffStringSets("colors", previous.Colors, snapshot.Colors), diffStringSets("fonts", previous.Fonts, snapshot.Fonts)...)
	}
	return out, nil
}

func buildSchemaLab(w *operatorWorkflow, urls []string, schemaPath string, schema any, instructions string) schemaLabOutput {
	fields := schemaFieldNames(schema)
	out := schemaLabOutput{
		SchemaFile:     schemaPath,
		Instructions:   instructions,
		FieldFillRates: map[string]schemaFillRate{},
		ExampleMisses:  []string{},
		Results:        []schemaLabResult{},
	}
	counts := map[string]int{}
	for _, field := range fields {
		out.FieldFillRates[field] = schemaFillRate{Total: len(urls)}
	}
	for _, u := range urls {
		body := map[string]any{"url": u, "schema": schema}
		if instructions != "" {
			body["instructions"] = instructions
		}
		raw, prov := w.postSoft("/web/extract", body, "extract")
		result := schemaLabResult{URL: u, Status: "ok", Provenance: prov}
		if raw == nil {
			result.Status = "error"
			result.Error = provenanceError(prov)
			out.ParseFailures++
			out.Results = append(out.Results, result)
			continue
		}
		result.Raw = raw
		data := extractData(raw)
		for _, field := range fields {
			if hasFilledField(data, field) {
				result.Filled = append(result.Filled, field)
				counts[field]++
			} else {
				result.Missing = append(result.Missing, field)
			}
		}
		if len(result.Missing) > 0 {
			out.ExampleMisses = append(out.ExampleMisses, fmt.Sprintf("%s missing %s", u, strings.Join(result.Missing, ",")))
		}
		out.Results = append(out.Results, result)
		out.Provenance = append(out.Provenance, prov...)
	}
	for _, field := range fields {
		filled := counts[field]
		total := len(urls)
		rate := 0.0
		if total > 0 {
			rate = float64(filled) / float64(total)
		}
		out.FieldFillRates[field] = schemaFillRate{Filled: filled, Total: total, Rate: rate}
	}
	return out
}

func buildAssetPack(w *operatorWorkflow, domain string) assetPackOutput {
	brand, brandProv := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand")
	style, styleProv := w.getSoft("/web/styleguide", map[string]string{"domain": domain}, "styleguide")
	screenshot, screenshotProv := w.getSoft("/web/screenshot", map[string]string{"domain": domain}, "screenshot")
	brand = brandData(brand)
	style = styleguideData(style)
	return assetPackOutput{
		Domain:     domain,
		Website:    firstNonEmpty(firstString(brand, "website", "url"), websiteURL(domain)),
		Title:      firstString(brand, "title", "name", "legalName"),
		Logo:       firstLogo(brand),
		Palette:    collectStrings(firstNonNil(firstAny(style, "colors", "palette"), firstAny(brand, "colors", "palette"))),
		Fonts:      collectFontFamilies(style),
		Styleguide: style,
		Screenshot: firstNonNil(firstAny(screenshot, "screenshot", "url", "image"), screenshot),
		Favicon:    firstNonEmpty(firstString(brand, "favicon", "icon"), websiteURL(domain)+"/favicon.ico"),
		Socials:    normalizeSocials(brand),
		Provenance: joinProvenance(brandProv, styleProv, screenshotProv),
	}
}

// brandQABody is the single source of truth for the /brand/ai/query request body
// so the --dry-run/--estimate plan and the live call never diverge. The API
// requires each data_to_extract item to be an object with a non-empty
// datapoint_example, so a free-form --question is wrapped as one text datapoint.
func brandQABody(domain, question string) map[string]any {
	return map[string]any{
		"domain": domain,
		"data_to_extract": []map[string]any{{
			"datapoint_name":        "answer",
			"datapoint_type":        "text",
			"datapoint_description": question,
			"datapoint_example":     "A concise answer drawn from the website content.",
		}},
	}
}

func buildBrandQA(w *operatorWorkflow, domain, question string) brandQAOutput {
	resp, prov := w.postSoft("/brand/ai/query", brandQABody(domain, question), "query")
	out := brandQAOutput{Domain: domain, Question: question, Provenance: prov}
	data := extractData(resp)
	if extracted, ok := data["data_extracted"].([]any); ok {
		for _, item := range extracted {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if v := firstString(obj, "datapoint_value", "value"); v != "" {
				out.Answer = v
				break
			}
		}
	}
	if urls, ok := data["urls_analyzed"].([]any); ok {
		for _, u := range urls {
			if s, ok := u.(string); ok && strings.TrimSpace(s) != "" {
				out.URLsAnalyzed = append(out.URLsAnalyzed, s)
			}
		}
	}
	return out
}

func buildEmailEnrich(w *operatorWorkflow, email string) emailEnrichOutput {
	brand, prov := w.getSoft("/brand/retrieve-by-email", map[string]string{"email": email}, "brand")
	brand = brandData(brand)
	out := emailEnrichOutput{Email: email, Domain: firstString(brand, "domain"), Provenance: prov}
	if len(brand) == 0 {
		return out
	}
	company := map[string]any{}
	if v := firstString(brand, "title", "name", "legalName"); v != "" {
		company["name"] = v
	}
	if v := firstString(brand, "domain"); v != "" {
		company["domain"] = v
	}
	if v := firstLogo(brand); v != "" {
		company["logo"] = v
	}
	if v := firstString(brand, "description", "slogan"); v != "" {
		company["description"] = v
	}
	if v := brandCategory(brand); v != "" {
		company["category"] = v
	}
	if len(company) > 0 {
		out.Company = company
	}
	prefill := map[string]any{}
	if v := firstString(brand, "title", "name"); v != "" {
		prefill["company_name"] = v
	}
	if d := firstString(brand, "domain"); d != "" {
		prefill["website"] = websiteURL(d)
	}
	if v := brandCategory(brand); v != "" {
		prefill["industry"] = v
	}
	if len(prefill) > 0 {
		out.Prefill = prefill
	}
	return out
}

func buildTickerEnrich(w *operatorWorkflow, id, idType, exchange string) tickerEnrichOutput {
	out := tickerEnrichOutput{Identifier: id, IdentifierType: idType}
	endpoint := "/brand/retrieve-by-ticker"
	params := map[string]string{"ticker": id}
	if idType == "isin" {
		endpoint = "/brand/retrieve-by-isin"
		params = map[string]string{"isin": id}
	} else if exchange != "" {
		params["ticker_exchange"] = exchange
	}
	brand, prov := w.getSoft(endpoint, params, "brand")
	out.Provenance = append(out.Provenance, prov...)
	brand = brandData(brand)
	domain := firstString(brand, "domain")
	out.Domain = domain
	if len(brand) > 0 {
		company := map[string]any{}
		if v := firstString(brand, "title", "name", "legalName"); v != "" {
			company["name"] = v
		}
		if domain != "" {
			company["domain"] = domain
		}
		if v := firstLogo(brand); v != "" {
			company["logo"] = v
		}
		if v := firstString(brand, "description", "slogan"); v != "" {
			company["description"] = v
		}
		if len(company) > 0 {
			out.Company = company
		}
	}
	if domain != "" {
		naics, naicsProv := w.getSoft("/web/naics", map[string]string{"input": domain}, "naics")
		out.Provenance = append(out.Provenance, naicsProv...)
		out.NAICS = firstAny(naics, "codes")
		sic, sicProv := w.getSoft("/web/sic", map[string]string{"input": domain}, "sic")
		out.Provenance = append(out.Provenance, sicProv...)
		out.SIC = firstAny(sic, "codes")
	}
	return out
}

// identifierType classifies a security identifier as an ISIN (12 chars:
// 2-letter country code + 9 alphanumeric + 1 check digit) or otherwise a ticker.
func identifierType(id string) string {
	if isinRE.MatchString(strings.ToUpper(strings.TrimSpace(id))) {
		return "isin"
	}
	return "ticker"
}

var isinRE = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{9}[0-9]$`)

func buildTrustCheck(w *operatorWorkflow, domain string) trustCheckOutput {
	brand, brandProv := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand")
	search, searchProv, searchErr := w.post("/web/search", map[string]any{"query": domain}, "web_signals")
	screenshot, screenshotProv := w.getSoft("/web/screenshot", map[string]string{"domain": domain}, "screenshot")
	brand = brandData(brand)
	out := trustCheckOutput{Domain: domain, Provenance: joinProvenance(brandProv, screenshotProv)}
	if searchErr != nil {
		out.Provenance = append(out.Provenance, searchProv...)
		out.MissingEvidence = append(out.MissingEvidence, "web search signals unavailable")
	} else {
		out.Provenance = append(out.Provenance, searchProv...)
		if len(extractWorkflowSearchResults(search)) > 0 {
			out.Signals = append(out.Signals, "domain appears in web search results")
		} else {
			out.MissingEvidence = append(out.MissingEvidence, "no web search results returned for domain")
		}
	}
	title := firstString(brand, "title", "name", "legalName")
	website := normalizeWebsiteString(firstString(brand, "website", "url", "domain"))
	if title != "" {
		out.Signals = append(out.Signals, "brand title present")
	} else {
		out.MissingEvidence = append(out.MissingEvidence, "brand title missing")
	}
	if website != "" && domainFromURL(website) != "" && domainFromURL(website) != domain {
		out.Inconsistencies = append(out.Inconsistencies, fmt.Sprintf("brand website domain %q differs from input %q", domainFromURL(website), domain))
	} else if website != "" {
		out.Signals = append(out.Signals, "brand website matches input domain")
	} else {
		out.MissingEvidence = append(out.MissingEvidence, "brand website missing")
	}
	if firstLogo(brand) != "" {
		out.Signals = append(out.Signals, "logo present")
	} else {
		out.MissingEvidence = append(out.MissingEvidence, "logo missing")
	}
	if len(normalizeSocials(brand)) > 0 {
		out.Signals = append(out.Signals, "social profiles present")
	} else {
		out.MissingEvidence = append(out.MissingEvidence, "social profiles missing")
	}
	if brandAddress(brand) != "" {
		out.Signals = append(out.Signals, "address present")
	}
	for _, key := range []string{"phone", "phoneNumber"} {
		if firstString(brand, key) != "" {
			out.Signals = append(out.Signals, key+" present")
		}
	}
	if screenshot == nil {
		out.MissingEvidence = append(out.MissingEvidence, "screenshot unavailable")
	}
	score := len(out.Inconsistencies)*2 + len(out.MissingEvidence)
	switch {
	case score >= 5:
		out.RiskLevel = "high"
	case score >= 2:
		out.RiskLevel = "medium"
	default:
		out.RiskLevel = "low"
	}
	return out
}

func buildLeadBatch(w *operatorWorkflow, inputPath string, rows []map[string]string, domainColumn, nameColumn, locationColumn string, maxRows int, resumeRows map[int]struct{}, strict bool) leadBatchOutput {
	out := leadBatchOutput{InputCSV: inputPath}
	processed := 0
	for i, row := range rows {
		rowNumber := i + 2
		if maxRows > 0 && processed >= maxRows {
			break
		}
		if _, ok := resumeRows[rowNumber]; ok {
			out.Rows = append(out.Rows, leadBatchRow{RowNumber: rowNumber, Success: true, FailureReason: "skipped_resume"})
			out.Summary.Skipped++
			continue
		}
		processed++
		batchRow := leadBatchRow{RowNumber: rowNumber, Domain: strings.TrimSpace(row[domainColumn]), Name: strings.TrimSpace(row[nameColumn]), Location: strings.TrimSpace(row[locationColumn])}
		if batchRow.Domain != "" {
			domain, err := normalizeDomainArg(batchRow.Domain)
			if err != nil {
				batchRow.Error = err.Error()
				batchRow.FailureReason = "invalid_domain"
			} else {
				batchRow.Domain = domain
				brand, prov := w.getSoft("/brand/retrieve", map[string]string{"domain": domain}, "brand_enrichment")
				batchRow.Provenance = append(batchRow.Provenance, prov...)
				if brand == nil {
					batchRow.Error = provenanceError(prov)
					batchRow.FailureReason = "brand_enrichment_failed"
				} else {
					batchRow.Record = brandData(brand)
					batchRow.Success = true
				}
			}
		} else if batchRow.Name != "" {
			query := strings.TrimSpace(batchRow.Name + " " + batchRow.Location)
			data, prov, err := w.post("/web/search", map[string]any{"query": query}, "name_search")
			batchRow.Provenance = append(batchRow.Provenance, prov...)
			if err != nil {
				batchRow.Error = err.Error()
				batchRow.FailureReason = "search_failed"
			} else if results := extractWorkflowSearchResults(data); len(results) > 0 {
				batchRow.Record = results[0]
				batchRow.Success = true
			} else {
				batchRow.Error = "zero results"
				batchRow.FailureReason = "zero_results"
			}
		} else {
			batchRow.Error = "missing domain and name"
			batchRow.FailureReason = "missing_input"
		}
		if batchRow.Success {
			out.Summary.Succeeded++
		} else {
			out.Summary.Failed++
			if strict {
				out.Rows = append(out.Rows, batchRow)
				out.Summary.Processed = processed
				return out
			}
		}
		out.Rows = append(out.Rows, batchRow)
	}
	out.Summary.Processed = processed
	return out
}

func readCSVRecords(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(all) == 0 {
		return nil, nil
	}
	headers := all[0]
	rows := make([]map[string]string, 0, len(all)-1)
	for _, values := range all[1:] {
		row := map[string]string{}
		for i, header := range headers {
			if i < len(values) {
				row[strings.TrimSpace(header)] = strings.TrimSpace(values[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readCompletedLeadRows(path string) map[int]struct{} {
	out := map[int]struct{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var prior leadBatchOutput
	if err := json.Unmarshal(data, &prior); err != nil {
		return out
	}
	for _, row := range prior.Rows {
		if row.Success {
			out[row.RowNumber] = struct{}{}
		}
	}
	return out
}

func estimateLeadBatchCredits(maxRows int, domainColumn, nameColumn string) int {
	if maxRows <= 0 {
		maxRows = 1
	}
	if domainColumn != "" {
		return maxRows * 10
	}
	if nameColumn != "" {
		return maxRows
	}
	return 0
}

func rawObject(data json.RawMessage) map[string]any {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		return obj
	}
	var arr []any
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		if first, ok := arr[0].(map[string]any); ok {
			return first
		}
	}
	return nil
}

// brandData unwraps the Context.dev /brand/retrieve envelope. The live API
// returns {"brand": {...}, "code", "status"}; every consumer wants the inner
// brand object. Falls back to the map itself when already unwrapped (flat
// fixtures or a future flat response), so both shapes keep working.
func brandData(m map[string]any) map[string]any {
	if inner, ok := m["brand"].(map[string]any); ok {
		return inner
	}
	return m
}

// styleguideData unwraps the /web/styleguide envelope ({"styleguide": {...}}),
// falling back to the map itself when the palette/typography live at the top.
func styleguideData(m map[string]any) map[string]any {
	if inner, ok := m["styleguide"].(map[string]any); ok {
		return inner
	}
	return m
}

// extractData unwraps the /web/extract envelope. The live API returns the
// schema-shaped result under "data" ({"data": {...}, "status", "metadata"});
// callers want the inner object. Falls back to the map itself when already flat.
func extractData(m map[string]any) map[string]any {
	if inner, ok := m["data"].(map[string]any); ok {
		return inner
	}
	return m
}

// normalizeSocials returns brand social links as a {platform: url} map. The live
// API returns socials as a [{"type","url"}] array; older/flat shapes use a map.
func normalizeSocials(brand map[string]any) map[string]any {
	raw, ok := brand["socials"]
	if !ok || raw == nil {
		return nil
	}
	if m, ok := raw.(map[string]any); ok {
		if len(m) == 0 {
			return nil
		}
		return m
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		platform := firstString(obj, "type", "platform", "name", "network")
		link := firstString(obj, "url", "link", "href")
		if platform == "" || link == "" {
			continue
		}
		out[platform] = link
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// brandCategory resolves a single category label. The live API has no flat
// category/industry scalar; it nests classifications under
// industries.eic[].{industry,subindustry}.
func brandCategory(brand map[string]any) string {
	if c := firstString(brand, "category", "industry", "type", "sector"); c != "" {
		return c
	}
	industries, ok := brand["industries"].(map[string]any)
	if !ok {
		return ""
	}
	eic, ok := industries["eic"].([]any)
	if !ok {
		return ""
	}
	for _, item := range eic {
		if obj, ok := item.(map[string]any); ok {
			if v := firstString(obj, "industry", "subindustry"); v != "" {
				return v
			}
		}
	}
	return ""
}

// brandAddress formats the brand address, which the live API returns as a
// structured object ({city, state_province, country, ...}) rather than a string.
func brandAddress(brand map[string]any) string {
	if s := firstString(brand, "address", "fullAddress", "location"); s != "" {
		return s
	}
	addr, ok := brand["address"].(map[string]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, key := range []string{"street", "address", "city", "state_province", "state", "postal_code", "country"} {
		if v := firstString(addr, key); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

func stringifyMap(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func cleanAnyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		switch typed := v.(type) {
		case string:
			if typed != "" {
				out[k] = typed
			}
		case []string:
			if len(typed) > 0 {
				out[k] = typed
			}
		default:
			if v != nil {
				out[k] = v
			}
		}
	}
	return out
}

func joinProvenance(groups ...[]workflowProvenance) []workflowProvenance {
	var out []workflowProvenance
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func provenanceError(prov []workflowProvenance) string {
	for _, p := range prov {
		if p.Error != "" {
			return p.Error
		}
	}
	return ""
}

func firstAny(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if obj != nil {
			if v, ok := obj[key]; ok && v != nil {
				return v
			}
		}
	}
	return nil
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeWebsiteString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		return value
	}
	if strings.Contains(value, ".") {
		return websiteURL(value)
	}
	return value
}

func websiteURL(domain string) string {
	domain = strings.TrimPrefix(strings.TrimSpace(domain), "www.")
	if domain == "" {
		return ""
	}
	return "https://" + domain
}

func summarizeRawMap(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	text := firstString(obj, "summary", "markdown", "content", "text", "description")
	if text == "" {
		if nested, ok := obj["data"].(map[string]any); ok {
			text = summarizeRawMap(nested)
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 700 {
		return text[:700]
	}
	return text
}

func contactSurfaces(brand, scrape map[string]any) []string {
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		seen[value] = struct{}{}
	}
	for _, key := range []string{"email", "phone", "phoneNumber", "contactUrl"} {
		add(firstString(brand, key))
	}
	add(brandAddress(brand))
	for _, value := range normalizeSocials(brand) {
		if s, ok := value.(string); ok {
			add(s)
		}
	}
	text := summarizeRawMap(scrape)
	for _, link := range extractLinksFromText(text) {
		if strings.Contains(strings.ToLower(link), "contact") {
			add(link)
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func collectStrings(v any) []string {
	seen := map[string]struct{}{}
	var walk func(any)
	walk = func(item any) {
		switch typed := item.(type) {
		case string:
			for _, token := range strings.FieldsFunc(typed, func(r rune) bool { return r == ',' || r == '\n' || r == '\t' }) {
				token = strings.TrimSpace(token)
				if token != "" && len(token) <= 80 {
					seen[token] = struct{}{}
				}
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		case map[string]any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(v)
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// collectFontFamilies extracts font family names from a styleguide, pulling
// fontFamily values and fontFallbacks entries out of the typography tree while
// ignoring the size/weight/spacing values (e.g. "16px", "0px") that
// collectStrings would otherwise mix into the font list. Falls back to a flat
// "fonts" list when the styleguide provides one.
func collectFontFamilies(style map[string]any) []string {
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && len(s) <= 80 {
			seen[s] = struct{}{}
		}
	}
	var walk func(any)
	walk = func(item any) {
		switch typed := item.(type) {
		case map[string]any:
			for key, value := range typed {
				switch key {
				case "fontFamily":
					if s, ok := value.(string); ok {
						add(s)
					}
				case "fontFallbacks":
					for _, f := range collectStrings(value) {
						add(f)
					}
				default:
					walk(value)
				}
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(firstAny(style, "typography"))
	for _, f := range collectStrings(firstAny(style, "fonts")) {
		add(f)
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func overlapSignals(seedDomain, query, market string, result searchResult, brand map[string]any) []string {
	var signals []string
	if seedDomain != "" && strings.EqualFold(domainFromURL(result.URL), normalizeDomainForCompare(seedDomain)) {
		signals = append(signals, "same registered domain as seed")
	}
	if query != "" && searchResultMatchesQuery(result, brand, query) {
		signals = append(signals, "result text matches search query")
	}
	if market != "" && strings.Contains(strings.ToLower(strings.Join([]string{result.Title, result.Snippet, firstString(brand, "description", "summary"), brandCategory(brand)}, " ")), strings.ToLower(market)) {
		signals = append(signals, "market: "+market)
	}
	if brandCategory(brand) != "" {
		signals = append(signals, "brand category available")
	}
	if result.Snippet != "" {
		signals = append(signals, "search snippet overlap")
	}
	return signals
}

func normalizeDomainForCompare(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "www.")
}

func searchResultMatchesQuery(result searchResult, brand map[string]any, query string) bool {
	haystack := strings.ToLower(strings.Join([]string{
		result.Title,
		result.URL,
		result.Snippet,
		firstString(brand, "title", "name", "legalName", "description", "summary"),
		brandCategory(brand),
	}, " "))
	for _, term := range strings.Fields(strings.ToLower(query)) {
		term = strings.Trim(term, "'\".,!?()[]{}#%")
		if len(term) >= 3 && strings.Contains(haystack, term) {
			return true
		}
	}
	return false
}

func whyRanked(result searchResult, signals []string) string {
	if len(signals) == 0 {
		return fmt.Sprintf("ranked from source result %d", result.Rank)
	}
	return fmt.Sprintf("ranked from source result %d with %s", result.Rank, strings.Join(signals, "; "))
}

func normalizeSeedURL(seed string) (*url.URL, error) {
	if strings.TrimSpace(seed) == "" {
		return nil, fmt.Errorf("seed is required")
	}
	if strings.Contains(seed, "://") {
		return normalizeURLArg(seed)
	}
	domain, err := normalizeDomainArg(seed)
	if err != nil {
		return nil, err
	}
	return url.Parse(websiteURL(domain))
}

func crawlRiskWarnings(parsed *url.URL, maxPages int) []string {
	var warnings []string
	if parsed == nil {
		return warnings
	}
	if parsed.RawQuery != "" {
		warnings = append(warnings, "seed URL contains a query string; crawl planning will still constrain by host")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		warnings = append(warnings, "seed URL starts below the homepage; important sibling pages may be missed")
	}
	if maxPages > 25 {
		warnings = append(warnings, "planned page count is above the interactive crawl confirmation threshold")
	}
	return warnings
}

func likelyCoverage(maxPages int) string {
	switch {
	case maxPages <= 5:
		return "homepage plus a few high-priority linked pages"
	case maxPages <= 25:
		return "small to medium marketing site coverage"
	default:
		return "broad crawl; review urlRegex and estimate before spending credits"
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.ContainsAny(s, " \t\n'\"") {
		return strconv.Quote(s)
	}
	return s
}

func extractLinksFromText(text string) []string {
	linkRE := regexp.MustCompile(`https?://[^\s\])>"']+`)
	matches := linkRE.FindAllString(text, -1)
	seen := map[string]struct{}{}
	for _, match := range matches {
		seen[match] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for link := range seen {
		out = append(out, link)
	}
	sort.Strings(out)
	return out
}

func workflowSnapshotDir(domain string) (string, error) {
	base, err := cliutil.StateDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(domain))
	return filepath.Join(base, "website-change-digest", domain+"-"+hex.EncodeToString(sum[:4])), nil
}

func readLatestSnapshot(dir string) (*websiteSnapshot, string) {
	path := filepath.Join(dir, "latest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ""
	}
	var snapshot websiteSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, ""
	}
	return &snapshot, path
}

func diffSummary(previous, current string) []string {
	if previous == current {
		return nil
	}
	return []string{"page summary changed"}
}

func diffStringSets(label string, previous, current []string) []string {
	prev := map[string]struct{}{}
	for _, item := range previous {
		prev[item] = struct{}{}
	}
	cur := map[string]struct{}{}
	for _, item := range current {
		cur[item] = struct{}{}
	}
	var changes []string
	for item := range cur {
		if _, ok := prev[item]; !ok {
			changes = append(changes, label+" added: "+item)
		}
	}
	for item := range prev {
		if _, ok := cur[item]; !ok {
			changes = append(changes, label+" removed: "+item)
		}
	}
	sort.Strings(changes)
	return changes
}

func screenshotReferences(screenshot map[string]any) []string {
	var refs []string
	for _, key := range []string{"url", "screenshot", "image", "src"} {
		if value := firstString(screenshot, key); value != "" {
			refs = append(refs, value)
		}
	}
	return refs
}

func schemaFieldNames(schema any) []string {
	obj, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	props, ok := obj["properties"].(map[string]any)
	if !ok {
		return nil
	}
	fields := make([]string, 0, len(props))
	for key := range props {
		fields = append(fields, key)
	}
	sort.Strings(fields)
	return fields
}

func hasFilledField(obj map[string]any, field string) bool {
	v, ok := obj[field]
	if !ok {
		if nested, ok := obj["data"].(map[string]any); ok {
			v, ok = nested[field]
		}
	}
	if !ok || v == nil {
		return false
	}
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
