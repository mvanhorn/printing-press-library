package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/homeassistant/internal/cliutil"
)

func newErrorLogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "error-log",
		Short: "Retrieve the Home Assistant error log for the current session",
		Example: `  # View the error log
  homeassistant-pp-cli error-log

  # Get as JSON for agent parsing
  homeassistant-pp-cli error-log --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "error_log.get_error_log",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "(no errors)")
				return nil
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(context.Background(), "GET", c.Config.BaseURL+"/error_log", nil)
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

			// error_log returns plain text
			body, _ := io.ReadAll(resp.Body)
			text := string(body)

			if flags.asJSON {
				result := map[string]string{"log": text}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if text == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "(no errors)")
			} else {
				fmt.Fprint(cmd.OutOrStdout(), text)
			}
			return nil
		},
	}
	return cmd
}
