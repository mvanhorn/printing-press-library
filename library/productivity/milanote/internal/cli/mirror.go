// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelMirrorCmd(flags *rootFlags) *cobra.Command {
	var board, direction, prefer string
	var once, watch bool
	var cols int
	var gutter float64
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "mirror <file.md>",
		Short: "Pair a Markdown file with a board so edits on either side flow to the other.",
		Long: `Keeps one Markdown file and one board in step. The first run lays the file
out as cards and remembers which card came from which part of the file. Later
runs compare both sides with what they last agreed on and carry each edit
across. A card edited on both sides is a conflict: it is reported and neither
side changes until you settle it (edit one side, or use --prefer).

Cards that were already on the board before pairing are left out of the sync.
Headings inside a note are split into their own cards when the file syncs.`,
		Example: `  milanote-pp-cli mirror script.md --board 1Ab2Cd3EfGh4iJ --once
  milanote-pp-cli mirror script.md --board 1Ab2Cd3EfGh4iJ --watch --interval 30s
  milanote-pp-cli mirror script.md --board 1Ab2Cd3EfGh4iJ --direction pull --dry-run`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror")
			}
			if board == "" {
				return usageErr(errors.New("--board is required"))
			}
			if direction != "push" && direction != "pull" && direction != "both" {
				return usageErr(fmt.Errorf("--direction must be push, pull or both, not %q", direction))
			}
			if prefer != "" && prefer != "file" && prefer != "board" {
				return usageErr(fmt.Errorf("--prefer must be file or board, not %q", prefer))
			}
			if once && watch {
				return usageErr(errors.New("choose --once or --watch"))
			}
			if watch && interval < 5*time.Second {
				return usageErr(errors.New("--interval must be at least 5s"))
			}
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			opts := syncOptions{direction: direction, prefer: prefer, cols: cols, colW: milanote.CardWidth, gutter: gutter, dryRun: dryRunOK(flags)}
			boardID := boardRef(board)
			pass := func(parent context.Context) (*mirrorResult, error) {
				ctx, cancel := boundCtx(parent, flags)
				defer cancel()
				return runSync(ctx, c, args[0], boardID, opts, false)
			}
			if !watch {
				res, err := pass(cmd.Context())
				if err != nil {
					return err
				}
				return printSync(cmd, flags, res)
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				res, err := pass(ctx)
				switch {
				case err != nil && ctx.Err() != nil:
					return nil
				case err != nil:
					fmt.Fprintf(cmd.ErrOrStderr(), "sync failed, will retry: %v\n", err)
				case len(res.Actions) > 0 || len(res.Conflicts) > 0:
					if err := printSync(cmd, flags, res); err != nil {
						return err
					}
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Board id or URL to pair with the file")
	cmd.Flags().BoolVar(&once, "once", false, "Sync once and exit (the default)")
	cmd.Flags().BoolVar(&watch, "watch", false, "Keep syncing until interrupted")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "How often --watch syncs")
	cmd.Flags().StringVar(&direction, "direction", "both", "push (file to board), pull (board to file) or both")
	cmd.Flags().StringVar(&prefer, "prefer", "", "Settle conflicts in favour of file or board (default: report them)")
	cmd.Flags().IntVar(&cols, "cols", 0, "Columns for the first layout (default: one per section, at most 4)")
	cmd.Flags().Float64Var(&gutter, "gutter", milanote.DefaultGutter, "Gap between cards in grid units")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}
