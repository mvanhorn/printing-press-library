package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/homeassistant/internal/cliutil"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var (
		entityIDs string
		startTime string
		endTime   string
		minimal   bool
	)

	cmd := &cobra.Command{
		Use:   "history",
		Short: "Query state change history for entities over a time period",
		Example: `  # History for a sensor over the past day
  homeassistant-pp-cli history --entity sensor.living_room_temperature

  # History with a custom time range
  homeassistant-pp-cli history --entity sensor.cpu --start 2026-05-09T00:00:00 --end 2026-05-09T12:00:00

  # Minimal response for faster queries
  homeassistant-pp-cli history --entity light.kitchen --minimal --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "history.get_history",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}

			if entityIDs == "" {
				return fmt.Errorf("--entity is required (comma-separated entity IDs)")
			}

			c, err := flags.newClient() // pp:client-call
			if err != nil {
				return err
			}

			path := c.Config.BaseURL + "/history/period"
			if startTime != "" {
				path += "/" + startTime
			}

			req, err := http.NewRequestWithContext(context.Background(), "GET", path, nil)
			if err != nil {
				return err
			}

			q := req.URL.Query()
			q.Set("filter_entity_id", entityIDs)
			if endTime != "" {
				q.Set("end_time", endTime)
			}
			if minimal {
				q.Set("minimal_response", "")
			}
			q.Set("no_attributes", "")
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

			// Response is array-of-arrays: [[state_changes_for_entity_1], [state_changes_for_entity_2]]
			var history [][]struct {
				EntityID    string `json:"entity_id"`
				State       string `json:"state"`
				LastChanged string `json:"last_changed"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), history, flags)
			}

			var rows [][]string
			for _, entityHistory := range history {
				for _, h := range entityHistory {
					ts := h.LastChanged
					if t, err := time.Parse(time.RFC3339, h.LastChanged); err == nil {
						ts = t.Local().Format("2006-01-02 15:04:05")
					}
					rows = append(rows, []string{h.EntityID, h.State, ts})
				}
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No history found for the specified entities.")
				return nil
			}

			return flags.printTable(cmd, []string{"ENTITY", "STATE", "CHANGED AT"}, rows)
		},
	}

	cmd.Flags().StringVar(&entityIDs, "entity", "", "Comma-separated entity IDs to query")
	cmd.Flags().StringVar(&startTime, "start", "", "Start time (YYYY-MM-DDThh:mm:ss)")
	cmd.Flags().StringVar(&endTime, "end", "", "End time (YYYY-MM-DDThh:mm:ss)")
	cmd.Flags().BoolVar(&minimal, "minimal", false, "Request minimal response (faster)")

	return cmd
}
