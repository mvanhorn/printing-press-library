package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-creator/internal/store"

	"github.com/spf13/cobra"
)

type churnEvent struct {
	Email       string `json:"email"`
	Event       string `json:"event"` // new, unsubscribed, upgraded, downgraded
	PrevTier    string `json:"prev_tier,omitempty"`
	NewTier     string `json:"new_tier,omitempty"`
	Publication string `json:"publication"`
	DeltaAt     string `json:"delta_at"`
}

func ensureSubscriberSnapshotsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS subscriber_snapshots (
		taken_at DATETIME NOT NULL,
		publication_id TEXT NOT NULL,
		email TEXT NOT NULL,
		tier TEXT,
		status TEXT,
		PRIMARY KEY (taken_at, publication_id, email)
	)`)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_snap_pub_taken
		ON subscriber_snapshots(publication_id, taken_at)`)
	return err
}

func newSubscribersChurnCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		publication string
		since       string
		snapshot    bool
	)
	cmd := &cobra.Command{
		Use:   "churn",
		Short: "Diff two subscriber snapshots to surface named gains, losses, and tier moves.",
		Long: `Compares the current subscribers table to a prior snapshot and classifies each
email as new, unsubscribed, upgraded (free→paid), or downgraded (paid→free).

Run with --snapshot at least once before regular runs so a baseline exists.`,
		Example: `  # Take a baseline snapshot
  substack-creator-pp-cli subscribers churn --snapshot

  # Show 7-day churn from the most recent snapshot
  substack-creator-pp-cli subscribers churn --since 7d --json --publication mypub-paid`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "subscribers churn"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-creator-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()
			if err := ensureSubscriberSnapshotsTable(cmd.Context(), db); err != nil {
				return err
			}

			if snapshot {
				return takeSubscriberSnapshot(cmd.Context(), cmd.OutOrStdout(), db, publication, flags)
			}

			// Find a baseline snapshot taken at or before `since`.
			// Bind the cutoff as a parameter instead of fmt.Sprintf'ing it
			// into the SQL. computeSinceCutoff currently restricts the value
			// to safe shapes, but a bind keeps the query safe under future
			// relaxation of that validator and matches how pubFilter is wired.
			//
			// pubArgs is shared with the diff query below; whereSince's
			// cutoff only feeds the baseline-lookup query, so the args
			// slices are kept separate.
			var baselineTime string
			whereSince := ""
			whereArgs := []any{}
			pubFilter := ""
			pubArgs := []any{}
			if since != "" {
				cutoff, err := computeSinceCutoff(since)
				if err != nil {
					return usageErr(err)
				}
				whereSince = " AND taken_at <= ?"
				whereArgs = append(whereArgs, cutoff)
			}
			if publication != "" {
				pubFilter = " AND (publication_id = ? OR publication_id IN (SELECT id FROM publications WHERE subdomain = ?))"
				pubArgs = append(pubArgs, publication, publication)
			}
			baselineArgs := append([]any{}, whereArgs...)
			baselineArgs = append(baselineArgs, pubArgs...)
			row := db.QueryRowContext(cmd.Context(),
				`SELECT COALESCE(MAX(taken_at), '') FROM subscriber_snapshots WHERE 1=1`+whereSince+pubFilter, baselineArgs...)
			if err := row.Scan(&baselineTime); err != nil {
				return err
			}
			if baselineTime == "" {
				return fmt.Errorf("no baseline snapshot found; run 'substack-creator-pp-cli subscribers churn --snapshot' first")
			}

			// Diff: current subscribers vs baseline snapshot
			diffQ := `SELECT
				COALESCE(s.email, ss.email) AS email,
				COALESCE(s.tier, '')        AS cur_tier,
				COALESCE(ss.tier, '')       AS prev_tier,
				COALESCE(s.publication_id, ss.publication_id) AS pub
			FROM (SELECT * FROM subscribers WHERE 1=1 ` + pubFilter + `) s
			FULL OUTER JOIN (SELECT * FROM subscriber_snapshots WHERE taken_at = ?) ss
			ON s.email = ss.email AND s.publication_id = ss.publication_id`
			// SQLite supports FULL OUTER JOIN since 3.39 (2022). modernc.org/sqlite ships current.
			args3 := append([]any{}, pubArgs...)
			args3 = append(args3, baselineTime)
			rows, err := db.QueryContext(cmd.Context(), diffQ, args3...)
			if err != nil {
				return fmt.Errorf("diffing snapshots: %w", err)
			}
			defer rows.Close()

			var out []churnEvent
			now := time.Now().UTC().Format(time.RFC3339)
			for rows.Next() {
				var email, curTier, prevTier, pub string
				if err := rows.Scan(&email, &curTier, &prevTier, &pub); err != nil {
					return err
				}
				ev := classifyChurn(curTier, prevTier)
				if ev == "" {
					continue
				}
				out = append(out, churnEvent{
					Email: email, Event: ev,
					PrevTier: prevTier, NewTier: curTier,
					Publication: pub, DeltaAt: now,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No changes since baseline.")
				return nil
			}
			fmt.Fprintf(w, "Churn since baseline %s:\n", baselineTime)
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, e := range out {
				fmt.Fprintf(w, "  %-12s %s  (%s → %s)  pub=%s\n",
					e.Event, e.Email, e.PrevTier, e.NewTier, truncate(e.Publication, 16))
			}
			fmt.Fprintln(w, strings.Repeat("─", 78))
			fmt.Fprintf(w, "%d event(s).\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&publication, "publication", "", "Filter to a single publication")
	cmd.Flags().StringVar(&since, "since", "", "Use baseline snapshot at or before this point (e.g. 7d, 30d, YYYY-MM-DD)")
	cmd.Flags().BoolVar(&snapshot, "snapshot", false, "Capture a new baseline snapshot instead of diffing")
	return cmd
}

func classifyChurn(curTier, prevTier string) string {
	switch {
	case prevTier == "" && curTier != "":
		return "new"
	case prevTier != "" && curTier == "":
		return "unsubscribed"
	case prevTier == "free" && (curTier == "paid" || curTier == "founding"):
		return "upgraded"
	case (prevTier == "paid" || prevTier == "founding") && curTier == "free":
		return "downgraded"
	}
	return ""
}

func takeSubscriberSnapshot(ctx context.Context, w interface{ Write([]byte) (int, error) }, db *sql.DB, publication string, flags *rootFlags) error {
	now := time.Now().UTC().Format(time.RFC3339)
	q := `INSERT OR REPLACE INTO subscriber_snapshots (taken_at, publication_id, email, tier, status)
	      SELECT ?, publication_id, email, tier, status FROM subscribers WHERE email IS NOT NULL AND email != ''`
	args := []any{now}
	if publication != "" {
		q += ` AND (publication_id = ? OR publication_id IN (SELECT id FROM publications WHERE subdomain = ?))`
		args = append(args, publication, publication)
	}
	res, err := db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("snapshotting subscribers: %w", err)
	}
	n, _ := res.RowsAffected()
	if flags.asJSON {
		raw, _ := json.Marshal(map[string]any{"taken_at": now, "rows": n})
		_, _ = w.Write(raw)
		_, _ = w.Write([]byte("\n"))
		return nil
	}
	fmt.Fprintf(stdoutFromWriter(w), "Snapshot taken at %s — %d rows\n", now, n)
	return nil
}

// computeSinceCutoff converts e.g. "7d" to an ISO timestamp 7 days ago.
// Also accepts a literal YYYY-MM-DD as-is.
func computeSinceCutoff(since string) (string, error) {
	since = strings.TrimSpace(since)
	if since == "" {
		return "", nil
	}
	if len(since) == 10 && since[4] == '-' && since[7] == '-' {
		return since + "T23:59:59Z", nil
	}
	d := parseWindowDays(since)
	if d == 0 {
		return "", fmt.Errorf("invalid --since %q (expected NNd or YYYY-MM-DD)", since)
	}
	return time.Now().UTC().AddDate(0, 0, -d).Format(time.RFC3339), nil
}

// stdoutFromWriter is a tiny shim so takeSubscriberSnapshot can accept any
// io.Writer-shaped interface. The Cobra cmd.OutOrStdout() already satisfies
// the io.Writer signature we need, but we keep the helper here for clarity.
func stdoutFromWriter(w interface{ Write([]byte) (int, error) }) interface{ Write([]byte) (int, error) } {
	return w
}
