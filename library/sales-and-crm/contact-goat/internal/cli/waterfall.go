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
	Source   string          `json:"source"`
	Tool     string          `json:"tool,omitempty"`
	Provider string          `json:"provider,omitempty"`
	BYOK     bool            `json:"byok,omitempty"`
	Cost     int             `json:"cost_credits"`
	Status   string          `json:"status"`
	Error    string          `json:"error,omitempty"`
	Fields   []string        `json:"fields_filled,omitempty"`
	Snippet  json.RawMessage `json:"snippet,omitempty"`
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
	var companyDomain string

	cmd := &cobra.Command{
		Use:   "waterfall <target>",
		Annotations: map[string]string{"mcp:read-only": "true"},
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
  3. Deepline provider chain (per target kind; burns Deepline credits)
     linkedin_url -> apollo_people_match -> hunter_people_find -> contactout_enrich_person
     email        -> apollo_people_match -> hunter_people_find
     name+domain  -> dropleads_email_finder -> hunter_email_finder -> datagma_find_email

Name targets require --company (or CONTACT_GOAT_COMPANY env var).

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
				return authErr(errors.New("no BYOK providers configured: run `contact-goat-pp-cli config byok set hunter HUNTER_API_KEY`"))
			}
			result.BYOKKeys = redactedBYOK(byok)

			if companyDomain == "" {
				companyDomain = strings.TrimSpace(os.Getenv("CONTACT_GOAT_COMPANY"))
			}
			if result.TargetKind == "name" && companyDomain == "" {
				return usageErr(errors.New("name targets require --company (e.g. --company stripe.com) or CONTACT_GOAT_COMPANY env"))
			}

			// Preflight: fail fast on missing auth rather than running the
			// LinkedIn + Happenstance chain first. The Deepline step is
			// ultimately where the work gets done; without a path to it the
			// whole waterfall can't satisfy the typical --enrich email,phone
			// ask.
			dlKey, _ := resolveDeeplineKey("")
			if err := preflightWaterfallDeepline(dlKey, requireBYOK, byok); err != nil {
				return err
			}

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
			// Step 3: Deepline provider chain.
			if !waterfallComplete(result, enrichFields) {
				runDeeplineChain(cmd.Context(), flags, target, enrichFields, companyDomain, requireBYOK, byok, maxCost, result)
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
	cmd.Flags().StringVar(&companyDomain, "company", "", "Company domain (e.g. stripe.com); required for bare-name targets")
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
	// Upstream linkedin-scraper-mcp requires linkedin_username (the slug
	// after /in/), not the full URL. Passing linkedin_url yields a pydantic
	// "Unexpected keyword argument" 422.
	result, err := client.CallTool(callCtx, linkedin.ToolNames.GetPerson, map[string]any{"linkedin_username": normalizePersonInput(target)})
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

// deeplineProviderAttempt is a single (tool, payload) pair in the Deepline
// provider chain. Each attempt becomes its own WaterfallStep in the result.
type deeplineProviderAttempt struct {
	toolID   string
	provider string // short label shown in step.Provider
	payload  map[string]any
}

// deeplineProviderChain returns the ordered list of provider attempts to
// execute for the given target. Order is cheapest/highest-hit first. Each
// attempt carries its own payload shape already matching the provider's
// documented schema (verified against `deepline tools get` on 2026-04-20).
func deeplineProviderChain(targetKind, target, companyDomain string) []deeplineProviderAttempt {
	switch targetKind {
	case "linkedin_url":
		return []deeplineProviderAttempt{
			{
				toolID:   deepline.ToolApolloPeopleMatch,
				provider: "apollo",
				payload: map[string]any{
					"linkedin_url":           target,
					"reveal_personal_emails": true,
				},
			},
			{
				toolID:   deepline.ToolHunterPeopleFind,
				provider: "hunter",
				// hunter_people_find requires linkedin_handle (the slug),
				// not linkedin_url.
				payload: map[string]any{"linkedin_handle": linkedinHandleFromURL(target)},
			},
			{
				toolID:   deepline.ToolContactOutEnrichPerson,
				provider: "contactout",
				payload:  map[string]any{"linkedin_url": target},
			},
		}
	case "email":
		return []deeplineProviderAttempt{
			{
				toolID:   deepline.ToolApolloPeopleMatch,
				provider: "apollo",
				payload: map[string]any{
					"email":                  target,
					"reveal_personal_emails": true,
				},
			},
			{
				toolID:   deepline.ToolHunterPeopleFind,
				provider: "hunter",
				payload:  map[string]any{"email": target},
			},
		}
	case "name":
		firstName, lastName := splitName(target)
		return []deeplineProviderAttempt{
			{
				toolID:   deepline.ToolDropleadsEmailFinder,
				provider: "dropleads",
				payload: map[string]any{
					"first_name":     firstName,
					"last_name":      lastName,
					"company_domain": companyDomain,
				},
			},
			{
				toolID:   deepline.ToolHunterEmailFinder,
				provider: "hunter",
				payload: map[string]any{
					"first_name": firstName,
					"last_name":  lastName,
					"domain":     companyDomain,
				},
			},
			{
				toolID:   deepline.ToolDatagmaFindEmail,
				provider: "datagma",
				payload: map[string]any{
					"first_name":     firstName,
					"last_name":      lastName,
					"company_domain": companyDomain,
				},
			},
		}
	}
	return nil
}

// runDeeplineChain walks the provider chain, appending one WaterfallStep per
// provider attempt. Stops early when the requested fields are filled or when
// max-cost would be exceeded. Provider-level errors (including entitlement
// 403s) do not abort the chain; the next provider is tried.
func runDeeplineChain(ctx context.Context, flags *rootFlags, target string, fields []string, companyDomain string, requireBYOK bool, byok map[string]string, maxCost int, r *WaterfallResult) {
	chain := deeplineProviderChain(r.TargetKind, target, companyDomain)
	if len(chain) == 0 {
		return
	}

	dlKey, _ := resolveDeeplineKey("")
	client := deepline.NewClient(dlKey)
	if err := client.ValidateKey(); err != nil {
		// One synthetic step surfaces the auth gate failure rather than
		// emitting N copies (one per provider in the chain).
		r.Steps = append(r.Steps, WaterfallStep{
			Source: "deepline",
			Status: "skipped",
			Error:  err.Error(),
		})
		return
	}

	for _, attempt := range chain {
		if waterfallComplete(r, fields) {
			return
		}
		step := WaterfallStep{
			Source:   "deepline",
			Tool:     attempt.toolID,
			Provider: attempt.provider,
			BYOK:     requireBYOK || len(byok) > 0,
		}

		est, _ := client.EstimateCost(attempt.toolID, attempt.payload)
		spent := r.TotalCredit
		for _, s := range r.Steps {
			spent += s.Cost
		}
		if maxCost > 0 && spent+est > maxCost {
			step.Status = "skipped"
			step.Error = fmt.Sprintf("would exceed --max-cost %d (already spent %d, next step ~%d)", maxCost, spent, est)
			r.Steps = append(r.Steps, step)
			return
		}

		execCtx, cancel := context.WithTimeout(ctx, flags.timeout)
		raw, err := client.Execute(execCtx, attempt.toolID, attempt.payload)
		cancel()
		if err != nil {
			step.Status = "error"
			step.Error = err.Error()
			r.Steps = append(r.Steps, step)
			continue
		}
		step.Cost = est
		step.Snippet = raw
		extractDeeplineFields(r, raw, attempt.provider, &step)
		if len(step.Fields) == 0 {
			step.Status = "empty"
		} else {
			step.Status = "ok"
		}
		r.Steps = append(r.Steps, step)
	}
}

// linkedinHandleFromURL extracts the vanity slug from a LinkedIn profile
// URL (e.g. "https://www.linkedin.com/in/mkscrg/" -> "mkscrg"). Falls back
// to normalizePersonInput which already handles bare slugs, /in/ prefixes,
// and trailing slashes.
func linkedinHandleFromURL(url string) string {
	return normalizePersonInput(url)
}

// splitName splits a "First Last" string into (first, last). A single-word
// name goes entirely into first_name; a name with three or more tokens
// treats the final token as last_name and everything else as first_name
// (handles "Jean-Paul Sartre" or "Mary Anne Smith" reasonably).
func splitName(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	parts := strings.Fields(s)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

// extractDeeplineFields drills into a provider-specific response and copies
// contact fields into the Waterfall result. The Deepline v2 HTTP response is
// wrapped as {"job_id":..., "status":..., "result":{"data":{...}}}; different
// providers nest their fields differently inside `data`, so we dispatch on
// provider name.
func extractDeeplineFields(r *WaterfallResult, raw json.RawMessage, provider string, step *WaterfallStep) {
	var envelope struct {
		Result struct {
			Data json.RawMessage `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Result.Data) == 0 {
		return
	}
	switch provider {
	case "apollo":
		extractApolloPerson(r, envelope.Result.Data, step)
	case "hunter":
		extractHunterResult(r, envelope.Result.Data, step)
	case "contactout":
		extractFlatContact(r, envelope.Result.Data, step,
			[]string{"email", "work_email"}, []string{"phone", "mobile"})
	case "dropleads":
		extractDropleadsResult(r, envelope.Result.Data, step)
	case "datagma":
		extractFlatContact(r, envelope.Result.Data, step,
			[]string{"email"}, []string{"phone"})
	default:
		var m map[string]any
		if err := json.Unmarshal(envelope.Result.Data, &m); err == nil {
			applyEnrichFields(r, m, step, []string{"email", "phone", "company", "name", "title", "linkedin_url"})
		}
	}
}

func extractApolloPerson(r *WaterfallResult, data json.RawMessage, step *WaterfallStep) {
	var wrap struct {
		Person struct {
			Name           string   `json:"name"`
			Title          string   `json:"title"`
			LinkedinURL    string   `json:"linkedin_url"`
			Email          string   `json:"email"`
			EmailStatus    string   `json:"email_status"`
			PersonalEmails []string `json:"personal_emails"`
			Organization   struct {
				Name string `json:"name"`
			} `json:"organization"`
		} `json:"person"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return
	}
	p := wrap.Person
	setField(r, step, "name", p.Name)
	setField(r, step, "title", p.Title)
	setField(r, step, "linkedin_url", p.LinkedinURL)
	setField(r, step, "company", p.Organization.Name)
	// Work email: only accept if upstream marked it verified or catchall
	// (not "unavailable"). Apollo returns email = "" when it has no hit.
	if p.Email != "" && p.EmailStatus != "unavailable" {
		setField(r, step, "email", p.Email)
	}
	if len(p.PersonalEmails) > 0 && p.PersonalEmails[0] != "" {
		setField(r, step, "personal_email", p.PersonalEmails[0])
	}
	if p.EmailStatus != "" {
		setField(r, step, "email_confidence", p.EmailStatus)
	}
}

func extractHunterResult(r *WaterfallResult, data json.RawMessage, step *WaterfallStep) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	if email, ok := stringField(m, "email"); ok {
		setField(r, step, "email", email)
	}
	if phone, ok := stringField(m, "phone_number"); ok {
		setField(r, step, "phone", phone)
	} else if phone, ok := stringField(m, "phone"); ok {
		setField(r, step, "phone", phone)
	}
	if co, ok := stringField(m, "company"); ok {
		setField(r, step, "company", co)
	}
	if title, ok := stringField(m, "position"); ok {
		setField(r, step, "title", title)
	}
	if score, ok := m["score"].(float64); ok && score > 0 {
		setField(r, step, "email_confidence", fmt.Sprintf("hunter_score_%d", int(score)))
	}
}

func extractDropleadsResult(r *WaterfallResult, data json.RawMessage, step *WaterfallStep) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	if email, ok := stringField(m, "email"); ok {
		setField(r, step, "email", email)
	}
	if status, ok := stringField(m, "status"); ok {
		setField(r, step, "email_confidence", status)
	}
	if co, ok := stringField(m, "company_domain"); ok {
		setField(r, step, "company_domain", co)
	}
}

func extractFlatContact(r *WaterfallResult, data json.RawMessage, step *WaterfallStep, emailKeys, phoneKeys []string) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	for _, k := range emailKeys {
		if v, ok := stringField(m, k); ok {
			setField(r, step, "email", v)
			break
		}
	}
	for _, k := range phoneKeys {
		if v, ok := stringField(m, k); ok {
			setField(r, step, "phone", v)
			break
		}
	}
}

func stringField(m map[string]any, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}
	return "", false
}

func setField(r *WaterfallResult, step *WaterfallStep, key, value string) {
	if value == "" {
		return
	}
	if _, already := r.Fields[key]; already {
		return
	}
	r.Fields[key] = value
	step.Fields = append(step.Fields, key)
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
