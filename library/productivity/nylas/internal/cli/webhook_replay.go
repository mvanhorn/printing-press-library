// Copyright 2026 Nathan Kettles and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
	"github.com/spf13/cobra"
)

// replayResult is one delivery attempt's outcome, reported as JSON.
type replayResult struct {
	WebhookID  string `json:"webhook_id"`
	Trigger    string `json:"trigger,omitempty"`
	SyncedAt   string `json:"synced_at"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Error      string `json:"error,omitempty"`
}

func newWebhookReplayCmd(flags *rootFlags) *cobra.Command {
	var since string
	var trigger string
	var toURL string
	var dbPath string
	var limit int
	var verify bool
	var allowRemote bool

	cmd := &cobra.Command{
		Use:   "webhook-replay",
		Short: "Re-fire persisted webhook deliveries from the local store",
		Long: `Re-fire any webhook delivery from the local mirror into a local
handler URL. Use this to reproduce a webhook payload in your handler
without waiting for the next live event.

Default behaviour prints the deliveries that WOULD fire and exits without
sending; pass --confirm to actually POST. This matches the safe-by-default
pattern used by every other side-effecting command in this CLI.`,
		Example: strings.Trim(`
  # Preview last 24 hours of webhook deliveries
  nylas-pp-cli webhook-replay --since 24h --to http://localhost:3000/hook

  # Actually replay (post to the URL)
  nylas-pp-cli webhook-replay --since 24h --to http://localhost:3000/hook --confirm

  # Only message.created events
  nylas-pp-cli webhook-replay --trigger message.created --to http://localhost:3000/hook --confirm
`, "\n"),
		Annotations: map[string]string{}, // not read-only; do NOT auto-mark
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would replay: webhook deliveries (verify mode, no IO)")
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			if toURL == "" {
				return fmt.Errorf("--to is required (e.g. http://localhost:3000/hook)")
			}
			parsed, err := url.Parse(toURL)
			if err != nil {
				return fmt.Errorf("invalid --to URL: %w", err)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				return fmt.Errorf("--to scheme must be http or https (got %q)", parsed.Scheme)
			}
			if !allowRemote && !isLoopbackHost(parsed.Hostname()) {
				return fmt.Errorf("--to host %q is not a loopback address; webhook-replay refuses non-loopback targets by default. Pass --allow-remote to deliver to %q", parsed.Hostname(), parsed.Hostname())
			}
			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			where := []string{}
			params := []any{}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				cutoff := ts.UTC().Format("2006-01-02 15:04:05")
				where = append(where, "synced_at >= ?")
				params = append(params, cutoff)
			}
			if trigger != "" {
				where = append(where, "json_extract(data,'$.trigger_types') LIKE ?")
				params = append(params, "%"+trigger+"%")
			}
			whereSQL := ""
			if len(where) > 0 {
				whereSQL = " WHERE " + strings.Join(where, " AND ")
			}

			q := `SELECT id, COALESCE(json_extract(data,'$.trigger_types'),'') AS trig, synced_at, data FROM webhooks` + whereSQL + ` ORDER BY synced_at DESC`
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), q, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			results := make([]replayResult, 0, 16)
			client := &http.Client{Timeout: 10 * time.Second}
			for rows.Next() {
				var id, trig, syncedAt string
				var data json.RawMessage
				if err := rows.Scan(&id, &trig, &syncedAt, &data); err != nil {
					continue
				}
				r := replayResult{WebhookID: id, Trigger: trig, SyncedAt: syncedAt}
				if !verify {
					r.Status = "preview"
					results = append(results, r)
					continue
				}
				req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, toURL, bytes.NewReader(data))
				if err != nil {
					r.Status = "error"
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Nylas-Webhook-Replay", "1")
				if trig != "" {
					req.Header.Set("X-Nylas-Trigger", trig)
				}
				resp, err := client.Do(req)
				if err != nil {
					r.Status = "error"
					r.Error = err.Error()
					results = append(results, r)
					continue
				}
				r.HTTPStatus = resp.StatusCode
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					r.Status = "delivered"
				} else {
					r.Status = "rejected"
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating webhooks: %w", err)
			}
			if !verify && !flags.asJSON {
				fmt.Fprintf(cmd.ErrOrStderr(), "preview only: would replay %d deliveries; pass --confirm to actually POST.\n", len(results))
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Restrict to webhook deliveries synced within this duration")
	cmd.Flags().StringVar(&trigger, "trigger", "", "Filter to deliveries whose trigger matches this substring (e.g. message.created)")
	cmd.Flags().StringVar(&toURL, "to", "", "Local handler URL to POST each replayed delivery (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum deliveries to replay (0 = no cap)")
	cmd.Flags().BoolVar(&verify, "confirm", false, "Actually POST. Without this flag, prints a preview and does nothing.")
	cmd.Flags().BoolVar(&allowRemote, "allow-remote", false, "Permit --to to point at a non-loopback host. Default refuses anything but loopback to prevent SSRF.")
	return cmd
}

// isLoopbackHost reports whether the given hostname resolves only to
// loopback addresses, or is a bare loopback literal (127.0.0.0/8, ::1,
// "localhost"). Empty hostnames are treated as non-loopback so a missing
// host in the URL is rejected by the caller.
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
