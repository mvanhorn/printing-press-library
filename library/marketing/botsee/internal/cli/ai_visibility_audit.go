// Copyright 2026 grahac. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// newAIVisibilityAuditCmd builds the flagship `ai-visibility-audit <url>` command.
// Idempotent: a second run on the same domain reuses the existing site and
// structure (unless --regenerate is passed) and only spends credits on a fresh
// analysis. Mirrors the Python skill's cmd_create_site + cmd_analyze chain at
// ~/.claude/skills/botsee/scripts/botsee.py.
func newAIVisibilityAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai-visibility-audit <url-or-domain>",
		Short: "Run a full AI visibility audit: bootstrap site + structure if needed, then run analysis.",
		Long: `Run a full AI visibility audit for a domain.

On first run for a domain: creates the site, generates customer types,
personas, and questions, then runs analysis across the selected LLMs and
prints a structured visibility report.

On subsequent runs for the same domain: reuses the existing site and
structure (idempotent by normalized-domain match against GET /sites) and
just runs a fresh analysis — no duplicate site creation.

Pass --reuse-latest to print the most recent completed analysis without
spending credits.`,
		Example: "  botsee-pp-cli ai-visibility-audit example.com --watch\n" +
			"  botsee-pp-cli ai-visibility-audit https://example.com --types 2 --personas 2 --questions 5 --watch\n" +
			"  botsee-pp-cli ai-visibility-audit example.com --estimate-only\n" +
			"  botsee-pp-cli ai-visibility-audit example.com --reuse-latest --agent",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:novel":      "ai-visibility-audit",
		},
	}
	attachAuditFlagsAndRun(cmd, flags)
	return cmd
}

// newAnalyzeCmd is the `analyze` alias of ai-visibility-audit. Identical
// behavior; users with structure already in place can call this for an "I
// just want a fresh analysis" reading of intent.
func newAnalyzeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <url-or-domain>",
		Short: "Alias of ai-visibility-audit: run the full audit lifecycle for a domain or site.",
		Long:  `Alias of ai-visibility-audit. Same behavior; use whichever reads better.`,
		Example: "  botsee-pp-cli analyze example.com --watch\n" +
			"  botsee-pp-cli analyze example.com --types 2 --personas 2 --questions 5 --watch\n" +
			"  botsee-pp-cli analyze example.com --estimate-only\n" +
			"  botsee-pp-cli analyze example.com --reuse-latest --agent",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:novel":      "ai-visibility-audit-alias",
		},
	}
	attachAuditFlagsAndRun(cmd, flags)
	return cmd
}

