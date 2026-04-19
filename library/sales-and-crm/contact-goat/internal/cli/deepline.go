// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Deepline subcommands: person/company search and enrich via the Deepline v2
// API (https://code.deepline.com/). The hybrid client prefers the official
// `deepline` CLI subprocess when present and falls back to direct HTTP.
//
// Every execute call costs credits, so every subcommand:
//   - prints an estimated cost before sending
//   - supports --dry-run to show the request without spending credits
//   - supports --yes to skip the confirmation gate (required for agents)
//   - logs the call (tool id, payload hash, estimated cost) to the local
//     store for budget tracking. The payload itself is never persisted
//     because it may contain PII. The API key value is never logged.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"
	"github.com/spf13/cobra"
)

type deeplineFlags struct {
	apiKey string
}

func newDeeplineCmd(flags *rootFlags) *cobra.Command {
	dl := &deeplineFlags{}

	cmd := &cobra.Command{
		Use:   "deepline",
		Short: "Deepline contact-data API: email, phone, and company enrichment (credit-priced)",
		Long: `Call Deepline (https://code.deepline.com/) contact-data tools from the terminal.

Auth: set DEEPLINE_API_KEY or pass --deepline-key. Keys start with "dlp_".

Hybrid execution: if the official 'deepline' CLI is on PATH it is used as a
subprocess (keeps behavior consistent with upstream). Otherwise the CLI falls
back to direct HTTPS calls against https://code.deepline.com/api/v2.

Deepline charges credits per execute call. Every subcommand prints an
estimated cost before spending and supports --dry-run to preview the request.`,
		Example: `  # Find Patrick Collison's email via the waterfall
  contact-goat-pp-cli deepline find-email "Patrick Collison" --company stripe.com --yes

  # Search Apollo for VP Eng in SF, 25 results
  contact-goat-pp-cli deepline search-people --title "VP Engineering" --location "San Francisco" --limit 25

  # Enrich a company by domain
  contact-goat-pp-cli deepline enrich-company stripe.com --yes`,
	}

	cmd.PersistentFlags().StringVar(&dl.apiKey, "deepline-key", "", "Deepline API key (default from $DEEPLINE_API_KEY)")

	cmd.AddCommand(newDeeplineFindEmailCmd(flags, dl))
	cmd.AddCommand(newDeeplineSearchPeopleCmd(flags, dl))
	cmd.AddCommand(newDeeplineEmailFindCmd(flags, dl))
	cmd.AddCommand(newDeeplinePhoneFindCmd(flags, dl))
	cmd.AddCommand(newDeeplineSearchCompaniesCmd(flags, dl))
	cmd.AddCommand(newDeeplineEnrichCompanyCmd(flags, dl))
	cmd.AddCommand(newDeeplineEnrichPersonCmd(flags, dl))
	cmd.AddCommand(newDeeplineExecuteCmd(flags, dl))
	cmd.AddCommand(newDeeplineCreditsCmd(flags, dl))

	return cmd
}

// resolveDeeplineKey returns the API key to use (flag wins, env fallback).
// The value is never logged anywhere.
func (dl *deeplineFlags) resolveKey() string {
	if dl.apiKey != "" {
		return dl.apiKey
	}
	return os.Getenv("DEEPLINE_API_KEY")
}

