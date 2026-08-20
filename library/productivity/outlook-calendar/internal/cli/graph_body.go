// Hand-built (Phase 5): Microsoft Graph body shapers.
// Microsoft Graph nests structured fields (start/end/location/attendees/body)
// in ways the spec's flat body emit can't model. These helpers wrap CSV input
// or strings into the wire shape Graph expects.

package cli

import (
	"strings"
)

// splitCSV trims and returns non-empty comma-separated tokens.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// expandAttendees turns a comma-separated list of email addresses into the
// Microsoft Graph attendees[] shape: each entry is
// { emailAddress: { address }, type: "required" }.
func expandAttendees(csv string) []map[string]any {
	emails := splitCSV(csv)
	out := make([]map[string]any, 0, len(emails))
	for _, e := range emails {
		out = append(out, map[string]any{
			"emailAddress": map[string]any{"address": e},
			"type":         "required",
		})
	}
	return out
}

// expandToRecipients turns a comma-separated list of emails into the Graph
// "toRecipients" shape used by /events/{id}/forward.
func expandToRecipients(csv string) []map[string]any {
	emails := splitCSV(csv)
	out := make([]map[string]any, 0, len(emails))
	for _, e := range emails {
		out = append(out, map[string]any{
			"emailAddress": map[string]any{"address": e},
		})
	}
	return out
}
