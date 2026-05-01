// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type journeyEvent struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`
	LinkID    string `json:"link_id,omitempty"`
	ShortLink string `json:"shortLink,omitempty"`
	Value     string `json:"value,omitempty"`
}

func newCustomersJourneyCmd(flags *rootFlags) *cobra.Command {
	var customerID string
	var externalID string
	var email string

	cmd := &cobra.Command{
		Use:   "journey",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "See every link a customer clicked, when they became a lead, and when they purchased",
		Long: `Stitch together a single customer's full attribution timeline across links,
events, leads, and sales. Live API call: pulls /events filtered by customerId
across event types and renders the chronological timeline.`,
		Example: `  dub-pp-cli customers journey --customer cus_abc --json
  dub-pp-cli customers journey --external-id user_42 --agent
  dub-pp-cli customers journey --email alice@example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if customerID == "" && externalID == "" && email == "" {
				return usageErr(fmt.Errorf("one of --customer, --external-id, or --email is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// resolve to customer ID if needed
			if customerID == "" {
				params := map[string]string{}
				if externalID != "" {
					params["externalId"] = externalID
				}
				if email != "" {
					params["email"] = email
				}
				resp, err := c.Get("/customers", params)
				if err != nil {
					return classifyAPIError(err)
				}
				var custs []map[string]any
				if err := json.Unmarshal(resp, &custs); err != nil {
					return apiErr(fmt.Errorf("parse customers: %w", err))
				}
				if len(custs) == 0 {
					return notFoundErr(fmt.Errorf("no customer matched the given identifier"))
				}
				customerID = stringField(custs[0], "id")
				if customerID == "" {
					return notFoundErr(fmt.Errorf("matched customer has no id"))
				}
			}

			fetchEvents := func(event string) ([]map[string]any, error) {
				params := map[string]string{
					"event":      event,
					"customerId": customerID,
					"interval":   "all",
				}
				resp, err := c.Get("/events", params)
				if err != nil {
					return nil, classifyAPIError(err)
				}
				var rows []map[string]any
				if err := json.Unmarshal(resp, &rows); err != nil {
					return nil, fmt.Errorf("parse /events for %s: %w", event, err)
				}
				return rows, nil
			}

			timeline := make([]journeyEvent, 0)
			for _, evType := range []string{"clicks", "leads", "sales"} {
				rows, err := fetchEvents(evType)
				if err != nil {
					// keep going — partial results still useful
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s events failed: %v\n", evType, err)
					continue
				}
				for _, r := range rows {
					timeline = append(timeline, journeyEvent{
						Timestamp: stringField(r, "timestamp"),
						Event:     stringField(r, "event"),
						LinkID:    stringField(r, "link_id"),
						ShortLink: stringField(r, "shortLink"),
						Value:     valueFor(r, evType),
					})
				}
			}
			sort.Slice(timeline, func(i, j int) bool { return timeline[i].Timestamp < timeline[j].Timestamp })

			result := map[string]any{
				"customer_id": customerID,
				"events":      timeline,
				"summary": map[string]int{
					"clicks": countEvents(timeline, "click"),
					"leads":  countEvents(timeline, "lead"),
					"sales":  countEvents(timeline, "sale"),
				},
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}
			if len(timeline) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no events recorded for customer %s\n", customerID)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Journey for customer %s — %d events:\n\n", customerID, len(timeline))
			headers := []string{"TIMESTAMP", "EVENT", "LINK", "VALUE"}
			rowsTbl := make([][]string, 0, len(timeline))
			for _, e := range timeline {
				link := e.ShortLink
				if link == "" {
					link = e.LinkID
				}
				rowsTbl = append(rowsTbl, []string{e.Timestamp, e.Event, link, e.Value})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().StringVar(&customerID, "customer", "", "Dub customer ID (cus_*)")
	cmd.Flags().StringVar(&externalID, "external-id", "", "External ID (your system's customer reference)")
	cmd.Flags().StringVar(&email, "email", "", "Customer email")
	return cmd
}

// valueFor extracts the human-meaningful payload for a given event row.
// For sales events this is amount + currency; for leads it is name; for clicks it is country.
func valueFor(r map[string]any, eventType string) string {
	switch eventType {
	case "sales":
		amt := intField(r, "saleAmount")
		cur := stringField(r, "currency")
		if amt == 0 {
			amt = intField(r, "amount")
		}
		if cur != "" {
			return fmt.Sprintf("%d %s", amt, cur)
		}
		if amt > 0 {
			return fmt.Sprintf("%d", amt)
		}
		return stringField(r, "saleName")
	case "leads":
		if name := stringField(r, "leadName"); name != "" {
			return name
		}
		return stringField(r, "name")
	default: // clicks
		country := stringField(r, "country")
		device := stringField(r, "device")
		if country != "" && device != "" {
			return country + "/" + device
		}
		return country
	}
}

func countEvents(events []journeyEvent, kind string) int {
	n := 0
	for _, e := range events {
		if e.Event == kind {
			n++
		}
	}
	return n
}
