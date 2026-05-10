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

func newHAConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the Home Assistant instance configuration",
		Example: `  # View full instance config
  homeassistant-pp-cli config

  # Get just the version and location
  homeassistant-pp-cli config --json --select version,location_name,time_zone`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "config.get_config",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"version":       "2026.5.0",
					"location_name": "Home",
					"time_zone":     "UTC",
				}, flags)
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(context.Background(), "GET", c.Config.BaseURL+"/config", nil)
			if err != nil {
				return err
			}
			if auth := c.Config.AuthHeader(); auth != "" {
				req.Header.Set("Authorization", auth)
			}

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("API error: %s (status %d)", string(body), resp.StatusCode)
			}

			var config map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), config, flags)
			}

			var rows [][]string
			for _, key := range []string{"version", "location_name", "time_zone", "config_dir", "elevation", "latitude", "longitude"} {
				if val, ok := config[key]; ok {
					rows = append(rows, []string{key, fmt.Sprintf("%v", val)})
				}
			}
			if components, ok := config["components"]; ok {
				if arr, ok := components.([]interface{}); ok {
					rows = append(rows, []string{"components", fmt.Sprintf("%d loaded", len(arr))})
				}
			}
			return flags.printTable(cmd, []string{"KEY", "VALUE"}, rows)
		},
	}
	return cmd
}
