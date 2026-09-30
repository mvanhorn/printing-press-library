// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	healthSeverityFail = "fail"
	healthSeverityWarn = "warn"
	healthSeverityInfo = "info"

	webhookMinSuccessPct  = 99.0
	streamBounceThreshold = 10.0
	streamSpamThreshold   = 0.1
	webhookStatusVerified = "verified"
	dkimUpdateStatusPend  = "Pending"
)

// ---- webhooks health ----

type webhookMetrics struct {
	TotalRequests int `json:"TotalRequests"`
	SuccessCount  int `json:"SuccessCount"`
	FailureCount  int `json:"FailureCount"`
	RetryCount    int `json:"RetryCount"`
}

type webhookHealthRow struct {
	Server         string   `json:"server"`
	ServerID       int64    `json:"server_id"`
	WebhookID      int64    `json:"webhook_id"`
	URL            string   `json:"url"`
	Stream         string   `json:"stream"`
	Status         string   `json:"status"`
	Triggers       []string `json:"triggers"`
	TotalRequests  int      `json:"total_requests"`
	SuccessCount   int      `json:"success_count"`
	FailureCount   int      `json:"failure_count"`
	RetryCount     int      `json:"retry_count"`
	SuccessRatePct *float64 `json:"success_rate_pct"`
	Healthy        bool     `json:"healthy"`
	Problems       []string `json:"problems"`
	Next           string   `json:"next,omitempty"`
}

type webhooksHealthResult struct {
	ServersChecked         int                    `json:"servers_checked"`
	Unhealthy              int                    `json:"unhealthy"`
	Webhooks               []webhookHealthRow     `json:"webhooks"`
	ServersWithoutWebhooks []string               `json:"servers_without_webhooks"`
	FetchFailures          []postmarkFetchFailure `json:"fetch_failures"`
}

// webhookSuccessPct computes the rate from the counts, whose scale is
// unambiguous; it is nil when the webhook had no requests in the window.
func webhookSuccessPct(m webhookMetrics) *float64 {
	if m.TotalRequests > 0 {
		v := ratePct(m.SuccessCount, m.TotalRequests)
		return &v
	}
	return nil
}

// webhookEvaluate lists problems for one webhook's status and 24h metrics.
func webhookEvaluate(status string, m webhookMetrics) []string {
	problems := make([]string, 0)
	if status != "" && !strings.EqualFold(status, webhookStatusVerified) {
		problems = append(problems, "status "+status+": Postmark could not verify the endpoint")
	}
	if m.FailureCount > 0 {
		problems = append(problems, fmt.Sprintf("%d failed deliveries in the last 24h", m.FailureCount))
	}
	if pct := webhookSuccessPct(m); pct != nil && *pct < webhookMinSuccessPct {
		problems = append(problems, fmt.Sprintf("success rate %.2f%% is under %.0f%%", *pct, webhookMinSuccessPct))
	}
	return problems
}

