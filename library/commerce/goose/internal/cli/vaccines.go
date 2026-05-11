package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newVaccinesCmd is the parent for `vaccines` subcommands.
func newVaccinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vaccines",
		Short: "Pet vaccination operations (expiring, missing)",
	}
	cmd.AddCommand(newVaccinesExpiringCmd(flags))
	return cmd
}

// newVaccinesExpiringCmd implements `vaccines expiring [--within Nd] [--by-visit]`.
// Uses the documented `expiring-or-missing-vaccinations-export` report endpoint.
// With --by-visit, filters to pets that also have a CONFIRMED booking within
// the same window by intersecting with the dashboard/invoices result.
func newVaccinesExpiringCmd(flags *rootFlags) *cobra.Command {
	var within string
	var byVisit bool
	cmd := &cobra.Command{
		Use:         "expiring",
		Short:       "Pets with expiring vaccinations; --by-visit filters to upcoming-booking pets",
		Example:     "  goose-pp-cli vaccines expiring --within 30d --by-visit --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			days, err := parseDayWindow(within)
			if err != nil {
				return err
			}
			now := time.Now()
			endDate := now.AddDate(0, 0, days).Format("2006-01-02")

			// Reports endpoint gives us all expiring vaccines for the window.
			params := map[string]string{
				"date":      endDate,
				"startDate": now.Format("2006-01-02"),
				"endDate":   endDate,
			}
			raw, err := c.Get("/reports/expiring-or-missing-vaccinations-export", params)
			if err != nil {
				return fmt.Errorf("fetching expiring-vaccinations export: %w", err)
			}

			// The report response shape can be a result array or a CSV-like
			// envelope; we accept either.
			rows := decodeReportRows(raw)
			if byVisit {
				visitingPetIds, vErr := collectUpcomingPetIds(c, now, days)
				if vErr != nil {
					return fmt.Errorf("collecting upcoming pet IDs: %w", vErr)
				}
				filtered := rows[:0]
				for _, r := range rows {
					if id := strOrEmpty(r["petId"]); id != "" {
						if visitingPetIds[id] {
							filtered = append(filtered, r)
						}
						continue
					}
					// If the export does not include a petId, fall back to a
					// loose match on petName+ownerName.
					name := strings.ToLower(strOrEmpty(r["petName"]))
					owner := strings.ToLower(strOrEmpty(r["ownerName"]))
					for k := range visitingPetIds {
						if strings.HasPrefix(k, "name:") && strings.Contains(k[5:], name) && strings.Contains(k[5:], owner) {
							filtered = append(filtered, r)
							break
						}
					}
				}
				rows = filtered
			}

			if flags.asJSON || flags.compact || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pets with vaccinations expiring within %dd", days)
			if byVisit {
				fmt.Fprintf(cmd.OutOrStdout(), " AND upcoming booking")
			}
			fmt.Fprintf(cmd.OutOrStdout(), ": %d\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s (%s)\n", strOrEmpty(r["petName"]), strOrEmpty(r["ownerName"]), strOrEmpty(r["vaccineExpiration"]))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&within, "within", "30d", "Window in days (e.g. 14d, 30d, 60d)")
	cmd.Flags().BoolVar(&byVisit, "by-visit", false, "Only pets that also have a CONFIRMED booking inside the window")
	return cmd
}

// decodeReportRows accepts both `{ "results": [...] }` and bare array shapes
// and returns []map[string]any. Reports vary slug-to-slug.
func decodeReportRows(raw json.RawMessage) []map[string]any {
	var env struct {
		Results []map[string]any `json:"results"`
		Data    []map[string]any `json:"data"`
		Rows    []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		switch {
		case len(env.Results) > 0:
			return env.Results
		case len(env.Data) > 0:
			return env.Data
		case len(env.Rows) > 0:
			return env.Rows
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

// collectUpcomingPetIds calls dashboard/invoices for each day in the window
// and returns the set of locationPetProfile IDs (plus name+owner fallback keys).
// This is intentionally simple — no pagination, since the dashboard endpoint
// returns the full day's roster in one call.
func collectUpcomingPetIds(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, start time.Time, days int) (map[string]bool, error) {
	out := map[string]bool{}
	for d := 0; d <= days; d++ {
		date := start.AddDate(0, 0, d).Format("2006-01-02")
		params := map[string]string{
			"visitDate":     date,
			"invoiceStatus": "CONFIRMED",
			"limit":         "500",
			"includes":      "order.pets.locationPetProfile",
		}
		raw, err := c.Get("/dashboard/invoices", params)
		if err != nil {
			return nil, err
		}
		var env struct {
			Results []json.RawMessage `json:"results"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		for _, r := range env.Results {
			collectInvoicePetIds(r, out)
		}
	}
	return out, nil
}

func collectInvoicePetIds(raw json.RawMessage, out map[string]bool) {
	var inv struct {
		Order struct {
			OrderUser struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"orderUser"`
			Pets []struct {
				LocationPetProfile struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"locationPetProfile"`
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"pets"`
		} `json:"order"`
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return
	}
	owner := strings.ToLower(strings.TrimSpace(inv.Order.OrderUser.FirstName + " " + inv.Order.OrderUser.LastName))
	for _, p := range inv.Order.Pets {
		if p.LocationPetProfile.ID != "" {
			out[p.LocationPetProfile.ID] = true
		}
		if p.ID != "" {
			out[p.ID] = true
		}
		name := strings.ToLower(p.LocationPetProfile.DisplayName)
		if name == "" {
			name = strings.ToLower(p.DisplayName)
		}
		if name != "" {
			out["name:"+name+" "+owner] = true
		}
	}
}

func parseDayWindow(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 30, nil
	}
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid day window %q (want e.g. 30d)", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("day window cannot be negative")
	}
	return n, nil
}

func strOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
