package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSetlistDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "setlist-diff <idA> <idB>",
		Short:   "Side-by-side diff of two setlists",
		Long:    `Shows added, removed, and moved songs between two setlists.`,
		Example: `  setlist-fm-pp-cli setlist-diff 53e3ab04 7be1aaa0`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreOrFail(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			songsA, err := db.GetSetlistSongs(args[0])
			if err != nil {
				return fmt.Errorf("fetching setlist %s: %w", args[0], err)
			}
			songsB, err := db.GetSetlistSongs(args[1])
			if err != nil {
				return fmt.Errorf("fetching setlist %s: %w", args[1], err)
			}

			// Build name sets with positions
			type songPos struct {
				Name     string `json:"name"`
				Position int    `json:"position"`
			}

			namesA := map[string]int{}
			var listA []songPos
			for _, s := range songsA {
				namesA[strings.ToLower(s.Name)] = s.Position
				listA = append(listA, songPos{s.Name, s.Position})
			}

			namesB := map[string]int{}
			var listB []songPos
			for _, s := range songsB {
				namesB[strings.ToLower(s.Name)] = s.Position
				listB = append(listB, songPos{s.Name, s.Position})
			}

			type diffEntry struct {
				Song   string `json:"song"`
				Status string `json:"status"`
				PosA   int    `json:"pos_a,omitempty"`
				PosB   int    `json:"pos_b,omitempty"`
			}
			var diffs []diffEntry

			// Removed: in A but not B
			for _, s := range listA {
				key := strings.ToLower(s.Name)
				if _, ok := namesB[key]; !ok {
					diffs = append(diffs, diffEntry{Song: s.Name, Status: "removed", PosA: s.Position})
				}
			}
			// Added: in B but not A
			for _, s := range listB {
				key := strings.ToLower(s.Name)
				if _, ok := namesA[key]; !ok {
					diffs = append(diffs, diffEntry{Song: s.Name, Status: "added", PosB: s.Position})
				}
			}
			// Moved: in both but different positions
			for _, s := range listB {
				key := strings.ToLower(s.Name)
				if posA, ok := namesA[key]; ok && posA != s.Position {
					diffs = append(diffs, diffEntry{Song: s.Name, Status: "moved", PosA: posA, PosB: s.Position})
				}
			}

			if flags.asJSON {
				return outputJSON(cmd, map[string]any{
					"setlist_a":   args[0],
					"setlist_b":   args[1],
					"songs_a":     len(songsA),
					"songs_b":     len(songsB),
					"differences": diffs,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Setlist Diff: %s vs %s\n\n", args[0], args[1])
			headers := []string{"SONG", "STATUS", "POS A", "POS B"}
			var rows [][]string
			for _, d := range diffs {
				posA := ""
				posB := ""
				if d.PosA > 0 {
					posA = fmt.Sprintf("%d", d.PosA)
				}
				if d.PosB > 0 {
					posB = fmt.Sprintf("%d", d.PosB)
				}
				prefix := ""
				switch d.Status {
				case "removed":
					prefix = "- "
				case "added":
					prefix = "+ "
				case "moved":
					prefix = "~ "
				}
				rows = append(rows, []string{prefix + d.Song, d.Status, posA, posB})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No differences found.\n")
				return nil
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}
