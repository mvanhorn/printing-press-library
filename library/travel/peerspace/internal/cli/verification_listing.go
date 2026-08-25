// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Guest verification check before booking (listing click-around HAR 2026-07-16).
// POST /v1/verification/listing {"sso_id","listing_id"}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newVerificationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "verification",
		Short:       "Guest verification checks",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newVerificationListingCmd(flags))
	return cmd
}

func newVerificationListingCmd(flags *rootFlags) *cobra.Command {
	var (
		flagListingID string
		flagSSO       string
	)
	cmd := &cobra.Command{
		Use:     "listing",
		Short:   "Check whether the guest needs verification for a listing (POST /v1/verification/listing).",
		Example: `  peerspace-pp-cli verification listing --listing-id demo-listing --dry-run --json`,
		Annotations: map[string]string{
			"pp:endpoint":         "verification.listing",
			"pp:method":           "POST",
			"pp:path":             "/v1/verification/listing",
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
				}
				return nil
			}
			listingID := strings.TrimSpace(flagListingID)
			if listingID == "" {
				return fmt.Errorf("--listing-id is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sso := strings.TrimSpace(flagSSO)
			if sso == "" && c.Config != nil {
				sso = cookieValue(c.Config.CookieCredential(), "PSUser")
			}
			if sso == "" {
				return fmt.Errorf("--sso-id required (or auth login so PSUser cookie is present)")
			}
			body := map[string]any{
				"sso_id":     sso,
				"listing_id": listingID,
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, status, err := c.PostWithHeaders(ctx, "/v1/verification/listing", body, listingAuthHeaders(c))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status >= 400 {
				return classifyAPIError(fmt.Errorf("POST /v1/verification/listing returned HTTP %d: %s", status, truncateForErr(data, 240)), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing id (required)")
	cmd.Flags().StringVar(&flagSSO, "sso-id", "", "Guest SSO id (defaults to PSUser cookie)")
	_ = cmd.MarkFlagRequired("listing-id")
	return cmd
}
