// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type watchOrderStep struct {
	Position int    `json:"position"`
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Relation string `json:"relation,omitempty"`
	Type     string `json:"type,omitempty"`
	Aired    string `json:"aired,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// newNovelWatchOrderCmd flattens MyAnimeList's per-title relation graph into a
// suggested viewing order with each edge labelled.
func newNovelWatchOrderCmd(flags *rootFlags) *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:     "watch-order <id>",
		Short:   "Turn a franchise's relation graph into a suggested watch order",
		Example: "  myanimelist-pp-cli watch-order 5114 --json",
		Annotations: map[string]string{
			"mcp:read-only":     "true",
			"pp:data-source":    "live",
			"pp:happy-args":     "id=5114;--depth=1",
			"pp:novel-scaffold": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch-order")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an anime id is required, e.g. myanimelist-pp-cli watch-order 5114"))
			}
			root, err := malIntArg(args[0], "id")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			type node struct {
				id       int
				title    string
				relation string
				typ      string
				aired    string
				depth    int
			}
			seen := map[int]bool{root: true}
			nodes := map[int]node{}
			frontier := []int{root}
			if isDogfoodEnv() && depth > 1 {
				depth = 1
			}
			for d := 0; d < depth && len(frontier) > 0; d++ {
				next := make([]int, 0, 8)
				for _, id := range frontier {
					detail, derr := malDetail(ctx, flags, "anime", id)
					if derr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read anime %d: %v\n", id, derr)
						continue
					}
					if _, ok := nodes[id]; !ok {
						nodes[id] = node{id: id, title: detail.Title, typ: detail.Type, aired: detail.Aired, relation: "root"}
					}
					for _, rel := range detail.Related {
						if rel.Kind != "anime" || seen[rel.ID] {
							continue
						}
						seen[rel.ID] = true
						nodes[rel.ID] = node{id: rel.ID, title: rel.Title, relation: rel.Relation, depth: d + 1}
						next = append(next, rel.ID)
					}
				}
				frontier = next
			}
			steps := make([]watchOrderStep, 0, len(nodes))
			for _, n := range nodes {
				steps = append(steps, watchOrderStep{ID: n.id, Title: n.title, Relation: n.relation, Type: n.typ, Aired: n.aired})
			}
			sort.SliceStable(steps, func(i, j int) bool {
				ri, rj := relationRank(steps[i].Relation), relationRank(steps[j].Relation)
				if ri != rj {
					return ri < rj
				}
				return steps[i].Title < steps[j].Title
			})
			for i := range steps {
				steps[i].Position = i + 1
				steps[i].Reason = relationRankReason(steps[i].Relation)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), steps, flags)
			}
			table := make([]map[string]any, 0, len(steps))
			for _, s := range steps {
				table = append(table, map[string]any{"step": s.Position, "title": s.Title, "relation": s.Relation, "why": s.Reason})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 2, "How many relation hops to walk (1 = direct relations only)")
	return cmd
}

// relationRank orders MAL's relation labels from "watch first" to "optional".
func relationRank(relation string) int {
	switch relation {
	case "root":
		return 1
	case "Prequel":
		return 0
	case "Parent Story", "Full Story":
		return 2
	case "Sequel":
		return 3
	case "Side Story", "Spin-Off":
		return 4
	case "Summary":
		return 6
	case "Alternative Version", "Alternative Setting":
		return 7
	case "Adaptation", "Character", "Other":
		return 8
	}
	return 5
}

func relationRankReason(relation string) string {
	switch relation {
	case "root":
		return "the title you asked about"
	case "Prequel":
		return "set before the title you asked about; watch first"
	case "Sequel":
		return "continues the story; watch after"
	case "Side Story", "Spin-Off":
		return "optional side content"
	case "Summary":
		return "recap; safe to skip"
	}
	return "related entry"
}
