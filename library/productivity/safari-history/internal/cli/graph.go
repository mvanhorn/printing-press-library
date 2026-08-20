package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func newGraphCmd(opts *RootOptions) *cobra.Command {
	var since, domain, format string
	cmd := &cobra.Command{
		Use:         "graph",
		Short:       "Build a navigation graph of page nodes and referrer edges over --since, as JSON or Graphviz --format dot (edges sparse on Safari, which lacks from_visit links)",
		Example:     strings.Trim("safari-history-pp-cli graph --since 7d --format dot", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			st, err := openSnapshotStore()
			if err != nil {
				return err
			}
			defer st.Close()
			events, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: end, Domain: domain, Limit: 10000, Device: opts.Device})
			if err != nil {
				return err
			}
			byID := map[int64]source.VisitRow{}
			for _, e := range events {
				byID[e.VisitID] = e
			}
			nodes := map[int64]map[string]any{}
			edges := []map[string]any{}
			for _, e := range events {
				nodes[e.VisitID] = map[string]any{"id": e.VisitID, "url": e.URL, "title": e.Title}
				if e.FromVisit > 0 {
					if _, ok := byID[e.FromVisit]; ok {
						edges = append(edges, map[string]any{"from": e.FromVisit, "to": e.VisitID})
					}
				}
			}
			if format == "dot" && !opts.Output.JSON && strings.TrimSpace(opts.Output.Select) == "" {
				fmt.Println("digraph history {")
				for _, e := range edges {
					fmt.Printf("  \"%v\" -> \"%v\";\n", e["from"], e["to"])
				}
				fmt.Println("}")
				return nil
			}
			nodeList := []map[string]any{}
			for _, n := range nodes {
				nodeList = append(nodeList, n)
			}
			out := map[string]any{"nodes": nodeList, "edges": edges}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	cmd.Flags().StringVar(&domain, "domain", "", "domain filter")
	cmd.Flags().StringVar(&format, "format", "json", "json|dot")
	return cmd
}
