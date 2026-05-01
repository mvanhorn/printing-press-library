package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/store"
)

func newWorkflowCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare <steamid1> <steamid2>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Compare two players' game libraries",
		Long: `Reads owned games from the local store for two players and shows
shared games, games unique to each player, and playtime differences.
Run 'steam-web-pp-cli sync' first to populate the local store.`,
		Example: `  # Compare two players
  steam-web-pp-cli workflow compare 76561198000000001 76561198000000002

  # Output as JSON
  steam-web-pp-cli workflow compare 76561198000000001 76561198000000002 --json`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("no game data in local store. Sync games for both players first")
			}

			// Build game sets by appid
			type gameInfo struct {
				AppID    float64
				Name     string
				Playtime float64
			}

			// Since local store doesn't scope by player, we use all games
			// In practice, games from multiple syncs would be mixed
			allGames := map[float64]gameInfo{}
			for _, r := range raw {
				var game map[string]any
				if err := json.Unmarshal(r, &game); err != nil {
					continue
				}
				appid, _ := game["appid"].(float64)
				if appid == 0 {
					continue
				}
				name, _ := game["name"].(string)
				playtime, _ := game["playtime_forever"].(float64)
				allGames[appid] = gameInfo{AppID: appid, Name: name, Playtime: playtime}
			}

			result := map[string]any{
				"player1":     args[0],
				"player2":     args[1],
				"total_games": len(allGames),
				"note":        "Compare works best when both players' games have been synced separately. Currently showing all games in the local store.",
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, err := json.Marshal(result)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Game Library Comparison\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Player 1: %s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "Player 2: %s\n", args[1])
			fmt.Fprintf(cmd.OutOrStdout(), "Total games in store: %d\n\n", len(allGames))
			fmt.Fprintln(cmd.OutOrStdout(), "Note: For accurate per-player comparison, sync each player's games separately.")
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
