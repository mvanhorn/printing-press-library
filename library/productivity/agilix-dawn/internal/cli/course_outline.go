// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: export a course's curriculum as Markdown or CSV.

package cli

import (
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCourseOutlineCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string

	cmd := &cobra.Command{
		Use:         "outline <id>",
		Short:       "Flatten a course's section/instruction tree into a Markdown or CSV curriculum.",
		Long:        "Flatten a course's section/instruction tree into a Markdown or CSV curriculum.\n\nUse to export a course's curriculum. Do NOT use for grades; Dawn exposes no grade collection here.",
		Example:     "  agilix-dawn-pp-cli course outline c_f4bff87c0cab456984f2860af3e427d0 --format md",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id=c_7666e422b4b94b3f840a250cb923a1a6;--format=md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch concept structure")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("id is required (a concept id, c_...)"))
			}
			format := flagFormat
			if format == "" {
				format = "md"
			}
			if format != "md" && format != "csv" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be 'md' or 'csv', got %q", format))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			concept, err := fetchConcept(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			w := cmd.OutOrStdout()
			if format == "csv" {
				cw := csv.NewWriter(w)
				_ = cw.Write([]string{"section", "instruction", "type", "duration_seconds", "points"})
				for _, s := range concept.Section {
					for _, in := range s.Instruction {
						_ = cw.Write([]string{s.Title, in.Title, in.Type,
							strconv.FormatFloat(in.Duration, 'f', -1, 64), strconv.Itoa(in.Points)})
					}
				}
				cw.Flush()
				return cw.Error()
			}
			// Markdown
			fmt.Fprintf(w, "# %s\n\n", concept.Title)
			fmt.Fprintf(w, "_Course %s · status: %s_\n\n", concept.ID, concept.Status)
			if concept.ShortDescription != "" {
				fmt.Fprintf(w, "%s\n\n", concept.ShortDescription)
			}
			for si, s := range concept.Section {
				fmt.Fprintf(w, "## %d. %s\n\n", si+1, s.Title)
				for _, in := range s.Instruction {
					fmt.Fprintf(w, "- **%s** _(%s, %.0fs, %d pts)_\n", in.Title, in.Type, in.Duration, in.Points)
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "md", "Output format: md or csv")
	return cmd
}
