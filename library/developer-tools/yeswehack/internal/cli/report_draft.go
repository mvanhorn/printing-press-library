// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add local-only markdown report draft generation.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newReportDraftCmd(flags *rootFlags) *cobra.Command {
	var output, severity string
	cmd := &cobra.Command{
		Use:   "draft <program-slug>",
		Short: "Create a local markdown vulnerability report draft without posting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			db, err := openDefaultStore()
			if err != nil {
				return err
			}
			defer db.Close()
			programs, err := selectedPrograms(db, slug)
			if err != nil {
				return err
			}
			if len(programs) == 0 {
				return notFoundErr(fmt.Errorf("program %q not found in local store", slug))
			}
			program := programs[0]
			if output == "" {
				output = fmt.Sprintf("./yeswehack-draft-%s-%d.md", slug, time.Now().Unix())
			}
			var assets []string
			for _, s := range scopesFromProgram(program) {
				assets = append(assets, s.Asset)
			}
			body := buildDraftMarkdown(slug, program, assets, severity)
			if err := os.WriteFile(output, []byte(body), 0o600); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Draft output path")
	cmd.Flags().StringVar(&severity, "severity", "", "Optional CVSS vector")
	return cmd
}

func buildDraftMarkdown(slug string, program map[string]any, assets []string, severity string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nprogram: %s\nprogram_name: %s\nreward_max: %.0f\n---\n\n", slug, programName(program), rewardMax(program))
	b.WriteString("# Title\n\n")
	b.WriteString("<!-- concise vulnerability title -->\n\n")
	b.WriteString("## CVSS vector\n\n")
	if severity != "" {
		fmt.Fprintf(&b, "%s\n\n", severity)
	} else {
		b.WriteString("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:N\n\n")
	}
	b.WriteString("## Asset\n\n")
	b.WriteString("```text\n")
	for _, asset := range assets {
		fmt.Fprintf(&b, "%s\n", asset)
	}
	b.WriteString("```\n\n")
	b.WriteString("## Steps to Reproduce\n\n1. \n2. \n3. \n\n")
	b.WriteString("## Impact\n\n\n")
	b.WriteString("## Recommendation\n\n\n")
	b.WriteString("## References\n\n- \n")
	return b.String()
}
