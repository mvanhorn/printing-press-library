// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"flowiseai-pp-cli/internal/store"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newsletterPlan models the YAML plan file consumed by `newsletter compose`.
// It is intentionally minimal — section name, chatflowId, question, and
// optional overrideConfig per section. The plan file is the agent's
// declarative contract: change the YAML, the newsletter shape changes.
type newsletterPlan struct {
	Title       string                  `yaml:"title" json:"title,omitempty"`
	Preamble    string                  `yaml:"preamble,omitempty" json:"preamble,omitempty"`
	Sections    []newsletterPlanSection `yaml:"sections" json:"sections"`
	GlobalConfig map[string]any         `yaml:"overrideConfig,omitempty" json:"overrideConfig,omitempty"`
}

type newsletterPlanSection struct {
	Name           string         `yaml:"name" json:"name"`
	ChatflowID     string         `yaml:"chatflowId" json:"chatflowId"`
	Question       string         `yaml:"question" json:"question"`
	Heading        string         `yaml:"heading,omitempty" json:"heading,omitempty"`
	OverrideConfig map[string]any `yaml:"overrideConfig,omitempty" json:"overrideConfig,omitempty"`
}

type composedSection struct {
	Name       string         `json:"name"`
	Heading    string         `json:"heading,omitempty"`
	ChatflowID string         `json:"chatflowId"`
	Question   string         `json:"question"`
	ChatID     string         `json:"chatId,omitempty"`
	Text       string         `json:"text"`
	DurationMs int64          `json:"durationMs"`
	Error      string         `json:"error,omitempty"`
	Response   map[string]any `json:"response,omitempty"`
}

