// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
	"github.com/spf13/cobra"
)

func newBookRegularCmd(flags *rootFlags) *cobra.Command {
	var slot string
	var auto bool
	var paymentOption string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "book-regular",
		Short: "Resolve a natural slot key against your regulars and book the next matching session",
		Long: `book-regular is a compound of regulars (your local affinity profile) + schedule
search (the local catalog) + reservation create. Slot keys use the natural
"<day>-<HH>am|pm-<type>" shorthand, e.g. "tue-7am-vinyasa".

The command:
  1. Parses the slot key into (weekday, hour, type substring)
  2. Searches the local class_sessions table for the next matching session
  3. Without --auto, prints the candidate and exits (review before booking)
  4. With --auto, calls POST /me/reservations on the candidate

Note: in v0.1 book-regular uses the single tenant the CLI is configured for.`,
		Example: `  # See what would be booked
  marianatek-pp-cli book-regular --slot "tue-7am-vinyasa"

  # Actually book it
  marianatek-pp-cli book-regular --slot "tue-7am-vinyasa" --auto`,
		Annotations: map[string]string{
			"pp:novel": "book-regular",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if slot == "" {
				return cmd.Help()
			}
			wd, hour, typeSub, err := parseSlotKey(slot)
			if err != nil {
				return usageErr(err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("marianatek-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.List("classes", 5000)
			if err != nil {
				return fmt.Errorf("listing classes: %w", err)
			}
			candidate := pickRegularCandidate(rows, wd, hour, typeSub)
			if candidate == nil {
				return fmt.Errorf("no matching class found for slot %q in local cache (try marianatek-pp-cli sync)", slot)
			}

			if !auto || dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), candidate, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH(greptile #487): share the JSONAPI reservation body shape
			// with watch and me reservations-create.
			body := newReservationCreateBody(candidate.ID, paymentOption, "")
			respBody, status, err := c.Post("/me/reservations", body)
			result := map[string]any{
				"class":  candidate,
				"status": status,
			}
			if err != nil {
				result["error"] = err.Error()
			} else {
				result["reservation"] = json.RawMessage(respBody)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&slot, "slot", "", `slot key e.g. "tue-7am-vinyasa"`)
	cmd.Flags().BoolVar(&auto, "auto", false, "actually book the candidate (default: dry-run preview only)")
	cmd.Flags().StringVar(&paymentOption, "payment-option", "", "payment option id (default: server picks)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/marianatek-pp-cli/data.db)")
	return cmd
}

type regularCandidate struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Instructor string    `json:"instructor,omitempty"`
	Location   string    `json:"location,omitempty"`
	Start      time.Time `json:"start"`
	SpotsLeft  int       `json:"spots_left,omitempty"`
}

func parseSlotKey(s string) (time.Weekday, int, string, error) {
	parts := strings.Split(strings.ToLower(s), "-")
	if len(parts) < 3 {
		return 0, 0, "", fmt.Errorf("slot key must look like 'day-HHam|pm-type' (got %q)", s)
	}
	wd, ok := parseWeekday(parts[0])
	if !ok {
		return 0, 0, "", fmt.Errorf("unknown weekday %q", parts[0])
	}
	hour, ok := parseHourAMPM(parts[1])
	if !ok {
		return 0, 0, "", fmt.Errorf("unparseable hour %q (try '7am' or '17')", parts[1])
	}
	typeSub := strings.Join(parts[2:], "-")
	return wd, hour, typeSub, nil
}

func parseWeekday(s string) (time.Weekday, bool) {
	switch strings.TrimSpace(s) {
	case "sun", "sunday":
		return time.Sunday, true
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "weds", "wednesday":
		return time.Wednesday, true
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	}
	return 0, false
}

func parseHourAMPM(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 23 {
		return n, true
	}
	suffix := ""
	num := s
	if strings.HasSuffix(s, "am") {
		suffix = "am"
		num = strings.TrimSuffix(s, "am")
	} else if strings.HasSuffix(s, "pm") {
		suffix = "pm"
		num = strings.TrimSuffix(s, "pm")
	} else {
		return 0, false
	}
	h, err := strconv.Atoi(num)
	if err != nil || h < 1 || h > 12 {
		return 0, false
	}
	if suffix == "am" {
		if h == 12 {
			return 0, true
		}
		return h, true
	}
	if h == 12 {
		return 12, true
	}
	return h + 12, true
}

func pickRegularCandidate(rows []json.RawMessage, wd time.Weekday, hour int, typeSub string) *regularCandidate {
	now := time.Now().UTC()
	var best *regularCandidate
	for _, raw := range rows {
		var rec map[string]any
		if err := json.Unmarshal(raw, &rec); err != nil {
			continue
		}
		attrs := pickAttrs(rec)
		if attrs == nil {
			continue
		}
		start := parseStart(attrs)
		if start.IsZero() || !start.After(now) {
			continue
		}
		if start.Weekday() != wd || start.Hour() != hour {
			continue
		}
		if typeSub != "" && !attrContains(attrs, []string{"class_type_name", "class_type", "name"}, typeSub) {
			continue
		}
		var id string
		if data, ok := rec["data"].(map[string]any); ok {
			id, _ = data["id"].(string)
		}
		if id == "" {
			id, _ = rec["id"].(string)
		}
		cand := &regularCandidate{
			ID:         id,
			Name:       stringAttr(attrs, "name", "class_type_name"),
			Instructor: stringAttr(attrs, "instructor_name", "instructor"),
			Location:   stringAttr(attrs, "location_name", "location"),
			Start:      start,
			SpotsLeft:  intAttr(attrs, "remaining_spots", "spots_remaining", "available_spots"),
		}
		if best == nil || cand.Start.Before(best.Start) {
			best = cand
		}
	}
	return best
}
