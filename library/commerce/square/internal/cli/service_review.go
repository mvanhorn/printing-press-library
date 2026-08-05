// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type serviceReviewSummary struct {
	StaffID                        string  `json:"staff_id"`
	StaffName                      string  `json:"staff_name,omitempty"`
	LocationID                     string  `json:"location_id"`
	LocationName                   string  `json:"location_name,omitempty"`
	BookingCount                   int     `json:"booking_count"`
	BookedMinutes                  int64   `json:"booked_minutes"`
	CompletedOrElapsedAccepted     int     `json:"completed_or_elapsed_accepted"`
	Cancelled                      int     `json:"cancelled"`
	Missed                         int     `json:"missed"`
	ScheduledOrPending             int     `json:"scheduled_or_pending"`
	CompletedWithoutPaymentOrOrder int     `json:"completed_without_payment_or_order_evidence"`
	ObservedCompletionRate         float64 `json:"observed_completion_rate"`
	ObservedCancellationOrMissRate float64 `json:"observed_cancellation_or_miss_rate"`
}

func newNovelServiceReviewCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:         "review",
		Short:       "Review completed, unpaid, and missed appointments alongside staff and location workload.",
		Example:     "  square-pp-cli service review --since 7d --agent",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "service review")
			}
			cutoff, err := parseSinceDuration(flagSince)
			if err != nil {
				return fmt.Errorf("invalid value %q for --since: %s", flagSince, err)
			}
			resources := []string{"bookings", "team-members", "locations", "payments", "orders"}
			db, err := openNovelLocalStore(cmd, flags, resources)
			if err != nil {
				return err
			}
			defer db.Close()
			records, err := loadLocalSquareRecords(cmd.Context(), db, resources)
			if err != nil {
				return err
			}

			staffNames, locationNames := map[string]string{}, map[string]string{}
			payments, orders := make([]localSquareRecord, 0), make([]localSquareRecord, 0)
			for _, record := range records {
				switch record.ResourceType {
				case "team-members":
					staffNames[record.ID] = teamMemberName(record.Data)
				case "locations":
					locationNames[record.ID] = firstString(record.Data, []string{"name"}, []string{"business_name"})
				case "payments":
					payments = append(payments, record)
				case "orders":
					orders = append(orders, record)
				}
			}

			byKey := map[string]*serviceReviewSummary{}
			bookings := 0
			now := time.Now()
			for _, record := range records {
				if record.ResourceType != "bookings" {
					continue
				}
				appointmentTime := bookingStart(record.Data)
				if appointmentTime.IsZero() {
					appointmentTime = recordTime(record)
				}
				if appointmentTime.Before(cutoff) || appointmentTime.After(now) {
					continue
				}
				bookings++
				locationID := stringValue(record.Data, "location_id")
				status := strings.ToUpper(stringValue(record.Data, "status"))
				end := bookingEnd(record.Data)
				completed := status == "COMPLETED" || (status == "ACCEPTED" && !end.IsZero() && !end.After(now))
				hasPaymentOrOrder := bookingHasPaymentOrOrderEvidence(record, payments, orders)
				workloads := bookingStaffWorkloads(record.Data)
				if len(workloads) == 0 {
					workloads = []bookingStaffWorkload{{}}
				}
				for _, workload := range workloads {
					key := workload.StaffID + "\x00" + locationID
					if byKey[key] == nil {
						byKey[key] = &serviceReviewSummary{StaffID: workload.StaffID, StaffName: staffNames[workload.StaffID], LocationID: locationID, LocationName: locationNames[locationID]}
					}
					s := byKey[key]
					s.BookingCount++
					s.BookedMinutes += workload.Minutes
					switch {
					case completed:
						s.CompletedOrElapsedAccepted++
						if !hasPaymentOrOrder {
							s.CompletedWithoutPaymentOrOrder++
						}
					case strings.Contains(status, "CANCEL") || status == "DECLINED":
						s.Cancelled++
					case status == "NO_SHOW" || status == "MISSED":
						s.Missed++
					default:
						s.ScheduledOrPending++
					}
				}
			}

			out := make([]serviceReviewSummary, 0, len(byKey))
			for _, s := range byKey {
				if s.BookingCount > 0 {
					s.ObservedCompletionRate = float64(s.CompletedOrElapsedAccepted) / float64(s.BookingCount)
					s.ObservedCancellationOrMissRate = float64(s.Cancelled+s.Missed) / float64(s.BookingCount)
				}
				out = append(out, *s)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].LocationID == out[j].LocationID {
					return out[i].StaffID < out[j].StaffID
				}
				return out[i].LocationID < out[j].LocationID
			})
			return flags.printJSON(cmd, map[string]any{
				"data_source": "local", "since": flagSince, "cutoff": cutoff, "booking_count": bookings,
				"by_staff_and_location": out,
				"workload_proxy": map[string]any{
					"basis":              "synced booking counts and scheduled minutes",
					"capacity_available": false,
				},
				"completion_basis":             "COMPLETED status, plus ACCEPTED appointments whose scheduled end is in the past",
				"payment_order_evidence_basis": "direct IDs on the booking, or a synced payment/order for the same customer and location within the appointment day",
				"limitations": []string{
					"Booked minutes and observed rates are workload proxies, not staff utilization; working hours and capacity are not available in the synced records.",
					"A missing payment/order match is a review flag, not proof that the customer did not pay.",
				},
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Review appointments within this period (for example 7d or 24h)")
	return cmd
}

func bookingDurationMinutes(data map[string]any) int64 {
	var total int64
	segments, _ := data["appointment_segments"].([]any)
	for _, raw := range segments {
		segment, _ := raw.(map[string]any)
		total += intValue(segment, "duration_minutes")
	}
	return total
}

func bookingStaffID(data map[string]any) string {
	workloads := bookingStaffWorkloads(data)
	if len(workloads) > 0 {
		return workloads[0].StaffID
	}
	return ""
}

type bookingStaffWorkload struct {
	StaffID string
	Minutes int64
}

// Square bookings may assign different appointment segments to different team
// members. Aggregate repeated segments for one person without charging every
// segment's duration to only the first team member.
func bookingStaffWorkloads(data map[string]any) []bookingStaffWorkload {
	byStaff := map[string]int64{}
	order := make([]string, 0)
	segments, _ := data["appointment_segments"].([]any)
	for _, raw := range segments {
		segment, _ := raw.(map[string]any)
		staffID := stringValue(segment, "team_member_id")
		if _, seen := byStaff[staffID]; !seen {
			order = append(order, staffID)
		}
		byStaff[staffID] += intValue(segment, "duration_minutes")
	}
	if len(order) == 0 {
		if staffID := stringValue(data, "team_member_id"); staffID != "" {
			return []bookingStaffWorkload{{StaffID: staffID, Minutes: bookingDurationMinutes(data)}}
		}
	}
	out := make([]bookingStaffWorkload, 0, len(order))
	for _, staffID := range order {
		out = append(out, bookingStaffWorkload{StaffID: staffID, Minutes: byStaff[staffID]})
	}
	return out
}

func teamMemberName(data map[string]any) string {
	if display := stringValue(data, "display_name"); display != "" {
		return display
	}
	return strings.TrimSpace(firstString(data, []string{"given_name"}, []string{"first_name"}) + " " + firstString(data, []string{"family_name"}, []string{"last_name"}))
}

func bookingEnd(data map[string]any) time.Time {
	start := bookingStart(data)
	if start.IsZero() {
		return time.Time{}
	}
	return start.Add(time.Duration(bookingDurationMinutes(data)) * time.Minute)
}

func bookingStart(data map[string]any) time.Time {
	value := stringValue(data, "start_at")
	if value == "" {
		segments, _ := data["appointment_segments"].([]any)
		for _, raw := range segments {
			segment, _ := raw.(map[string]any)
			if value = stringValue(segment, "start_at"); value != "" {
				break
			}
		}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

func bookingHasPaymentOrOrderEvidence(booking localSquareRecord, payments, orders []localSquareRecord) bool {
	if firstString(booking.Data, []string{"payment_id"}, []string{"order_id"}) != "" {
		return true
	}
	customerID := stringValue(booking.Data, "customer_id")
	locationID := stringValue(booking.Data, "location_id")
	if customerID == "" || locationID == "" {
		return false
	}
	start := bookingStart(booking.Data)
	for _, record := range append(append([]localSquareRecord{}, payments...), orders...) {
		if stringValue(record.Data, "customer_id") != customerID || stringValue(record.Data, "location_id") != locationID {
			continue
		}
		if record.ResourceType == "payments" && !strings.EqualFold(stringValue(record.Data, "status"), "COMPLETED") {
			continue
		}
		if record.ResourceType == "orders" && strings.EqualFold(stringValue(record.Data, "state"), "CANCELED") {
			continue
		}
		if sameServiceDay(start, recordTime(record)) {
			return true
		}
	}
	return false
}

func sameServiceDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	year, month, day := a.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, a.Location())
	end := start.AddDate(0, 0, 1)
	localEvent := b.In(a.Location())
	return !localEvent.Before(start) && localEvent.Before(end)
}
