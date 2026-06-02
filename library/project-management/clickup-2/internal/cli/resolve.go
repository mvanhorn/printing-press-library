// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built transcendence command for clickup-2-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup-2/internal/store"
	"github.com/spf13/cobra"
)

func newNovelResolveCmd(flags *rootFlags) *cobra.Command {
	var flagStatus string
	var flagAssignee string
	var flagDue string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Turn fuzzy input (status name, assignee me/name, natural-language due date) into hard ClickUp IDs, offline",
		Long: `Resolves human-friendly input into ClickUp identifiers using only the
local store and a built-in date parser, so an agent can resolve many tasks
without spending an API call per lookup.

  --status <name>     matched (case-insensitive, then substring) against
                      synced list statuses; returns the status id when known
  --assignee <who>    "me" (when a user is synced), a username, or a numeric
                      id; returns the member id
  --due <phrase>      natural language: today, tomorrow, yesterday, a weekday
                      (mon..sun / "next friday"), or Nd/Nw offsets, with an
                      optional time (e.g. "friday 5pm"); returns a ms epoch

Pass any combination. Output is JSON when --json/--agent or piped.`,
		Example: `  # Resolve a triage instruction for an agent
  clickup-2-pp-cli resolve --status review --assignee me --due "friday 5pm" --agent

  # Just a date
  clickup-2-pp-cli resolve --due tomorrow --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagStatus == "" && flagAssignee == "" && flagDue == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("clickup-2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'clickup-2-pp-cli sync' first.", err)
			}
			defer db.Close()

			result := map[string]any{}

			if flagStatus != "" {
				id, name, ok := resolveStatus(db, flagStatus)
				result["status"] = map[string]any{"input": flagStatus, "resolved": ok, "id": id, "status": name}
			}
			if flagAssignee != "" {
				id, username, ok := resolveAssignee(db, flagAssignee)
				result["assignee"] = map[string]any{"input": flagAssignee, "resolved": ok, "id": id, "username": username}
			}
			if flagDue != "" {
				ms, ok := resolveDue(flagDue, time.Now())
				result["due"] = map[string]any{"input": flagDue, "resolved": ok, "due_date_ms": ms, "iso": msToString(ms)}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			for _, k := range []string{"status", "assignee", "due"} {
				if v, ok := result[k]; ok {
					b, _ := json.Marshal(v)
					fmt.Fprintf(w, "%-10s %s\n", k+":", string(b))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagStatus, "status", "", "Status name to resolve to an id")
	cmd.Flags().StringVar(&flagAssignee, "assignee", "", "Assignee (me, username, or id) to resolve")
	cmd.Flags().StringVar(&flagDue, "due", "", "Natural-language due date to resolve to a ms epoch")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// resolveStatus matches a status name against synced list statuses (which
// carry ids), falling back to task status names (no id). Exact case-insensitive
// match wins; otherwise the first substring match.
func resolveStatus(db *store.Store, input string) (id, name string, ok bool) {
	type st struct{ id, name string }
	var statuses []st
	seen := map[string]bool{}
	addFromStatusesArray := func(obj map[string]any) {
		arr, ok := obj["statuses"].([]any)
		if !ok {
			return
		}
		for _, s := range arr {
			m, ok := s.(map[string]any)
			if !ok {
				continue
			}
			nm, _ := m["status"].(string)
			if nm == "" {
				continue
			}
			sid, _ := m["id"].(string)
			key := strings.ToLower(nm)
			if seen[key] {
				continue
			}
			seen[key] = true
			statuses = append(statuses, st{id: sid, name: nm})
		}
	}
	for _, rt := range []string{"space_list", "folder_list", "list"} {
		rows, err := db.List(rt, pmListLimit)
		if err != nil {
			continue
		}
		for _, r := range rows {
			var obj map[string]any
			if json.Unmarshal(r, &obj) == nil {
				addFromStatusesArray(obj)
			}
		}
	}
	// Fallback: distinct task status names.
	if tasks, err := loadPMTasks(db); err == nil {
		for _, t := range tasks {
			if t.Status == "" {
				continue
			}
			key := strings.ToLower(t.Status)
			if seen[key] {
				continue
			}
			seen[key] = true
			statuses = append(statuses, st{name: t.Status})
		}
	}

	in := strings.ToLower(strings.TrimSpace(input))
	for _, s := range statuses {
		if strings.ToLower(s.name) == in {
			return s.id, s.name, true
		}
	}
	for _, s := range statuses {
		if strings.Contains(strings.ToLower(s.name), in) {
			return s.id, s.name, true
		}
	}
	return "", "", false
}

// resolveAssignee maps "me"/username/id to a member id using synced tasks'
// assignees plus team member records.
func resolveAssignee(db *store.Store, input string) (id int64, username string, ok bool) {
	input = strings.TrimSpace(input)
	if strings.EqualFold(input, "me") {
		if me := resolveMeID(db); me != 0 {
			return me, "me", true
		}
		return 0, "", false
	}
	// Numeric id passes through.
	if n, err := strconv.ParseInt(input, 10, 64); err == nil {
		return n, "", true
	}
	// Build username -> id from task assignees and team members.
	byName := map[string]pmAssignee{}
	if tasks, err := loadPMTasks(db); err == nil {
		for _, t := range tasks {
			for _, a := range t.Assignees {
				if a.Username != "" {
					byName[strings.ToLower(a.Username)] = a
				}
			}
		}
	}
	for _, rt := range []string{"team", "team_member", "member"} {
		rows, err := db.List(rt, pmListLimit)
		if err != nil {
			continue
		}
		for _, r := range rows {
			var obj map[string]any
			if json.Unmarshal(r, &obj) != nil {
				continue
			}
			if members, ok := obj["members"].([]any); ok {
				for _, m := range members {
					mm, ok := m.(map[string]any)
					if !ok {
						continue
					}
					u, _ := mm["user"].(map[string]any)
					if u == nil {
						u = mm
					}
					un, _ := u["username"].(string)
					if un == "" {
						continue
					}
					a := pmAssignee{Username: un}
					switch uid := u["id"].(type) {
					case float64:
						a.ID = int64(uid)
					case string:
						a.ID, _ = strconv.ParseInt(uid, 10, 64)
					}
					byName[strings.ToLower(un)] = a
				}
			}
		}
	}
	if a, found := byName[strings.ToLower(input)]; found {
		return a.ID, a.Username, true
	}
	return 0, "", false
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// resolveDue parses a small natural-language date grammar into a ms epoch
// relative to now. Returns (0, false) when it cannot parse.
func resolveDue(input string, now time.Time) (int64, bool) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return 0, false
	}
	fields := strings.Fields(s)

	// Pull an optional trailing time token (e.g. "5pm", "17:00", "9am").
	hour, min := 23, 59
	haveTime := false
	if len(fields) > 0 {
		if h, m, ok := parseClock(fields[len(fields)-1]); ok {
			hour, min, haveTime = h, m, true
			fields = fields[:len(fields)-1]
		}
	}
	_ = haveTime

	day := now
	switch {
	case len(fields) == 0:
		return 0, false
	case fields[0] == "today":
		// day = now
	case fields[0] == "tomorrow":
		day = now.AddDate(0, 0, 1)
	case fields[0] == "yesterday":
		day = now.AddDate(0, 0, -1)
	case fields[0] == "next" && len(fields) >= 2 && isWeekday(fields[1]):
		// "next friday" = the upcoming friday plus a week.
		day = nextWeekday(now, weekdays[fields[1]], false).AddDate(0, 0, 7)
	case isWeekday(fields[0]):
		day = nextWeekday(now, weekdays[fields[0]], false)
	default:
		// Nd / Nw offset forms.
		if d, err := parseDurationWindow(fields[0]); err == nil {
			day = now.Add(d)
		} else {
			// RFC3339 / YYYY-MM-DD passthrough.
			if t, err := time.Parse("2006-01-02", fields[0]); err == nil {
				day = t
			} else if t, err := time.Parse(time.RFC3339, input); err == nil {
				return t.UnixMilli(), true
			} else {
				return 0, false
			}
		}
	}
	due := time.Date(day.Year(), day.Month(), day.Day(), hour, min, 0, 0, now.Location())
	return due.UnixMilli(), true
}

func isWeekday(s string) bool { _, ok := weekdays[s]; return ok }

// nextWeekday returns the next occurrence of wd. If forceNext is false and
// today is wd, today is returned; otherwise the upcoming wd (1..7 days ahead).
func nextWeekday(now time.Time, wd time.Weekday, forceNext bool) time.Time {
	delta := (int(wd) - int(now.Weekday()) + 7) % 7
	if delta == 0 && forceNext {
		delta = 7
	}
	return now.AddDate(0, 0, delta)
}

// parseClock parses "5pm", "5:30pm", "17:00", "9am" into hour/min (24h).
func parseClock(s string) (int, int, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	ampm := ""
	if strings.HasSuffix(s, "am") {
		ampm, s = "am", strings.TrimSuffix(s, "am")
	} else if strings.HasSuffix(s, "pm") {
		ampm, s = "pm", strings.TrimSuffix(s, "pm")
	}
	if s == "" {
		return 0, 0, false
	}
	hh, mm := 0, 0
	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 2)
		var err1, err2 error
		hh, err1 = strconv.Atoi(parts[0])
		mm, err2 = strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
	} else {
		var err error
		hh, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, false
		}
	}
	if ampm == "pm" && hh < 12 {
		hh += 12
	}
	if ampm == "am" && hh == 12 {
		hh = 0
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, false
	}
	// Bare numbers with no am/pm and no colon are ambiguous as a clock;
	// only treat as time when am/pm or a colon was present.
	if ampm == "" && !strings.Contains(strings.ToLower(s), ":") {
		return 0, 0, false
	}
	return hh, mm, true
}
