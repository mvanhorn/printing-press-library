// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add guarded report submission with local validation and dry-run default.
// PATCH(asset-in-scope-helper): use wildcard-aware assetInScope helper (defined in
// transcend_helpers.go) instead of strings.Contains, which never matched *.foo.com
// wildcard scopes against concrete assets like api.foo.com.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/internal/cliutil"

	"github.com/spf13/cobra"
)

func newReportSubmitCmd(flags *rootFlags) *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:     "submit <draft-file>",
		Short:   "Validate and optionally submit a markdown report draft",
		Example: "  yeswehack-pp-cli report submit ./drafts/sqli-acme-2026-05.md\n  yeswehack-pp-cli report submit ./drafts/sqli-acme-2026-05.md --confirm",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			draft, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			parsed := parseDraft(string(draft))
			db, err := openDefaultStore()
			if err != nil {
				return err
			}
			defer db.Close()
			programs, err := selectedPrograms(db, parsed["program"])
			if err != nil {
				return err
			}
			inScope := false
			if len(programs) > 0 {
				for _, s := range scopesFromProgram(programs[0]) {
					if parsed["asset"] != "" && assetInScope(parsed["asset"], s.Asset) {
						inScope = true
					}
				}
			}
			collisions, err := reportDedupeCollisions(db, parsed["title"], parsed["asset"], "", 5)
			if err != nil {
				return err
			}
			body := map[string]any{
				"program":            parsed["program"],
				"title":              parsed["title"],
				"cvss_vector":        parsed["cvss_vector"],
				"asset":              parsed["asset"],
				"steps_to_reproduce": parsed["steps"],
				"impact":             parsed["impact"],
				"recommendation":     parsed["recommendation"],
				"references":         parsed["references"],
			}
			preview := map[string]any{"would_post": body, "asset_in_scope": inScope, "dedupe": collisions, "dry_run": !confirm || cliutil.IsVerifyEnv()}
			if !confirm || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
			}
			if !inScope {
				return usageErr(fmt.Errorf("draft asset is not in local scope for program %q", parsed["program"]))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, _, err := c.Post("/reports", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), resp, flags)
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually POST the report after validation")
	return cmd
}

func parseDraft(markdown string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(markdown, "\n")
	if len(lines) > 0 && lines[0] == "---" {
		for i := 1; i < len(lines); i++ {
			if lines[i] == "---" {
				break
			}
			kv := strings.SplitN(lines[i], ":", 2)
			if len(kv) == 2 {
				out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}
	out["title"] = sectionText(markdown, "# Title")
	out["cvss_vector"] = firstNonCommentLine(sectionText(markdown, "## CVSS vector"))
	out["asset"] = firstFencedLine(sectionText(markdown, "## Asset"))
	out["steps"] = sectionText(markdown, "## Steps to Reproduce")
	out["impact"] = sectionText(markdown, "## Impact")
	out["recommendation"] = sectionText(markdown, "## Recommendation")
	out["references"] = sectionText(markdown, "## References")
	return out
}

func sectionText(markdown, heading string) string {
	idx := strings.Index(markdown, heading)
	if idx < 0 {
		return ""
	}
	rest := markdown[idx+len(heading):]
	next := strings.Index(rest, "\n## ")
	if heading == "# Title" {
		next = strings.Index(rest, "\n## ")
	}
	if next >= 0 {
		rest = rest[:next]
	}
	return strings.TrimSpace(rest)
}

func firstNonCommentLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "<!--") && !strings.HasPrefix(line, "```") {
			return line
		}
	}
	return ""
}

func firstFencedLine(s string) string {
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence && line != "" {
			return line
		}
	}
	return firstNonCommentLine(s)
}
