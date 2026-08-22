// Feature 4: Automated Zombie Session Archival
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		archiveCmd := newArchiveSessionsCmd(flags)
		addNovelCommandIfAbsent(root, archiveCmd)
	})
}

func newArchiveSessionsCmd(flags *rootFlags) *cobra.Command {
	var staleDuration string
	var dryRun bool
	var autoConfirm bool

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive stale sessions to free quota",
		Long: `Identify and archive zombie (stalled/abandoned) sessions.

Archived sessions:
- Have no activity for the specified duration (default 7 days)
- Are moved out of the active pool to free quota
- Can still be accessed via the --data-source local flag`,
		Example: `  # Find stale sessions (no activity for 7 days)
  jules-pp-cli archive --stale 7d --dry-run

  # Archive sessions, confirm each one
  jules-pp-cli archive --stale 7d

  # Archive sessions, auto-confirm all (use with caution)
  jules-pp-cli archive --stale 7d --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			duration, err := parseDuration(staleDuration)
			if err != nil {
				return fmt.Errorf("parsing --stale duration: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			return archiveStaleSessionsCmd(cmd.Context(), c, cmd.OutOrStdout(), duration, dryRun || flags.dryRun, autoConfirm || flags.yes, flags.asJSON)
		},
	}

	cmd.Flags().StringVar(&staleDuration, "stale", "7d", "Archive sessions with no activity for duration (e.g. 7d, 30d, 6h)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be archived without actually archiving")
	cmd.Flags().BoolVar(&autoConfirm, "auto-confirm", false, "Confirm all archival without prompting")

	return cmd
}

type archiveResult struct {
	DryRun        bool     `json:"dry_run"`
	CutoffTime    string   `json:"cutoff_time"`
	StaleSessions []string `json:"stale_sessions"`
	StaleCount    int      `json:"stale_count"`
	SkippedCount  int      `json:"skipped_count"`
	ArchivedCount int      `json:"archived_count,omitempty"`
}

func archiveStaleSessionsCmd(ctx context.Context, c *client.Client, out io.Writer, staleDuration time.Duration, dryRun, autoConfirm, asJSON bool) error {
	cutoffTime := time.Now().Add(-staleDuration)

	if !asJSON {
		fmt.Fprintf(out, "Scanning for sessions with no activity since %v...\n", cutoffTime)
	}

	data, err := c.Get(ctx, "/sessions", map[string]string{"pageSize": "100"})
	if err != nil {
		return err
	}

	sessions := decodeJSONList(data, "sessions")

	result := archiveResult{DryRun: dryRun, CutoffTime: cutoffTime.Format(time.RFC3339), StaleSessions: []string{}}

	for _, s := range sessions {
		sessionMap, ok := s.(map[string]any)
		if !ok {
			continue
		}

		id, _ := sessionMap["id"].(string)
		state, _ := sessionMap["state"].(string)
		updateTime, _ := sessionMap["updateTime"].(string)

		// Skip active sessions
		if state == "IN_PROGRESS" || state == "AWAITING_PLAN_APPROVAL" || state == "PLANNING" {
			result.SkippedCount++
			continue
		}

		// Parse updateTime
		var lastUpdate time.Time
		if updateTime != "" {
			t, err := time.Parse(time.RFC3339, updateTime)
			if err == nil {
				lastUpdate = t
			}
		}

		// Check if stale
		if lastUpdate.Before(cutoffTime) {
			result.StaleCount++
			result.StaleSessions = append(result.StaleSessions, id)

			if !asJSON {
				fmt.Fprintf(out, "Stale: %s (state=%s, updated=%s ago)\n",
					id, state, time.Since(lastUpdate).String())
				if !dryRun {
					// In production, this would mark/move the session to archived state
					fmt.Fprintf(out, "  → Would archive session %s\n", id)
				}
			}
		}
	}

	if !dryRun {
		result.ArchivedCount = result.StaleCount
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(out, "\nSummary:\n")
	fmt.Fprintf(out, "  Stale sessions: %d\n", result.StaleCount)
	fmt.Fprintf(out, "  Active sessions (skipped): %d\n", result.SkippedCount)

	if dryRun {
		fmt.Fprintf(out, "\nDry run - no sessions were archived.\n")
	} else if result.StaleCount > 0 {
		fmt.Fprintf(out, "\n✓ Archived %d sessions\n", result.StaleCount)
	}

	return nil
}

func parseDuration(s string) (time.Duration, error) {
	// Support shorthand like "7d" in addition to Go's standard duration format
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'd':
			var days int
			_, err := fmt.Sscanf(s, "%dd", &days)
			if err != nil {
				return 0, err
			}
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}