func attachAuditFlagsAndRun(cmd *cobra.Command, flags *rootFlags) {
	var (
		types        int
		personas     int
		questions    int
		models       string
		scope        string
		estimateOnly bool
		costCapUSD   float64
		watch        bool
		regenerate   bool
		reuseLatest  bool
		outputFile   string
	)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		rawTarget := args[0]
		normalized := normalizeDomain(rawTarget)
		if normalized == "" {
			return fmt.Errorf("invalid domain or URL: %q", rawTarget)
		}

		c, err := flags.newClient()
		if err != nil {
			return err
		}
		ctx := cmd.Context()

		result := &auditResult{
			InputURL:         rawTarget,
			NormalizedDomain: normalized,
			StartedAt:        time.Now().UTC().Format(time.RFC3339),
		}

		// 1. Find or create site
		siteUUID, existing, err := findOrCreateSite(ctx, c, rawTarget, normalized, flags, result)
		if err != nil {
			return err
		}
		result.SiteUUID = siteUUID
		result.SiteExisted = existing

		// 2. Discover or generate structure (CTs → personas → questions)
		if !reuseLatest {
			if err := ensureStructure(ctx, c, siteUUID, types, personas, questions, regenerate, flags, result); err != nil {
				return err
			}
		}

		// 3. Cost estimate
		modelsList := splitCSV(models)
		est, err := estimateCost(ctx, c, siteUUID, modelsList, result.QuestionsCount)
		if err == nil {
			result.Estimate = est
		}

		if estimateOnly {
			result.Status = "estimate_only"
			return emitAuditResult(cmd, flags, result, outputFile)
		}

		// --reuse-latest consumes zero credits, so the cap doesn't apply.
		if costCapUSD > 0 && !reuseLatest {
			if est == nil {
				return fmt.Errorf("--cost-cap set but cost estimation failed; cannot verify cost is within cap — rerun with --estimate-only to debug")
			}
			if est.PredictedUSD > costCapUSD {
				return fmt.Errorf("predicted cost $%.2f exceeds --cost-cap $%.2f", est.PredictedUSD, costCapUSD)
			}
		}

		// 4. Reuse latest or run new analysis
		var analysisUUID string
		if reuseLatest {
			analysisUUID, err = findLatestCompletedAnalysis(ctx, c, siteUUID)
			if err != nil {
				return err
			}
			result.AnalysisReused = true
		} else {
			analysisUUID, err = startAnalysis(ctx, c, siteUUID, modelsList, scope, flags)
			if err != nil {
				return err
			}
		}
		result.AnalysisUUID = analysisUUID

		// 5. Poll until completion (unless --no-watch on a fresh run)
		if !reuseLatest && watch {
			status, err := pollAnalysis(ctx, c, analysisUUID)
			if err != nil {
				return err
			}
			result.AnalysisStatus = status
			if status != "completed" {
				return emitAuditResult(cmd, flags, result, outputFile)
			}
		}

		// 6. Fetch results (only if completed)
		if reuseLatest || (watch && result.AnalysisStatus == "completed") {
			if err := fetchAnalysisResults(ctx, c, analysisUUID, result); err != nil {
				return err
			}
		}

		result.Status = "ok"
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		return emitAuditResult(cmd, flags, result, outputFile)
	}
	cmd.Flags().IntVar(&types, "types", 2, "Number of customer types to generate on first run")
	cmd.Flags().IntVar(&personas, "personas", 2, "Number of personas per customer type to generate")
	cmd.Flags().IntVar(&questions, "questions", 5, "Number of questions per persona to generate")
	cmd.Flags().StringVar(&models, "models", "", "Comma-separated LLMs: openai,claude,perplexity,gemini,grok (empty = server default)")
	cmd.Flags().StringVar(&scope, "scope", "", "Analysis scope: site|customer_type|persona|questions (empty = site)")
	cmd.Flags().BoolVar(&estimateOnly, "estimate-only", false, "Predict credit + USD cost without spending")
	cmd.Flags().Float64Var(&costCapUSD, "cost-cap", 0, "Abort if predicted cost exceeds this USD amount (0 = no cap)")
	cmd.Flags().BoolVar(&watch, "watch", true, "Poll until analysis completes")
	cmd.Flags().BoolVar(&regenerate, "regenerate", false, "Force LLM re-generation of customer types/personas/questions even if structure exists")
	cmd.Flags().BoolVar(&reuseLatest, "reuse-latest", false, "Skip running a new analysis; print the most recent completed analysis (no spend)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Write structured report to this file (markdown)")
}

// --- helpers ---

type auditResult struct {
	InputURL         string            `json:"input_url"`
	NormalizedDomain string            `json:"normalized_domain"`
	SiteUUID         string            `json:"site_uuid"`
	SiteExisted      bool              `json:"site_existed"`
	StructureExisted bool              `json:"structure_existed"`
	CustomerTypes    int               `json:"customer_types"`
	PersonasCount    int               `json:"personas"`
	QuestionsCount   int               `json:"questions"`
	Estimate         *costEstimate     `json:"estimate,omitempty"`
	AnalysisUUID     string            `json:"analysis_uuid,omitempty"`
	AnalysisReused   bool              `json:"analysis_reused,omitempty"`
	AnalysisStatus   string            `json:"analysis_status,omitempty"`
	Competitors      []json.RawMessage `json:"competitors,omitempty"`
	Keywords         []json.RawMessage `json:"keywords,omitempty"`
	Sources          []json.RawMessage `json:"sources,omitempty"`
	StartedAt        string            `json:"started_at"`
	CompletedAt      string            `json:"completed_at,omitempty"`
	Status           string            `json:"status,omitempty"`
}

