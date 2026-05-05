package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type pickerWaitSession struct {
	ID            string `json:"id"`
	PickerURI     string `json:"pickerUri"`
	ExpireTime    string `json:"expireTime"`
	MediaItemsSet bool   `json:"mediaItemsSet"`
	PollingConfig struct {
		PollInterval string `json:"pollInterval"`
		TimeoutIn    string `json:"timeoutIn"`
	} `json:"pollingConfig"`
}

func newPickerWaitCmd(flags *rootFlags) *cobra.Command {
	var fallbackInterval time.Duration
	var maxWait time.Duration

	cmd := &cobra.Command{
		Use:   "wait <session-id>",
		Short: "Poll a Picker session until selected media items are ready.",
		Example: "  google-photos-pp-cli picker wait picker-session-id --json\n" +
			"  google-photos-pp-cli picker wait picker-session-id --interval 5s --wait-timeout 2m",
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			sessionID := args[0]
			path := "https://photospicker.googleapis.com/v1/sessions/{sessionId}"
			path = replacePathParam(path, "sessionId", sessionID)

			start := time.Now()
			for {
				data, err := c.Get(path, nil)
				if err != nil {
					return classifyAPIError(err)
				}

				var session pickerWaitSession
				if err := json.Unmarshal(data, &session); err != nil {
					return fmt.Errorf("parsing picker session: %w", err)
				}
				if session.MediaItemsSet {
					return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
				}

				wait := fallbackInterval
				if parsed := parseGoogleDuration(session.PollingConfig.PollInterval); parsed > 0 {
					wait = parsed
				}

				deadline := maxWait
				if deadline == 0 {
					deadline = parseGoogleDuration(session.PollingConfig.TimeoutIn)
				}
				if deadline > 0 && time.Since(start)+wait > deadline {
					return fmt.Errorf("picker session %s was not ready before timeout %s", sessionID, deadline)
				}

				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				case <-time.After(wait):
				}
			}
		},
	}

	cmd.Flags().DurationVar(&fallbackInterval, "interval", 3*time.Second, "Fallback poll interval when the API does not return pollingConfig.pollInterval")
	cmd.Flags().DurationVar(&maxWait, "wait-timeout", 0, "Maximum time to wait; default uses pollingConfig.timeoutIn when returned by the API")
	return cmd
}

func parseGoogleDuration(value string) time.Duration {
	if value == "" || value == "0" {
		return 0
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return d
}
