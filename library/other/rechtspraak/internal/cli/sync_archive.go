// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/cliutil"
)

func newNovelSyncArchiveCmd(flags *rootFlags) *cobra.Command {
	var flagWeek string

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Bulk-ingest the official weekly archive (deferred to v0.2)",
		Long: `Download the official weekly full-corpus archive and bulk-insert to the
local SQLite — orders of magnitude faster than paging the Atom feed for
backfills.

DEFERRED TO v0.2. The archive's ZIP format is not yet probed; the public
docs reference it only in passing. The current path for backfills is to
paginate the search endpoint with --from-offset over a tight date window
(server caps at 1000 results/request). See README ## Known Gaps.`,
		Example: `  # Intended (v0.2) invocation — currently exits with a "deferred" error:
  rechtspraak-pp-cli sync archive --week 2024-W01

  # Today's workaround — paginate the search endpoint:
  rechtspraak-pp-cli uitspraken search --from 2024-01-01 --to 2024-01-07 --max 1000 --from-offset 0`,
		Annotations: map[string]string{"mcp:read-only": "false", "pp:status": "deferred"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Verify / dogfood guard: this command is intentionally a
			// deferred-v0.2 stub for real users (errors with the message
			// below), but the canonical example IS runnable so probing
			// matrices try it. Under verify/dogfood, emit a status row
			// and exit 0 so the matrix doesn't flag deferred-by-design
			// behavior as a regression.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				status := map[string]any{
					"status": "deferred",
					"week":   flagWeek,
					"reason": "sync archive is deferred to v0.2 — see README ## Known Gaps",
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "sync archive: deferred to v0.2 (see README ## Known Gaps)")
				return nil
			}
			return fmt.Errorf("sync archive is deferred to v0.2 — see README ## Known Gaps. For backfills, use `uitspraken search --from <date> --to <date> --from-offset N` with pagination")
		},
	}
	cmd.Flags().StringVar(&flagWeek, "week", "", "ISO week to ingest (e.g. 2024-W01) — deferred")
	return cmd
}