// deeplineExecute is the shared "estimate cost, maybe dry-run, confirm,
// execute, log" pipeline used by every typed subcommand.
func deeplineExecute(cmd *cobra.Command, flags *rootFlags, dl *deeplineFlags, toolID string, payload map[string]any) error {
	client := deepline.NewClient(dl.resolveKey())

	cost, _ := client.EstimateCost(toolID, payload)
	payloadHash := hashPayload(payload)

	if flags.dryRun {
		logDeeplineSafely(toolID, payloadHash, cost, "dry-run")
		return emitDeeplineDryRun(cmd, flags, toolID, payload, cost)
	}

	// Confirmation gate: agent/script must pass --yes. No TTY prompt.
	if !flags.yes {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"This call will cost ~%d credit(s) for tool %q. Re-run with --yes to spend credits, or --dry-run to preview.\n",
			cost, toolID)
		return usageErr(fmt.Errorf("confirmation required: pass --yes to proceed"))
	}

	if err := client.ValidateKey(); err != nil {
		logDeeplineSafely(toolID, payloadHash, cost, "auth-error")
		if errors.Is(err, deepline.ErrMissingKey) {
			return authErr(fmt.Errorf("%w\nhint: export DEEPLINE_API_KEY=dlp_...\n      or pass --deepline-key dlp_...", err))
		}
		return authErr(err)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
	defer cancel()

	result, err := client.Execute(ctx, toolID, payload)
	if err != nil {
		logDeeplineSafely(toolID, payloadHash, cost, "error")
		return apiErr(err)
	}

	logDeeplineSafely(toolID, payloadHash, cost, "ok")

	return emitDeeplineResult(cmd, flags, result)
}

// hashPayload returns a stable SHA-256 prefix of the canonical JSON payload.
// Used for local budget bookkeeping; never includes key material.
func hashPayload(payload map[string]any) string {
	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16]) // 32-char prefix is plenty for dedupe
}

// logDeeplineSafely appends to the local deepline_log. A missing store is a
// soft failure: we warn to stderr and continue (the call itself still
// succeeded). We never log the payload body or API key value.
func logDeeplineSafely(toolID, payloadHash string, cost int, status string) {
	dbPath := defaultDBPath("contact-goat-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: deepline log open failed: %v\n", err)
		return
	}
	defer s.Close()
	if err := s.LogDeeplineCall(toolID, payloadHash, cost, status); err != nil {
		fmt.Fprintf(os.Stderr, "warning: deepline log write failed: %v\n", err)
	}
}

// emitDeeplineDryRun prints the request the CLI *would* send.
func emitDeeplineDryRun(cmd *cobra.Command, flags *rootFlags, toolID string, payload map[string]any, cost int) error {
	req := map[string]any{
		"dry_run":          true,
		"tool_id":          toolID,
		"payload":          payload,
		"estimated_credit": cost,
		"endpoint":         fmt.Sprintf("%s/integrations/%s/execute", deepline.BaseURL, toolID),
	}
	if flags.asJSON {
		return flags.printJSON(cmd, req)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "dry-run: tool %s\n", toolID)
	fmt.Fprintf(w, "estimated cost: ~%d credit(s)\n", cost)
	b, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Fprintf(w, "payload:\n%s\n", string(b))
	return nil
}

