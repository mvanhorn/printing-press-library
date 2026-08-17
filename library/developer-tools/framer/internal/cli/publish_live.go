// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newPublishLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Create a preview deployment and optionally deploy to production via the Server API",
		Long: `Create a preview deployment via the Framer Server API (WebSocket bridge),
show the preview URL and deployment ID, then optionally promote to production.

Requires FRAMER_PROJECT_URL and FRAMER_API_KEY environment variables.`,
		Example: `  # Interactive: preview then confirm deploy
  framer-pp-cli publish

  # Non-interactive: auto-deploy to production
  framer-pp-cli publish --yes

  # JSON output for agents
  framer-pp-cli publish --json --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would create preview deployment and optionally deploy to production")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			// Create preview deployment.
			fmt.Fprintln(cmd.ErrOrStderr(), "Creating preview deployment...")
			raw, err := bc.Call("publish-preview")
			if err != nil {
				return fmt.Errorf("publish-preview failed: %w", err)
			}

			// Parse the preview response to extract deployment ID and URL.
			var preview struct {
				DeploymentID string `json:"deploymentId"`
				URL          string `json:"url"`
			}
			if err := json.Unmarshal(raw, &preview); err != nil {
				return fmt.Errorf("parsing preview response: %w", err)
			}

			if flags.asJSON {
				result := map[string]any{
					"stage":        "preview",
					"deploymentId": preview.DeploymentID,
					"url":          preview.URL,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(result); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Preview URL:     %s\n", preview.URL)
				fmt.Fprintf(cmd.OutOrStdout(), "Deployment ID:   %s\n", preview.DeploymentID)
			}

			// Ask for confirmation unless --yes is set.
			if !flags.yes && !flags.noInput {
				fmt.Fprint(cmd.ErrOrStderr(), "\nDeploy to production? [y/N] ")
				reader := bufio.NewReader(cmd.InOrStdin())
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
					return nil
				}
			}

			// Deploy to production.
			fmt.Fprintln(cmd.ErrOrStderr(), "Deploying to production...")
			deployRaw, err := bc.Call("deploy", preview.DeploymentID)
			if err != nil {
				return fmt.Errorf("deploy failed: %w", err)
			}

			if flags.asJSON {
				var deployResult any
				if json.Unmarshal(deployRaw, &deployResult) == nil {
					result := map[string]any{
						"stage":        "deployed",
						"deploymentId": preview.DeploymentID,
						"url":          preview.URL,
						"deploy":       deployResult,
					}
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(result)
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Deployed to production.")
			return nil
		},
	}

	return cmd
}
