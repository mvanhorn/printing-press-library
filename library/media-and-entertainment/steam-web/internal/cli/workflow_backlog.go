package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/store"
)

func newWorkflowBacklogCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "backlog <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find owned games with zero playtime (your backlog)",
		Long: `Reads owned games from the local store and lists those with 0 playtime.
Run 'steam-web-pp-cli sync' first to populate the local store, or use
'steam-web-pp-cli games <steamid> --include-info --json' to fetch live data.`,
		Example: `  # Show your backlog from local store
  steam-web-pp-cli workflow backlog 76561198000000000

  # Output as JSON
  steam-web-pp-cli workflow backlog 76561198000000000 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			steamID := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("steam-web-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'steam-web-pp-cli sync' first", err)
			}
			defer s.Close()

			raw, err := s.List("iplayer-service", 0)
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}
			if len(raw) == 0 {
				return fmt.Errorf("no game data in local store. Run 'steam-web-pp-cli games %s --include-info --json' first", steamID)
			}

			// Filter games with 0 playtime
			var backlog []map[string]any
			for _, r := range raw {
				var game map[string]any
				if err := json.Unmarshal(r, &game); err != nil {
					continue
				}
				playtime, _ := game["playtime_forever"].(float64)
				if playtime == 0 {
					backlog = append(backlog, game)
				}
			}

			// Sort by name if available
			sort.Slice(backlog, func(i, j int) bool {
				ni, _ := backlog[i]["name"].(string)
				nj, _ := backlog[j]["name"].(string)
				return ni < nj
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, err := json.Marshal(backlog)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Backlog: %d unplayed games\n\n", len(backlog))

			if len(backlog) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unplayed games found. Nice work!")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "APP ID\tNAME")
			for _, g := range backlog {
				appid := ""
				if v, ok := g["appid"]; ok {
					appid = fmt.Sprintf("%v", v)
				}
				name := ""
				if v, ok := g["name"]; ok {
					name = fmt.Sprintf("%v", v)
				}
				fmt.Fprintf(tw, "%s\t%s\n", appid, name)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
