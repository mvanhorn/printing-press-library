// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: viewing your own appointments. GET /api/v2/appointments is
// session-gated and its exact field names vary across Jane versions, so parsing
// is defensive — we extract the start time from any of several likely keys and
// pass the full record through for --json/--select. `upcoming`/`past` split on
// that time; agenda (agenda.go) reuses fetchClinicAppointments to merge tenants.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/janeapp/internal/client"
	"github.com/spf13/cobra"
)

// fetchLocationNames returns a location_id -> name map for the active clinic.
// Jane's appointment payload carries only location_id, so this resolves names
// for display. Best-effort: returns an empty map on any error.
func fetchLocationNames(ctx context.Context, c *client.Client) map[int]string {
	out := map[int]string{}
	data, err := c.Get(ctx, "/api/v2/locations", nil)
	if err != nil {
		return out
	}
	var locs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &locs) == nil {
		for _, l := range locs {
			out[l.ID] = l.Name
		}
	}
	return out
}

// apptRecord is one appointment plus the clinic it came from and its parsed
// start time (zero if none could be extracted).
type apptRecord struct {
	Clinic string         `json:"clinic"`
	Start  time.Time      `json:"-"`
	Raw    map[string]any `json:"appointment"`
	view   map[string]any // flattened, agent-friendly projection
}

var apptStartKeys = []string{"start_at", "starts_at", "start", "start_time", "appointment_start", "scheduled_at", "begins_at"}
var apptEndKeys = []string{"end_at", "ends_at", "end", "end_time", "finish_at", "end_datetime"}

func extractTimeFromKeys(m map[string]any, keys []string) (time.Time, string) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		// Offset-carrying formats are absolute — parse as-is.
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700"} {
			if t, err := time.Parse(layout, s); err == nil {
				return t, s
			}
		}
		// Naive (no offset) formats are wall-clock: interpret in the local zone
		// to stay consistent with parseFlexibleDate, so conflict-check compares
		// a local candidate against like-for-like appointment times rather than
		// a UTC-shifted one and missing a real overlap.
		for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				return t, s
			}
		}
		return time.Time{}, s
	}
	return time.Time{}, ""
}

