// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSendBudgetCmd(flags))
	})
}

type sendBudgetUsage struct {
	Used  *float64  `json:"usage_used"`
	Cap   *float64  `json:"usage_max"`
	Unit  string    `json:"unit"`
	Start time.Time `json:"period_start"`
	End   time.Time `json:"period_end"`
	AsOf  time.Time `json:"as_of"`
}

type sendBudgetPeriod struct {
	Period           int       `json:"period"`
	Start            time.Time `json:"period_start"`
	End              time.Time `json:"period_end"`
	Cap              float64   `json:"cap"`
	Scheduled        int64     `json:"scheduled_estimate"`
	UnknownCount     int       `json:"unknown_count"`
	ScheduledPercent *float64  `json:"scheduled_percent"`
}

type sendBudgetCurrent struct {
	sendBudgetPeriod
	Used                    float64  `json:"used"`
	Remaining               float64  `json:"remaining"`
	Committed               float64  `json:"committed"`
	RemainingAfterScheduled float64  `json:"remaining_after_scheduled"`
	OverBy                  float64  `json:"over_by"`
	PaceProjection          *float64 `json:"pace_projection"`
	PaceOverBy              float64  `json:"pace_over_by"`
	Verdict                 string   `json:"verdict"`
}

type sendBudgetMessage struct {
	ID             string    `json:"message_id"`
	CampaignName   string    `json:"campaign_name"`
	MessageName    string    `json:"message_name"`
	SendTime       time.Time `json:"send_time"`
	Estimate       *int64    `json:"estimate"`
	EstimateStatus string    `json:"estimate_status"`
	Period         int       `json:"period"`
}

type sendBudgetReport struct {
	AsOf      time.Time           `json:"as_of"`
	Current   sendBudgetCurrent   `json:"current_period"`
	Later     []sendBudgetPeriod  `json:"later_periods"`
	Messages  []sendBudgetMessage `json:"messages"`
	PaceNote  string              `json:"pace_note"`
	LaterNote string              `json:"later_period_note"`
}