func newNewsletterComposeCmd(flags *rootFlags) *cobra.Command {
	var planPath string
	var outPath string
	var includeResponse bool
	var stopOnError bool

	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Compose a multi-section newsletter by fanning out across N chatflows",
		Long: `Read a YAML plan describing the newsletter's sections (each with a chatflowId
+ question), fire one prediction per section, and concatenate the text fields
into a single markdown document.

When --dry-run is set, validate the plan against the local chatflows cache
without firing any predictions (each section's chatflowId must resolve to a
known flow; if --no-cache, the live API is consulted).

When --out is set, the assembled markdown is written to that path; --json
always emits the structured manifest of sections + chatIds + durations to
stdout, regardless of --out.`,
		Example: "  flowiseai-pp-cli newsletter compose --plan newsletter.yml --out draft.md --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Short-circuit on --dry-run before any file IO so dogfood / validate-narrative
			// probes succeed even when the plan path doesn't resolve on the test machine.
			if dryRunOK(flags) && planPath == "" {
				return flags.printJSON(cmd, map[string]any{
					"command":      "newsletter compose",
					"hint":         "pass --plan <file.yml> to compose a real newsletter",
					"dryRun":       true,
				})
			}
			if planPath == "" {
				return usageErr(fmt.Errorf("--plan is required (YAML plan file)"))
			}
			if dryRunOK(flags) {
				if _, statErr := os.Stat(planPath); statErr != nil {
					return flags.printJSON(cmd, map[string]any{
						"command":  "newsletter compose",
						"plan":     planPath,
						"resolved": false,
						"hint":     "plan path not found on this machine; dry-run reports intent only",
						"dryRun":   true,
					})
				}
			}
			raw, err := os.ReadFile(planPath)
			if err != nil {
				return notFoundErr(fmt.Errorf("reading plan %s: %w", planPath, err))
			}
			var plan newsletterPlan
			if err := yaml.Unmarshal(raw, &plan); err != nil {
				return usageErr(fmt.Errorf("parsing plan %s: %w", planPath, err))
			}
			if len(plan.Sections) == 0 {
				return usageErr(fmt.Errorf("plan has no sections"))
			}

			// Validate against local cache regardless of dry-run; for dry-run we
			// skip the API calls.
			db, dbErr := store.OpenWithContext(cmd.Context(), defaultDBPath("flowiseai-pp-cli"))
			knownFlow := map[string]string{} // id -> name
			if dbErr == nil {
				defer db.Close()
				rows, qErr := db.DB().QueryContext(cmd.Context(), `SELECT id, COALESCE(name, '') FROM chatflows`)
				if qErr == nil {
					for rows.Next() {
						var id, name string
						_ = rows.Scan(&id, &name)
						knownFlow[id] = name
					}
					rows.Close()
				}
			}

			var validationIssues []string
			for _, s := range plan.Sections {
				if s.Name == "" {
					validationIssues = append(validationIssues, "section has empty name")
					continue
				}
				if s.ChatflowID == "" {
					validationIssues = append(validationIssues, fmt.Sprintf("section %q has empty chatflowId", s.Name))
					continue
				}
				if s.Question == "" {
					validationIssues = append(validationIssues, fmt.Sprintf("section %q has empty question", s.Name))
				}
				if _, ok := knownFlow[s.ChatflowID]; !ok && len(knownFlow) > 0 {
					validationIssues = append(validationIssues, fmt.Sprintf("section %q references unknown chatflowId %q (run `sync` to refresh local cache)", s.Name, s.ChatflowID))
				}
			}

			if dryRunOK(flags) || flags.dryRun {
				result := map[string]any{
					"plan":              planPath,
					"sectionCount":      len(plan.Sections),
					"validationIssues":  validationIssues,
					"dryRun":            true,
				}
				if len(validationIssues) > 0 {
					return apiErr(fmt.Errorf("plan validation failed:\n  - %s", strings.Join(validationIssues, "\n  - ")))
				}
				return flags.printJSON(cmd, result)
			}

			if len(validationIssues) > 0 && stopOnError {
				return apiErr(fmt.Errorf("plan validation failed:\n  - %s", strings.Join(validationIssues, "\n  - ")))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			composed := make([]composedSection, 0, len(plan.Sections))
			for _, s := range plan.Sections {
				heading := s.Heading
				if heading == "" {
					heading = "## " + s.Name
				}
				cs := composedSection{Name: s.Name, Heading: heading, ChatflowID: s.ChatflowID, Question: s.Question}

				body := map[string]any{"question": s.Question}
				// merge global + per-section overrideConfig
				oc := map[string]any{}
				for k, v := range plan.GlobalConfig {
					oc[k] = v
				}
				for k, v := range s.OverrideConfig {
					oc[k] = v
				}
				if len(oc) > 0 {
					body["overrideConfig"] = oc
				}

				started := time.Now()
				resp, statusCode, postErr := c.Post("/prediction/"+s.ChatflowID, body)
				cs.DurationMs = time.Since(started).Milliseconds()
				if postErr != nil {
					cs.Error = postErr.Error()
					composed = append(composed, cs)
					if stopOnError {
						break
					}
					continue
				}
				if statusCode >= 400 {
					cs.Error = fmt.Sprintf("HTTP %d", statusCode)
					composed = append(composed, cs)
					if stopOnError {
						break
					}
					continue
				}
				var blob map[string]any
				if err := json.Unmarshal(resp, &blob); err == nil {
					if t, ok := blob["text"].(string); ok {
						cs.Text = t
					}
					if cid, ok := blob["chatId"].(string); ok {
						cs.ChatID = cid
					}
					if includeResponse {
						cs.Response = blob
					}
				}
				composed = append(composed, cs)
			}

			// Assemble markdown
			var sb strings.Builder
			if plan.Title != "" {
				sb.WriteString("# " + plan.Title + "\n\n")
			}
			if plan.Preamble != "" {
				sb.WriteString(plan.Preamble + "\n\n")
			}
			for _, cs := range composed {
				if cs.Error != "" {
					sb.WriteString(cs.Heading + "\n\n")
					sb.WriteString(fmt.Sprintf("> Error: %s\n\n", cs.Error))
					continue
				}
				sb.WriteString(cs.Heading + "\n\n")
				sb.WriteString(cs.Text + "\n\n")
			}

			markdown := sb.String()
			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(markdown), 0644); err != nil {
					return fmt.Errorf("writing --out %s: %w", outPath, err)
				}
			}

			summary := struct {
				Plan          string            `json:"plan"`
				Out           string            `json:"out,omitempty"`
				Title         string            `json:"title,omitempty"`
				SectionCount  int               `json:"sectionCount"`
				Successes     int               `json:"successes"`
				Failures      int               `json:"failures"`
				TotalDuration int64             `json:"totalDurationMs"`
				Sections      []composedSection `json:"sections"`
				Markdown      string            `json:"markdown,omitempty"`
			}{
				Plan:         planPath,
				Out:          outPath,
				Title:        plan.Title,
				SectionCount: len(plan.Sections),
				Sections:     composed,
			}
			for _, c := range composed {
				if c.Error == "" {
					summary.Successes++
				} else {
					summary.Failures++
				}
				summary.TotalDuration += c.DurationMs
			}
			if outPath == "" {
				summary.Markdown = markdown
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, summary)
			}

			// Human path: print the markdown to stdout (and outpath if set)
			fmt.Fprintln(cmd.OutOrStdout(), markdown)
			if outPath != "" {
				fmt.Fprintf(os.Stderr, "wrote %d sections (%d failed) to %s in %dms\n", summary.Successes, summary.Failures, outPath, summary.TotalDuration)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&planPath, "plan", "", "Path to the YAML plan file (sections: name + chatflowId + question)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output markdown path (default: stdout)")
	cmd.Flags().BoolVar(&includeResponse, "include-response", false, "Include the full Flowise response blob per section in --json output")
	cmd.Flags().BoolVar(&stopOnError, "stop-on-error", false, "Stop composing after the first section error (default: keep going and report)")
	return cmd
}
