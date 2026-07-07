// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newAdviceCmd(flags *rootFlags) *cobra.Command {
	var copyBlock bool
	var checkID string
	var limit int

	cmd := &cobra.Command{
		Use:   "advice <url>",
		Short: "Show the prioritized, copy-paste fixes to reach the next readiness level",
		Long: "Print the prioritized fixes (the scanner's next-level requirements) for a site's most\n" +
			"recent stored scan: a short description, the full fix prompt, the relevant spec URLs,\n" +
			"and a link to the SKILL.md guide for each. Pass --copy to print just the prompts as one\n" +
			"block ready to paste into a coding agent. Run 'check <url>' first, or pass --data-source\n" +
			"live to scan fresh.",
		Example: "  isitagentready-pp-cli advice https://example.com\n" +
			"  isitagentready-pp-cli advice https://example.com --copy",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show next-level fix advice for a URL")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. advice https://example.com"))
			}
			url := args[0]

			raw, err := resolveReport(cmd, flags, url)
			if err != nil {
				return err
			}
			rep, err := store.ParseReport(raw)
			if err != nil {
				return err
			}
			if rep.SiteError != nil {
				return apiErr(fmt.Errorf("%s could not be scanned (HTTP %d %s); no advice available",
					rep.URL, rep.SiteError.HTTPStatus, rep.SiteError.StatusText))
			}

			reqs := rep.NextLevel.Requirements
			if checkID != "" {
				var f []store.Requirement
				for _, r := range reqs {
					if r.Check == checkID {
						f = append(f, r)
					}
				}
				reqs = f
			}
			if limit > 0 && len(reqs) > limit {
				reqs = reqs[:limit]
			}

			if copyBlock {
				return renderAdviceCopy(cmd, rep, reqs)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), adviceItems(rep, reqs), flags)
			}
			return renderAdviceHuman(cmd, rep, reqs)
		},
	}
	cmd.Flags().BoolVar(&copyBlock, "copy", false, "Print just the fix prompts as one block ready to paste into a coding agent")
	cmd.Flags().StringVar(&checkID, "check", "", "Show advice for a single check only (e.g. mcpServerCard)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Show at most N fixes (0 = all)")
	return cmd
}

// adviceItem is the machine-readable shape of one fix.
type adviceItem struct {
	Check       string   `json:"check"`
	Description string   `json:"description"`
	ShortPrompt string   `json:"shortPrompt,omitempty"`
	Prompt      string   `json:"prompt"`
	SpecURLs    []string `json:"specUrls,omitempty"`
	SkillURL    string   `json:"skillUrl,omitempty"`
}

func adviceItems(rep *store.Report, reqs []store.Requirement) map[string]any {
	items := make([]adviceItem, 0, len(reqs))
	for _, r := range reqs {
		items = append(items, adviceItem{
			Check: r.Check, Description: r.Description, ShortPrompt: r.ShortPrompt,
			Prompt: r.Prompt, SpecURLs: r.SpecURLs, SkillURL: r.SkillURL,
		})
	}
	return map[string]any{
		"url":       rep.URL,
		"level":     rep.Level,
		"levelName": rep.LevelName,
		"nextLevel": rep.NextLevel.Name,
		"fixes":     items,
	}
}

func renderAdviceHuman(cmd *cobra.Command, rep *store.Report, reqs []store.Requirement) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, bold(rep.URL))
	fmt.Fprintf(out, "  Level %d — %s\n", rep.Level, rep.LevelName)
	if len(reqs) == 0 {
		if rep.NextLevel.Name == "" {
			fmt.Fprintln(out, "  No fixes listed: this site is at the top readiness level.")
		} else {
			fmt.Fprintln(out, "  No fixes match. The scanner listed none for the current filter.")
		}
		return nil
	}
	if rep.NextLevel.Name != "" {
		fmt.Fprintf(out, "  To reach %s, fix:\n", bold(rep.NextLevel.Name))
	}
	for i, r := range reqs {
		fmt.Fprintf(out, "\n  %d. [%s] %s\n", i+1, r.Check, r.Description)
		fmt.Fprintf(out, "     %s\n", r.Prompt)
		if len(r.SpecURLs) > 0 {
			fmt.Fprintf(out, "     spec: %s\n", strings.Join(r.SpecURLs, ", "))
		}
		if r.SkillURL != "" {
			fmt.Fprintf(out, "     guide: isitagentready-pp-cli guide %s\n", r.Check)
		}
	}
	return nil
}

// renderAdviceCopy prints a plain-text block ready to paste into a coding agent.
func renderAdviceCopy(cmd *cobra.Command, rep *store.Report, reqs []store.Requirement) error {
	out := cmd.OutOrStdout()
	next := rep.NextLevel.Name
	if next == "" {
		next = "the next readiness level"
	}
	if len(reqs) == 0 {
		fmt.Fprintf(out, "No outstanding agent-readiness fixes for %s (level %d, %s).\n", rep.URL, rep.Level, rep.LevelName)
		return nil
	}
	fmt.Fprintf(out, "Make %s more AI-agent-ready. It is currently level %d (%s); apply these fixes to reach %s:\n",
		rep.URL, rep.Level, rep.LevelName, next)
	for i, r := range reqs {
		fmt.Fprintf(out, "\n%d. %s\n%s\n", i+1, r.Description, r.Prompt)
		if len(r.SpecURLs) > 0 {
			fmt.Fprintf(out, "Reference: %s\n", strings.Join(r.SpecURLs, ", "))
		}
	}
	return nil
}
