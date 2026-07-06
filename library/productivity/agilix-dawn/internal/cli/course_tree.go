// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: render a concept's full section/instruction/interaction tree.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type treeInstruction struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	Duration     float64 `json:"duration_seconds"`
	Points       int     `json:"points"`
	Interactions int     `json:"interactions"`
}

type treeSection struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Instructions []treeInstruction `json:"instructions"`
}

type treeView struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Status   string        `json:"status"`
	Sections []treeSection `json:"sections"`
}

// pp:data-source live
func newNovelCourseTreeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tree <id>",
		Short:       "Render a course's full section → instruction → interaction hierarchy as a tree.",
		Long:        "Render a course's full section → instruction → interaction hierarchy as a tree.\n\nUse to inspect a whole course's layout at once. Do NOT use for a flat title list; use 'concept list'.",
		Example:     "  agilix-dawn-pp-cli course tree c_f4bff87c0cab456984f2860af3e427d0 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "id=c_7666e422b4b94b3f840a250cb923a1a6"},
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
			view := treeView{ID: concept.ID, Title: concept.Title, Status: concept.Status}
			for _, s := range concept.Section {
				ts := treeSection{ID: s.ID, Title: s.Title, Status: s.Status}
				for _, in := range s.Instruction {
					ts.Instructions = append(ts.Instructions, treeInstruction{
						ID: in.ID, Title: in.Title, Type: in.Type, Status: in.Status,
						Duration: in.Duration, Points: in.Points, Interactions: in.interactionCount(),
					})
				}
				view.Sections = append(view.Sections, ts)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, view)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  (%s)  status=%s\n", view.Title, view.ID, view.Status)
			for si, s := range view.Sections {
				fmt.Fprintf(w, "├─ Section %d: %s  (%d instructions)\n", si+1, s.Title, len(s.Instructions))
				for _, in := range s.Instructions {
					fmt.Fprintf(w, "│    • %s  [%s, %.0fs, %dpts, %d interactions]\n",
						in.Title, in.Type, in.Duration, in.Points, in.Interactions)
				}
			}
			return nil
		},
	}
	return cmd
}
