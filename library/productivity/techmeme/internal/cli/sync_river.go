package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
)

type riverHeadline struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Author    string `json:"author"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
	Date      string `json:"date"`
	Time      string `json:"time"`
}

var (
	dateHeaderRE   = regexp.MustCompile(`<H2>([^<]+)</H2>`)
	ritemRE        = regexp.MustCompile(`<tr class="ritem"><td>([^<]*?)&nbsp;`)
	citeRE         = regexp.MustCompile(`<cite>([^<]*?)(?:<A[^>]*>([^<]*)</A>)?[^<]*</cite>`)
	headlineLinkRE = regexp.MustCompile(`</cite>&nbsp;\s*<a href="([^"]+)"[^>]*>([^<]+)</a>`)
	pmlRE          = regexp.MustCompile(`pml="([^"]+)"`)
)

func fetchRiverHTML() ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://www.techmeme.com/river")
	if err != nil {
		return nil, fmt.Errorf("fetching river: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func syncRiverData(db *store.Store, data []byte) (int, error) {
	html := string(data)
	lines := strings.Split(html, "\n")

	var headlines []riverHeadline
	currentDate := ""
	currentYear := time.Now().Year()

	for _, line := range lines {
		if m := dateHeaderRE.FindStringSubmatch(line); m != nil {
			currentDate = m[1]
			continue
		}

		if !strings.Contains(line, `class="ritem"`) {
			continue
		}

		var h riverHeadline

		if m := ritemRE.FindStringSubmatch(line); m != nil {
			h.Time = strings.TrimSpace(m[1])
		}

		if m := citeRE.FindStringSubmatch(line); m != nil {
			authorPart := strings.TrimSpace(m[1])
			authorPart = strings.TrimSuffix(authorPart, "/")
			authorPart = strings.TrimSpace(authorPart)
			h.Author = authorPart
			if m[2] != "" {
				h.Source = strings.TrimSuffix(strings.TrimSpace(m[2]), ":")
			}
		}

		if m := headlineLinkRE.FindStringSubmatch(line); m != nil {
			h.Link = m[1]
			h.Title = strings.ReplaceAll(m[2], "&amp;", "&")
			h.Title = strings.ReplaceAll(h.Title, "&nbsp;", " ")
			h.Title = strings.ReplaceAll(h.Title, "&#39;", "'")
			h.Title = strings.ReplaceAll(h.Title, "&quot;", "\"")
		}

		if m := pmlRE.FindStringSubmatch(line); m != nil {
			h.ID = m[1]
		}

		if h.Title == "" || h.Link == "" {
			continue
		}

		h.Date = currentDate
		if currentDate != "" && h.Time != "" {
			ts := parseRiverTimestamp(currentDate, h.Time, currentYear)
			if !ts.IsZero() {
				h.Timestamp = ts.UTC().Format(time.RFC3339)
			}
		}

		if h.ID == "" {
			sum := sha256.Sum256([]byte(h.Link))
			h.ID = "h-" + hex.EncodeToString(sum[:8])
		}

		headlines = append(headlines, h)
	}

	count := 0
	for _, h := range headlines {
		data, err := json.Marshal(h)
		if err != nil {
			continue
		}
		if err := db.Upsert("headline", h.ID, data); err != nil {
			continue
		}
		count++
	}

	if count > 0 {
		_ = db.SaveSyncState("headline", "", count)
	}

	return count, nil
}

func parseRiverTimestamp(dateStr, timeStr string, fallbackYear int) time.Time {
	dateStr = strings.TrimSpace(dateStr)
	timeStr = strings.TrimSpace(timeStr)

	full := dateStr + " " + timeStr
	for _, layout := range []string{
		"January 2, 2006 3:04 PM",
		"Jan 2, 2006 3:04 PM",
		"January 2, 2006 3:04PM",
	} {
		if t, err := time.ParseInLocation(layout, full, time.Local); err == nil {
			return t
		}
	}

	withYear := fmt.Sprintf("%s, %d %s", dateStr, fallbackYear, timeStr)
	for _, layout := range []string{
		"January 2, 2006 3:04 PM",
		"Jan 2, 2006 3:04 PM",
		"May 2, 2006 3:04 PM",
	} {
		if t, err := time.ParseInLocation(layout, withYear, time.Local); err == nil {
			return t
		}
	}

	return time.Time{}
}