type costEstimate struct {
	Models            []string `json:"models"`
	QuestionsCount    int      `json:"questions_count"`
	ReservedCredits   int      `json:"reserved_credits"`
	PredictedCredits  int      `json:"predicted_credits"`
	PredictedUSD      float64  `json:"predicted_usd"`
	CostMultiplier    float64  `json:"cost_multiplier"`
	ReservationBuffer float64  `json:"reservation_buffer"`
	Source            string   `json:"source"`
}

// normalizeDomain strips scheme, www, paths, query, trailing slashes and lowercases.
func normalizeDomain(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	host = strings.TrimPrefix(host, "www.")
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host
}

func findOrCreateSite(ctx context.Context, c httpClient, rawURL, normalized string, flags *rootFlags, result *auditResult) (string, bool, error) {
	// Paginate through /sites until the domain is found or the list is
	// exhausted. The endpoint uses cursor pagination (`cursor` + `limit`,
	// per the spec). Without pagination, a user with more sites than the
	// API default page size would silently get a duplicate site created.
	cursor := ""
	const maxPages = 50 // safety cap; 50 * 100 = 5000 sites covered
	for page := 0; page < maxPages; page++ {
		params := map[string]string{"limit": "100"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := c.Get(ctx, "/api/v1/sites", params)
		if err != nil {
			return "", false, fmt.Errorf("listing sites: %w", err)
		}
		if uuid := matchSiteByDomain(data, normalized); uuid != "" {
			return uuid, true, nil
		}
		// Extract next cursor (common shapes: next_cursor, cursor, pagination.next_cursor)
		nextCursor := extractNextCursor(data)
		if nextCursor == "" || nextCursor == cursor {
			break
		}
		cursor = nextCursor
	}
	if dryRunOK(flags) {
		return "", false, nil
	}
	body := map[string]any{"url": ensureScheme(rawURL)}
	resp, status, err := c.Post(ctx, "/api/v1/sites", body)
	if err != nil || status < 200 || status >= 300 {
		if err == nil {
			err = fmt.Errorf("HTTP %d: %s", status, string(resp))
		}
		return "", false, fmt.Errorf("creating site: %w", err)
	}
	return extractUUID(resp, "site"), false, nil
}

// extractNextCursor probes common cursor field locations in a paginated
// JSON response. Returns "" if no next-page indicator is present.
func extractNextCursor(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	// Top-level next_cursor / cursor
	for _, key := range []string{"next_cursor", "nextCursor", "cursor"} {
		if raw, ok := m[key]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil && s != "" {
				return s
			}
		}
	}
	// Nested under pagination / meta
	for _, parent := range []string{"pagination", "meta"} {
		if raw, ok := m[parent]; ok {
			var inner map[string]json.RawMessage
			if err := json.Unmarshal(raw, &inner); err == nil {
				for _, key := range []string{"next_cursor", "nextCursor", "cursor"} {
					if r, ok := inner[key]; ok {
						var s string
						if err := json.Unmarshal(r, &s); err == nil && s != "" {
							return s
						}
					}
				}
			}
		}
	}
	return ""
}

