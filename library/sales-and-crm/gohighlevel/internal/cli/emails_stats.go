// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-05-20: email stats — GET /emails/public/v2/locations/:locationId/campaigns/stats/email-campaigns/:sourceId)

package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/client"
)

func newEmailsStatsCmd(flags *rootFlags) *cobra.Command {
	var flagLocationId string

	cmd := &cobra.Command{
		Use:   "stats <campaign-id>",
		Short: "Fetch v2 statistics for a single email campaign",
		Long: `Fetch per-campaign statistics from the GHL v2 stats endpoint:
GET /emails/public/v2/locations/:locationId/campaigns/stats/email-campaigns/:sourceId

Returns openRate, clickRate, unsubscribeRate, and bounceRate.

NOTE: This endpoint may return HTTP 403 when authenticated with a PIT (Private Integration
Token). If you see a 403, use a full OAuth token or an Agency API key instead.`,
		Example: "  gohighlevel-pp-cli email stats abc123 --location-id F9YlSB15qA1pRCrPsTSw",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:endpoint":   "emails.stats",
			"pp:method":     "GET",
			"pp:path":       "/emails/public/v2/locations/{locationId}/campaigns/stats/email-campaigns/{sourceId}",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceID := args[0]

			// Resolve location ID: flag > global profile
			if flagLocationId == "" {
				flagLocationId = resolveLocationID()
			}
			if flagLocationId == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"location-id\" not set (or set via --location / active profile)")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/emails/public/v2/locations/%s/campaigns/stats/email-campaigns/%s",
				flagLocationId, sourceID)

			data, prov, err := resolveRead(cmd.Context(), c, flags, "emails", false, path, nil, nil)
			if err != nil {
				apiErr := classifyAPIError(err, flags)
				// Surface a targeted hint for the common PIT-token 403 on this endpoint.
				if isHTTP403(err) {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"hint: The v2 email stats endpoint often returns 403 for PIT tokens.\n"+
							"      Try an Agency API key or full OAuth token instead.\n")
				}
				return apiErr
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, 1, prov)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&flagLocationId, "location-id", "", "GHL sub-account (location) ID (overrides --location / active profile)")
	return cmd
}

// isHTTP403 reports whether err is a client.APIError with status 403.
func isHTTP403(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 403
	}
	return false
}
