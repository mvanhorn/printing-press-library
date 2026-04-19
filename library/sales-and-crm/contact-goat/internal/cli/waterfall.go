// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// waterfall: Clay-style multi-source enrichment. Tries free sources first
// (LinkedIn + Happenstance), then Deepline with BYOK when configured, and
// finally Deepline managed mode (burns credits). Prints a per-step cost
// ledger so agents can track spend.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/linkedin"

	"github.com/spf13/cobra"
)

var linkedInURLPattern = regexp.MustCompile(`^https?://(www\.)?linkedin\.com/in/[^/?#]+/?$`)
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// WaterfallStep logs a single attempt in the waterfall.
type WaterfallStep struct {
	Source  string          `json:"source"`
	Tool    string          `json:"tool,omitempty"`
	BYOK    bool            `json:"byok,omitempty"`
	Cost    int             `json:"cost_credits"`
	Status  string          `json:"status"`
	Error   string          `json:"error,omitempty"`
	Fields  []string        `json:"fields_filled,omitempty"`
	Snippet json.RawMessage `json:"snippet,omitempty"`
}

// WaterfallResult is the final output shape.
type WaterfallResult struct {
	Target      string            `json:"target"`
	TargetKind  string            `json:"target_kind"`
	Fields      map[string]any    `json:"fields"`
	Missing     []string          `json:"missing"`
	Steps       []WaterfallStep   `json:"steps"`
	TotalCredit int               `json:"total_credits_spent"`
	BYOKKeys    map[string]string `json:"byok_providers,omitempty"`
}

func newWaterfallCmd(flags *rootFlags) *cobra.Command {
	var enrichCSV string
	var maxCost int
	var requireBYOK bool

	cmd := &cobra.Command{
		Use:   "waterfall <target>",
		Short: "Clay-style waterfall enrichment: free sources first, Deepline with BYOK or managed",
		Long: `Enrich a person starting from the cheapest source and waterfalling into
progressively more expensive ones.

Targets:
  - an email (alice@stripe.com)
  - a LinkedIn URL (https://www.linkedin.com/in/satyanadella/)
  - a bare name (use --company for disambiguation)

Step order:
  1. LinkedIn get_person_profile (free, scraper subprocess)
  2. Happenstance research (free if you have a cookie)
  3. Deepline BYOK (uses your own Hunter/Apollo keys, no Deepline markup)
  4. Deepline managed (burns Deepline credits, last resort)

Configure BYOK keys with:
  contact-goat-pp-cli config byok set <provider> <env-var-name>`,
		Example: `  contact-goat-pp-cli waterfall https://www.linkedin.com/in/patrickcollison/ --enrich email,phone
  contact-goat-pp-cli waterfall alice@stripe.com --max-cost 2 --json
  contact-goat-pp-cli waterfall "Brian Chesky" --company airbnb.com --byok`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.TrimSpace(args[0])
			enrichFields := parseCSVFields(enrichCSV)
			if len(enrichFields) == 0 {
				enrichFields = []string{"email", "phone"}
			}

			result := &WaterfallResult{
				Target:     target,
				TargetKind: classifyTarget(target),
				Fields:     map[string]any{},
			}

			byok := readBYOKConfig()
			if requireBYOK && len(byok) == 0 {
				return authErr(errors.New("no BYOK providers configured — run `contact-goat-pp-cli config byok set hunter HUNTER_API_KEY`"))
			}
			result.BYOKKeys = redactedBYOK(byok)

			// Step 1: LinkedIn profile.
			if !waterfallComplete(result, enrichFields) {
				step := tryLinkedIn(cmd.Context(), flags, target, result)
				result.Steps = append(result.Steps, step)
			}
			// Step 2: Happenstance research.
			if !waterfallComplete(result, enrichFields) {
				step := tryHappenstance(cmd, flags, target, result)
				result.Steps = append(result.Steps, step)
			}
			// Step 3 & 4: Deepline (BYOK first when configured, then managed).
			if !waterfallComplete(result, enrichFields) && !requireBYOK {
				// BYOK path first when keys are present.
				if len(byok) > 0 {
					step := tryDeepline(cmd.Context(), flags, target, enrichFields, true, byok, maxCost, result)
					result.Steps = append(result.Steps, step)
				}
				if !waterfallComplete(result, enrichFields) {
					step := tryDeepline(cmd.Context(), flags, target, enrichFields, false, byok, maxCost, result)
					result.Steps = append(result.Steps, step)
				}
			} else if !waterfallComplete(result, enrichFields) && requireBYOK {
				step := tryDeepline(cmd.Context(), flags, target, enrichFields, true, byok, maxCost, result)
				result.Steps = append(result.Steps, step)
			}

			for _, f := range enrichFields {
				if _, ok := result.Fields[f]; !ok {
					result.Missing = append(result.Missing, f)
				}
			}
			for _, s := range result.Steps {
				result.TotalCredit += s.Cost
			}

			return emitWaterfall(cmd, flags, result)
		},
	}

	cmd.Flags().StringVar(&enrichCSV, "enrich", "email,phone", "Comma-separated fields to fill (email, phone, company, name, title)")
	cmd.Flags().IntVar(&maxCost, "max-cost", 5, "Max Deepline credits to spend across the whole run")
	cmd.Flags().BoolVar(&requireBYOK, "byok", false, "Require BYOK for Deepline steps; error if no BYOK keys configured")
	return cmd
}

