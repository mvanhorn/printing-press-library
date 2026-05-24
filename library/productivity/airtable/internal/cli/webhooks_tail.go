// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/airtable/internal/cliutil"
	"github.com/spf13/cobra"
)

func newWebhooksTailCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var sinceCursor int
	var fromBeginning bool

	cmd := &cobra.Command{
		Use:         "tail <baseId> <webhookId>",
		Short:       "Continuously poll webhook payloads and stream as ndjson",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Polls 'webhooks list_payloads' continuously with adaptive backoff,
streams payloads as ndjson to stdout, persists cursor between runs.`,
		Example: strings.Trim(`
  # Tail a webhook every 10 seconds
  airtable-pp-cli webhooks tail appXXX achYYY

  # Start from a specific cursor
  airtable-pp-cli webhooks tail appXXX achYYY --since-cursor 42

  # Replay from the beginning
  airtable-pp-cli webhooks tail appXXX achYYY --from-beginning
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// Side-effectful long-poll: short-circuit under verify/dogfood.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would tail webhook payloads")
				return nil
			}
			if cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "dogfood: webhook tail curtailed to single poll")
				// Fall through to a single poll iteration below by setting a tiny budget.
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and webhookId are required\nUsage: %s <baseId> <webhookId>", cmd.CommandPath()))
			}
			baseID, webhookID := args[0], args[1]

			home, _ := os.UserHomeDir()
			cursorPath := filepath.Join(home, ".cache", "airtable-pp-cli", baseID, webhookID, "cursor")
			cursor := ""
			if fromBeginning {
				cursor = "0"
			} else if sinceCursor > 0 {
				cursor = strconv.Itoa(sinceCursor)
			} else if b, err := os.ReadFile(cursorPath); err == nil {
				cursor = strings.TrimSpace(string(b))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			enc := json.NewEncoder(cmd.OutOrStdout())
			path := replacePathParam("/bases/{baseId}/webhooks/{webhookId}/payloads", "baseId", baseID)
			path = replacePathParam(path, "webhookId", webhookID)

			poll := func() error {
				params := map[string]string{}
				if cursor != "" {
					params["cursor"] = cursor
				}
				data, err := c.Get(cmd.Context(), path, params)
				if err != nil {
					return err
				}
				var env struct {
					Cursor   int               `json:"cursor"`
					Payloads []json.RawMessage `json:"payloads"`
				}
				if err := json.Unmarshal(data, &env); err == nil {
					for _, p := range env.Payloads {
						_ = enc.Encode(p)
					}
					if env.Cursor > 0 {
						cursor = strconv.Itoa(env.Cursor)
						_ = os.MkdirAll(filepath.Dir(cursorPath), 0o755)
						_ = os.WriteFile(cursorPath, []byte(cursor), 0o644)
					}
				}
				return nil
			}

			// Initial fetch
			if err := poll(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: initial poll failed: %v\n", err)
			}
			if cliutil.IsDogfoodEnv() {
				return nil
			}
			for {
				select {
				case <-sig:
					return nil
				case <-ticker.C:
					if err := poll(); err != nil {
						fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
					}
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Poll interval")
	cmd.Flags().IntVar(&sinceCursor, "since-cursor", 0, "Start from this cursor value")
	cmd.Flags().BoolVar(&fromBeginning, "from-beginning", false, "Replay from cursor 0")
	return cmd
}