func matchSiteByDomain(listResp json.RawMessage, normalized string) string {
	var wrap struct {
		Sites []map[string]any `json:"sites"`
		Data  []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(listResp, &wrap); err == nil {
		items := wrap.Sites
		if len(items) == 0 {
			items = wrap.Data
		}
		for _, s := range items {
			if normalizeDomain(asString(s["domain"])) == normalized ||
				normalizeDomain(asString(s["url"])) == normalized {
				if u := asString(s["uuid"]); u != "" {
					return u
				}
			}
		}
	}
	var direct []map[string]any
	if err := json.Unmarshal(listResp, &direct); err == nil {
		for _, s := range direct {
			if normalizeDomain(asString(s["domain"])) == normalized ||
				normalizeDomain(asString(s["url"])) == normalized {
				if u := asString(s["uuid"]); u != "" {
					return u
				}
			}
		}
	}
	return ""
}

func ensureStructure(ctx context.Context, c httpClient, siteUUID string, types, personas, questions int, regen bool, flags *rootFlags, result *auditResult) error {
	// Look at existing customer types
	data, _ := c.Get(ctx, "/api/v1/sites/"+siteUUID+"/customer-types", nil)
	ctList := extractList(data, "customer_types")
	if len(ctList) > 0 && !regen {
		result.StructureExisted = true
		result.CustomerTypes = len(ctList)
		// Count personas + questions under existing structure
		for _, ct := range ctList {
			ctUUID := asString(ct["uuid"])
			if ctUUID == "" {
				continue
			}
			pData, _ := c.Get(ctx, "/api/v1/customer-types/"+ctUUID+"/personas", nil)
			pList := extractList(pData, "personas")
			result.PersonasCount += len(pList)
			for _, p := range pList {
				pUUID := asString(p["uuid"])
				if pUUID == "" {
					continue
				}
				qData, _ := c.Get(ctx, "/api/v1/personas/"+pUUID+"/questions", nil)
				qList := extractList(qData, "questions")
				result.QuestionsCount += len(qList)
			}
		}
		return nil
	}

	if dryRunOK(flags) {
		return nil
	}

	// Generate customer types
	ctBody := map[string]any{"count": types}
	ctResp, status, err := c.Post(ctx, "/api/v1/sites/"+siteUUID+"/customer-types/generate", ctBody)
	if err != nil || status < 200 || status >= 300 {
		return fmt.Errorf("generating customer types: %v (status=%d)", err, status)
	}
	newCTs := extractList(ctResp, "customer_types")
	result.CustomerTypes = len(newCTs)

	// Generate personas per CT — track partial failures so we can refuse
	// to run an analysis backed by an empty question set.
	personaFailures := 0
	questionFailures := 0
	for _, ct := range newCTs {
		ctUUID := asString(ct["uuid"])
		if ctUUID == "" {
			continue
		}
		pBody := map[string]any{"count": personas}
		pResp, pStatus, perr := c.Post(ctx, "/api/v1/customer-types/"+ctUUID+"/personas/generate", pBody)
		if perr != nil || pStatus < 200 || pStatus >= 300 {
			personaFailures++
			fmt.Fprintf(os.Stderr, "warning: persona generation failed for customer-type %s (status=%d, err=%v)\n", ctUUID, pStatus, perr)
			continue
		}
		pList := extractList(pResp, "personas")
		result.PersonasCount += len(pList)

		// Generate questions per persona
		for _, p := range pList {
			pUUID := asString(p["uuid"])
			if pUUID == "" {
				continue
			}
			qBody := map[string]any{"count": questions}
			qResp, qStatus, qerr := c.Post(ctx, "/api/v1/personas/"+pUUID+"/questions/generate", qBody)
			if qerr != nil || qStatus < 200 || qStatus >= 300 {
				questionFailures++
				fmt.Fprintf(os.Stderr, "warning: question generation failed for persona %s (status=%d, err=%v)\n", pUUID, qStatus, qerr)
				continue
			}
			qList := extractList(qResp, "questions")
			result.QuestionsCount += len(qList)
		}
	}
	// Refuse to proceed if structure bootstrapping produced no questions.
	// Without this, startAnalysis would POST a paid /analysis call backed by
	// an empty question set — silently consuming credits for no usable data.
	if result.QuestionsCount == 0 {
		return fmt.Errorf("structure bootstrap produced 0 questions (%d persona failures, %d question failures); refusing to start a paid analysis — run with --regenerate or check API status", personaFailures, questionFailures)
	}
	return nil
}

func estimateCost(ctx context.Context, c httpClient, siteUUID string, models []string, questionCount int) (*costEstimate, error) {
	if questionCount == 0 {
		return nil, errors.New("no questions to estimate")
	}
	data, err := c.Get(ctx, "/api/v1/pricing", nil)
	if err != nil {
		return nil, err
	}
	var p struct {
		Pricing struct {
			AnalysisEstimatedPerQuery map[string]int `json:"analysis_estimated_per_query"`
			AnalysisReservationBuffer float64        `json:"analysis_reservation_buffer"`
			CostMultiplier            float64        `json:"cost_multiplier"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if len(models) == 0 {
		for m := range p.Pricing.AnalysisEstimatedPerQuery {
			if m != "default" {
				models = append(models, m)
			}
		}
	}
	multiplier := p.Pricing.CostMultiplier
	if multiplier <= 0 {
		multiplier = 1
	}
	buffer := p.Pricing.AnalysisReservationBuffer
	if buffer <= 0 {
		buffer = 1
	}
	perQueryTotal := 0
	for _, m := range models {
		if v, ok := p.Pricing.AnalysisEstimatedPerQuery[m]; ok {
			perQueryTotal += v
		} else if v, ok := p.Pricing.AnalysisEstimatedPerQuery["default"]; ok {
			perQueryTotal += v
		}
	}
	baseCredits := perQueryTotal * questionCount
	reserved := int(float64(baseCredits) * buffer)
	predicted := int(float64(baseCredits) * multiplier)
	predictedUSD := float64(predicted) * 0.01
	return &costEstimate{
		Models:            models,
		QuestionsCount:    questionCount,
		ReservedCredits:   reserved,
		PredictedCredits:  predicted,
		PredictedUSD:      predictedUSD,
		CostMultiplier:    multiplier,
		ReservationBuffer: buffer,
		Source:            "live /pricing",
	}, nil
}

func startAnalysis(ctx context.Context, c httpClient, siteUUID string, models []string, scope string, flags *rootFlags) (string, error) {
	if dryRunOK(flags) {
		return "", nil
	}
	body := map[string]any{"site_uuid": siteUUID}
	if scope != "" {
		body["scope"] = scope
	}
	if len(models) > 0 {
		body["models"] = models
	}
	resp, status, err := c.Post(ctx, "/api/v1/analysis", body)
	if err != nil || status < 200 || status >= 300 {
		return "", fmt.Errorf("starting analysis: %v (status=%d)", err, status)
	}
	return extractUUID(resp, "analysis"), nil
}

func pollAnalysis(ctx context.Context, c httpClient, analysisUUID string) (string, error) {
	if analysisUUID == "" {
		return "pending", nil
	}
	wait := time.Second
	maxWait := 30 * time.Second
	deadline := time.Now().Add(10 * time.Minute)
	first := true
	for time.Now().Before(deadline) {
		if !first {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
		}
		first = false
		// Bypass the 5-minute HTTP cache — polling requires live state
		// transitions, not a cached snapshot of the first response.
		data, err := c.GetNoCache(ctx, "/api/v1/analysis/"+analysisUUID, nil)
		if err != nil {
			continue
		}
		var wrap struct {
			Analysis struct {
				Status string `json:"status"`
			} `json:"analysis"`
		}
		if err := json.Unmarshal(data, &wrap); err == nil {
			switch wrap.Analysis.Status {
			case "completed":
				return "completed", nil
			case "failed":
				return "failed", fmt.Errorf("analysis %s failed", analysisUUID)
			}
		}
	}
	return "timeout", fmt.Errorf("analysis %s polling timed out after 10m", analysisUUID)
}

func findLatestCompletedAnalysis(ctx context.Context, c httpClient, siteUUID string) (string, error) {
	data, err := c.Get(ctx, "/api/v1/sites/"+siteUUID+"/analysis", nil)
	if err != nil {
		return "", err
	}
	list := extractList(data, "analyses")
	// Pick the completed analysis with the greatest completed_at (falling
	// back to started_at / created_at). The API does not guarantee sorted
	// order, so iterating in list order risks returning stale data.
	var bestUUID, bestKey string
	for _, a := range list {
		if asString(a["status"]) != "completed" {
			continue
		}
		u := asString(a["uuid"])
		if u == "" {
			continue
		}
		// Prefer completed_at, fall back to started_at, then created_at.
		// All three are RFC3339 strings per the spec; lexicographic order
		// over RFC3339 is equivalent to chronological order.
		key := asString(a["completed_at"])
		if key == "" {
			key = asString(a["started_at"])
		}
		if key == "" {
			key = asString(a["created_at"])
		}
		if bestUUID == "" || key > bestKey {
			bestUUID = u
			bestKey = key
		}
	}
	if bestUUID != "" {
		return bestUUID, nil
	}
	return "", fmt.Errorf("no completed analysis found for site %s — run without --reuse-latest", siteUUID)
}

func fetchAnalysisResults(ctx context.Context, c httpClient, analysisUUID string, result *auditResult) error {
	if analysisUUID == "" {
		return nil
	}
	for _, ep := range []struct {
		path string
		key  string
		dst  *[]json.RawMessage
	}{
		{"/api/v1/analysis/" + analysisUUID + "/competitors", "competitors", &result.Competitors},
		{"/api/v1/analysis/" + analysisUUID + "/keywords", "keywords", &result.Keywords},
		{"/api/v1/analysis/" + analysisUUID + "/sources", "sources", &result.Sources},
	} {
		data, err := c.Get(ctx, ep.path, nil)
		if err != nil {
			// Surface per-endpoint failure to stderr so users see which
			// result section is missing instead of finding an empty array
			// in the report. The audit still continues so partial results
			// are reported for the endpoints that succeeded.
			fmt.Fprintf(os.Stderr, "warning: fetch %s failed: %v\n", ep.key, err)
			continue
		}
		*ep.dst = extractRawList(data, ep.key)
	}
	return nil
}

func emitAuditResult(cmd *cobra.Command, flags *rootFlags, result *auditResult, outputFile string) error {
	out := cmd.OutOrStdout()
	if outputFile != "" {
		if err := writeMarkdownReport(outputFile, result); err != nil {
			return err
		}
	}
	if flags.asJSON || flags.agent || !isTerminal(out) {
		return printJSONFiltered(out, result, flags)
	}
	fmt.Fprintf(out, "Domain: %s\n", result.NormalizedDomain)
	fmt.Fprintf(out, "Site:   %s%s\n", result.SiteUUID, ifS(result.SiteExisted, " (existing)", " (created)"))
	fmt.Fprintf(out, "Structure: %d customer types / %d personas / %d questions%s\n",
		result.CustomerTypes, result.PersonasCount, result.QuestionsCount,
		ifS(result.StructureExisted, " (existing)", " (generated)"))
	if result.Estimate != nil {
		fmt.Fprintf(out, "Estimate: ~%d reserved / ~%d predicted credits ($%.2f USD, multiplier=%g)\n",
			result.Estimate.ReservedCredits, result.Estimate.PredictedCredits,
			result.Estimate.PredictedUSD, result.Estimate.CostMultiplier)
	}
	if result.AnalysisUUID != "" {
		fmt.Fprintf(out, "Analysis: %s (status=%s%s)\n", result.AnalysisUUID,
			ifS(result.AnalysisStatus == "", "pending", result.AnalysisStatus),
			ifS(result.AnalysisReused, ", reused", ""))
	}
	if n := len(result.Competitors); n > 0 {
		fmt.Fprintf(out, "Competitors found: %d\n", n)
	}
	if n := len(result.Keywords); n > 0 {
		fmt.Fprintf(out, "Keywords found:    %d\n", n)
	}
	if n := len(result.Sources); n > 0 {
		fmt.Fprintf(out, "Sources cited:     %d\n", n)
	}
	return nil
}

func writeMarkdownReport(path string, result *auditResult) error {
	var b strings.Builder
	b.WriteString("# AI Visibility Audit — " + result.NormalizedDomain + "\n\n")
	fmt.Fprintf(&b, "- Site: `%s`%s\n", result.SiteUUID, ifS(result.SiteExisted, " (existing)", " (created)"))
	fmt.Fprintf(&b, "- Customer types: %d  •  Personas: %d  •  Questions: %d\n",
		result.CustomerTypes, result.PersonasCount, result.QuestionsCount)
	if result.Estimate != nil {
		fmt.Fprintf(&b, "- Estimate: %d credits ($%.2f USD)\n", result.Estimate.PredictedCredits, result.Estimate.PredictedUSD)
	}
	if result.AnalysisUUID != "" {
		fmt.Fprintf(&b, "- Analysis: `%s` (%s)\n\n", result.AnalysisUUID, result.AnalysisStatus)
	}
	if n := len(result.Competitors); n > 0 {
		fmt.Fprintf(&b, "## Competitors (%d)\n\n", n)
		for _, c := range result.Competitors {
			b.WriteString("- `" + string(c) + "`\n")
		}
		b.WriteString("\n")
	}
	if n := len(result.Keywords); n > 0 {
		fmt.Fprintf(&b, "## Keywords (%d)\n\n", n)
		for _, k := range result.Keywords {
			b.WriteString("- `" + string(k) + "`\n")
		}
		b.WriteString("\n")
	}
	if n := len(result.Sources); n > 0 {
		fmt.Fprintf(&b, "## Sources (%d)\n\n", n)
		for _, s := range result.Sources {
			b.WriteString("- `" + string(s) + "`\n")
		}
	}
	return writeFileAtomic(path, []byte(b.String()))
}

// httpClient narrows the generated Client to what this novel command needs,
// for easier testability later. GetNoCache is used by poll loops that must
// see live state (e.g. analysis status transitions) — without it, the
// 5-minute HTTP cache would lock the poll loop on the first response.
type httpClient interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
}

func extractList(data json.RawMessage, key string) []map[string]any {
	if len(data) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err == nil {
		if raw, ok := m[key]; ok {
			var out []map[string]any
			if err := json.Unmarshal(raw, &out); err == nil {
				return out
			}
		}
		if raw, ok := m["data"]; ok {
			var out []map[string]any
			if err := json.Unmarshal(raw, &out); err == nil {
				return out
			}
		}
	}
	var out []map[string]any
	if err := json.Unmarshal(data, &out); err == nil {
		return out
	}
	return nil
}

func extractRawList(data json.RawMessage, key string) []json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err == nil {
		if raw, ok := m[key]; ok {
			var out []json.RawMessage
			if err := json.Unmarshal(raw, &out); err == nil {
				return out
			}
		}
	}
	var out []json.RawMessage
	if err := json.Unmarshal(data, &out); err == nil {
		return out
	}
	return nil
}

func extractUUID(data json.RawMessage, parentKey string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err == nil {
		if raw, ok := m[parentKey]; ok {
			var inner map[string]any
			if err := json.Unmarshal(raw, &inner); err == nil {
				return asString(inner["uuid"])
			}
		}
		if raw, ok := m["uuid"]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				return s
			}
		}
	}
	return ""
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func ensureScheme(s string) string {
	if strings.Contains(s, "://") {
		return s
	}
	return "https://" + s
}

func ifS(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}