func parseCSVFields(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func classifyTarget(t string) string {
	switch {
	case emailPattern.MatchString(t):
		return "email"
	case linkedInURLPattern.MatchString(t):
		return "linkedin_url"
	case strings.Contains(t, "linkedin.com/in/"):
		return "linkedin_url"
	default:
		return "name"
	}
}

func waterfallComplete(r *WaterfallResult, fields []string) bool {
	for _, f := range fields {
		if _, ok := r.Fields[f]; !ok {
			return false
		}
	}
	return true
}

// tryLinkedIn attempts to fill fields via the LinkedIn MCP subprocess.
// Free (no credit cost), but slow because it uses Selenium.
func tryLinkedIn(parentCtx context.Context, flags *rootFlags, target string, r *WaterfallResult) WaterfallStep {
	step := WaterfallStep{Source: "linkedin", Tool: "get_person_profile"}
	if r.TargetKind != "linkedin_url" {
		// Without a LinkedIn URL we'd need to search_people first. Skip for
		// the v1 path — search_people is still reachable via the `linkedin
		// search-people` command directly.
		step.Status = "skipped"
		step.Error = "target is not a LinkedIn URL"
		return step
	}
	if ok, _ := linkedin.IsLoggedIn(); !ok {
		step.Status = "skipped"
		step.Error = "linkedin-mcp not logged in"
		return step
	}

	ctx, cancel := signalCtx(parentCtx)
	defer cancel()
	client, err := spawnLIClient(ctx)
	if err != nil {
		step.Status = "error"
		step.Error = err.Error()
		return step
	}
	defer client.Close()
	if _, err := client.Initialize(ctx, linkedin.Implementation{Name: "contact-goat-pp-cli", Version: version}); err != nil {
		step.Status = "error"
		step.Error = err.Error()
		return step
	}
	callCtx, callCancel := context.WithTimeout(ctx, flags.timeout)
	defer callCancel()
	result, err := client.CallTool(callCtx, linkedin.ToolNames.GetPerson, map[string]any{"linkedin_url": target})
	if err != nil {
		step.Status = "error"
		step.Error = err.Error()
		return step
	}
	body := linkedin.TextPayload(result)
	if body == "" {
		step.Status = "empty"
		return step
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err == nil {
		applyEnrichFields(r, parsed, &step, []string{"email", "phone", "company", "name", "title", "headline", "location"})
	}
	// LinkedIn almost never surfaces email/phone for third parties; this is
	// still worth it because we fill "name", "title", "company".
	step.Status = "ok"
	step.Snippet = json.RawMessage(body)
	return step
}

// tryHappenstance queries the Happenstance research endpoint. The sniffed
// endpoint shape is POST /api/research?target=<...>, but the free-tier CLI
// only has a read path; we call /api/research/find as a research lookup.
func tryHappenstance(cmd *cobra.Command, flags *rootFlags, target string, r *WaterfallResult) WaterfallStep {
	step := WaterfallStep{Source: "happenstance", Tool: "research"}
	c, err := flags.newClientRequireCookies("happenstance")
	if err != nil {
		step.Status = "skipped"
		step.Error = err.Error()
		return step
	}
	// Try a handful of likely paths; the happenstance research surface is
	// still experimental and we don't want to hard-code one.
	paths := []string{"/api/research/find", "/api/research/recent"}
	for _, p := range paths {
		data, err := c.Get(p, map[string]string{"query": target, "limit": "1"})
		if err != nil {
			continue
		}
		data = extractResponseData(data)
		var m map[string]any
		if err := json.Unmarshal(data, &m); err == nil {
			applyEnrichFields(r, m, &step, []string{"email", "phone", "company", "name", "title", "linkedin_url"})
			step.Snippet = json.RawMessage(data)
			step.Status = "ok"
			return step
		}
	}
	step.Status = "empty"
	return step
}

// tryDeepline drives the Deepline waterfall, either BYOK or managed.
func tryDeepline(ctx context.Context, flags *rootFlags, target string, fields []string, useBYOK bool, byok map[string]string, maxCost int, r *WaterfallResult) WaterfallStep {
	step := WaterfallStep{Source: "deepline", Tool: deepline.ToolPersonEnrich, BYOK: useBYOK}

	if useBYOK && len(byok) == 0 {
		step.Status = "skipped"
		step.Error = "no BYOK providers configured"
		return step
	}

	spent := r.TotalCredit
	client := deepline.NewClient(os.Getenv("DEEPLINE_API_KEY"))
	if err := client.ValidateKey(); err != nil {
		step.Status = "skipped"
		step.Error = err.Error()
		return step
	}

	// Decide which tool to run based on target kind.
	toolID := deepline.ToolPersonEnrich
	payload := map[string]any{}
	switch r.TargetKind {
	case "linkedin_url":
		payload["linkedin_url"] = target
	case "email":
		toolID = deepline.ToolEmailFind
		payload["email"] = target
	case "name":
		toolID = deepline.ToolPersonSearchToEmailWaterfall
		payload["name"] = target
		if co := strings.TrimSpace(os.Getenv("CONTACT_GOAT_COMPANY")); co != "" {
			payload["company"] = co
		}
	}
	if useBYOK {
		payload["byok"] = true
		// Forward the env var name(s) — we never store the value.
		if envVar, ok := byok["hunter"]; ok && os.Getenv(envVar) != "" {
			payload["byok_hunter_key_env"] = envVar
		}
		if envVar, ok := byok["apollo"]; ok && os.Getenv(envVar) != "" {
			payload["byok_apollo_key_env"] = envVar
		}
	}

	est, _ := client.EstimateCost(toolID, payload)
	if maxCost > 0 && spent+est > maxCost {
		step.Status = "skipped"
		step.Error = fmt.Sprintf("would exceed --max-cost %d (already spent %d, next step ~%d)", maxCost, spent, est)
		step.Cost = 0
		return step
	}
	step.Tool = toolID

	execCtx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()
	raw, err := client.Execute(execCtx, toolID, payload)
	if err != nil {
		step.Status = "error"
		step.Error = err.Error()
		return step
	}
	// BYOK stills costs the upstream provider but we don't pay Deepline markup.
	if useBYOK {
		step.Cost = 0
	} else {
		step.Cost = est
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err == nil {
		applyEnrichFields(r, m, &step, []string{"email", "phone", "company", "name", "title", "linkedin_url"})
	}
	step.Snippet = raw
	step.Status = "ok"
	return step
}

// applyEnrichFields copies the listed fields from src into the running result.
// Strings are only accepted if non-empty; already-filled fields are not
// overwritten so cheaper sources "win".
func applyEnrichFields(r *WaterfallResult, src map[string]any, step *WaterfallStep, fields []string) {
	for _, f := range fields {
		if _, ok := r.Fields[f]; ok {
			continue
		}
		if v, ok := src[f]; ok {
			if s, ok := v.(string); ok && s != "" {
				r.Fields[f] = s
				step.Fields = append(step.Fields, f)
			} else if v != nil {
				r.Fields[f] = v
				step.Fields = append(step.Fields, f)
			}
		}
	}
}

func emitWaterfall(cmd *cobra.Command, flags *rootFlags, r *WaterfallResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Waterfall: %s (%s)\n", r.Target, r.TargetKind)
	fmt.Fprintf(w, "  credits spent: %d\n", r.TotalCredit)
	for _, s := range r.Steps {
		tag := s.Status
		if s.BYOK {
			tag += " byok"
		}
		fmt.Fprintf(w, "  - [%s] %s (%s) cost=%d fields=%s\n",
			tag, s.Source, s.Tool, s.Cost, strings.Join(s.Fields, ","))
		if s.Error != "" {
			fmt.Fprintf(w, "      error: %s\n", s.Error)
		}
	}
	fmt.Fprintf(w, "Fields filled:\n")
	keys := make([]string, 0, len(r.Fields))
	for k := range r.Fields {
		keys = append(keys, k)
	}
	// deterministic order
	sortStrings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %s = %v\n", k, r.Fields[k])
	}
	if len(r.Missing) > 0 {
		fmt.Fprintf(w, "Missing: %s\n", strings.Join(r.Missing, ", "))
	}
	return nil
}

// redactedBYOK returns the provider -> env-var-name map for display. The VALUE
// of each env var is never echoed (defense in depth; we never store it).
func redactedBYOK(byok map[string]string) map[string]string {
	if len(byok) == 0 {
		return nil
	}
	out := make(map[string]string, len(byok))
	for p, envVar := range byok {
		out[p] = envVar // env var *name*, not value
	}
	return out
}

// Ensure signalCtx import isn't dead if the tail command file is rewritten.
var _ = time.Now
