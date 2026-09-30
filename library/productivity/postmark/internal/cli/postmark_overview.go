// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

type overviewRow struct {
	Server             string  `json:"server"`
	ServerID           int64   `json:"server_id"`
	DeliveryType       string  `json:"delivery_type"`
	Sent               int     `json:"sent"`
	Bounced            int     `json:"bounced"`
	BounceRate         float64 `json:"bounce_rate"`
	SpamComplaints     int     `json:"spam_complaints"`
	SpamComplaintsRate float64 `json:"spam_complaints_rate"`
	Streams            int     `json:"streams"`
}

type overviewTotals struct {
	Sent               int     `json:"sent"`
	Bounced            int     `json:"bounced"`
	BounceRate         float64 `json:"bounce_rate"`
	SpamComplaints     int     `json:"spam_complaints"`
	SpamComplaintsRate float64 `json:"spam_complaints_rate"`
}

type overviewResult struct {
	WindowDays    int                    `json:"window_days"`
	FromDate      string                 `json:"fromdate"`
	ToDate        string                 `json:"todate"`
	RateUnit      string                 `json:"rate_unit"`
	Servers       []overviewRow          `json:"servers"`
	Totals        overviewTotals         `json:"totals"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures"`
}

// overviewTotalsOf sums successful rows; failed servers never reach it.
func overviewTotalsOf(rows []overviewRow) overviewTotals {
	var t overviewTotals
	for _, r := range rows {
		t.Sent += r.Sent
		t.Bounced += r.Bounced
		t.SpamComplaints += r.SpamComplaints
	}
	t.BounceRate = ratePct(t.Bounced, t.Sent)
	t.SpamComplaintsRate = ratePct(t.SpamComplaints, t.Sent)
	return t
}

func newOverviewCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Snapshot every server: delivery type, sends, bounce and spam rates, stream count",
		Long: strings.Trim(`
Use this command for a current snapshot of every server in the account. Do NOT use it to find servers whose sending changed against their own history; use 'pulse' instead.

Reads /stats/outbound for the window (Eastern Time days ending today) and the
stream list for every server the account token lists, or only --server. Rates
are percentages, as Postmark reports them.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli overview --agent
  postmark-pp-cli overview --window 30d --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "overview")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			days, err := postmarkParseDays(flagWindow, "window")
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			to := postmarkDayStart(time.Now())
			from := to.AddDate(0, 0, -(days - 1))
			res := overviewResult{
				WindowDays:    days,
				FromDate:      from.Format(postmarkDateLayout),
				ToDate:        to.Format(postmarkDateLayout),
				RateUnit:      "percent",
				Servers:       make([]overviewRow, 0),
				FetchFailures: make([]postmarkFetchFailure, 0),
			}
			targets, err := resolvePostmarkTargets(ctx, flags, targetScope{allServers: true})
			if err != nil {
				return err
			}
			rows, failures := fanoutTargets(ctx, targets, func(ctx context.Context, t postmarkTarget) (overviewRow, error) {
				return overviewServer(ctx, t, res.FromDate, res.ToDate)
			})
			res.Servers, res.FetchFailures = rows, failures
			res.Totals = overviewTotalsOf(rows)
			warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(targets), "server")
			if err := postmarkAllFailed(failures, len(targets)); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Servers, %s..%s (Eastern)\n", res.FromDate, res.ToDate)
			if len(rows) == 0 {
				fmt.Fprintln(out, "No servers returned stats.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "SERVER\tTYPE\tSENT\tBOUNCED\tBOUNCE%\tSPAM\tSPAM%\tSTREAMS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%.2f\t%d\t%.3f\t%d\n", r.Server, r.DeliveryType, r.Sent, r.Bounced, r.BounceRate, r.SpamComplaints, r.SpamComplaintsRate, r.Streams)
			}
			fmt.Fprintf(tw, "TOTAL\t\t%d\t%d\t%.2f\t%d\t%.3f\t\n", res.Totals.Sent, res.Totals.Bounced, res.Totals.BounceRate, res.Totals.SpamComplaints, res.Totals.SpamComplaintsRate)
			if err := tw.Flush(); err != nil {
				return err
			}
			if len(failures) > 0 {
				fmt.Fprintf(out, "\npartial results: %d of %d servers failed; totals cover the remaining %d\n", len(failures), len(targets), len(rows))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Stats window ending today (e.g. 7d, 30d)")
	return cmd
}

func overviewServer(ctx context.Context, t postmarkTarget, from, to string) (overviewRow, error) {
	row := overviewRow{Server: t.Name, ServerID: t.ID, DeliveryType: t.deliveryType}
	var stats map[string]json.RawMessage
	if err := postmarkGetJSON(ctx, t.client, "/stats/outbound", map[string]string{"fromdate": from, "todate": to}, &stats); err != nil {
		return row, fmt.Errorf("stats: %w", err)
	}
	// Counts are read as numbers and truncated, so a fractional value never
	// zeroes a row.
	number := func(key string) float64 {
		v, _ := cliutil.ExtractNumber(stats, key)
		return v
	}
	row.Sent = int(number("Sent"))
	row.Bounced = int(number("Bounced"))
	row.SpamComplaints = int(number("SpamComplaints"))
	row.BounceRate = number("BounceRate")
	row.SpamComplaintsRate = number("SpamComplaintsRate")
	streams, err := listPostmarkStreams(ctx, t.client, false)
	if err != nil {
		return row, err
	}
	row.Streams = len(streams)
	if t.ID == 0 || row.DeliveryType == "" {
		overviewFillFromServer(ctx, t.client, &row)
	}
	return row, nil
}

// overviewFillFromServer reads /server when the account listing is not
// available (a single configured server token).
func overviewFillFromServer(ctx context.Context, c *client.Client, row *overviewRow) {
	var info struct {
		ID           int64  `json:"ID"`
		DeliveryType string `json:"DeliveryType"`
	}
	if postmarkGetJSON(ctx, c, "/server", nil, &info) == nil {
		row.DeliveryType = info.DeliveryType
		if row.ServerID == 0 {
			row.ServerID = info.ID
		}
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newOverviewCmd(flags))
	})
}