func firstStringKey(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			// nested {name: ...}
			if nm, ok := v.(map[string]any); ok {
				if s, ok := nm["name"].(string); ok && s != "" {
					return s
				}
				if s, ok := nm["full_name"].(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// nestedString pulls a string field from a nested object (e.g. staff_member.professional_name).
func nestedString(m map[string]any, obj, field string) string {
	if sub, ok := m[obj].(map[string]any); ok {
		if s, ok := sub[field].(string); ok {
			return s
		}
	}
	return ""
}

// deriveApptStatus infers a status from Jane's timestamp fields — the payload
// has no explicit status, only cancelled_at / archived_at (nil when not set).
func deriveApptStatus(m map[string]any) string {
	if v, ok := m["cancelled_at"]; ok && v != nil {
		return "cancelled"
	}
	if v, ok := m["archived_at"]; ok && v != nil {
		return "archived"
	}
	return "booked"
}

// projectAppointment builds a compact, uniform view of an appointment. locNames
// maps location_id -> name (Jane's appointment payload carries only the id).
func projectAppointment(clinic string, m map[string]any, locNames map[int]string) map[string]any {
	start, startRaw := extractTimeFromKeys(m, apptStartKeys)
	_, endRaw := extractTimeFromKeys(m, apptEndKeys)
	practitioner := nestedString(m, "staff_member", "professional_name")
	if practitioner == "" {
		practitioner = firstStringKey(m, "staff_member_name", "practitioner", "full_name")
	}
	location := ""
	if lid, ok := m["location_id"].(float64); ok {
		if n, ok := locNames[int(lid)]; ok {
			location = n
		} else {
			location = fmt.Sprintf("location %d", int(lid))
		}
	}
	view := map[string]any{
		"clinic":       clinic,
		"start_at":     startRaw,
		"end_at":       endRaw,
		"treatment":    firstStringKey(m, "treatment_name", "treatment", "name"),
		"practitioner": practitioner,
		"location":     location,
		"status":       deriveApptStatus(m),
	}
	if id, ok := m["id"]; ok {
		view["id"] = id
	}
	if !start.IsZero() {
		view["date"] = start.Format("2006-01-02")
	}
	return view
}

// fetchClinicAppointments loads a clinic's appointments via the session-gated
// endpoint and returns parsed records sorted by start time ascending.
func fetchClinicAppointments(ctx context.Context, flags *rootFlags, clinic *Clinic) ([]apptRecord, error) {
	c, err := flags.newClientForClinic(clinic)
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/api/v2/appointments", nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	// The endpoint may return a bare array or an object wrapping one.
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		var wrapper map[string]json.RawMessage
		matched := false
		if err2 := json.Unmarshal(data, &wrapper); err2 == nil {
			for _, key := range []string{"appointments", "data", "results"} {
				if raw, ok := wrapper[key]; ok {
					// A recognized key whose value isn't an appointment array
					// (e.g. {"appointments":{"items":[...]}}) must not be
					// silently swallowed into an empty list — that would hide
					// bookings from conflict-check just like an unknown wrapper.
					if err := json.Unmarshal(raw, &items); err != nil {
						return nil, fmt.Errorf("appointments %q field for clinic %q is not a recognized array shape; refusing to proceed so safety checks don't run on incomplete data: %w", key, clinic.Name, err)
					}
					matched = true
					break
				}
			}
			// A non-empty object whose shape we don't recognize must be a hard
			// error, not an empty list: silently returning zero appointments
			// would let conflict-check report a clinic as clear and miss a real
			// booking hidden in an unsupported response shape.
			if !matched && len(wrapper) > 0 {
				keys := make([]string, 0, len(wrapper))
				for k := range wrapper {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return nil, fmt.Errorf("unrecognized appointments response shape for clinic %q (top-level keys: %v); refusing to proceed so safety checks don't run on incomplete data", clinic.Name, keys)
			}
		}
	}
	locNames := fetchLocationNames(ctx, c)
	out := make([]apptRecord, 0, len(items))
	for _, m := range items {
		start, _ := extractTimeFromKeys(m, apptStartKeys)
		out = append(out, apptRecord{
			Clinic: clinic.Name,
			Start:  start,
			Raw:    m,
			view:   projectAppointment(clinic.Name, m, locNames),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// clinicsForRead resolves the clinics a read command should query: all
// logged-in clinics when --all-clinics is set, else just the active one.
func clinicsForRead(flags *rootFlags, allClinics bool) ([]Clinic, error) {
	if allClinics {
		cs, err := loggedInClinics()
		if err != nil {
			return nil, err
		}
		if len(cs) == 0 {
			return nil, usageErr(fmt.Errorf("no logged-in clinics; run 'janeapp-pp-cli auth login --clinic <name>' first"))
		}
		return cs, nil
	}
	c, err := requireActiveClinic(flags)
	if err != nil {
		return nil, err
	}
	return []Clinic{*c}, nil
}

func renderAppointments(cmd *cobra.Command, flags *rootFlags, recs []apptRecord) error {
	views := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		views = append(views, r.view)
	}
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		b, err := json.Marshal(views)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
	}
	if len(views) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No appointments.")
		return nil
	}
	headers := []string{"CLINIC", "DATE", "START", "PRACTITIONER", "TREATMENT", "STATUS"}
	rows := make([][]string, 0, len(views))
	for _, v := range views {
		rows = append(rows, []string{
			str(v["clinic"]), str(v["date"]), str(v["start_at"]),
			str(v["practitioner"]), str(v["treatment"]), str(v["status"]),
		})
	}
	return flags.printTable(cmd, headers, rows)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// gatherAppointments queries the requested clinics and returns all records.
// Per-clinic failures (e.g. an expired session) become stderr warnings so one
// bad clinic doesn't sink an --all-clinics view.
func gatherAppointments(cmd *cobra.Command, flags *rootFlags, clinics []Clinic) ([]apptRecord, error) {
	var all []apptRecord
	var firstErr error
	for i := range clinics {
		recs, err := fetchClinicAppointments(cmd.Context(), flags, &clinics[i])
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", clinics[i].Name, err)
			continue
		}
		all = append(all, recs...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

func newAppointmentsViewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "appointments",
		Short:       "View your Jane appointments (upcoming, past)",
		Long:        "View your own appointments for the active clinic (or --all-clinics). Requires a logged-in session (see 'auth login').",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare `appointments` defaults to upcoming.
			return runAppointments(cmd, flags, false, false)
		},
	}
	var upAll, pastAll bool
	upcoming := &cobra.Command{
		Use:         "upcoming",
		Short:       "List your upcoming appointments",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  janeapp-pp-cli appointments upcoming --clinic embophysio\n  janeapp-pp-cli appointments upcoming --all-clinics --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppointmentsFiltered(cmd, flags, upAll, "upcoming")
		},
	}
	upcoming.Flags().BoolVar(&upAll, "all-clinics", false, "Fan out across every logged-in clinic")
	past := &cobra.Command{
		Use:         "past",
		Short:       "List your past appointments",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  janeapp-pp-cli appointments past --clinic embophysio",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAppointmentsFiltered(cmd, flags, pastAll, "past")
		},
	}
	past.Flags().BoolVar(&pastAll, "all-clinics", false, "Fan out across every logged-in clinic")
	cmd.AddCommand(upcoming)
	cmd.AddCommand(past)
	return cmd
}

// runAppointments lists everything (used by the bare command).
func runAppointments(cmd *cobra.Command, flags *rootFlags, allClinics, _ bool) error {
	return runAppointmentsFiltered(cmd, flags, allClinics, "all")
}

func runAppointmentsFiltered(cmd *cobra.Command, flags *rootFlags, allClinics bool, window string) error {
	if flags.dryRun {
		return nil
	}
	clinics, err := clinicsForRead(flags, allClinics)
	if err != nil {
		return err
	}
	recs, err := gatherAppointments(cmd, flags, clinics)
	if err != nil {
		return err
	}
	now := time.Now()
	filtered := make([]apptRecord, 0, len(recs))
	for _, r := range recs {
		cancelled := str(r.view["status"]) == "cancelled"
		switch window {
		case "upcoming":
			// Upcoming = future AND not cancelled. A cancelled appointment is
			// still "in the future" by time but must not read as a live booking.
			if (r.Start.IsZero() || !r.Start.Before(now)) && !cancelled {
				filtered = append(filtered, r)
			}
		case "past":
			if !r.Start.IsZero() && r.Start.Before(now) {
				filtered = append(filtered, r)
			}
		default:
			filtered = append(filtered, r)
		}
	}
	if window == "past" {
		// Most-recent first for history.
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Start.After(filtered[j].Start) })
	}
	return renderAppointments(cmd, flags, filtered)
}
