// Copyright 2026 jacobprice. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/runcloud/internal/cliutil"
)

type onboardSummary struct {
	ClientEmail  string `json:"client_email"`
	ClientID     string `json:"client_id,omitempty"`
	ServerID     string `json:"server_id,omitempty"`
	MagicLinkURL string `json:"magic_link_url,omitempty"`
	Stage        string `json:"stage,omitempty"`
	Error        string `json:"error,omitempty"`
}

func newAgencyOnboardCmd(flags *rootFlags) *cobra.Command {
	var clientEmail, packageID, serverName string
	var magicLink bool

	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Onboard a new agency client end-to-end: create client → assign package → spin up server → optional magic link",
		Long: `Chains POST /agency/clients, POST /agency/client-servers, and optionally
POST /agency/clients/{id}/magiclink. Emits a credentials summary at the end
with client_id, server_id, and (when requested) the magic-link URL.

On partial failure the summary still reports whichever steps succeeded so the
operator can finish onboarding manually instead of starting over.`,
		Example: `  runcloud-pp-cli agency onboard \
    --client-email alice@example.com \
    --package pkg_abc123 \
    --server-name prod-1 \
    --magic-link`,
		RunE: func(cmd *cobra.Command, args []string) error {
			summary := onboardSummary{ClientEmail: clientEmail}

			if clientEmail == "" || packageID == "" || serverName == "" {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "(dry run - need --client-email, --package, --server-name to call agency onboarding chain)")
					return nil
				}
				return cmd.Help()
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(),
					"(dry run - would POST /agency/clients then /agency/client-servers (package=%s, server-name=%s)",
					packageID, serverName)
				if magicLink {
					fmt.Fprint(cmd.OutOrStdout(), " then /agency/clients/{id}/magiclink")
				}
				fmt.Fprintln(cmd.OutOrStdout(), ")")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				summary.Stage = "verify-env-skip"
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: create client
			clientBody := map[string]any{"email": clientEmail}
			raw, _, err := c.Post("/agency/clients", clientBody)
			if err != nil {
				summary.Stage = "create_client"
				summary.Error = err.Error()
				_ = printJSONFiltered(cmd.OutOrStdout(), summary, flags)
				return classifyAPIError(err, flags)
			}
			summary.ClientID = extractIDField(raw)

			// Step 2: assign client-server
			serverBody := map[string]any{
				"clientId":        summary.ClientID,
				"serverPackageId": packageID,
				"name":            serverName,
			}
			raw2, _, err := c.Post("/agency/client-servers", serverBody)
			if err != nil {
				summary.Stage = "create_client_server"
				summary.Error = err.Error()
				_ = printJSONFiltered(cmd.OutOrStdout(), summary, flags)
				return classifyAPIError(err, flags)
			}
			summary.ServerID = extractIDField(raw2)

			// Step 3 (optional): magic link
			if magicLink && summary.ClientID != "" {
				path := fmt.Sprintf("/agency/clients/%s/magiclink", summary.ClientID)
				raw3, _, err := c.Post(path, map[string]any{})
				if err != nil {
					summary.Stage = "magic_link"
					summary.Error = err.Error()
					_ = printJSONFiltered(cmd.OutOrStdout(), summary, flags)
					return classifyAPIError(err, flags)
				}
				summary.MagicLinkURL = jsonStringField(string(raw3), "url", "magicLink", "data.url")
			}

			summary.Stage = "complete"
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}

	cmd.Flags().StringVar(&clientEmail, "client-email", "", "Email of the new agency client (required)")
	cmd.Flags().StringVar(&packageID, "package", "", "Server package ID to assign (required)")
	cmd.Flags().StringVar(&serverName, "server-name", "", "Name for the new client-server (required)")
	cmd.Flags().BoolVar(&magicLink, "magic-link", false, "Also generate a magic-link URL for the new client")

	return cmd
}

// extractIDField pulls an id from a POST response, tolerating both bare and
// wrapped envelopes ({"id":...} vs {"data":{"id":...}}).
func extractIDField(raw json.RawMessage) string {
	s := string(raw)
	if id := jsonAnyField(s, "id", "data.id"); id != "" {
		return id
	}
	return ""
}
