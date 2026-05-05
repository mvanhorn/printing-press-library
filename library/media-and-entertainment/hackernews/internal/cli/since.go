package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/store"
)

// `since` shows what changed on the front page since the previous sync
// snapshot. We track snapshots in a separate table so we can compute
// deltas without re-fetching: each sync writes the current top-N IDs
// in rank order, then `since` joins the latest two snapshots.

func ensureSnapshotsTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS frontpage_snapshots (
		taken_at TEXT NOT NULL,
		list TEXT NOT NULL,
		rank INTEGER NOT NULL,
		item_id TEXT NOT NULL,
		PRIMARY KEY (taken_at, list, rank)
	)`)
	return err
}

// recordFrontPageSnapshot writes the current top-30 of each list as a
// snapshot row. Called from sync after stories sync.
func recordFrontPageSnapshot(db *store.Store, list string, ids []string) error {
	if err := ensureSnapshotsTable(db); err != nil {
		return err
	}
	taken := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.DB().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT INTO frontpage_snapshots(taken_at, list, rank, item_id) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range ids {
		if _, err := stmt.Exec(taken, list, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// fetchTwoLatestSnapshots returns (current rank → id, previous rank → id).
// Either map can be empty.
func fetchTwoLatestSnapshots(db *sql.DB, list string) (map[int]string, map[int]string, string, string, error) {
	rows, err := db.Query(`SELECT DISTINCT taken_at FROM frontpage_snapshots WHERE list = ? ORDER BY taken_at DESC LIMIT 2`, list)
	if err != nil {
		return nil, nil, "", "", err
	}
	defer rows.Close()
	var stamps []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, nil, "", "", err
		}
		stamps = append(stamps, s)
	}
	if len(stamps) == 0 {
		return nil, nil, "", "", nil
	}
	var current, previous string
	current = stamps[0]
	if len(stamps) > 1 {
		previous = stamps[1]
	}

	curMap, err := loadSnapshot(db, list, current)
	if err != nil {
		return nil, nil, "", "", err
	}
	if previous == "" {
		return curMap, map[int]string{}, current, "", nil
	}
	prevMap, err := loadSnapshot(db, list, previous)
	if err != nil {
		return nil, nil, "", "", err
	}
	return curMap, prevMap, current, previous, nil
}

func loadSnapshot(db *sql.DB, list, takenAt string) (map[int]string, error) {
	rows, err := db.Query(`SELECT rank, item_id FROM frontpage_snapshots WHERE list = ? AND taken_at = ? ORDER BY rank`, list, takenAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]string)
	for rows.Next() {
		var rank int
		var id string
		if err := rows.Scan(&rank, &id); err != nil {
			return nil, err
		}
		out[rank] = id
	}
	return out, nil
}

type frontPageDelta struct {
	List      string      `json:"list"`
	Current   string      `json:"current_taken_at"`
	Previous  string      `json:"previous_taken_at"`
	Added     []string    `json:"added"`
	Removed   []string    `json:"removed"`
	Moved     []moveEvent `json:"moved"`
	Unchanged int         `json:"unchanged_count"`
	Hint      string      `json:"hint,omitempty"`
}

type moveEvent struct {
	ID   string `json:"id"`
	From int    `json:"from_rank"`
	To   int    `json:"to_rank"`
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var list string
	cmd := &cobra.Command{
		Use:         "since",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Show what changed on the front page since last sync (added, removed, moved stories)",
		Long: `Diff the most recent two front-page snapshots taken by sync.

