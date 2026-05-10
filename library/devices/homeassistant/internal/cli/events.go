package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"homeassistant-pp-cli/internal/cliutil"
)

func newEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "List event types or fire custom events",
	}
	cmd.AddCommand(newEventsListCmd(flags))
	cmd.AddCommand(newEventsFireCmd(flags))
	return cmd
}

func newEventsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered event types and their listener counts",
		Example: `  # List all events
  homeassistant-pp-cli events list

  # As JSON for filtering
  homeassistant-pp-cli events list --json --select event,listener_count`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "events.list_events",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]any{
					{"event": "state_changed", "listener_count": 5},
				}, flags)
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			data, err := c.Get("/events", nil)
			if err != nil {
				return err
			}

			var events []struct {
				Event         string `json:"event"`
				ListenerCount int    `json:"listener_count"`
			}
			if err := json.Unmarshal(data, &events); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}

			var rows [][]string
			for _, e := range events {
				rows = append(rows, []string{e.Event, fmt.Sprintf("%d", e.ListenerCount)})
			}
			return flags.printTable(cmd, []string{"EVENT TYPE", "LISTENERS"}, rows)
		},
	}
	return cmd
}

func newEventsFireCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fire <event_type> [event_data_json]",
		Short: "Fire a custom event on the Home Assistant event bus",
		Example: `  # Fire a simple event
  homeassistant-pp-cli events fire my_custom_event

  # Fire an event with data
  homeassistant-pp-cli events fire download_file '{"url":"https://example.com/file.txt"}'`,
		Annotations: map[string]string{
			"pp:endpoint": "events.fire_event",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "Event %s fired.\n", args[0])
				return nil
			}

			eventType := args[0]
			payload := "{}"
			if len(args) > 1 {
				payload = args[1]
				if !json.Valid([]byte(payload)) {
					return fmt.Errorf("invalid JSON payload: %s", payload)
				}
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			var body map[string]any
			if err := json.Unmarshal([]byte(payload), &body); err != nil {
				return fmt.Errorf("invalid JSON payload: %w", err)
			}

			path := fmt.Sprintf("/events/%s", url.PathEscape(eventType))
			data, _, err := c.Post(path, body)
			if err != nil {
				return err
			}

			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Event %s fired.\n", eventType)
			}
			return nil
		},
	}
	return cmd
}