func newSendBudgetCmd(flags *rootFlags) *cobra.Command {
	var noRefresh bool
	var wait time.Duration
	cmd := &cobra.Command{
		Use:   "send-budget",
		Short: "Compare scheduled email sends with billing-period capacity",
		Long:  "Compare email usage and scheduled message estimates by billing period. Missing estimates are refreshed by default; estimation jobs compute counts without changing campaigns. Use --no-refresh for GET requests only.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if wait <= 0 {
				return usageErr(fmt.Errorf("--wait must be greater than zero"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "refresh_estimates": !noRefresh}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := collectSendBudget(cmd.Context(), c, noRefresh, wait)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			return printSendBudget(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "Do not create estimation jobs; use available estimates only (fully read-only)")
	cmd.Flags().DurationVar(&wait, "wait", 2*time.Minute, "Total time budget for refreshing missing or stale estimates")
	return cmd
}

// A broken next link must fail instead of presenting a partial budget as complete.
func sendBudgetCollection(ctx context.Context, c *client.Client, path string, params map[string]string) ([]json.RawMessage, error) {
	query := make(map[string]string, len(params)+1)
	for k, v := range params {
		query[k] = v
	}
	if path == "/api/campaign-messages" {
		query["page[size]"] = "100"
	}
	var items []json.RawMessage
	seen := map[string]bool{}
	for {
		raw, err := c.GetNoCache(ctx, path, query)
		if err != nil {
			return nil, err
		}
		var page struct {
			Data  []json.RawMessage `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		if page.Data == nil {
			return nil, fmt.Errorf("reading %s: missing data collection", path)
		}
		items = append(items, page.Data...)
		if page.Links.Next == "" {
			return items, nil
		}
		next, err := url.Parse(page.Links.Next)
		if err != nil {
			return nil, fmt.Errorf("invalid pagination link for %s: %w", path, err)
		}
		cursor := next.Query().Get("page[cursor]")
		if cursor == "" || seen[cursor] {
			return nil, fmt.Errorf("missing or repeated pagination cursor for %s", path)
		}
		seen[cursor] = true
		query["page[cursor]"] = cursor
	}
}

func readSendBudgetUsage(ctx context.Context, c *client.Client) (sendBudgetUsage, error) {
	rows, err := sendBudgetCollection(ctx, c, "/api/billing-usage", nil)
	if err != nil {
		return sendBudgetUsage{}, err
	}
	for _, row := range rows {
		var resource struct {
			ID         string          `json:"id"`
			Attributes json.RawMessage `json:"attributes"`
		}
		if err := json.Unmarshal(row, &resource); err != nil {
			return sendBudgetUsage{}, err
		}
		if resource.ID != "email_sends" {
			continue
		}
		var usage sendBudgetUsage
		if err := json.Unmarshal(resource.Attributes, &usage); err != nil {
			return usage, fmt.Errorf("reading email_sends billing usage: %w", err)
		}
		if usage.Cap == nil {
			return usage, fmt.Errorf("plan has no email cap via API (email_sends.usage_max is missing)")
		}
		if usage.Used == nil || *usage.Used < 0 || *usage.Cap < 0 || usage.Unit != "messages" || usage.Start.IsZero() || !usage.End.After(usage.Start) || usage.AsOf.IsZero() {
			return usage, fmt.Errorf("invalid email_sends billing usage: expected nonnegative counts, messages unit, billing boundaries and as_of")
		}
		return usage, nil
	}
	return sendBudgetUsage{}, fmt.Errorf("plan has no email cap via API: email_sends is missing from billing usage")
}

func collectSendBudget(ctx context.Context, c *client.Client, noRefresh bool, wait time.Duration) (sendBudgetReport, error) {
	usage, err := readSendBudgetUsage(ctx, c)
	if err != nil {
		return sendBudgetReport{}, err
	}
	rows, err := sendBudgetCollection(ctx, c, "/api/campaign-messages", map[string]string{"filter": "equals(status,'scheduled')"})
	if err != nil {
		return sendBudgetReport{}, err
	}
	messages := make([]sendBudgetMessage, 0)
	seen := map[string]bool{}
	for _, row := range rows {
		var resource struct {
			ID         string `json:"id"`
			Attributes struct {
				Name       string `json:"name"`
				Definition struct {
					Name        string `json:"name"`
					SendOptions struct {
						SendTime time.Time `json:"send_time"`
					} `json:"send_options"`
				} `json:"definition"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal(row, &resource); err != nil {
			return sendBudgetReport{}, fmt.Errorf("reading scheduled message: %w", err)
		}
		if resource.ID == "" {
			return sendBudgetReport{}, fmt.Errorf("scheduled message has no id")
		}
		if seen[resource.ID] {
			continue
		}
		seen[resource.ID] = true
		path := "/api/campaign-messages/" + url.PathEscape(resource.ID)
		variations, err := sendBudgetCollection(ctx, c, path+"/campaign-variations", nil)
		if err != nil {
			return sendBudgetReport{}, err
		}
		email := false
		for _, variation := range variations {
			if strings.EqualFold(firstString(variation, "attributes.definition.details.channel"), "email") {
				email = true
			}
		}
		if !email {
			continue
		}
		if resource.Attributes.Definition.SendOptions.SendTime.IsZero() {
			return sendBudgetReport{}, fmt.Errorf("scheduled email message %s has no definition.send_options.send_time", resource.ID)
		}
		container, err := c.GetNoCache(ctx, path+"/campaign", nil)
		if err != nil {
			return sendBudgetReport{}, err
		}
		campaignName := firstString(container, "data.attributes.name", "data.attributes.definition.name")
		if campaignName == "" {
			return sendBudgetReport{}, fmt.Errorf("campaign for message %s has no name", resource.ID)
		}
		name := resource.Attributes.Definition.Name
		if name == "" {
			name = resource.Attributes.Name
		}
		estimate, err := readSendBudgetEstimate(ctx, c, resource.ID)
		if err != nil {
			return sendBudgetReport{}, err
		}
		messages = append(messages, sendBudgetMessage{ID: resource.ID, CampaignName: campaignName, MessageName: name, SendTime: resource.Attributes.Definition.SendOptions.SendTime, Estimate: estimate})
	}
	if !noRefresh {
		if err := refreshSendBudgetEstimates(ctx, c, messages, wait); err != nil {
			return sendBudgetReport{}, err
		}
	}
	return buildSendBudgetReport(usage, messages)
}

func readSendBudgetEstimate(ctx context.Context, c *client.Client, id string) (*int64, error) {
	raw, err := c.GetNoCache(ctx, "/api/campaign-recipient-estimations/"+url.PathEscape(id), nil)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}
	var response struct {
		Data struct {
			Attributes struct {
				Count *int64 `json:"estimated_recipient_count"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("reading estimate for %s: %w", id, err)
	}
	if response.Data.Attributes.Count != nil && *response.Data.Attributes.Count < 0 {
		return nil, fmt.Errorf("negative recipient estimate for message %s", id)
	}
	return response.Data.Attributes.Count, nil
}

func refreshSendBudgetEstimates(ctx context.Context, c *client.Client, messages []sendBudgetMessage, wait time.Duration) error {
	refreshCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	// Start all missing jobs before polling so each has the same total wait budget.
	pending := map[int]string{}
	for i := range messages {
		if messages[i].Estimate != nil {
			continue
		}
		body := map[string]any{"data": map[string]string{"type": "campaign-recipient-estimation-job", "id": messages[i].ID}}
		raw, _, err := c.Post(refreshCtx, "/api/campaign-recipient-estimation-jobs", body)
		if err != nil {
			return sendBudgetRefreshError(ctx, refreshCtx, err)
		}
		id := firstString(raw, "data.id")
		if id == "" {
			return fmt.Errorf("estimation job for message %s has no id", messages[i].ID)
		}
		pending[i] = id
	}
	backoff := 250 * time.Millisecond
	for len(pending) > 0 {
		for i := range messages {
			id, ok := pending[i]
			if !ok {
				continue
			}
			raw, err := c.GetNoCache(refreshCtx, "/api/campaign-recipient-estimation-jobs/"+url.PathEscape(id), nil)
			if err != nil {
				return sendBudgetRefreshError(ctx, refreshCtx, err)
			}
			switch strings.ToLower(firstString(raw, "data.attributes.status")) {
			case "complete":
				estimate, err := readSendBudgetEstimate(refreshCtx, c, messages[i].ID)
				if err != nil {
					return sendBudgetRefreshError(ctx, refreshCtx, err)
				}
				if estimate != nil {
					messages[i].Estimate = estimate
					delete(pending, i)
				}
			case "cancelled", "failed":
				delete(pending, i)
			case "queued", "processing":
			default:
				return fmt.Errorf("estimation job %s returned an invalid status", id)
			}
		}
		if len(pending) == 0 {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-refreshCtx.Done():
			timer.Stop()
			return sendBudgetRefreshError(ctx, refreshCtx, refreshCtx.Err())
		case <-timer.C:
		}
		backoff = min(2*backoff, 5*time.Second)
	}
	return nil
}

func sendBudgetRefreshError(ctx, refreshCtx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(refreshCtx.Err(), context.DeadlineExceeded) {
		return nil // Expired estimates stay nil and are counted as unknown.
	}
	return err
}

// Keep the billing day anchored; AddDate alone rolls January 31 into March.
func sendBudgetMonthBoundary(anchor time.Time, months int) time.Time {
	first := time.Date(anchor.Year(), anchor.Month()+time.Month(months), 1, anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), anchor.Location())
	lastDay := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(anchor.Day(), lastDay)-1)
}

func buildSendBudgetReport(usage sendBudgetUsage, messages []sendBudgetMessage) (sendBudgetReport, error) {
	periods := map[int]*sendBudgetPeriod{0: {Start: usage.Start, End: usage.End, Cap: *usage.Cap}}
	for i := range messages {
		message := &messages[i]
		// An overdue scheduled send (send time already past) still goes out now, so it counts in the current period.
		period, start, end := 0, usage.Start, usage.End
		for !message.SendTime.Before(end) {
			period++
			start, end = end, sendBudgetMonthBoundary(usage.End, period)
		}
		message.Period = period
		if periods[period] == nil {
			periods[period] = &sendBudgetPeriod{Period: period, Start: start, End: end, Cap: *usage.Cap}
		}
		if message.Estimate == nil {
			message.EstimateStatus = "no estimate"
			periods[period].UnknownCount++
		} else {
			message.EstimateStatus = "available"
			periods[period].Scheduled += *message.Estimate
		}
	}
	for _, period := range periods {
		if period.Cap > 0 {
			percentage := float64(period.Scheduled) / period.Cap * 100
			period.ScheduledPercent = &percentage
		}
	}
	current := sendBudgetCurrent{sendBudgetPeriod: *periods[0], Used: *usage.Used, Verdict: "ok"}
	current.Remaining = current.Cap - current.Used
	current.Committed = current.Used + float64(current.Scheduled)
	current.RemainingAfterScheduled = current.Cap - current.Committed
	current.OverBy = math.Max(0, current.Committed-current.Cap)
	elapsed := usage.AsOf.Sub(usage.Start)
	if elapsed > 0 {
		fraction := math.Min(1, float64(elapsed)/float64(usage.End.Sub(usage.Start)))
		projection := current.Used / fraction
		current.PaceProjection = &projection
		current.PaceOverBy = math.Max(0, projection-current.Cap)
	}
	if current.OverBy > 0 {
		current.Verdict = "over"
	} else if current.PaceOverBy > 0 {
		current.Verdict = "at risk"
	}
	report := sendBudgetReport{
		AsOf: usage.AsOf, Current: current, Later: []sendBudgetPeriod{}, Messages: messages,
		PaceNote:  "Pace includes flows and past campaigns; it is not added to scheduled estimates.",
		LaterNote: "Usage in later periods has not started; comparisons assume the same cap.",
	}
	for index, period := range periods {
		if index > 0 {
			report.Later = append(report.Later, *period)
		}
	}
	sort.Slice(report.Later, func(i, j int) bool { return report.Later[i].Period < report.Later[j].Period })
	sort.SliceStable(report.Messages, func(i, j int) bool { return report.Messages[i].SendTime.Before(report.Messages[j].SendTime) })
	return report, nil
}

func sendBudgetNumber(value float64) string {
	formatted := strconv.FormatFloat(math.Round(value), 'f', 0, 64)
	start := 0
	if strings.HasPrefix(formatted, "-") {
		start = 1
	}
	for i := len(formatted) - 3; i > start; i -= 3 {
		formatted = formatted[:i] + "," + formatted[i:]
	}
	return formatted
}

func printSendBudget(w io.Writer, report sendBudgetReport) error {
	var out strings.Builder
	c := report.Current
	fmt.Fprintf(&out, "This period [%s, %s): %s of %s sent, %s more scheduled.\n", c.Start.Format(time.RFC3339), c.End.Format(time.RFC3339), sendBudgetNumber(c.Used), sendBudgetNumber(c.Cap), sendBudgetNumber(float64(c.Scheduled)))
	fmt.Fprintf(&out, "Remaining before scheduled: %s. Committed: %s.\n", sendBudgetNumber(c.Remaining), sendBudgetNumber(c.Committed))
	qualifier := ""
	if c.UnknownCount > 0 {
		qualifier = " (known estimates only)"
	}
	switch c.Verdict {
	case "over":
		fmt.Fprintf(&out, "Over budget%s: %s above cap.\n", qualifier, sendBudgetNumber(c.OverBy))
	case "at risk":
		fmt.Fprintf(&out, "At risk%s: pace projects %s above cap; %s left after scheduled.\n", qualifier, sendBudgetNumber(c.PaceOverBy), sendBudgetNumber(c.RemainingAfterScheduled))
	default:
		fmt.Fprintf(&out, "On track%s: %s left after scheduled.\n", qualifier, sendBudgetNumber(c.RemainingAfterScheduled))
	}
	if c.UnknownCount > 0 {
		fmt.Fprintf(&out, "%s scheduled message(s) have no estimate; their sends are unknown and excluded from totals.\n", sendBudgetNumber(float64(c.UnknownCount)))
	}
	if c.PaceProjection == nil {
		fmt.Fprintln(&out, "Pace projection: unavailable before billing usage has elapsed.")
	} else {
		fmt.Fprintf(&out, "Pace projection: %s sends.\n", sendBudgetNumber(*c.PaceProjection))
	}
	fmt.Fprintln(&out, report.PaceNote)
	for _, period := range report.Later {
		percent := "percentage unavailable for zero cap"
		if period.ScheduledPercent != nil {
			percent = fmt.Sprintf("%.1f%%", *period.ScheduledPercent)
		}
		fmt.Fprintf(&out, "Period %d [%s, %s): %s scheduled of %s cap (%s), %s with no estimate.\n", period.Period, period.Start.Format(time.RFC3339), period.End.Format(time.RFC3339), sendBudgetNumber(float64(period.Scheduled)), sendBudgetNumber(period.Cap), percent, sendBudgetNumber(float64(period.UnknownCount)))
	}
	if len(report.Later) > 0 {
		fmt.Fprintln(&out, report.LaterNote)
	}
	for _, message := range report.Messages {
		estimate := "no estimate"
		if message.Estimate != nil {
			estimate = sendBudgetNumber(float64(*message.Estimate))
		}
		fmt.Fprintf(&out, "%s / %s (%s): %s, estimate %s, period %d\n", message.CampaignName, message.MessageName, message.ID, message.SendTime.Format(time.RFC3339), estimate, message.Period)
	}
	_, err := fmt.Fprint(w, terminalText(w, out.String()))
	return err
}
