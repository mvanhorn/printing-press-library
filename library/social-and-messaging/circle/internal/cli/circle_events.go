// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/client"
)

type circleEvent struct {
	ID              json.Number `json:"id"`
	Name            string      `json:"name"`
	StartsAt        time.Time   `json:"starts_at"`
	EndsAt          time.Time   `json:"ends_at,omitempty"`
	DurationSeconds *int64      `json:"duration_in_seconds,omitempty"`
	SpaceID         json.Number `json:"space_id,omitempty"`
	Space           struct {
		ID json.Number `json:"id"`
	} `json:"space"`
}

func (e circleEvent) schedule() (time.Time, time.Duration, error) {
	if e.StartsAt.IsZero() {
		return time.Time{}, 0, fmt.Errorf("event has no starts_at")
	}
	duration := e.EndsAt.Sub(e.StartsAt)
	if e.EndsAt.IsZero() {
		if e.DurationSeconds == nil || *e.DurationSeconds < 0 || *e.DurationSeconds > int64((1<<63-1)/int64(time.Second)) {
			return time.Time{}, 0, fmt.Errorf("event has no valid ends_at or duration_in_seconds")
		}
		duration = time.Duration(*e.DurationSeconds) * time.Second
	}
	if duration < 0 {
		return time.Time{}, 0, fmt.Errorf("event ends before it starts")
	}
	return e.StartsAt.UTC(), duration, nil
}

func circlePositiveID(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 || strings.TrimLeft(id, "0123456789") != "" {
		return fmt.Errorf("expected a positive numeric ID")
	}
	return nil
}

func circleDecode(raw json.RawMessage, dest any) error {
	var response struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return fmt.Errorf("invalid Circle response: %w", err)
	}
	if response.Success != nil && !*response.Success {
		return fmt.Errorf("circle API: %s", response.Message)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func circleGet(ctx context.Context, c *client.Client, path string, params map[string]string, dest any) error {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return err
	}
	return circleDecode(raw, dest)
}

func circleList[T any](ctx context.Context, c *client.Client, path string, params map[string]string) ([]T, error) {
	result := make([]T, 0)
	query := make(map[string]string, len(params)+2)
	for key, value := range params {
		query[key] = value
	}
	query["per_page"] = "100"
	for page := 1; ; page++ {
		query["page"] = strconv.Itoa(page)
		var envelope struct {
			Records     []T   `json:"records"`
			HasNextPage *bool `json:"has_next_page"`
		}
		if err := circleGet(ctx, c, path, query, &envelope); err != nil {
			return nil, err
		}
		if envelope.HasNextPage == nil || envelope.Records == nil {
			return nil, fmt.Errorf("%s returned an invalid pagination envelope", path)
		}
		result = append(result, envelope.Records...)
		if !*envelope.HasNextPage {
			return result, nil
		}
		if len(envelope.Records) == 0 {
			return nil, fmt.Errorf("%s returned an empty page with has_next_page=true", path)
		}
	}
}

type circleAttendee struct {
	ID   json.Number `json:"id"`
	Name string      `json:"member_name"`
}

type circleRoom struct {
	ID     json.Number `json:"id"`
	LiveAt *time.Time  `json:"live_at"`
}

func showTellRoom(rooms []circleRoom, start, end time.Time) json.Number {
	var best json.Number
	var distance time.Duration
	for _, room := range rooms {
		if room.LiveAt == nil || room.LiveAt.Before(start.Add(-30*time.Minute)) || room.LiveAt.After(end.Add(30*time.Minute)) {
			continue
		}
		d := room.LiveAt.Sub(start)
		if d < 0 {
			d = -d
		}
		if best == "" || d < distance {
			best, distance = room.ID, d
		}
	}
	return best
}

var circleVTTVoice = regexp.MustCompile(`<v(?:\.[^ >]+)*\s+([^>]+)>`)
var circleVTTTags = regexp.MustCompile(`<[^>]*>`)

func showTellVTT(text string) string {
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\ufeff")
	lines := strings.Split(text, "\n")
	result := make([]string, 0)
	speaker := ""
	inCue, skip := false, false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			inCue, skip = false, false
			continue
		}
		if skip {
			continue
		}
		if line == "NOTE" || strings.HasPrefix(line, "NOTE ") || strings.HasPrefix(line, "NOTE\t") || line == "STYLE" || line == "REGION" {
			skip = true
			continue
		}
		if strings.HasPrefix(line, "WEBVTT") {
			continue
		}
		if strings.Contains(line, "-->") {
			inCue = true
			continue
		}
		if !inCue {
			continue
		} // Cue identifiers and header metadata are not transcript text.
		if match := circleVTTVoice.FindStringSubmatch(line); len(match) > 0 {
			line = circleVTTVoice.ReplaceAllString(line, "$1: ")
		}
		line = strings.TrimSpace(html.UnescapeString(circleVTTTags.ReplaceAllString(line, "")))
		if line == "" {
			continue
		}
		name, words, hasName := strings.Cut(line, ": ")
		if hasName && name != "" {
			if speaker == name && len(result) > 0 {
				result[len(result)-1] += " " + words
			} else {
				result = append(result, line)
			}
			speaker = name
		} else if len(result) > 0 {
			result[len(result)-1] += " " + line
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func circleTranscript(ctx context.Context, entry string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(entry))
	if strings.HasPrefix(strings.TrimSpace(entry), "WEBVTT") || strings.HasPrefix(entry, "\ufeffWEBVTT") {
		return showTellVTT(entry), nil
	}
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("transcript URL must use HTTPS and must not contain credentials")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	// Use a separate HTTP client so Circle authentication is never forwarded to recording storage.
	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: circleTranscriptRedirect}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("transcript download returned HTTP %d", resp.StatusCode)
	}
	const maxBody = 10 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxBody {
		return "", fmt.Errorf("transcript exceeds 10 MiB")
	}
	return showTellVTT(string(data)), nil
}

func circleTranscriptRedirect(req *http.Request, via []*http.Request) error {
	if req.URL.Scheme != "https" || req.URL.User != nil {
		return fmt.Errorf("transcript redirect must use HTTPS and must not contain credentials")
	}
	if len(via) >= 10 {
		return fmt.Errorf("too many transcript redirects")
	}
	return nil
}
