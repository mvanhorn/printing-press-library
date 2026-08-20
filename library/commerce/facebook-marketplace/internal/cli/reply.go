package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newReplyCmd(flags *rootFlags) *cobra.Command {
	var threadID, listingID, message string
	cmd := &cobra.Command{
		Use:   "reply",
		Short: "Send a write-gated Marketplace seller reply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if threadID == "" || message == "" {
				return fmt.Errorf("--thread and --message are required")
			}
			key, err := requireWriteCheckpoint(flags)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				_ = recordWriteState(key, "failed", err.Error())
				return err
			}
			variables := map[string]any{
				"input": map[string]any{
					"client_mutation_id": key,
					"listing_id":         listingID,
					"message":            message,
					"referral_surface":   "inbox",
					"surface":            "marketplace_inbox",
					"thread_id":          threadID,
				},
			}
			vars, _ := json.Marshal(variables)
			fields := url.Values{}
			fields.Set("fb_api_req_friendly_name", "CometMarketplaceMessageSellerMutation")
			fields.Set("doc_id", "29483122031334726")
			fields.Set("variables", string(vars))
			data, _, err := c.PostForm("/api/graphql/", fields)
			if err != nil {
				_ = recordWriteState(key, "unknown_outcome", err.Error())
				return classifyAPIError(err, flags)
			}
			_ = recordWriteState(key, "submitted", "reply mutation submitted")
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&threadID, "thread", "", "Marketplace thread id")
	cmd.Flags().StringVar(&listingID, "listing", "", "Optional listing id")
	cmd.Flags().StringVar(&message, "message", "", "Reply text")
	return cmd
}
