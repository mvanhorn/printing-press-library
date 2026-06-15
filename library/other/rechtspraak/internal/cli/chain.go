// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelChainCmd(flags *rootFlags) *cobra.Command {
	var flagDepth int
	var flagDirection string

	cmd := &cobra.Command{
		Use:   "chain <ecli>",
		Short: "Walk the cassation, conclusie, and eerdere-aanleg graph for a decision",
		Long: `Recursively walk dcterms:relation edges (psi:cassatie, psi:conclusie,
psi:eerdereAanleg, dcterms:replaces) starting from a given ECLI. Each edge
is labeled with its relation type and outcome (AfhandelingsWijze) drawn
from the FormeleRelaties vocabulary.

Direction controls how the walk fans out:
  both    - default; traverse every relation
  up      - only follow earlier-aanleg / cassatie edges (toward higher courts)
  down    - only follow latere-aanleg edges (toward the original instance)

Tree output is the default; --json emits a structured graph that an agent
can consume directly.`,
		Example: `  rechtspraak-pp-cli chain ECLI:NL:HR:2024:1
  rechtspraak-pp-cli chain ECLI:NL:HR:2024:1 --depth 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			rootECLI := args[0]
			if _, err := rechtspraak.ParseECLI(rootECLI); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			http := mustHTTP()
			visited := map[string]bool{}
			tree, walkErr := walkChain(ctx, http, rootECLI, flagDepth, flagDirection, visited)
			// Emit the partial tree on cancellation so the user sees how
			// far the walk got, THEN surface the typed error so agents
			// and scripts see a non-zero exit. Bailing on walkErr before
			// printing the tree would hide the partial result the user
			// usually wants.
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				if outErr := writeJSONOut(cmd.OutOrStdout(), tree); outErr != nil {
					return outErr
				}
			} else {
				printChainTree(cmd.OutOrStdout(), tree, 0)
			}
			if walkErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "chain: walk interrupted (%v); partial tree emitted above.\n", walkErr)
				return walkErr
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDepth, "depth", 3, "Maximum recursion depth (default 3)")
	cmd.Flags().StringVar(&flagDirection, "direction", "both", "Walk direction: both | up | down")
	return cmd
}

// ChainNode is the structured shape for one node in the appeal chain.
type ChainNode struct {
	ECLI         string      `json:"ecli"`
	Court        string      `json:"court,omitempty"`
	CourtCode    string      `json:"court_code,omitempty"`
	DecisionDate string      `json:"decision_date,omitempty"`
	Type         string      `json:"type,omitempty"`
	Procedure    string      `json:"procedure,omitempty"`
	Edges        []ChainEdge `json:"edges,omitempty"`
	Error        string      `json:"error,omitempty"`
}

// ChainEdge describes how this node connects to a related node.
type ChainEdge struct {
	Relation string     `json:"relation,omitempty"`
	Outcome  string     `json:"outcome,omitempty"`
	Aanleg   string     `json:"aanleg,omitempty"`
	Label    string     `json:"label,omitempty"`
	Target   *ChainNode `json:"target"`
}

func walkChain(ctx context.Context, http *rechtspraak.HTTP, ecli string, depth int, direction string, visited map[string]bool) (*ChainNode, error) {
	if visited[ecli] {
		return &ChainNode{ECLI: ecli, Court: "(already visited)"}, nil
	}
	visited[ecli] = true
	node := &ChainNode{ECLI: ecli}
	if depth < 0 {
		return node, nil
	}
	// Honour context cancellation before each potentially expensive HTTP
	// call. Without this the walk silently records every error in
	// node.Error and returns nil at the top — RunE then exits 0 even when
	// the deadline fired mid-walk. An agent passing --timeout 5s would
	// never detect the truncated output.
	if err := ctx.Err(); err != nil {
		node.Error = err.Error()
		return node, err
	}
	d, err := http.Get(ctx, ecli, false)
	if err != nil {
		node.Error = err.Error()
		// Distinguish context errors (propagate so the outer RunE exits
		// non-zero) from transient per-ECLI fetch failures (which stay
		// embedded in the leaf node's Error field and the walk continues).
		if ctxErr := ctx.Err(); ctxErr != nil {
			return node, ctxErr
		}
		return node, nil
	}
	node.Court = d.Court
	node.DecisionDate = d.DecisionDate
	node.Type = d.Type
	node.Procedure = d.Procedure
	if p, perr := rechtspraak.ParseECLI(d.ECLI); perr == nil {
		node.CourtCode = p.Court
	}
	if depth == 0 {
		return node, nil
	}
	for _, rel := range d.Relations {
		if !directionAllows(rel.Aanleg, direction) {
			continue
		}
		edge := ChainEdge{
			Relation: shortFromURI(rel.TypeRelatie),
			Outcome:  shortFromURI(rel.Gevolg),
			Aanleg:   shortFromURI(rel.Aanleg),
			Label:    rel.Label,
		}
		if rel.Target == "" {
			edge.Target = &ChainNode{Court: "(no target)"}
			node.Edges = append(node.Edges, edge)
			continue
		}
		child, childErr := walkChain(ctx, http, rel.Target, depth-1, direction, visited)
		edge.Target = child
		node.Edges = append(node.Edges, edge)
		// Propagate context errors up the recursion so the partial tree
		// reaches the caller AND the RunE returns non-zero. Non-ctx
		// fetch errors stay in child.Error and the walk continues across
		// sibling edges.
		if childErr != nil && ctx.Err() != nil {
			return node, ctx.Err()
		}
	}
	return node, nil
}

func directionAllows(aanleg, direction string) bool {
	switch direction {
	case "both", "":
		return true
	case "up":
		// "up" = toward higher court = walking from cassatie root down to eerdere aanleg.
		return strings.Contains(aanleg, "eerdereAanleg") || strings.Contains(aanleg, "conclusie")
	case "down":
		return strings.Contains(aanleg, "latereAanleg") || strings.Contains(aanleg, "cassatie")
	default:
		return true
	}
}

func shortFromURI(uri string) string {
	if i := strings.LastIndex(uri, "#"); i >= 0 {
		return uri[i+1:]
	}
	if i := strings.LastIndex(uri, "/"); i >= 0 {
		return uri[i+1:]
	}
	return uri
}

func printChainTree(w interface{ Write(p []byte) (int, error) }, node *ChainNode, depth int) {
	if node == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	header := node.ECLI
	if node.Court != "" {
		header += "  (" + node.Court
		if node.DecisionDate != "" {
			header += ", " + node.DecisionDate
		}
		header += ")"
	}
	fmt.Fprintf(w, "%s%s\n", indent, header)
	if node.Procedure != "" {
		fmt.Fprintf(w, "%s  procedure: %s\n", indent, node.Procedure)
	}
	if node.Error != "" {
		fmt.Fprintf(w, "%s  error: %s\n", indent, node.Error)
	}
	for _, edge := range node.Edges {
		label := edge.Relation
		if edge.Outcome != "" {
			label += " (" + edge.Outcome + ")"
		}
		if edge.Aanleg != "" {
			label += " [" + edge.Aanleg + "]"
		}
		fmt.Fprintf(w, "%s  ├─ %s\n", indent, label)
		printChainTree(w, edge.Target, depth+2)
	}
}
