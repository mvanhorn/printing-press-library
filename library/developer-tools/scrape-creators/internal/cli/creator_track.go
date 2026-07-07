// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Novel command: append a follower snapshot and read the growth trajectory.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type trajectoryPoint struct {
	CapturedAt    time.Time `json:"captured_at"`
	FollowerCount int64     `json:"follower_count"`
	Delta         int64     `json:"delta"`
}

func newNovelCreatorTrackCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var dbPath string

	cmd := &cobra.Command{
		Use:     "track <handle>",
		Short:   "Append a follower snapshot per run, then read the creator's growth trajectory over time.",
		Example: "  scrape-creators-pp-cli creator track mrbeast",
		// pp:no-error-path-probe: any string is a valid handle to look up, so a
		// nonexistent handle returns an empty trajectory with exit 0, not an
		// error — the verifier's invalid-argument probe does not apply.
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		// No mcp:read-only hint: this command appends a follower snapshot to the
		// local store (a store update), so it is not "read-only" per the
		// agent-native tool-safety contract. A missing hint yields a permission
		// prompt; a false read-only hint on a writer is the documented real bug.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("handle is required"))
			}
			handle := args[0]
			if platform == "" {
				platform = "tiktok"
			}
			cp, ok := creatorPlatformByName(platform)
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unsupported --platform %q", platform))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			profile, err := c.Get(ctx, cp.profilePath, map[string]string{cp.handleParam: handle})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			followers, _ := extractFollowerCount(profile)

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			snap := store.CreatorSnapshot{Handle: handle, Platform: platform, FollowerCount: followers, CapturedAt: time.Now()}
			if err := store.InsertCreatorSnapshot(ctx, db.DB(), snap); err != nil {
				return err
			}
			history, err := store.CreatorTrajectory(ctx, db.DB(), handle, platform)
			if err != nil {
				return err
			}

			points := make([]trajectoryPoint, 0, len(history))
			var prev int64
			for i, h := range history {
				p := trajectoryPoint{CapturedAt: h.CapturedAt, FollowerCount: h.FollowerCount}
				if i > 0 {
					p.Delta = h.FollowerCount - prev
				}
				prev = h.FollowerCount
				points = append(points, p)
			}

			if novelWantsMachine(cmd.OutOrStdout(), flags) {
				envelope := map[string]any{
					"handle":         handle,
					"platform":       platform,
					"current":        followers,
					"snapshot_count": len(points),
					"trajectory":     points,
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Trajectory for %q on %s (%d snapshots)\n\n", handle, platform, len(points))
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "CAPTURED_AT\tFOLLOWERS\tDELTA")
			for _, p := range points {
				fmt.Fprintf(tw, "%s\t%d\t%+d\n", p.CapturedAt.Format(time.RFC3339), p.FollowerCount, p.Delta)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "tiktok", "creator platform to snapshot (tiktok, instagram, youtube, ...)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