If you only have one snapshot, the command reports that and lists the
current ranking; otherwise it returns three sets — stories that
appeared on the front page, stories that fell off, and stories that
moved more than one rank in either direction.`,
		Example: strings.Trim(`
  # Run once after a fresh sync
  hackernews-pp-cli sync
  hackernews-pp-cli since

  # Diff "best" instead of "top"
  hackernews-pp-cli since --list best --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			if err := ensureSnapshotsTable(db); err != nil {
				return apiErr(err)
			}

			// Take a fresh snapshot of the requested list before
			// computing the diff. The first run will only have one
			// snapshot and the diff message will say so; the second
			// run onwards always has a meaningful previous snapshot.
			c, err := flags.newClient()
			if err == nil {
				path := snapshotPathForList(list)
				if path != "" {
					if data, getErr := c.Get(path, nil); getErr == nil {
						var ids []int
						if jerr := json.Unmarshal(data, &ids); jerr == nil {
							strIDs := make([]string, 0, len(ids))
							limit := 30
							if len(ids) < limit {
								limit = len(ids)
							}
							for i := 0; i < limit; i++ {
								strIDs = append(strIDs, fmt.Sprintf("%d", ids[i]))
							}
							_ = recordFrontPageSnapshot(db, list, strIDs)
						}
					}
				}
			}

			cur, prev, curStamp, prevStamp, err := fetchTwoLatestSnapshots(db.DB(), list)
			if err != nil {
				return apiErr(err)
			}
			delta := frontPageDelta{
				List:     list,
				Current:  curStamp,
				Previous: prevStamp,
			}
			if len(cur) == 0 {
				delta.Hint = "no snapshots yet — run 'hackernews-pp-cli sync' to create one"
			} else if len(prev) == 0 {
				delta.Hint = fmt.Sprintf("only one snapshot so far (taken %s) — run sync again to enable diffs", curStamp)
			}

			// Compute diffs.
			curIDs := mapValues(cur)
			prevIDs := mapValues(prev)

			delta.Added = diffSet(curIDs, prevIDs)
			delta.Removed = diffSet(prevIDs, curIDs)

			// Moves: same id present in both, with rank delta.
			for rank, id := range cur {
				if prevRank, ok := keyOf(prev, id); ok && prevRank != rank {
					if abs(prevRank-rank) >= 1 {
						delta.Moved = append(delta.Moved, moveEvent{ID: id, From: prevRank, To: rank})
					}
				}
			}
			// Unchanged: in both, same rank.
			for rank, id := range cur {
				if prevRank, ok := keyOf(prev, id); ok && prevRank == rank {
					delta.Unchanged++
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(delta, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}

			if delta.Hint != "" {
				fmt.Fprintln(cmd.OutOrStdout(), delta.Hint)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Diff for %q (current: %s, previous: %s)\n",
				list, displayStamp(delta.Current), displayStamp(delta.Previous))
			fmt.Fprintf(cmd.OutOrStdout(), "  added:     %d\n", len(delta.Added))
			fmt.Fprintf(cmd.OutOrStdout(), "  removed:   %d\n", len(delta.Removed))
			fmt.Fprintf(cmd.OutOrStdout(), "  moved:     %d\n", len(delta.Moved))
			fmt.Fprintf(cmd.OutOrStdout(), "  unchanged: %d\n", delta.Unchanged)
			if len(delta.Added) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nAdded:")
				for _, id := range delta.Added {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", id)
				}
			}
			if len(delta.Removed) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRemoved:")
				for _, id := range delta.Removed {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", id)
				}
			}
			if len(delta.Moved) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nMoved:")
				for _, m := range delta.Moved {
					arrow := "↓"
					if m.To < m.From {
						arrow = "↑"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s %s (rank %d → %d)\n", arrow, m.ID, m.From+1, m.To+1)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&list, "list", "topstories", "Front-page list to diff (topstories, beststories, newstories)")
	return cmd
}

func mapValues(m map[int]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func diffSet(a, b []string) []string {
	bset := make(map[string]struct{}, len(b))
	for _, v := range b {
		bset[v] = struct{}{}
	}
	out := []string{}
	for _, v := range a {
		if _, ok := bset[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func keyOf(m map[int]string, id string) (int, bool) {
	for k, v := range m {
		if v == id {
			return k, true
		}
	}
	return 0, false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// snapshotPathForList maps a CLI --list value to its Firebase path.
// Returns the empty string for unknown lists; callers skip snapshotting
// and the user just sees an empty diff with a hint.
func snapshotPathForList(list string) string {
	switch list {
	case "topstories", "top":
		return "/topstories.json"
	case "beststories", "best":
		return "/beststories.json"
	case "newstories", "new":
		return "/newstories.json"
	case "askstories", "ask":
		return "/askstories.json"
	case "showstories", "show":
		return "/showstories.json"
	case "jobstories", "jobs":
		return "/jobstories.json"
	default:
		return ""
	}
}

func displayStamp(s string) string {
	if s == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
