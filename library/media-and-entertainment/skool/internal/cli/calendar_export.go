// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newCalendarExportCmd: emits an iCalendar (.ics) document for the
// community's upcoming/recent events. With --json, returns the structured
// list instead.
func newCalendarExportCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagFrom string
	var flagTo string
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export community calendar events as iCalendar (.ics) or JSON",
		Example: "  skool-pp-cli calendar export --community bewarethedefault > btd.ics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required"))
			}

			path := "/_next/data/{buildId}/" + community + "/calendar.json"
			params := map[string]string{"g": community}
			raw, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				PageProps struct {
					Events []struct {
						ID        string `json:"id"`
						StartTime string `json:"startTime"`
						EndTime   string `json:"endTime"`
						Metadata  struct {
							Title       string `json:"title"`
							Description string `json:"description"`
							Location    string `json:"location"`
							Timezone    string `json:"timezone"`
							CoverImage  string `json:"coverImage"`
						} `json:"metadata"`
					} `json:"events"`
					Timezone string `json:"timezone"`
				} `json:"pageProps"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parsing calendar response: %w", err)
			}

			fromTS, _ := parseEventTime(flagFrom)
			toTS, _ := parseEventTime(flagTo)

			events := make([]map[string]any, 0, len(env.PageProps.Events))
			for _, e := range env.PageProps.Events {
				start, _ := parseEventTime(e.StartTime)
				if !fromTS.IsZero() && start.Before(fromTS) {
					continue
				}
				if !toTS.IsZero() && start.After(toTS) {
					continue
				}
				events = append(events, map[string]any{
					"id":          e.ID,
					"name":        e.Metadata.Title,
					"starts_at":   e.StartTime,
					"ends_at":     e.EndTime,
					"description": e.Metadata.Description,
					"location":    e.Metadata.Location,
				})
			}

			format := strings.ToLower(flagFormat)
			if format == "" {
				if flags.asJSON {
					format = "json"
				} else {
					format = "ics"
				}
			}
			if format == "json" {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}

			// Emit iCalendar (RFC 5545)
			var b strings.Builder
			b.WriteString("BEGIN:VCALENDAR\r\n")
			b.WriteString("VERSION:2.0\r\n")
			b.WriteString("PRODID:-//skool-pp-cli//Skool Community Export//EN\r\n")
			b.WriteString("CALSCALE:GREGORIAN\r\n")
			b.WriteString("METHOD:PUBLISH\r\n")
			b.WriteString("X-WR-CALNAME:Skool — " + community + "\r\n")
			for _, e := range events {
				b.WriteString("BEGIN:VEVENT\r\n")
				fmt.Fprintf(&b, "UID:%s@skool.com\r\n", e["id"])
				fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(toString(e["name"])))
				if start, err := parseEventTime(toString(e["starts_at"])); err == nil {
					fmt.Fprintf(&b, "DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z"))
				}
				if end, err := parseEventTime(toString(e["ends_at"])); err == nil {
					fmt.Fprintf(&b, "DTEND:%s\r\n", end.UTC().Format("20060102T150405Z"))
				}
				if d := toString(e["description"]); d != "" {
					fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", icsEscape(d))
				}
				b.WriteString("END:VEVENT\r\n")
			}
			b.WriteString("END:VCALENDAR\r\n")
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest event time (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Latest event time (RFC3339 or YYYY-MM-DD)")
	var flagICS bool
	cmd.Flags().BoolVar(&flagICS, "ics", false, "Output as iCalendar (.ics) — same as --format ics")
	cmd.Flags().StringVar(&flagFormat, "format", "", "Output format: ics (default) or json")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error { if flagICS { flagFormat = "ics" }; return nil }
	return cmd
}

func parseEventTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("could not parse %q as date/time", s)
}

func icsEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, ";", `\;`)
	return s
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
