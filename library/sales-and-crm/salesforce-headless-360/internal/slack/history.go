package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	applog "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/log"
)

const defaultSlackAPIBase = "https://slack.com/api"

type HistoryOptions struct {
	BotToken   string
	Start      time.Time
	End        time.Time
	HTTPClient *http.Client
	BaseURL    string
}

type MessageSummary struct {
	TS             string         `json:"ts"`
	User           string         `json:"user,omitempty"`
	Text           string         `json:"text,omitempty"`
	ReactionCounts map[string]int `json:"reaction_counts,omitempty"`
}

type ChannelHistory struct {
	ChannelID string
	Messages  []MessageSummary
	Warning   string
}

func HistoryFetch(ctx context.Context, linkages []Linkage, opts HistoryOptions) ([]ChannelHistory, Availability, error) {
	availability := Availability{Source: "slack_history"}
	if len(linkages) == 0 {
		availability.Reason = "no_linkage"
		return nil, availability, nil
	}
	if strings.TrimSpace(opts.BotToken) == "" {
		availability.Reason = "no_token"
		return nil, availability, nil
	}
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(firstString(opts.BaseURL, defaultSlackAPIBase), "/")
	var out []ChannelHistory
	for _, linkage := range linkages {
		history, status, err := fetchChannelHistory(ctx, client, baseURL, opts.BotToken, linkage.ChannelID, opts.Start, opts.End)
		if status == http.StatusTooManyRequests {
			availability.Warning = "rate_limited"
			out = append(out, ChannelHistory{ChannelID: linkage.ChannelID, Warning: "rate_limited"})
			continue
		}
		if err != nil {
			return out, availability, err
		}
		out = append(out, history)
	}
	if len(out) > 0 {
		availability.Available = true
	}
	return out, availability, nil
}

func fetchChannelHistory(ctx context.Context, client *http.Client, baseURL, token, channel string, start, end time.Time) (ChannelHistory, int, error) {
	values := url.Values{}
	values.Set("channel", channel)
	values.Set("limit", "10")
	if !start.IsZero() {
		values.Set("oldest", strconv.FormatFloat(float64(start.Unix()), 'f', 0, 64))
	}
	if !end.IsZero() {
		values.Set("latest", strconv.FormatFloat(float64(end.Unix()), 'f', 0, 64))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/conversations.history?"+values.Encode(), nil)
	if err != nil {
		return ChannelHistory{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/1.0.0")
	resp, err := client.Do(req)
	if err != nil {
		return ChannelHistory{}, 0, fmt.Errorf("slack conversations.history %s: %w", channel, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return ChannelHistory{ChannelID: channel, Warning: "rate_limited"}, resp.StatusCode, nil
	}
	if resp.StatusCode >= 400 {
		return ChannelHistory{}, resp.StatusCode, fmt.Errorf("slack conversations.history %s returned HTTP %d: %s", channel, resp.StatusCode, applog.Redact(string(body)))
	}
	var parsed struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Messages []struct {
			TS        string `json:"ts"`
			User      string `json:"user"`
			Text      string `json:"text"`
			Reactions []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"reactions"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ChannelHistory{}, resp.StatusCode, fmt.Errorf("parse slack history: %w", err)
	}
	if !parsed.OK {
		return ChannelHistory{}, resp.StatusCode, fmt.Errorf("slack conversations.history %s failed: %s", channel, parsed.Error)
	}
	history := ChannelHistory{ChannelID: channel}
	for i, msg := range parsed.Messages {
		if i >= 10 {
			break
		}
		summary := MessageSummary{TS: msg.TS, User: msg.User, Text: msg.Text}
		if len(msg.Reactions) > 0 {
			summary.ReactionCounts = map[string]int{}
			for _, reaction := range msg.Reactions {
				summary.ReactionCounts[reaction.Name] += reaction.Count
			}
		}
		history.Messages = append(history.Messages, summary)
	}
	sort.SliceStable(history.Messages, func(i, j int) bool {
		return history.Messages[i].TS > history.Messages[j].TS
	})
	return history, resp.StatusCode, nil
}
