// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

func newWebhookAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var probe bool
	var concurrency int

	cmd := &cobra.Command{
		Use:   "webhook-audit",
		Short: "Group your IncomingPhoneNumbers by Voice/SMS webhook URL to find single-use URLs that may be orphans",
		Long: `Local groupby over the synced incoming_phone_numbers_json table. Buckets
each number's voice_url and sms_url; URLs that appear on only one number are
candidate orphans (often left behind after a redeploy or refactor).

With --probe, sends a HEAD request to each unique URL to confirm reachability.
HEAD probes obey PRINTING_PRESS_VERIFY=1 (verifier short-circuit) so the audit
won't fan out HEAD requests during shipcheck.

Run 'twilio-pp-cli sync --resources incoming-phone-numbers' first.`,
		Example: `  twilio-pp-cli webhook-audit --json
  twilio-pp-cli webhook-audit --probe --concurrency 4 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("twilio-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			query := `
				WITH urls AS (
					SELECT
						json_extract(data, '$.phone_number') AS phone_number,
						json_extract(data, '$.friendly_name') AS friendly_name,
						json_extract(data, '$.voice_url') AS voice_url,
						json_extract(data, '$.sms_url') AS sms_url
					FROM incoming_phone_numbers_json
				)
				SELECT
					COALESCE(voice_url, '') AS url,
					'voice' AS kind,
					COUNT(*) AS uses,
					GROUP_CONCAT(phone_number) AS phone_numbers
				FROM urls
				WHERE COALESCE(voice_url, '') != ''
				GROUP BY voice_url
				UNION ALL
				SELECT
					COALESCE(sms_url, '') AS url,
					'sms' AS kind,
					COUNT(*) AS uses,
					GROUP_CONCAT(phone_number) AS phone_numbers
				FROM urls
				WHERE COALESCE(sms_url, '') != ''
				GROUP BY sms_url
				ORDER BY uses ASC, kind, url`

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var hooks []webhookEntry
			for rows.Next() {
				var url, kind, phoneNums string
				var uses int
				if err := rows.Scan(&url, &kind, &uses, &phoneNums); err != nil {
					return err
				}
				hooks = append(hooks, webhookEntry{
					URL:          url,
					Kind:         kind,
					Uses:         uses,
					PhoneNumbers: strings.Split(phoneNums, ","),
					LikelyOrphan: uses == 1,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if probe && !cliutil.IsVerifyEnv() {
				if concurrency < 1 {
					concurrency = 4
				}
				probeWebhooks(cmd.Context(), hooks, concurrency)
			}

			envelope := map[string]any{
				"unique_urls":   len(hooks),
				"likely_orphan": countOrphans(hooks),
				"webhooks":      hooks,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&probe, "probe", false, "Send a HEAD request to each unique URL to confirm reachability (off by default)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel HEAD requests during --probe")
	return cmd
}

type webhookEntry struct {
	URL          string   `json:"url"`
	Kind         string   `json:"kind"`
	Uses         int      `json:"uses"`
	PhoneNumbers []string `json:"phone_numbers"`
	LikelyOrphan bool     `json:"likely_orphan"`
	ProbeStatus  int      `json:"probe_status,omitempty"`
	ProbeError   string   `json:"probe_error,omitempty"`
}

func countOrphans(hooks []webhookEntry) int {
	n := 0
	for _, h := range hooks {
		if h.LikelyOrphan {
			n++
		}
	}
	return n
}

func probeWebhooks(ctx context.Context, hooks []webhookEntry, concurrency int) {
	client := &http.Client{Timeout: 10 * time.Second}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range hooks {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			req, err := http.NewRequestWithContext(ctx, "HEAD", hooks[i].URL, nil)
			if err != nil {
				hooks[i].ProbeError = err.Error()
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				hooks[i].ProbeError = err.Error()
				return
			}
			defer resp.Body.Close()
			hooks[i].ProbeStatus = resp.StatusCode
		}(i)
	}
	wg.Wait()
}
