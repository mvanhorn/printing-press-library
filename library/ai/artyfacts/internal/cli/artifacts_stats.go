// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newArtifactsStatsCmd(flags *rootFlags) *cobra.Command {
	var flagByAgent bool
	var flagByType bool
	var flagByStatus bool
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Workspace statistics: contributions by agent, type, or status",
		Long: `Aggregate the local store to produce workspace statistics.

  --by-agent   Section and artifact count per agent (who wrote what)
  --by-type    Artifact count per type
  --by-status  Artifact count per status

Requires a prior 'sync' to populate the local store.`,
		Example:     "  artyfacts-pp-cli artifacts stats --by-agent\n  artyfacts-pp-cli artifacts stats --by-type --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'sync' first.", err)
			}
			defer db.Close()

			// Default to showing all dimensions if none specified
			showAll := !flagByAgent && !flagByType && !flagByStatus

			if flags.asJSON {
				out := map[string]any{}
				if flagByAgent || showAll {
					agents, err := db.AgentContributions()
					if err == nil {
						out["by_agent"] = agents
					}
				}
				if flagByType || showAll {
					types, err := db.TypeStats()
					if err == nil {
						out["by_type"] = types
					}
				}
				if flagByStatus || showAll {
					statuses, err := db.StatusStats()
					if err == nil {
						out["by_status"] = statuses
					}
				}
				b, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			if flagByAgent || showAll {
				agents, err := db.AgentContributions()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Contributions by Agent:")
				fmt.Fprintln(w, "  AGENT\tSECTIONS\tARTIFACTS\tLAST ACTIVE")
				for _, a := range agents {
					fmt.Fprintf(w, "  %s\t%d\t%d\t%s\n", a.AgentName, a.SectionCount, a.ArtifactCount, a.LastActive)
				}
				w.Flush()
				if len(agents) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  (no agent-attributed sections found — sync sections first)")
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if flagByType || showAll {
				types, err := db.TypeStats()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Artifacts by Type:")
				fmt.Fprintln(w, "  TYPE\tCOUNT")
				for _, t := range types {
					fmt.Fprintf(w, "  %s\t%s\n", t.Type, t.Count)
				}
				w.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if flagByStatus || showAll {
				statuses, err := db.StatusStats()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Artifacts by Status:")
				fmt.Fprintln(w, "  STATUS\tCOUNT")
				for _, s := range statuses {
					fmt.Fprintf(w, "  %s\t%s\n", s.Status, s.Count)
				}
				w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagByAgent, "by-agent", false, "Show section/artifact counts per agent_name")
	cmd.Flags().BoolVar(&flagByType, "by-type", false, "Show artifact counts per type")
	cmd.Flags().BoolVar(&flagByStatus, "by-status", false, "Show artifact counts per status")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path")

	return cmd
}
