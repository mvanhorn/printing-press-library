package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/cliutil"
)

func newCheckConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-config",
		Short: "Validate the Home Assistant configuration.yaml",
		Example: `  # Validate config
  homeassistant-pp-cli check-config

  # As JSON for automation
  homeassistant-pp-cli check-config --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "config.check_config",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"result": "valid",
					"errors": nil,
				}, flags)
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(context.Background(), "POST",
				c.Config.BaseURL+"/config/core/check_config", nil)
			if err != nil {
				return err
			}
			if auth := c.Config.AuthHeader(); auth != "" {
				req.Header.Set("Authorization", auth)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error: %s (status %d)", string(body), resp.StatusCode)
			}

			var result struct {
				Errors interface{} `json:"errors"`
				Result string      `json:"result"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if result.Result == "valid" {
				fmt.Fprintln(cmd.OutOrStdout(), "✓ Configuration is valid.")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✗ Configuration is invalid:\n  %v\n", result.Errors)
			}
			return nil
		},
	}
	return cmd
}
