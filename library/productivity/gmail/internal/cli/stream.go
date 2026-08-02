// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: stream new mail as NDJSON by polling history.list.
// Absorbs gws +watch without requiring a Pub/Sub topic.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newStreamCmd(flags))
	})
}

func newStreamCmd(flags *rootFlags) *cobra.Command {
	var interval string
	var once bool

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream new mail as NDJSON by polling the history feed",
		Long: `Polls history.list (2 quota units per call) from the current mailbox
state and emits one JSON line per new message. Ideal for piping into agent
loops or shell pipelines. --once polls a single time and exits.`,
		Example: strings.Trim(`
  gmail-pp-cli stream
  gmail-pp-cli stream --interval 60s
  gmail-pp-cli stream --once`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "poll the history feed for new mail")
			}
			pollEvery, err := cliutil.ParseDurationLoose(interval)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --interval %q: %w", interval, err))
			}
			if pollEvery < 5*time.Second {
				pollEvery = 5 * time.Second
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				once = true
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, cursor, err := gmailProfile(cmd.Context(), c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for {
				changed, _, newest, expired, err := historyChangedIDs(cmd, c, cursor)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if expired {
					// Cursor raced past retention: restart from the current profile state.
					_, cursor, err = gmailProfile(cmd.Context(), c)
					if err != nil {
						return classifyAPIError(err, flags)
					}
				} else {
					cursor = newest
					if len(changed) > 0 {
						msgs, skipped, err := gmailmail.BatchGetMessages(cmd.Context(), c, changed, "metadata", gmailmail.DefaultMetadataHeaders)
						if err != nil {
							return classifyAPIError(err, flags)
						}
						warnSkipped(cmd, skipped)
						for _, r := range toMessageRows(msgs) {
							if err := enc.Encode(r); err != nil {
								return err
							}
						}
					}
				}
				if once {
					return nil
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(pollEvery):
				}
			}
		},
	}
	cmd.Flags().StringVar(&interval, "interval", "30s", "Poll interval (min 5s)")
	cmd.Flags().BoolVar(&once, "once", false, "Poll one time and exit")
	return cmd
}
