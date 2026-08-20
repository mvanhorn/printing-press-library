// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(ticketmaster-presale-planner): expose real Discovery API presale windows.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type ticketmasterPresale struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	URL           string `json:"url,omitempty"`
	StartDateTime string `json:"startDateTime,omitempty"`
	EndDateTime   string `json:"endDateTime,omitempty"`
}

type ticketmasterEvent struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Dates struct {
		Start struct {
			LocalDate string `json:"localDate,omitempty"`
			DateTime  string `json:"dateTime,omitempty"`
		} `json:"start"`
	} `json:"dates"`
	Sales struct {
		Presales []ticketmasterPresale `json:"presales"`
	} `json:"sales"`
	Embedded struct {
		Venues []struct {
			Name string `json:"name,omitempty"`
			City struct {
				Name string `json:"name,omitempty"`
			} `json:"city"`
		} `json:"venues"`
	} `json:"_embedded"`
}

type presaleResult struct {
	EventID      string `json:"event_id"`
	Event        string `json:"event"`
	EventDate    string `json:"event_date,omitempty"`
	Venue        string `json:"venue,omitempty"`
	City         string `json:"city,omitempty"`
	Presale      string `json:"presale"`
	Status       string `json:"status"`
	StartsAt     string `json:"starts_at,omitempty"`
	EndsAt       string `json:"ends_at,omitempty"`
	CheckoutURL  string `json:"checkout_url,omitempty"`
	HoursToOpen  *int64 `json:"hours_to_open,omitempty"`
	EventPageURL string `json:"event_page_url,omitempty"`
}

func newEventsPresalesCmd(flags *rootFlags) *cobra.Command {
	var keyword, city, countryCode, stateCode, attractionID, venueID, classification, status string
	var window, opensWithin, limit int

	cmd := &cobra.Command{
		Use:   "presales",
		Short: "Find real presale windows and show whether each is upcoming, open, or ended",
		Long: `Query Ticketmaster Discovery directly for events whose presale windows
intersect the requested period. Unlike events on-sale-soon, this reads
sales.presales rather than treating the public on-sale date as a presale.

This command discovers official checkout URLs but does not buy tickets.`,
		Example: `  ticketmaster-pp-cli events presales --keyword "The National" --window 30
  ticketmaster-pp-cli events presales --city Brooklyn --status upcoming --opens-within 48 --json`,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:client-call": "Ticketmaster Discovery GET /events",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if window < 1 || window > 365 {
				return usageErr(fmt.Errorf("--window must be between 1 and 365 days"))
			}
			if limit < 1 || limit > 200 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 200"))
			}
			if opensWithin < 0 {
				return usageErr(fmt.Errorf("--opens-within cannot be negative"))
			}
			status = strings.ToLower(strings.TrimSpace(status))
			if status != "all" && status != "upcoming" && status != "open" && status != "ended" && status != "unknown" {
				return usageErr(fmt.Errorf("--status must be one of all, upcoming, open, ended, unknown"))
			}

			now := time.Now().UTC()
			end := now.Add(time.Duration(window) * 24 * time.Hour)
			params := map[string]string{
				"preSaleDateTime":    now.Format(time.RFC3339) + "," + end.Format(time.RFC3339),
				"sort":               "date,asc",
				"size":               fmt.Sprintf("%d", limit),
				"keyword":            keyword,
				"city":               city,
				"countryCode":        countryCode,
				"stateCode":          stateCode,
				"attractionId":       attractionID,
				"venueId":            venueID,
				"classificationName": classification,
			}
			for key, value := range params {
				if strings.TrimSpace(value) == "" {
					delete(params, key)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetNoCache(cmd.Context(), "/events", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events, err := decodeTicketmasterEvents(raw)
			if err != nil {
				return err
			}
			results := flattenPresales(events, now, status, opensWithin)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching presale windows found.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "STATUS\tOPENS\tEVENT DATE\tEVENT\tPRESALE\tVENUE\tCITY")
			for _, result := range results {
				opens := result.StartsAt
				if result.Status == "open" {
					opens = "now"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					result.Status, opens, result.EventDate, result.Event, result.Presale, result.Venue, result.City)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&keyword, "keyword", "", "Artist, team, show, or event keyword")
	cmd.Flags().StringVar(&city, "city", "", "City name")
	cmd.Flags().StringVar(&countryCode, "country-code", "", "ISO country code")
	cmd.Flags().StringVar(&stateCode, "state-code", "", "State or province code")
	cmd.Flags().StringVar(&attractionID, "attraction-id", "", "Ticketmaster attraction ID")
	cmd.Flags().StringVar(&venueID, "venue-id", "", "Ticketmaster venue ID")
	cmd.Flags().StringVar(&classification, "classification", "", "Classification name such as music or sports")
	cmd.Flags().StringVar(&status, "status", "all", "Window status: all, upcoming, open, ended, or unknown")
	cmd.Flags().IntVar(&window, "window", 30, "Days ahead to search (1-365)")
	cmd.Flags().IntVar(&opensWithin, "opens-within", 0, "Keep upcoming presales opening within this many hours (0 disables)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum events to request (1-200)")
	return cmd
}

func decodeTicketmasterEvents(raw json.RawMessage) ([]ticketmasterEvent, error) {
	var envelope struct {
		Embedded struct {
			Events []ticketmasterEvent `json:"events"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode Ticketmaster events: %w", err)
	}
	return envelope.Embedded.Events, nil
}

func flattenPresales(events []ticketmasterEvent, now time.Time, wantedStatus string, opensWithin int) []presaleResult {
	results := make([]presaleResult, 0)
	for _, event := range events {
		venue, city := "", ""
		if len(event.Embedded.Venues) > 0 {
			venue = event.Embedded.Venues[0].Name
			city = event.Embedded.Venues[0].City.Name
		}
		for _, presale := range event.Sales.Presales {
			start, startOK := parseTicketmasterTime(presale.StartDateTime)
			end, endOK := parseTicketmasterTime(presale.EndDateTime)
			windowStatus := presaleStatus(now, start, startOK, end, endOK)
			if wantedStatus != "all" && windowStatus != wantedStatus {
				continue
			}
			var hoursToOpen *int64
			if startOK && start.After(now) {
				hours := int64(start.Sub(now).Hours())
				hoursToOpen = &hours
				if opensWithin > 0 && start.Sub(now) > time.Duration(opensWithin)*time.Hour {
					continue
				}
			} else if opensWithin > 0 && windowStatus == "upcoming" {
				continue
			}
			results = append(results, presaleResult{
				EventID: event.ID, Event: event.Name, EventDate: event.Dates.Start.LocalDate,
				Venue: venue, City: city, Presale: firstNonEmpty(presale.Name, presale.Description, "Presale"),
				Status: windowStatus, StartsAt: presale.StartDateTime, EndsAt: presale.EndDateTime,
				CheckoutURL: presale.URL, HoursToOpen: hoursToOpen, EventPageURL: event.URL,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return firstNonEmpty(results[i].StartsAt, results[i].EventDate) < firstNonEmpty(results[j].StartsAt, results[j].EventDate)
	})
	return results
}

func parseTicketmasterTime(value string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, value)
	return parsed, err == nil
}

func presaleStatus(now, start time.Time, startOK bool, end time.Time, endOK bool) string {
	if !startOK && !endOK {
		return "unknown"
	}
	if endOK && !now.Before(end) {
		return "ended"
	}
	if startOK && now.Before(start) {
		return "upcoming"
	}
	return "open"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
