package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/homeassistant/internal/cliutil"
)

func newMonitorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor [entity_id]",
		Short: "Stream state changes live to the terminal",
		Example: `  # Watch all state changes
  homeassistant-pp-cli monitor

  # Watch a specific entity
  homeassistant-pp-cli monitor sensor.dream_router_7_cpu_utilization`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			targetEntity := ""
			if len(args) > 0 {
				targetEntity = args[0]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Connecting to stream...")

			req, err := http.NewRequestWithContext(context.Background(), "GET", c.Config.BaseURL+"/stream", nil)
			if err != nil {
				return err
			}
			auth := c.Config.AuthHeader()
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			req.Header.Set("Accept", "text/event-stream")

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("stream returned status %d", resp.StatusCode)
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Listening for events...")

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				dataStr := strings.TrimPrefix(line, "data: ")
				if dataStr == "ping" {
					continue
				}

				var event struct {
					EventType string `json:"event_type"`
					Data      struct {
						EntityID string `json:"entity_id"`
						NewState struct {
							State      string `json:"state"`
							Attributes map[string]interface{} `json:"attributes"`
						} `json:"new_state"`
					} `json:"data"`
					TimeFired string `json:"time_fired"`
				}

				if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
					continue
				}

				if event.EventType != "state_changed" {
					continue
				}

				if targetEntity != "" && event.Data.EntityID != targetEntity {
					continue
				}

				if flags.asJSON {
					b, _ := json.Marshal(event)
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
				} else {
					attrBytes, _ := json.Marshal(event.Data.NewState.Attributes)
					fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s -> %s %s\n",
						cliutil.ParseStoredTime(event.TimeFired).Format("15:04:05"),
						event.Data.EntityID,
						event.Data.NewState.State,
						cliutil.Truncate(string(attrBytes), 100))
				}
			}

			return scanner.Err()
		},
	}
	return cmd
}
