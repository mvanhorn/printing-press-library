package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/store"
)

func newWorkflowPlaytimeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "playtime <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show top games by playtime, leaderboard style",
		Long: `Reads owned games from the local store and ranks them by total playtime.
Run 'steam-web-pp-cli sync' first to populate the local store.`,
		Example: `  # Show top 10 games by playtime
  steam-web-pp-cli workflow playtime 76561198000000000

  # Show top 25
  steam-web-pp-cli workflow playtime 76561198000000000 --limit 25

  # Output as JSON
  steam-web-pp-cli workflow playtime 76561198000000000 --json`,
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

			// Parse and sort by playtime
			type gameEntry struct {
				AppID    float64 `json:"appid"`
				Name     string  `json:"name"`
				Playtime float64 `json:"playtime_forever"`
			}

			var games []gameEntry
			for _, r := range raw {
				var game map[string]any
				if err := json.Unmarshal(r, &game); err != nil {
					continue
				}
				playtime, _ := game["playtime_forever"].(float64)
				if playtime == 0 {
					continue
				}
				appid, _ := game["appid"].(float64)
				name, _ := game["name"].(string)
				games = append(games, gameEntry{AppID: appid, Name: name, Playtime: playtime})
			}

			// Sort descending by playtime
			sort.Slice(games, func(i, j int) bool {
				return games[i].Playtime > games[j].Playtime
			})

			// Apply limit
			if limit > 0 && limit < len(games) {
				games = games[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, err := json.Marshal(games)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Top %d games by playtime:\n\n", len(games))

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "#\tGAME\tHOURS")
			for i, g := range games {
				hours := g.Playtime / 60.0
				name := g.Name
				if name == "" {
					name = fmt.Sprintf("App %v", g.AppID)
				}
				fmt.Fprintf(tw, "%d\t%s\t%.1f\n", i+1, name, hours)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of top games to show")

	return cmd
}
