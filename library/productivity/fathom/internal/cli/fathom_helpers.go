package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
)

// parseFlexTime parses an ISO 8601 timestamp string, trying common layouts.
func parseFlexTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.999999999Z",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// parseSince parses a duration string like "7d", "30d", "2w", "3m" into a time.Time cutoff.
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	s = strings.TrimSpace(s)
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: expected format like 7d, 4w, 3m", s)
	}
	switch unit {
	case 'd':
		return time.Now().AddDate(0, 0, -n), nil
	case 'w':
		return time.Now().AddDate(0, 0, -n*7), nil
	case 'm':
		return time.Now().AddDate(0, -n, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid --since unit %q: use d (days), w (weeks), or m (months)", string(unit))
	}
}

// loadAllMeetings loads all meetings from the local store.
func loadAllMeetings(ctx context.Context, db *store.Store) ([]fathomMeeting, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'meetings' ORDER BY synced_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying meetings: %w", err)
	}
	defer rows.Close()

	var meetings []fathomMeeting
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var m fathomMeeting
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		meetings = append(meetings, m)
	}
	return meetings, rows.Err()
}

// isoWeek returns an "YYYY-WNN" string for a time.
func isoWeek(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// calMonth returns "YYYY-MM" for a time.
func calMonth(t time.Time) string {
	return t.Format("2006-01")
}

// containsIgnoreCase returns true if s contains sub (case-insensitive).
func containsIgnoreCase(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// extractSummaryTopics pulls topic headings and bullet titles from a Fathom
// AI summary instead of doing raw word frequency (which produces stop words).
// Fathom summaries have sections like "## Topics", "## Most Relevant Topics",
// "## Key Takeaways" with sub-headings or bold bullets that are the actual names.
func extractSummaryTopics(md string) []string {
	var topics []string
	inTopicSection := false

	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "##") {
			heading := strings.ToLower(strings.TrimLeft(trimmed, "# "))
			inTopicSection = strings.Contains(heading, "topic") ||
				strings.Contains(heading, "takeaway") ||
				strings.Contains(heading, "relevant") ||
				strings.Contains(heading, "discussed")
			continue
		}
		if !inTopicSection {
			continue
		}
		// Sub-headings are topic names
		if strings.HasPrefix(trimmed, "###") {
			topic := stripMarkdownLink(strings.TrimSpace(strings.TrimLeft(trimmed, "# ")))
			if topic != "" {
				topics = append(topics, topic)
			}
			continue
		}
		// Bold bullet items: "- **Topic Name**" or "- [**Topic**](...)"
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
			content := stripMarkdownLink(strings.TrimLeft(trimmed, "-* "))
			if strings.Contains(content, "**") {
				start := strings.Index(content, "**")
				end := strings.Index(content[start+2:], "**")
				if end > 0 {
					topic := strings.TrimRight(content[start+2:start+2+end], ": ")
					if len(topic) > 3 {
						topics = append(topics, topic)
					}
				}
			}
		}
	}
	return topics
}

// stripMarkdownLink converts [text](url) to text.
func stripMarkdownLink(s string) string {
	for strings.Contains(s, "](") {
		start := strings.LastIndex(s, "[")
		if start == -1 {
			break
		}
		mid := strings.Index(s[start:], "](")
		if mid == -1 {
			break
		}
		end := strings.Index(s[start+mid+2:], ")")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+1:start+mid] + s[start+mid+2+end+1:]
	}
	return strings.TrimSpace(s)
}
