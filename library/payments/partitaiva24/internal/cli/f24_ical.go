// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored F24 iCal export.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newF24IcalCmd(flags *rootFlags) *cobra.Command {
	output := ""
	cmd := &cobra.Command{
		Use:   "ical",
		Short: "Export F24 deadlines as iCal",
		Long:  "Fetch live F24 records and emit RFC 5545 calendar events for due dates. PDFs are not included.",
		Example: `  partitaiva24-pp-cli f24 ical
  partitaiva24-pp-cli f24 ical -o f24-2026.ics`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/user/f24", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ical, events, count := f24ToICal(data)
			// JSON mode emits the parsed event list (with the iCal text inline);
			// agents that pipe through --json get a structured payload, file mode
			// always writes RFC 5545 text regardless of --json.
			if output == "" {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					payload := map[string]any{"count": count, "events": events, "ical": ical}
					return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
				}
				fmt.Fprint(cmd.OutOrStdout(), ical)
				return nil
			}
			path := homeExpanded(output)
			if err := os.WriteFile(path, []byte(ical), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d events to %s\n", count, path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write iCal to path instead of stdout")
	return cmd
}

type f24Event struct {
	UID         string `json:"uid"`
	Summary     string `json:"summary"`
	DueDate     string `json:"due_date"`
	Description string `json:"description,omitempty"`
}

func f24ToICal(data json.RawMessage) (string, []f24Event, int) {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		var wrapped map[string]any
		if json.Unmarshal(data, &wrapped) == nil {
			for _, key := range []string{"data", "items", "results"} {
				if arr, ok := wrapped[key].([]any); ok {
					for _, x := range arr {
						if m, ok := x.(map[string]any); ok {
							items = append(items, m)
						}
					}
				}
			}
		}
	}
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//partitaiva24-pp-cli//F24//EN\r\n")
	var events []f24Event
	for i, item := range items {
		due := firstMapString(item, "due_date", "data_scadenza", "scadenza", "expires_at")
		date, ok := normalizeICalDate(due)
		if !ok {
			continue
		}
		id := firstMapString(item, "id", "uuid", "number")
		if id == "" {
			id = fmt.Sprintf("%d", i+1)
		}
		desc := firstMapString(item, "description", "descrizione", "name", "title")
		if desc == "" {
			desc = id
		}
		uid := fmt.Sprintf("f24-%s@partitaiva24-pp-cli", id)
		summary := "F24: " + desc
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s\r\n", escapeICal(uid))
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", escapeICal(summary))
		fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\n", date)
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z"))
		b.WriteString("END:VEVENT\r\n")
		events = append(events, f24Event{UID: uid, Summary: summary, DueDate: due[:10], Description: desc})
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String(), events, len(events)
}

func firstMapString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch x := v.(type) {
			case string:
				if strings.TrimSpace(x) != "" {
					return strings.TrimSpace(x)
				}
			case float64:
				return fmt.Sprintf("%.0f", x)
			}
		}
	}
	return ""
}

func normalizeICalDate(s string) (string, bool) {
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.Format("20060102"), true
		}
	}
	return "", false
}

func escapeICal(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
