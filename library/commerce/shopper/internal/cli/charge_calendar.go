// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/cliutil"
)

// chargeCalendarEntry is one row in the charge-calendar output: a delivery
// cycle with its derived charge and edit-lock dates.
type chargeCalendarEntry struct {
	DeliveryDate string `json:"delivery_date"`
	ChargeDate   string `json:"charge_date"`
	EditLockDate string `json:"edit_lock_date"`
	LocksInDays  int    `json:"locks_in_days"`
	Status       string `json:"status"`
	IsFresh      bool   `json:"is_fresh,omitempty"`
}

// chargeCalendarView is the full command output.
type chargeCalendarView struct {
	NextDelivery     *chargeCalendarEntry `json:"next_delivery"`
	RescheduleWindow *rescheduleWindow    `json:"reschedule_window,omitempty"`
	Message          string               `json:"message,omitempty"`
	Note             string               `json:"note,omitempty"`
}

type rescheduleWindow struct {
	Earliest string `json:"earliest"`
	Latest   string `json:"latest"`
}

func newNovelChargeCalendarCmd(flags *rootFlags) *cobra.Command {
	var flagWeeks string
	var flagLockingSoon bool

	cmd := &cobra.Command{
		Use:     "charge-calendar",
		Short:   "Your next delivery's charge date, edit-lock deadline, and reschedule window in one view",
		Example: "  shopper-pp-cli charge-calendar --weeks 8 --json",
		Long: `Combines /delivery/summary (your scheduled delivery) and /delivery/v2/calendar
(the allowed reschedule window) into one timeline and computes:
  - charge_date    = delivery_date - 7 days (Shopper charges ~7d ahead)
  - edit_lock_date = delivery_date - 5 days (3 days for Fresh/perishable plans)
  - locks_in_days  = days until the edit window closes
  - status         = "editable" | "locking-soon" | "locks-today" | "locked"

Use --locking-soon to only show the delivery when its edit window closes within 3 days.
--weeks N (or Nw) suppresses the delivery if it falls beyond the horizon.`,
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				// fall through: charge-calendar is a no-required-input read command
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"fetch /delivery/summary + /delivery/v2/calendar and build charge timeline"}`)
				return nil
			}

			var horizon time.Duration
			if flagWeeks != "" {
				clean := strings.TrimSpace(flagWeeks)
				if !strings.HasSuffix(clean, "w") && !strings.HasSuffix(clean, "d") {
					clean += "w"
				}
				d, err := cliutil.ParseDurationLoose(clean)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --weeks value %q: %w", flagWeeks, err))
				}
				horizon = d
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			summary, err := c.Get(cmd.Context(), "/delivery/summary", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			calendar, err := c.Get(cmd.Context(), "/delivery/v2/calendar", nil)
			if err != nil {
				// Calendar is enrichment; proceed with summary-only if it fails.
				calendar = nil
			}

			view := buildChargeCalendar(summary, calendar, horizon, flagLockingSoon)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWeeks, "weeks", "", "Suppress the delivery if it falls beyond this horizon (e.g. 8 or 8w)")
	cmd.Flags().BoolVar(&flagLockingSoon, "locking-soon", false, "Only show the delivery when its edit window closes within 3 days")
	return cmd
}

// resultsEnvelope unwraps the {"results": {...}} envelope the Shopper API uses.
func resultsEnvelope(data json.RawMessage) map[string]json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return nil
	}
	if r, ok := top["results"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(r, &inner) == nil {
			return inner
		}
	}
	return top
}

func buildChargeCalendar(summary, calendar json.RawMessage, horizon time.Duration, onlyLockingSoon bool) chargeCalendarView {
	now := time.Now().Truncate(24 * time.Hour)
	view := chargeCalendarView{}

	res := resultsEnvelope(summary)

	// deliveryDate: ISO timestamp like "2026-06-23T03:00:00.000Z"
	var deliveryDateStr string
	if raw, ok := res["deliveryDate"]; ok {
		_ = json.Unmarshal(raw, &deliveryDateStr)
	}
	// message.text (HTML) — strip tags for a clean note
	if raw, ok := res["message"]; ok {
		var msg map[string]any
		if json.Unmarshal(raw, &msg) == nil {
			if t, ok := msg["text"].(string); ok {
				view.Message = stripHTMLTags(t)
			}
		}
	}

	// Detect Fresh/perishable plan from specific plan-type fields only — never
	// the whole JSON blob, since a product named "Leite Fresco" or "Suco Fresh"
	// in the basket would otherwise misclassify the plan and surface the wrong
	// 3-day edit-lock offset.
	isFresh := false
	for _, k := range []string{"planType", "plan_type", "deliveryType", "delivery_type", "plan", "modality", "type"} {
		raw, ok := res[k]
		if !ok {
			continue
		}
		var v string
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		lv := strings.ToLower(v)
		if strings.Contains(lv, "fresh") || strings.Contains(lv, "pereci") {
			isFresh = true
			break
		}
	}

	if deliveryDateStr != "" {
		if delivDate, err := parseShopperDate(deliveryDateStr); err == nil {
			lockOffset := -5 * 24 * time.Hour
			if isFresh {
				lockOffset = -3 * 24 * time.Hour
			}
			chargeDate := delivDate.Add(-7 * 24 * time.Hour)
			editLock := delivDate.Add(lockOffset)
			locksInDays := int(editLock.Sub(now).Hours() / 24)

			var status string
			switch {
			case locksInDays < 0:
				status = "locked"
			case locksInDays == 0:
				status = "locks-today"
			case locksInDays <= 3:
				status = "locking-soon"
			default:
				status = "editable"
			}

			withinHorizon := horizon == 0 || !delivDate.After(now.Add(horizon))
			passesLockFilter := !onlyLockingSoon || status == "locking-soon" || status == "locks-today"

			if withinHorizon && passesLockFilter {
				view.NextDelivery = &chargeCalendarEntry{
					DeliveryDate: delivDate.Format("2006-01-02"),
					ChargeDate:   chargeDate.Format("2006-01-02"),
					EditLockDate: editLock.Format("2006-01-02"),
					LocksInDays:  locksInDays,
					Status:       status,
					IsFresh:      isFresh,
				}
			}
		}
	}

	// Reschedule window from the calendar's allowed range.
	if cal := resultsEnvelope(calendar); cal != nil {
		if rawCal, ok := cal["calendar"]; ok {
			var calObj map[string]json.RawMessage
			if json.Unmarshal(rawCal, &calObj) == nil {
				if rawAllowed, ok := calObj["allowed"]; ok {
					var allowed struct {
						Min string `json:"min"`
						Max string `json:"max"`
					}
					if json.Unmarshal(rawAllowed, &allowed) == nil && (allowed.Min != "" || allowed.Max != "") {
						view.RescheduleWindow = &rescheduleWindow{Earliest: allowed.Min, Latest: allowed.Max}
					}
				}
			}
		}
	}

	if view.NextDelivery == nil {
		view.Note = "No scheduled delivery found within the requested window (or the edit window does not match --locking-soon). The reschedule window, if shown, lists the dates you can still pick."
	}

	return view
}

// parseShopperDate accepts the API's ISO timestamp or a bare YYYY-MM-DD.
func parseShopperDate(s string) (time.Time, error) {
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date %q", s)
}

// stripHTMLTags removes simple HTML tags from an API message string.
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