func webhookEnabledTriggers(raw map[string]map[string]any) []string {
	out := make([]string, 0, len(raw))
	for name, cfg := range raw {
		if enabled, _ := cfg["Enabled"].(bool); enabled {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func newWebhooksHealthCmd(flags *rootFlags) *cobra.Command {
	var allServers bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check every webhook's verification status and last-24h delivery failures",
		Long: strings.Trim(`
Lists each webhook on the server (or every server with --all-servers) with its
verification status and last-24-hour delivery metrics from
/webhooks/{id}/statistics. A webhook is unhealthy when it is unverified, had any
failed deliveries, or delivered under 99% of requests. This command never calls
the verify endpoint, which sends test requests to your URL.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli webhooks health --json
  postmark-pp-cli webhooks health --server "Main App" --json
  postmark-pp-cli webhooks health --all-servers --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhooks health")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			targets, err := resolvePostmarkTargets(ctx, flags, targetScope{allServers: allServers})
			if err != nil {
				return err
			}
			perServer, failures := fanoutTargets(ctx, targets, webhooksHealthServer)
			res := webhooksHealthResult{
				ServersChecked:         len(perServer),
				Webhooks:               make([]webhookHealthRow, 0),
				ServersWithoutWebhooks: make([]string, 0),
				FetchFailures:          failures,
			}
			for _, sr := range perServer {
				if len(sr.Rows) == 0 {
					res.ServersWithoutWebhooks = append(res.ServersWithoutWebhooks, sr.Server)
					continue
				}
				for _, r := range sr.Rows {
					if !r.Healthy {
						res.Unhealthy++
					}
					res.Webhooks = append(res.Webhooks, r)
				}
			}
			warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(targets), "server")
			if err := postmarkAllFailed(failures, len(targets)); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			if len(res.Webhooks) == 0 {
				fmt.Fprintf(out, "No webhooks on %d checked server(s).\n", res.ServersChecked)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "SERVER\tID\tSTATUS\tREQUESTS\tFAILED\tSUCCESS%\tURL")
			for _, r := range res.Webhooks {
				rate := "-"
				if r.SuccessRatePct != nil {
					rate = fmt.Sprintf("%.2f", *r.SuccessRatePct)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%d\t%d\t%s\t%s\n", r.Server, r.WebhookID, r.Status, r.TotalRequests, r.FailureCount, rate, r.URL)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, r := range res.Webhooks {
				for _, p := range r.Problems {
					fmt.Fprintf(out, "\n%s webhook %d: %s\n  next: %s\n", r.Server, r.WebhookID, p, r.Next)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&allServers, "all-servers", false, "Check every server the account token lists")
	return cmd
}

type webhookServerResult struct {
	Server string
	Rows   []webhookHealthRow
}

func webhooksHealthServer(ctx context.Context, t postmarkTarget) (webhookServerResult, error) {
	out := webhookServerResult{Server: t.Name}
	var list struct {
		Webhooks []struct {
			ID            int64                     `json:"ID"`
			URL           string                    `json:"Url"`
			MessageStream string                    `json:"MessageStream"`
			Status        string                    `json:"Status"`
			Triggers      map[string]map[string]any `json:"Triggers"`
		} `json:"Webhooks"`
	}
	if err := postmarkGetJSON(ctx, t.client, "/webhooks", nil, &list); err != nil {
		return out, fmt.Errorf("listing webhooks: %w", err)
	}
	rows := make([]webhookHealthRow, 0, len(list.Webhooks))
	for _, w := range list.Webhooks {
		var stats struct {
			Statuses map[string]string `json:"Statuses"`
			Metrics  webhookMetrics    `json:"Metrics"`
		}
		if err := postmarkGetJSON(ctx, t.client, "/webhooks/"+strconv.FormatInt(w.ID, 10)+"/statistics", nil, &stats); err != nil {
			return out, fmt.Errorf("webhook %d statistics: %w", w.ID, err)
		}
		status := w.Status
		if status == "" {
			status = webhookStatusVerified
			for _, s := range stats.Statuses {
				if !strings.EqualFold(s, webhookStatusVerified) {
					status = s
					break
				}
			}
		}
		row := webhookHealthRow{
			Server: t.Name, ServerID: t.ID, WebhookID: w.ID, URL: redactURLSecrets(w.URL), Stream: w.MessageStream, Status: status,
			Triggers:      webhookEnabledTriggers(w.Triggers),
			TotalRequests: stats.Metrics.TotalRequests, SuccessCount: stats.Metrics.SuccessCount,
			FailureCount: stats.Metrics.FailureCount, RetryCount: stats.Metrics.RetryCount,
			SuccessRatePct: webhookSuccessPct(stats.Metrics),
		}
		row.Problems = webhookEvaluate(status, stats.Metrics)
		row.Healthy = len(row.Problems) == 0
		switch {
		case row.Healthy:
		case !strings.EqualFold(status, webhookStatusVerified):
			row.Next = fmt.Sprintf("postmark-pp-cli webhooks verify %d%s", w.ID, postmarkServerArg(t.Name))
		default:
			row.Next = fmt.Sprintf("postmark-pp-cli webhooks statistics %d%s --json", w.ID, postmarkServerArg(t.Name))
		}
		rows = append(rows, row)
	}
	out.Rows = rows
	return out, nil
}

// ---- domains health ----

type healthIssue struct {
	Check    string              `json:"check"`
	Severity string              `json:"severity"`
	Problem  string              `json:"problem"`
	Fix      string              `json:"fix"`
	Record   *bootstrapDNSRecord `json:"record,omitempty"`
}

type domainHealthRow struct {
	DomainID                      int64         `json:"domain_id"`
	Domain                        string        `json:"domain"`
	DKIMVerified                  bool          `json:"dkim_verified"`
	DKIMUpdateStatus              string        `json:"dkim_update_status"`
	ReturnPathDomain              string        `json:"return_path_domain"`
	ReturnPathDomainVerified      bool          `json:"return_path_verified"`
	SafeToRemoveRevokedKeyFromDNS bool          `json:"safe_to_remove_revoked_key_from_dns"`
	Healthy                       bool          `json:"healthy"`
	Issues                        []healthIssue `json:"issues"`
}

type senderHealthRow struct {
	SenderID  int64         `json:"sender_id"`
	Email     string        `json:"email"`
	Domain    string        `json:"domain"`
	Confirmed bool          `json:"confirmed"`
	Healthy   bool          `json:"healthy"`
	Issues    []healthIssue `json:"issues"`
}

type domainsHealthResult struct {
	Domains       []domainHealthRow      `json:"domains"`
	Senders       []senderHealthRow      `json:"senders"`
	Failures      int                    `json:"failures"`
	Warnings      int                    `json:"warnings"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures"`
}

// domainIssues evaluates one domain's DKIM and Return-Path state.
func domainIssues(d bootstrapDomain) []healthIssue {
	issues := make([]healthIssue, 0)
	id := strconv.FormatInt(d.ID, 10)
	records := bootstrapDNSRecords(d, "")
	recordOf := func(kind string) *bootstrapDNSRecord {
		for i := range records {
			if records[i].Type == kind {
				r := records[i]
				return &r
			}
		}
		return nil
	}
	switch {
	case !d.DKIMVerified:
		issues = append(issues, healthIssue{Check: "dkim", Severity: healthSeverityFail, Problem: "DKIM is not verified; mail is not signed for this domain", Fix: "add the TXT record, then run: postmark-pp-cli domains verify-dkim " + id, Record: recordOf("TXT")})
	case strings.EqualFold(d.DKIMUpdateStatus, dkimUpdateStatusPend):
		issues = append(issues, healthIssue{Check: "dkim-rotation", Severity: healthSeverityWarn, Problem: "a new DKIM key is pending", Fix: "add the pending TXT record, then run: postmark-pp-cli domains verify-dkim " + id, Record: recordOf("TXT")})
	}
	if d.WeakDKIM {
		issues = append(issues, healthIssue{Check: "dkim-strength", Severity: healthSeverityWarn, Problem: "the DKIM key is weak (1024-bit)", Fix: "postmark-pp-cli domains rotate-dkim " + id})
	}
	switch {
	case d.ReturnPathDomain == "":
		issues = append(issues, healthIssue{Check: "return-path", Severity: healthSeverityWarn, Problem: "no custom Return-Path; bounces use Postmark's domain, which weakens DMARC alignment", Fix: fmt.Sprintf("postmark-pp-cli domains update %s --return-path-domain pm-bounces.%s", id, d.Name)})
	case !d.ReturnPathDomainVerified:
		issues = append(issues, healthIssue{Check: "return-path", Severity: healthSeverityFail, Problem: "Return-Path " + d.ReturnPathDomain + " is not verified", Fix: "add the CNAME record, then run: postmark-pp-cli domains verify-return-path " + id, Record: recordOf("CNAME")})
	}
	if d.SafeToRemoveRevokedKey && d.DKIMRevokedHost != "" {
		issues = append(issues, healthIssue{Check: "revoked-dkim", Severity: healthSeverityInfo, Problem: "the old DKIM key was revoked and can come out of DNS", Fix: "delete the TXT record at " + d.DKIMRevokedHost})
	}
	return issues
}

func healthHasFail(issues []healthIssue) bool {
	for _, i := range issues {
		if i.Severity == healthSeverityFail {
			return true
		}
	}
	return false
}

func newDomainsHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check DKIM, Return-Path, and sender confirmation for your sending domains",
		Long: strings.Trim(`
Use this command for DKIM, Return-Path, and sender confirmation status of your own sending domains. Do NOT use it for delivery results by recipient provider; use 'recipient-domains' instead.

Reads every domain (with its DNS details) and sender signature through the
account token. Each failure or warning carries the command or DNS record that
fixes it.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli domains health --json
  postmark-pp-cli domains health --agent --select domains.domain,domains.issues`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "domains health")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if !hasAccountToken(c) {
				return configErr(fmt.Errorf("domains health needs POSTMARK_ACCOUNT_TOKEN (domains and sender signatures use the account API)"))
			}
			domains, err := listPostmarkPages(postmarkDomainPages(ctx, c))
			if err != nil {
				return err
			}
			type sender struct {
				ID           int64  `json:"ID"`
				Domain       string `json:"Domain"`
				EmailAddress string `json:"EmailAddress"`
				Confirmed    bool   `json:"Confirmed"`
			}
			senders, err := listPostmarkPages(func(offset int) ([]sender, int, error) {
				var page struct {
					TotalCount       int      `json:"TotalCount"`
					SenderSignatures []sender `json:"SenderSignatures"`
				}
				if err := postmarkGetJSON(ctx, c, "/senders", postmarkPageParams(offset), &page); err != nil {
					return nil, 0, fmt.Errorf("listing sender signatures: %w", err)
				}
				return page.SenderSignatures, page.TotalCount, nil
			})
			if err != nil {
				return err
			}
			details, failures := postmarkFanout(ctx, domains,
				func(d postmarkDomainRef) string { return d.Name },
				func(name string, err error) postmarkFetchFailure {
					return postmarkFetchFailure{Unit: name, Error: err.Error()}
				},
				func(ctx context.Context, d postmarkDomainRef) (bootstrapDomain, error) {
					return getPostmarkDomain(ctx, c, d.ID)
				})
			res := domainsHealthResult{Domains: make([]domainHealthRow, 0, len(details)), Senders: make([]senderHealthRow, 0, len(senders)), FetchFailures: failures}
			warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(domains), "domain")
			for _, d := range details {
				row := domainHealthRow{
					DomainID: d.ID, Domain: d.Name, DKIMVerified: d.DKIMVerified, DKIMUpdateStatus: d.DKIMUpdateStatus,
					ReturnPathDomain: d.ReturnPathDomain, ReturnPathDomainVerified: d.ReturnPathDomainVerified,
					SafeToRemoveRevokedKeyFromDNS: d.SafeToRemoveRevokedKey, Issues: domainIssues(d),
				}
				row.Healthy = !healthHasFail(row.Issues)
				res.Domains = append(res.Domains, row)
			}
			for _, s := range senders {
				row := senderHealthRow{SenderID: s.ID, Email: s.EmailAddress, Domain: s.Domain, Confirmed: s.Confirmed, Issues: make([]healthIssue, 0)}
				if !s.Confirmed {
					row.Issues = append(row.Issues, healthIssue{Check: "confirmation", Severity: healthSeverityFail, Problem: "sender signature is not confirmed; Postmark rejects sends from it", Fix: fmt.Sprintf("postmark-pp-cli senders resend-confirmation %d", s.ID)})
				}
				row.Healthy = !healthHasFail(row.Issues)
				res.Senders = append(res.Senders, row)
			}
			for _, d := range res.Domains {
				res.Failures, res.Warnings = healthCount(d.Issues, res.Failures, res.Warnings)
			}
			for _, s := range res.Senders {
				res.Failures, res.Warnings = healthCount(s.Issues, res.Failures, res.Warnings)
			}
			// Health is unknown, not good, when no domain could be read.
			var readErr error
			if len(domains) > 0 && len(failures) == len(domains) {
				readErr = apiErr(fmt.Errorf("every domain check failed (%d of %d): %s", len(failures), len(domains), failures[0].Error))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
					return err
				}
				return readErr
			}
			out := cmd.OutOrStdout()
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "DOMAIN\tDKIM\tRETURN-PATH\tHEALTHY")
			for _, d := range res.Domains {
				fmt.Fprintf(tw, "%s\t%t\t%s (%t)\t%t\n", d.Domain, d.DKIMVerified, d.ReturnPathDomain, d.ReturnPathDomainVerified, d.Healthy)
			}
			fmt.Fprintln(tw, "\nSENDER\tCONFIRMED\t\t")
			for _, s := range res.Senders {
				fmt.Fprintf(tw, "%s\t%t\t\t\n", s.Email, s.Confirmed)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, d := range res.Domains {
				for _, i := range d.Issues {
					fmt.Fprintf(out, "\n[%s] %s %s: %s\n  fix: %s\n", i.Severity, d.Domain, i.Check, i.Problem, i.Fix)
				}
			}
			for _, s := range res.Senders {
				for _, i := range s.Issues {
					fmt.Fprintf(out, "\n[%s] %s: %s\n  fix: %s\n", i.Severity, s.Email, i.Problem, i.Fix)
				}
			}
			return readErr
		},
	}
	return cmd
}

