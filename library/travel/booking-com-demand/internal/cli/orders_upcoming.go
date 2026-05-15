package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com-demand/internal/store"

	"github.com/spf13/cobra"
)

func newOrdersUpcomingCmd(flags *rootFlags) *cobra.Command {
	var days int
	var withMessages bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "upcoming",
		Short: "List orders with imminent check-in dates and message status",
		Long: `Upcoming shows orders with check-in dates within the specified number of days,
optionally enriched with the latest message thread status.

Joins the orders and messages tables from the local store.
Requires synced data: run 'booking-com-demand-pp-cli sync --full' first.`,
		Example: strings.Trim(`
  booking-com-demand-pp-cli orders upcoming
  booking-com-demand-pp-cli orders upcoming --days 3
  booking-com-demand-pp-cli orders upcoming --with-messages
  booking-com-demand-pp-cli orders upcoming --days 14 --with-messages --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nhint: run 'booking-com-demand-pp-cli sync --full' first", err)
			}
			defer s.Close()

			now := time.Now()
			cutoff := now.Add(time.Duration(days) * 24 * time.Hour)

			rows, err := s.DB().Query("SELECT id, data FROM orders")
			if err != nil {
				return fmt.Errorf("querying orders: %w", err)
			}
			defer rows.Close()

			type upcomingOrder struct {
				ID            string `json:"id"`
				Accommodation string `json:"accommodation,omitempty"`
				CheckIn       string `json:"check_in"`
				CheckOut      string `json:"check_out,omitempty"`
				Status        string `json:"status,omitempty"`
				DaysUntil     int    `json:"days_until"`
				GuestName     string `json:"guest_name,omitempty"`
				HasMessages   bool   `json:"has_messages,omitempty"`
				LastMessage   string `json:"last_message,omitempty"`
			}

			var upcoming []upcomingOrder
			for rows.Next() {
				var id, data string
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				var obj map[string]any
				if json.Unmarshal([]byte(data), &obj) != nil {
					continue
				}

				checkinStr := stringFromAny(obj["checkin"])
				if checkinStr == "" {
					checkinStr = stringFromAny(obj["check_in"])
				}
				if checkinStr == "" {
					checkinStr = stringFromAny(obj["checkin_date"])
				}
				if checkinStr == "" {
					continue
				}

				checkin, err := time.Parse("2006-01-02", checkinStr[:10])
				if err != nil {
					continue
				}

				if checkin.Before(now) || checkin.After(cutoff) {
					continue
				}

				order := upcomingOrder{
					ID:        id,
					CheckIn:   checkinStr,
					DaysUntil: int(checkin.Sub(now).Hours() / 24),
				}

				order.CheckOut = stringFromAny(obj["checkout"])
				if order.CheckOut == "" {
					order.CheckOut = stringFromAny(obj["check_out"])
				}
				order.Status = stringFromAny(obj["status"])
				order.GuestName = stringFromAny(obj["guest_name"])
				order.Accommodation = stringFromAny(obj["accommodation_name"])
				if order.Accommodation == "" {
					order.Accommodation = stringFromAny(obj["property_name"])
				}

				upcoming = append(upcoming, order)
			}

			// Enrich with message data if requested
			if withMessages && len(upcoming) > 0 {
				msgRows, err := s.DB().Query("SELECT id, data FROM messages")
				if err == nil {
					defer msgRows.Close()
					orderMessages := map[string]string{}
					orderHasMsg := map[string]bool{}
					for msgRows.Next() {
						var mid, mdata string
						if msgRows.Scan(&mid, &mdata) != nil {
							continue
						}
						var msg map[string]any
						if json.Unmarshal([]byte(mdata), &msg) != nil {
							continue
						}
						oid := stringFromAny(msg["order_id"])
						if oid == "" {
							oid = stringFromAny(msg["booking_id"])
						}
						if oid != "" {
							orderHasMsg[oid] = true
							text := stringFromAny(msg["text"])
							if text == "" {
								text = stringFromAny(msg["message"])
							}
							orderMessages[oid] = truncate(text, 80)
						}
					}
					for i := range upcoming {
						if orderHasMsg[upcoming[i].ID] {
							upcoming[i].HasMessages = true
							upcoming[i].LastMessage = orderMessages[upcoming[i].ID]
						}
					}
				}
			}

			if len(upcoming) == 0 {
				return writeNoop(flags, "no_upcoming", fmt.Sprintf("no orders with check-in within %d days", days))
			}

			return flags.printJSON(cmd, upcoming)
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Number of days ahead to look for check-ins")
	cmd.Flags().BoolVar(&withMessages, "with-messages", false, "Include latest message thread info for each order")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to SQLite store")

	return cmd
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
