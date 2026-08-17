package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newInboxThreadCmd(flags *rootFlags) *cobra.Command {
	var threadID string

	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Fetch a direct Marketplace/Messenger thread by thread id.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if threadID == "" {
				return fmt.Errorf("--thread is required")
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := c.MarketplaceThreadSnapshot(threadID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "get",
					"resource": "inbox_thread",
					"path":     "/messages/t/" + threadID + "/",
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&threadID, "thread", "", "Marketplace/Messenger thread id")
	return cmd
}
