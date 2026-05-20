package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newBingoCmd(flags *rootFlags) *cobra.Command {
	var songCount int
	var last int

	cmd := &cobra.Command{
		Use:   "bingo <artist>",
		Short: "Generate a bingo card of most-likely songs for an upcoming show",
		Long: `Creates a bingo card grid of songs ranked by recency-weighted probability.
Default is 25 songs (5x5 grid). Uses the same probability model as predict.`,
		Example: `  setlist-fm-pp-cli bingo "Radiohead"
  setlist-fm-pp-cli bingo "Radiohead" --songs 16 --last 30`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreOrFail(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			mbid, name, err := resolveArtistFromStore(db, args[0])
			if err != nil {
				return err
			}

			setlists, err := db.GetSetlistsByArtistLimit(mbid, last)
			if err != nil {
				return err
			}
			if len(setlists) == 0 {
				return fmt.Errorf("no setlists found for %s", name)
			}

			// Compute probabilities with exponential decay
			type songProb struct {
				Name string  `json:"name"`
				Prob float64 `json:"probability"`
			}
			songMap := map[string]*songProb{}
			totalWeight := 0.0
			decay := 0.95

			for i, sl := range setlists {
				weight := math.Pow(decay, float64(i))
				totalWeight += weight

				songs, err := db.GetSetlistSongs(sl.ID)
				if err != nil {
					continue
				}
				for _, song := range songs {
					if song.Name == "" {
						continue
					}
					key := strings.ToLower(song.Name)
					if _, ok := songMap[key]; !ok {
						songMap[key] = &songProb{Name: song.Name}
					}
					songMap[key].Prob += weight
				}
			}

			var predictions []songProb
			for _, sp := range songMap {
				sp.Prob = sp.Prob / totalWeight * 100
				predictions = append(predictions, *sp)
			}
			sort.Slice(predictions, func(i, j int) bool {
				return predictions[i].Prob > predictions[j].Prob
			})

			if songCount > len(predictions) {
				songCount = len(predictions)
			}
			bingoSongs := predictions[:songCount]

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"artist":   name,
					"based_on": len(setlists),
					"songs":    bingoSongs,
				}, flags)
			}

			// Print as a grid
			gridSize := 5
			for gridSize*gridSize < songCount {
				gridSize++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "BINGO CARD: %s (based on last %d shows)\n\n", name, len(setlists))

			// Find max song name width
			maxWidth := 0
			for _, s := range bingoSongs {
				if len(s.Name) > maxWidth {
					maxWidth = len(s.Name)
				}
			}
			if maxWidth > 25 {
				maxWidth = 25
			}
			cellWidth := maxWidth + 2

			// Print grid
			idx := 0
			for row := 0; row < gridSize && idx < songCount; row++ {
				// Top border
				for col := 0; col < gridSize && idx+col < songCount; col++ {
					fmt.Fprintf(cmd.OutOrStdout(), "+%s", strings.Repeat("-", cellWidth))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "+")

				// Song names
				for col := 0; col < gridSize && idx < songCount; col++ {
					name := bingoSongs[idx].Name
					if len(name) > maxWidth {
						name = name[:maxWidth-1] + "."
					}
					fmt.Fprintf(cmd.OutOrStdout(), "| %-*s", cellWidth-1, name)
					idx++
				}
				fmt.Fprintln(cmd.OutOrStdout(), "|")
			}
			// Bottom border
			for col := 0; col < gridSize && col < songCount; col++ {
				fmt.Fprintf(cmd.OutOrStdout(), "+%s", strings.Repeat("-", cellWidth))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "+")

			return nil
		},
	}

	cmd.Flags().IntVar(&songCount, "songs", 25, "Number of songs on the card")
	cmd.Flags().IntVar(&last, "last", 20, "Number of recent setlists to analyze")
	return cmd
}