// emitDeeplineResult pretty-prints by default, emits raw JSON when --json is set.
func emitDeeplineResult(cmd *cobra.Command, flags *rootFlags, result json.RawMessage) error {
	if flags.asJSON {
		// Re-parse to normalize and respect --compact downstream, but raw is fine.
		var v any
		if err := json.Unmarshal(result, &v); err != nil {
			// Fall back to raw bytes.
			fmt.Fprintln(cmd.OutOrStdout(), string(result))
			return nil
		}
		return flags.printJSON(cmd, v)
	}
	var v any
	if err := json.Unmarshal(result, &v); err != nil {
		fmt.Fprintln(cmd.OutOrStdout(), string(result))
		return nil
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}

// --- Typed subcommands ---

func newDeeplineFindEmailCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var company string
	cmd := &cobra.Command{
		Use:   "find-email <name>",
		Short: "Find a person's email via the waterfall (Apollo + fallbacks)",
		Long:  "Runs Deepline's person_search_to_email_waterfall tool: resolves name+company to an email using multiple providers in cost order.",
		Example: `  contact-goat-pp-cli deepline find-email "Patrick Collison" --company stripe.com --yes
  contact-goat-pp-cli deepline find-email "Brian Chesky" --company airbnb.com --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(company) == "" {
				return usageErr(fmt.Errorf("--company is required (e.g. --company stripe.com)"))
			}
			payload := map[string]any{
				"name":    args[0],
				"company": company,
			}
			return deeplineExecute(cmd, flags, dl, deepline.ToolPersonSearchToEmailWaterfall, payload)
		},
	}
	cmd.Flags().StringVar(&company, "company", "", "Company domain (e.g. stripe.com)")
	return cmd
}

func newDeeplineSearchPeopleCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var title, location, industry string
	var limit int
	cmd := &cobra.Command{
		Use:   "search-people",
		Short: "Search Apollo for people matching title/location/industry",
		Example: `  contact-goat-pp-cli deepline search-people --title "VP Engineering" --location "San Francisco" --limit 25 --yes
  contact-goat-pp-cli deepline search-people --industry fintech --limit 50 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{}
			if title != "" {
				payload["title"] = title
			}
			if location != "" {
				payload["location"] = location
			}
			if industry != "" {
				payload["industry"] = industry
			}
			if limit > 0 {
				payload["limit"] = limit
			}
			if len(payload) == 0 {
				return usageErr(fmt.Errorf("at least one of --title, --location, --industry is required"))
			}
			return deeplineExecute(cmd, flags, dl, deepline.ToolApolloPeopleSearch, payload)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Job title filter (e.g. \"VP Engineering\")")
	cmd.Flags().StringVar(&location, "location", "", "Location filter (e.g. \"San Francisco\")")
	cmd.Flags().StringVar(&industry, "industry", "", "Industry filter (e.g. fintech)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results (cost scales with limit)")
	return cmd
}

func newDeeplineEmailFindCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var domain string
	cmd := &cobra.Command{
		Use:     "email-find",
		Short:   "Find public emails for a company domain",
		Example: `  contact-goat-pp-cli deepline email-find --domain stripe.com --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(domain) == "" {
				return usageErr(fmt.Errorf("--domain is required (e.g. --domain stripe.com)"))
			}
			payload := map[string]any{"domain": domain}
			return deeplineExecute(cmd, flags, dl, deepline.ToolEmailFind, payload)
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "Company domain (e.g. stripe.com)")
	return cmd
}

func newDeeplinePhoneFindCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var linkedinURL string
	cmd := &cobra.Command{
		Use:     "phone-find",
		Short:   "Find a phone number by LinkedIn URL",
		Example: `  contact-goat-pp-cli deepline phone-find --linkedin-url https://www.linkedin.com/in/patrickcollison --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(linkedinURL) == "" {
				return usageErr(fmt.Errorf("--linkedin-url is required"))
			}
			payload := map[string]any{"linkedin_url": linkedinURL}
			return deeplineExecute(cmd, flags, dl, deepline.ToolPhoneFind, payload)
		},
	}
	cmd.Flags().StringVar(&linkedinURL, "linkedin-url", "", "LinkedIn profile URL")
	return cmd
}

func newDeeplineSearchCompaniesCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var industry, size, location string
	cmd := &cobra.Command{
		Use:     "search-companies",
		Short:   "Search for companies by industry/size/location",
		Example: `  contact-goat-pp-cli deepline search-companies --industry fintech --size 201-500 --location "United States" --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{}
			if industry != "" {
				payload["industry"] = industry
			}
			if size != "" {
				payload["size"] = size
			}
			if location != "" {
				payload["location"] = location
			}
			if len(payload) == 0 {
				return usageErr(fmt.Errorf("at least one of --industry, --size, --location is required"))
			}
			return deeplineExecute(cmd, flags, dl, deepline.ToolCompanySearch, payload)
		},
	}
	cmd.Flags().StringVar(&industry, "industry", "", "Industry filter (e.g. fintech)")
	cmd.Flags().StringVar(&size, "size", "", "Headcount bucket (e.g. 201-500)")
	cmd.Flags().StringVar(&location, "location", "", "Location filter (e.g. \"United States\")")
	return cmd
}

func newDeeplineEnrichCompanyCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrich-company <domain>",
		Short: "Enrich a company by domain (firmographics, HQ, headcount)",
		Example: `  contact-goat-pp-cli deepline enrich-company stripe.com --yes
  contact-goat-pp-cli deepline enrich-company airbnb.com --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{"domain": args[0]}
			return deeplineExecute(cmd, flags, dl, deepline.ToolCompanyEnrich, payload)
		},
	}
	return cmd
}

func newDeeplineEnrichPersonCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enrich-person <linkedin-url>",
		Short:   "Enrich a person by LinkedIn URL (title, company, location)",
		Example: `  contact-goat-pp-cli deepline enrich-person https://www.linkedin.com/in/patrickcollison --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := map[string]any{"linkedin_url": args[0]}
			return deeplineExecute(cmd, flags, dl, deepline.ToolPersonEnrich, payload)
		},
	}
	return cmd
}

func newDeeplineExecuteCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	var payloadArg string
	cmd := &cobra.Command{
		Use:   "execute <toolID>",
		Short: "Generic passthrough: call any Deepline tool with an arbitrary JSON payload",
		Long: `Generic passthrough for Deepline tools not covered by a typed subcommand.
Pass the payload inline as JSON, or read it from a file with @path.`,
		Example: `  contact-goat-pp-cli deepline execute company-enrich --payload '{"domain":"stripe.com"}' --yes
  contact-goat-pp-cli deepline execute person-enrich --payload @./person.json --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolID := args[0]
			if strings.TrimSpace(payloadArg) == "" {
				return usageErr(fmt.Errorf("--payload is required (inline JSON or @file.json)"))
			}

			raw := []byte(payloadArg)
			if strings.HasPrefix(payloadArg, "@") {
				b, err := os.ReadFile(strings.TrimPrefix(payloadArg, "@"))
				if err != nil {
					return usageErr(fmt.Errorf("reading payload file: %w", err))
				}
				raw = b
			}

			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				return usageErr(fmt.Errorf("payload is not valid JSON object: %w", err))
			}

			return deeplineExecute(cmd, flags, dl, toolID, payload)
		},
	}
	cmd.Flags().StringVar(&payloadArg, "payload", "", "JSON payload (inline or @file.json)")
	return cmd
}

func newDeeplineCreditsCmd(flags *rootFlags, dl *deeplineFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "credits",
		Short: "Show credit balance (stub: upstream endpoint not confirmed)",
		Long: `STUB. Deepline has no confirmed credit-balance endpoint at time of writing,
so this command always exits with an error. See https://code.deepline.com/docs/quickstart
for upstream docs; when an endpoint is published, wire it into
internal/deepline/client.go#GetCredits.

Meanwhile, the CLI logs every execute call to ~/.local/share/contact-goat-pp-cli/data.db
so you can sum local estimated spend with the 'analytics' command.`,
		Example: `  contact-goat-pp-cli deepline credits`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := deepline.NewClient(dl.resolveKey())
			ctx, cancel := context.WithTimeout(cmd.Context(), flags.timeout)
			defer cancel()
			_, err := client.GetCredits(ctx)
			// Surface the stub message with a non-zero exit code; include
			// locally-tracked spend as a fallback signal.
			dbPath := defaultDBPath("contact-goat-pp-cli")
			local := 0
			if s, openErr := store.Open(dbPath); openErr == nil {
				local = s.SumDeeplineCost(24 * 30) // last 30 days
				s.Close()
			}
			if flags.asJSON {
				_ = flags.printJSON(cmd, map[string]any{
					"status":              "stub",
					"error":               err.Error(),
					"local_spend_30d":     local,
					"note":                "upstream credit endpoint not yet wired; see upstream docs",
					"upstream_docs":       "https://code.deepline.com/docs/quickstart",
					"subprocess_detected": client.SubprocessAvailable(),
				})
				return apiErr(err)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, yellow("credits: not yet wired, see upstream"))
			fmt.Fprintln(w, "  upstream docs: https://code.deepline.com/docs/quickstart")
			fmt.Fprintf(w, "  locally-tracked estimated spend (last 30d): %d credit(s)\n", local)
			return apiErr(err)
		},
	}
}
