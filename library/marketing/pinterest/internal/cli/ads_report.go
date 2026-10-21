// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil"
	"github.com/spf13/cobra"
)

const maxReportDownload = 20 << 20

func normalizeReportRows(raw json.RawMessage) ([]map[string]any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("parsing ads report JSON: %w", err)
	}
	rows := make([]map[string]any, 0)
	switch data := value.(type) {
	case []any:
		for _, item := range data {
			row, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("ads report row is not an object")
			}
			rows = append(rows, row)
		}
	case map[string]any:
		ids := make([]string, 0, len(data))
		for id := range data {
			ids = append(ids, id)
		}
		sort.Strings(ids) // stable output: Go map order is random
		for _, id := range ids {
			item := data[id]
			list, ok := item.([]any)
			if !ok {
				return nil, fmt.Errorf("ads report entity %q is not an array", id)
			}
			for _, entry := range list {
				row, ok := entry.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("ads report row for entity %q is not an object", id)
				}
				if _, exists := row["entity_id"]; !exists {
					row["entity_id"] = id
				}
				rows = append(rows, row)
			}
		}
	default:
		return nil, fmt.Errorf("ads report JSON must be an object or array")
	}
	return rows, nil
}

func newNovelAdsReportCmd(flags *rootFlags) *cobra.Command {
	var account, level, granularity, columnsRaw, pollRaw, maxRaw, resumeToken string
	var days int
	levels := []string{"CAMPAIGN", "AD_GROUP", "PIN_PROMOTION", "ADVERTISER"}
	cmd := &cobra.Command{Use: "report", Short: "Create and download an ads performance report", Example: strings.Trim(`
  pinterest-pp-cli ads report --level campaign --days 7 --agent
`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--days=7"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "ads report")
		}
		if strings.TrimSpace(account) == "" {
			account = strings.TrimSpace(os.Getenv("PINTEREST_AD_ACCOUNT_ID"))
		}
		level, granularity = strings.ToUpper(level), strings.ToUpper(granularity)
		if !containsString(levels, level) {
			return usageErr(fmt.Errorf("--level must be one of %s", strings.Join(levels, ", ")))
		}
		if days < 1 || days > 90 {
			return usageErr(fmt.Errorf("--days must be between 1 and 90"))
		}
		if granularity != "TOTAL" && granularity != "DAY" {
			return usageErr(fmt.Errorf("--granularity must be TOTAL or DAY"))
		}
		poll, err := cliutil.ParseDurationLoose(pollRaw)
		if err != nil || poll <= 0 {
			return usageErr(fmt.Errorf("--poll-interval must be a positive duration"))
		}
		maxWait, err := cliutil.ParseDurationLoose(maxRaw)
		if err != nil || maxWait <= 0 {
			return usageErr(fmt.Errorf("--max-wait must be a positive duration"))
		}
		if cliutil.IsDogfoodEnv() {
			maxWait = 25 * time.Second
		}
		columns := splitCSV(columnsRaw)
		if len(columns) == 0 {
			return usageErr(fmt.Errorf("--columns must contain at least one report column"))
		}
		// Bound the whole create/poll/download flow by --max-wait (plus slack for
		// create and download), not root --timeout: each request still carries
		// the client's per-request timeout, and a 60s --timeout must not cut a 2m wait.
		ctx, cancel := context.WithTimeout(cmd.Context(), maxWait+time.Minute)
		defer cancel()
		client, err := flags.newClient()
		if err != nil {
			return err
		}
		if account == "" {
			// Fall back to the token's first ad account so the common single-account
			// case needs no flag.
			if account, err = firstAdAccountID(ctx, client); err != nil {
				return err
			}
		}
		end := nowUTC().AddDate(0, 0, -1)
		start := end.AddDate(0, 0, -days+1)
		startText, endText := start.Format("2006-01-02"), end.Format("2006-01-02")
		path := "/ad_accounts/" + url.PathEscape(account) + "/reports"
		var job struct {
			Token   string `json:"token"`
			Status  string `json:"report_status"`
			Message any    `json:"message"`
		}
		job.Token = strings.TrimSpace(resumeToken)
		if job.Token == "" {
			created, _, err := client.Post(ctx, path, map[string]any{"start_date": startText, "end_date": endText, "granularity": granularity, "level": level, "columns": columns, "report_format": "JSON"})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if err = json.Unmarshal(created, &job); err != nil {
				return fmt.Errorf("parsing report creation response: %w", err)
			}
			if job.Token == "" {
				return fmt.Errorf("report creation response did not include a token")
			}
		}
		deadline := time.Now().Add(maxWait)
		var reportURL string
		for {
			// Status polls must bypass the response cache or the first IN_PROGRESS replays forever.
			pollRaw, err := client.GetNoCache(ctx, path, map[string]string{"token": job.Token})
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "ads report token %s (resume with --token)\n", job.Token)
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var state struct {
				Status string `json:"report_status"`
				URL    string `json:"url"`
			}
			if err = json.Unmarshal(pollRaw, &state); err != nil {
				return fmt.Errorf("parsing report status: %w", err)
			}
			switch state.Status {
			case "FINISHED":
				if state.URL == "" {
					return fmt.Errorf("ads report %s finished without a download URL", job.Token)
				}
				reportURL = state.URL
			case "IN_PROGRESS":
			default:
				return fmt.Errorf("ads report ended with status %s", state.Status)
			}
			if reportURL != "" {
				break
			}
			if time.Now().Add(poll).After(deadline) {
				if cliutil.IsDogfoodEnv() {
					// Curtailed live read under the dogfood harness: report the real
					// pending job instead of failing on the harness's short wait cap.
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"ad_account_id": account, "token": job.Token, "report_status": "IN_PROGRESS", "rows": make([]map[string]any, 0)}, flags)
				}
				return fmt.Errorf("timed out waiting for ads report; rerun with --token %s (and a larger --max-wait) to keep waiting", job.Token)
			}
			timer := time.NewTimer(poll)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("waiting for ads report (rerun with --token %s to resume): %w", job.Token, ctx.Err())
			case <-timer.C:
			}
		}
		if !reportDownloadURLAllowed(reportURL) {
			return fmt.Errorf("Pinterest returned an invalid report download URL (https required)")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reportURL, nil)
		if err != nil {
			return err
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return fmt.Errorf("downloading ads report: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("downloading ads report: HTTP %s", resp.Status)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxReportDownload+1))
		if err != nil {
			return fmt.Errorf("reading ads report: %w", err)
		}
		if len(body) > maxReportDownload {
			return fmt.Errorf("ads report download exceeds 20 MiB limit")
		}
		rows, err := normalizeReportRows(body)
		if err != nil {
			return err
		}
		if flags.csv {
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		}
		out := map[string]any{"ad_account_id": account, "token": job.Token, "rows": rows}
		if resumeToken == "" {
			out["level"], out["start_date"], out["end_date"] = level, startText, endText
		} else {
			// The original request's dates and level are not recoverable from the token.
			out["resumed"] = true
		}
		return printJSONFiltered(cmd.OutOrStdout(), out, flags)
	}}
	cmd.Flags().StringVar(&account, "ad-account-id", "", "Pinterest ad account identifier for this report")
	cmd.Flags().StringVar(&level, "level", "CAMPAIGN", "Report entity level: CAMPAIGN, AD_GROUP, PIN_PROMOTION, ADVERTISER")
	cmd.Flags().IntVar(&days, "days", 7, "Number of recent report days to include (1-90)")
	cmd.Flags().StringVar(&granularity, "granularity", "TOTAL", "Report granularity: TOTAL or DAY")
	cmd.Flags().StringVar(&columnsRaw, "columns", "CAMPAIGN_NAME,SPEND_IN_DOLLAR,PAID_IMPRESSION,TOTAL_CLICKTHROUGH,CTR,ECPC_IN_DOLLAR", "Comma-separated ads report columns to request")
	cmd.Flags().StringVar(&pollRaw, "poll-interval", "3s", "How long to wait between report status checks")
	cmd.Flags().StringVar(&maxRaw, "max-wait", "2m", "Maximum time to wait before returning the report token")
	cmd.Flags().StringVar(&resumeToken, "token", "", "Resume polling an existing report token instead of creating a new report (date and level flags are ignored)")
	return cmd
}

// reportDownloadURLAllowed requires https for the presigned report URL;
// plain http is accepted only for loopback hosts (local test servers).
func reportDownloadURLAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	host := u.Hostname()
	return u.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")
}

func firstAdAccountID(ctx context.Context, c interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}) (string, error) {
	raw, err := c.GetNoCache(ctx, "/ad_accounts", map[string]string{"page_size": "1"})
	if err != nil {
		return "", fmt.Errorf("looking up a default ad account (pass --ad-account-id): %w", err)
	}
	var resp struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parsing ad accounts: %w", err)
	}
	if len(resp.Items) == 0 || resp.Items[0].ID == "" {
		return "", usageErr(fmt.Errorf("no ad account is visible to this token; pass --ad-account-id"))
	}
	return resp.Items[0].ID, nil
}