func healthCount(issues []healthIssue, fails, warns int) (int, int) {
	for _, i := range issues {
		switch i.Severity {
		case healthSeverityFail:
			fails++
		case healthSeverityWarn:
			warns++
		}
	}
	return fails, warns
}

// ---- streams health ----

type streamHealthRow struct {
	Server        string   `json:"server"`
	ServerID      int64    `json:"server_id"`
	Stream        string   `json:"stream"`
	StreamType    string   `json:"stream_type"`
	Sent          int      `json:"sent"`
	Bounced       int      `json:"bounced"`
	Spam          int      `json:"spam"`
	BounceRatePct float64  `json:"bounce_rate_pct"`
	SpamRatePct   float64  `json:"spam_rate_pct"`
	Healthy       bool     `json:"healthy"`
	Problems      []string `json:"problems"`
	Next          string   `json:"next,omitempty"`
}

type streamsHealthResult struct {
	WindowDays         int                    `json:"window_days"`
	FromDate           string                 `json:"fromdate"`
	ToDate             string                 `json:"todate"`
	BounceThresholdPct float64                `json:"bounce_threshold_pct"`
	SpamThresholdPct   float64                `json:"spam_threshold_pct"`
	Unhealthy          int                    `json:"unhealthy"`
	Streams            []streamHealthRow      `json:"streams"`
	FetchFailures      []postmarkFetchFailure `json:"fetch_failures"`
}

