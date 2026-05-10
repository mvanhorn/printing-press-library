package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/cliutil"
)

func newTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template <jinja2_template>",
		Short: "Render a Home Assistant Jinja2 template server-side",
		Example: `  # Get the current time from HA
  homeassistant-pp-cli template "It is {{ now() }}!"

  # Get a device tracker state
  homeassistant-pp-cli template "{{ states('device_tracker.phone') }}"

  # Count entities in a domain
  homeassistant-pp-cli template "{{ states.light | count }} lights"`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "template.render_template",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "template rendered")
				return nil
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			payload, _ := json.Marshal(map[string]string{"template": args[0]})

			req, err := http.NewRequestWithContext(context.Background(), "POST",
				c.Config.BaseURL+"/template",
				bytes.NewBuffer(payload))
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

			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("API error: %s (status %d)", string(body), resp.StatusCode)
			}

			// Template endpoint returns plain text
			if flags.asJSON {
				result := map[string]string{"result": string(body)}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(body))
			return nil
		},
	}
	return cmd
}
