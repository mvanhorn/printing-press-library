package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/homeassistant/internal/cliutil"
)

func newLogbookCmd(flags *rootFlags) *cobra.Command {
	var (
		entityID  string
		startTime string
		endTime   string
	)

	cmd := &cobra.Command{
		Use:   "logbook",
		Short: "View the activity logbook with human-readable event descriptions",
		Example: `  # View recent logbook entries
  homeassistant-pp-cli logbook

  # Filter to a specific entity
  homeassistant-pp-cli logbook --entity alarm_control_panel.area_001

  # View logbook for a date range
  homeassistant-pp-cli logbook --start 2026-05-09T00:00:00 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "logbook.get_logbook",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			path := c.Config.BaseURL + "/logbook"
			if startTime != "" {
				path += "/" + startTime
			}

			req, err := http.NewRequestWithContext(context.Background(), "GET", path, nil)
			if err != nil {
				return err
			}

			q := req.URL.Query()
			if entityID != "" {
				q.Set("entity", entityID)
			}
			if endTime != "" {
				q.Set("end_time", endTime)
			}
			if len(q) > 0 {
				req.URL.RawQuery = q.Encode()
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

			var entries []struct {
				Domain    string `json:"domain"`
				EntityID  string `json:"entity_id"`
				Message   string `json:"message"`
				Name      string `json:"name"`
				When      string `json:"when"`
				ContextID string `json:"context_user_id"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}

			if len(entries) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No logbook entries found.")
				return nil
			}

			var rows [][]string
			for _, e := range entries {
				rows = append(rows, []string{e.When, e.Name, e.Message, e.EntityID})
			}
			return flags.printTable(cmd, []string{"WHEN", "NAME", "MESSAGE", "ENTITY"}, rows)
		},
	}

	cmd.Flags().StringVar(&entityID, "entity", "", "Filter to a specific entity ID")
	cmd.Flags().StringVar(&startTime, "start", "", "Start time (YYYY-MM-DDThh:mm:ssTZD)")
	cmd.Flags().StringVar(&endTime, "end", "", "End time (YYYY-MM-DDThh:mm:ssTZD)")

	return cmd
}
