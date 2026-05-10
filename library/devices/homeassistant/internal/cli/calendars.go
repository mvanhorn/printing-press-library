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

func newCalendarsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calendars",
		Short: "List calendar entities or query calendar events",
	}
	cmd.AddCommand(newCalendarsListCmd(flags))
	cmd.AddCommand(newCalendarsEventsCmd(flags))
	return cmd
}

func newCalendarsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all calendar entities",
		Example: `  # List calendars
  homeassistant-pp-cli calendars list

  # As JSON
  homeassistant-pp-cli calendars list --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "calendars.list_calendars",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []map[string]string{
					{"entity_id": "calendar.personal", "name": "Personal"},
				}, flags)
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(context.Background(), "GET", c.Config.BaseURL+"/calendars", nil)
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

			var calendars []struct {
				EntityID string `json:"entity_id"`
				Name     string `json:"name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&calendars); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), calendars, flags)
			}

			var rows [][]string
			for _, cal := range calendars {
				rows = append(rows, []string{cal.EntityID, cal.Name})
			}
			return flags.printTable(cmd, []string{"ENTITY ID", "NAME"}, rows)
		},
	}
	return cmd
}

func newCalendarsEventsCmd(flags *rootFlags) *cobra.Command {
	var (
		start string
		end   string
	)

	cmd := &cobra.Command{
		Use:   "events <calendar_entity_id>",
		Short: "Query events from a specific calendar",
		Example: `  # Get events for a calendar
  homeassistant-pp-cli calendars events calendar.holidays --start 2026-05-01T00:00:00Z --end 2026-06-01T00:00:00Z`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "calendars.get_calendar_events",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}

			if start == "" || end == "" {
				return fmt.Errorf("--start and --end are required")
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			entityID := args[0]
			req, err := http.NewRequestWithContext(context.Background(), "GET",
				fmt.Sprintf("%s/calendars/%s", c.Config.BaseURL, entityID), nil)
			if err != nil {
				return err
			}

			q := req.URL.Query()
			q.Set("start", start)
			q.Set("end", end)
			req.URL.RawQuery = q.Encode()

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

			var events []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}

			if len(events) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No events found in the specified range.")
				return nil
			}

			var rows [][]string
			for _, e := range events {
				summary, _ := e["summary"].(string)
				location, _ := e["location"].(string)
				rows = append(rows, []string{summary, location})
			}
			return flags.printTable(cmd, []string{"SUMMARY", "LOCATION"}, rows)
		},
	}

	cmd.Flags().StringVar(&start, "start", "", "Start time (ISO 8601)")
	cmd.Flags().StringVar(&end, "end", "", "End time (ISO 8601)")

	return cmd
}