// streamEvaluate applies the absolute thresholds to one stream's totals.
func streamEvaluate(row *streamHealthRow) {
	row.BounceRatePct = ratePct(row.Bounced, row.Sent)
	row.SpamRatePct = ratePct(row.Spam, row.Sent)
	row.Problems = make([]string, 0)
	if row.Sent > 0 && row.BounceRatePct >= streamBounceThreshold {
		row.Problems = append(row.Problems, fmt.Sprintf("bounce rate %.2f%% is at or over %.0f%%", row.BounceRatePct, streamBounceThreshold))
	}
	if row.Sent > 0 && row.SpamRatePct >= streamSpamThreshold {
		row.Problems = append(row.Problems, fmt.Sprintf("spam complaint rate %.3f%% is at or over %.1f%%", row.SpamRatePct, streamSpamThreshold))
	}
	row.Healthy = len(row.Problems) == 0
}

func newStreamsHealthCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var allServers bool
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Flag message streams whose bounce rate is 10%+ or spam rate is 0.1%+",
		Long: strings.Trim(`
Use this command for absolute bounce and spam threshold checks per stream. Do NOT use it to find changes against a stream's own history; use 'pulse' instead.

Sums the sends, bounces, and spam complaint stats for each outbound stream over
the window (Eastern Time days ending today) on the server, or every server with
--all-servers.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli streams health --json
  postmark-pp-cli streams health --server Staging --json
  postmark-pp-cli streams health --all-servers --window 30d --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "streams health")
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
			targets, err := resolvePostmarkTargets(ctx, flags, targetScope{allServers: allServers})
			if err != nil {
				return err
			}
			perServer, failures := fanoutTargets(ctx, targets, func(ctx context.Context, t postmarkTarget) ([]streamHealthRow, error) {
				return streamsHealthServer(ctx, t, from, to)
			})
			res := streamsHealthResult{
				WindowDays: days, FromDate: from.Format(postmarkDateLayout), ToDate: to.Format(postmarkDateLayout),
				BounceThresholdPct: streamBounceThreshold, SpamThresholdPct: streamSpamThreshold,
				Streams: make([]streamHealthRow, 0), FetchFailures: failures,
			}
			for _, rows := range perServer {
				for _, r := range rows {
					if !r.Healthy {
						res.Unhealthy++
					}
					res.Streams = append(res.Streams, r)
				}
			}
			warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(targets), "server")
			if err := postmarkAllFailed(failures, len(targets)); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			if len(res.Streams) == 0 {
				fmt.Fprintln(out, "No outbound streams found.")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "SERVER\tSTREAM\tSENT\tBOUNCE%\tSPAM%\tHEALTHY")
			for _, r := range res.Streams {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%.2f\t%.3f\t%t\n", r.Server, r.Stream, r.Sent, r.BounceRatePct, r.SpamRatePct, r.Healthy)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			for _, r := range res.Streams {
				for _, p := range r.Problems {
					fmt.Fprintf(out, "\n%s/%s: %s\n  next: %s\n", r.Server, r.Stream, p, r.Next)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Stats window ending today (e.g. 7d, 30d)")
	cmd.Flags().BoolVar(&allServers, "all-servers", false, "Check every server the account token lists")
	return cmd
}

func streamsHealthServer(ctx context.Context, t postmarkTarget, from, to time.Time) ([]streamHealthRow, error) {
	streams, err := listPostmarkStreams(ctx, t.client, true)
	if err != nil {
		return nil, err
	}
	rows := make([]streamHealthRow, 0, len(streams))
	for _, s := range streams {
		if !s.active() {
			continue
		}
		days, err := fetchPostmarkDailyStats(ctx, t.client, from, to, s.ID, "")
		if err != nil {
			return nil, fmt.Errorf("stream %s: %w", s.ID, err)
		}
		row := streamHealthRow{Server: t.Name, ServerID: t.ID, Stream: s.ID, StreamType: s.MessageStreamType}
		for _, d := range days {
			row.Sent += d.Sent
			row.Bounced += d.Bounced
			row.Spam += d.Spam
		}
		streamEvaluate(&row)
		if !row.Healthy {
			typeFilter := ""
			if row.BounceRatePct < streamBounceThreshold {
				typeFilter = " --type " + postmarkSpamComplaint
			}
			row.Next = fmt.Sprintf("postmark-pp-cli bounces list%s --messagestream %s%s --fromdate %s --count 50 --json", postmarkServerArg(t.Name), shellQuoteWord(s.ID), typeFilter, from.Format(postmarkDateLayout))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if parent, _, err := root.Find([]string{"webhooks"}); err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newWebhooksHealthCmd(flags))
		}
		if parent, _, err := root.Find([]string{"domains"}); err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newDomainsHealthCmd(flags))
		}
		if parent, _, err := root.Find([]string{"streams"}); err == nil && parent != root {
			addNovelCommandIfAbsent(parent, newStreamsHealthCmd(flags))
		}
	})
}
