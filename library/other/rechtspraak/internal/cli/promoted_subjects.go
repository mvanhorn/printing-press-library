// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newSubjectsPromotedCmd(flags *rootFlags) *cobra.Command {
	var treeView bool

	cmd := &cobra.Command{
		Use:         "subjects",
		Short:       "Subject-area (rechtsgebied) vocabulary, hierarchical",
		Long:        "Subject areas with PSI URIs, names, slugs, and parent links. --tree renders the hierarchy.",
		Example:     "  rechtspraak-pp-cli subjects\n  rechtspraak-pp-cli subjects --tree",
		Annotations: map[string]string{"pp:endpoint": "subjects.list", "pp:method": "GET", "pp:path": "/Waardelijst/Rechtsgebieden", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			idx, err := getSubjectIndex(cmd.Context())
			if err != nil {
				return err
			}
			subjects := idx.All()
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), subjects)
			}
			if treeView {
				printSubjectTree(io.Writer(cmd.OutOrStdout()), subjects)
				return nil
			}
			for _, s := range subjects {
				fmt.Fprintf(cmd.OutOrStdout(), "%-44s  %s\n", s.Slug, s.Name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&treeView, "tree", false, "Render as a hierarchical tree")
	return cmd
}

func printSubjectTree(w io.Writer, subjects []rechtspraak.Subject) {
	children := map[string][]rechtspraak.Subject{}
	for _, s := range subjects {
		children[s.Parent] = append(children[s.Parent], s)
	}
	for _, sl := range children {
		sort.SliceStable(sl, func(i, j int) bool { return sl[i].Name < sl[j].Name })
	}
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, s := range children[parent] {
			fmt.Fprintf(w, "%s%s\n", strings.Repeat("  ", depth), s.Name)
			walk(s.Identifier, depth+1)
		}
	}
	walk("", 0)
}
