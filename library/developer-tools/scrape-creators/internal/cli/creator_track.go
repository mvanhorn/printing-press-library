package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCreatorTrackCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var report bool
	cmd := &cobra.Command{
		Use:   "track <handle>",
		Short: "Snapshot a creator's follower count and report growth across snapshots",
		Long: `Records a row to the local profile_snapshots table for the given handle on
the chosen platform. Re-running this command on a cron produces a growth
trajectory that no single API call exposes.

Pass --report to skip the API call and just print the local snapshot history.`,
		Example:     "  scrape-creators-pp-cli creator track mrbeast --platform tiktok --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			handle := strings.TrimPrefix(args[0], "@")
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			if !report {
				probe := pickProbe(platform)
				if probe == nil {
					return fmt.Errorf("unsupported platform %q", platform)
				}
				h := handle
				if probe.transform != nil {
					h = probe.transform(h)
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				raw, err := c.Get(probe.path, map[string]string{probe.param: h})
				if err != nil {
					return classifyAPIError(err)
				}
				var anyJSON map[string]any
				_ = json.Unmarshal(raw, &anyJSON)
				followers := lookupInt(anyJSON, probe.followerKy)
				following := lookupInt(anyJSON, []string{
					"user.stats.followingCount", "stats.followingCount", "following_count",
				})
				videos := lookupInt(anyJSON, []string{
					"user.stats.videoCount", "stats.videoCount", "video_count",
				})
				_, err = s.DB().ExecContext(cmd.Context(),
					`INSERT INTO profile_snapshots (handle, platform, follower_count, following_count, content_count, data) VALUES (?, ?, ?, ?, ?, ?)`,
					handle, probe.platform, followers, following, videos, string(raw))
				if err != nil {
					return fmt.Errorf("write snapshot: %w", err)
				}
			}

			// Read history (most-recent 30 snapshots) and compute deltas.
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT snapshot_at, follower_count, following_count, content_count
				 FROM profile_snapshots
				 WHERE handle = ? AND platform = ?
				 ORDER BY snapshot_at DESC
				 LIMIT 30`, handle, platform)
			if err != nil {
				return fmt.Errorf("read history: %w", err)
			}
			defer rows.Close()

			type histRow struct {
				SnapshotAt     string `json:"snapshot_at"`
				FollowerCount  int64  `json:"follower_count"`
				FollowingCount int64  `json:"following_count"`
				ContentCount   int64  `json:"content_count"`
				DeltaFollower  int64  `json:"delta_follower,omitempty"`
			}
			var history []histRow
			for rows.Next() {
				var r histRow
				if err := rows.Scan(&r.SnapshotAt, &r.FollowerCount, &r.FollowingCount, &r.ContentCount); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				history = append(history, r)
			}
			// Compute deltas (newer minus next-older row).
			for i := 0; i < len(history)-1; i++ {
				history[i].DeltaFollower = history[i].FollowerCount - history[i+1].FollowerCount
			}

			result := map[string]any{
				"handle":         handle,
				"platform":       platform,
				"recorded_at":    time.Now().UTC().Format(time.RFC3339),
				"snapshot_count": len(history),
				"history":        history,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "creator track %s on %s — %d snapshots\n", handle, platform, len(history))
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "WHEN\tFOLLOWERS\tDELTA\tFOLLOWING\tCONTENT")
			for _, r := range history {
				delta := ""
				if r.DeltaFollower != 0 {
					delta = fmt.Sprintf("%+d", r.DeltaFollower)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%d\n",
					r.SnapshotAt, r.FollowerCount, delta, r.FollowingCount, r.ContentCount)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "Platform to track on")
	cmd.Flags().BoolVar(&report, "report", false, "Skip API call; print local snapshot history only")
	return cmd
}
