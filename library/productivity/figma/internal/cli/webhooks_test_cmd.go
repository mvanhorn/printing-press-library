// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/figma/internal/store"
	"github.com/spf13/cobra"
)

func newWebhooksTestCmd(flags *rootFlags) *cobra.Command {
	var replayFailed bool
	var targetURL, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "test <id>",
		Short: "Replay stored webhook deliveries against a target URL.",
		Long: `Pull cached webhook delivery payloads (or fetch live when the cache is empty)
and POST each one to --target-url. Print-by-default: if --target-url is empty, only
the replay plan is printed.`,
		Example: strings.Trim(`
  # Print plan only — no HTTP fired
  figma-pp-cli webhooks test 9876

  # Replay last 10 deliveries to a local handler
  figma-pp-cli webhooks test 9876 --target-url http://localhost:8080/figma-events

  # Only replay failed deliveries
  figma-pp-cli webhooks test 9876 --replay-failed --target-url https://example.test/handler
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "webhooks test"}`)
				return nil
			}
			id := args[0]

			// Side-effect floor: short-circuit during verify before any I/O.
			// loadDeliveries can make a live API call when the cache misses,
			// and the replay loop itself dials --target-url; both must be
			// suppressed when running under the printing-press verifier.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"plan_only":  true,
					"reason":     "verify env",
					"webhook_id": id,
				}, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("figma-pp-cli")
			}
			deliveries, src, err := loadDeliveries(cmd, flags, id, dbPath)
			if err != nil {
				return err
			}
			if replayFailed {
				deliveries = filterFailed(deliveries)
			}
			if limit > 0 && len(deliveries) > limit {
				deliveries = deliveries[:limit]
			}

			plan := []map[string]any{}
			for _, d := range deliveries {
				plan = append(plan, map[string]any{
					"id":     d.ID,
					"status": d.Status,
					"sent":   d.SentAt,
				})
			}

			if strings.TrimSpace(targetURL) == "" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"plan_only": true,
					"reason":    "no --target-url provided",
					"source":    src,
					"plan":      plan,
					"hint":      "pass --target-url <url> to actually replay",
				}, flags)
			}

			results := []map[string]any{}
			httpClient := &http.Client{Timeout: 30 * time.Second}
			for _, d := range deliveries {
				req, rerr := http.NewRequestWithContext(cmd.Context(), "POST", targetURL, bytes.NewReader(d.Body))
				if rerr != nil {
					results = append(results, map[string]any{"id": d.ID, "error": rerr.Error()})
					continue
				}
				for k, v := range d.Headers {
					if strings.EqualFold(k, "Host") {
						continue
					}
					req.Header.Set(k, v)
				}
				if req.Header.Get("Content-Type") == "" {
					req.Header.Set("Content-Type", "application/json")
				}
				resp, herr := httpClient.Do(req)
				row := map[string]any{"id": d.ID, "warning": "replayed without re-signing X-Figma-Webhook-Signature — your handler must accept unsigned replays for testing"}
				if herr != nil {
					row["error"] = herr.Error()
				} else {
					row["status"] = resp.StatusCode
					_ = resp.Body.Close()
				}
				results = append(results, row)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"target_url": targetURL,
				"source":     src,
				"replayed":   len(results),
				"results":    results,
			}, flags)
		},
	}

	cmd.Flags().BoolVar(&replayFailed, "replay-failed", false, "Only replay deliveries with status >= 400")
	cmd.Flags().StringVar(&targetURL, "target-url", "", "Where to POST each replayed delivery")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum deliveries to replay")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/figma-pp-cli/data.db)")

	return cmd
}

type delivery struct {
	ID      string
	Status  int
	SentAt  string
	Body    []byte
	Headers map[string]string
}

// loadDeliveries reads from the local store first; falls back to live fetch
// when nothing is cached.
func loadDeliveries(cmd *cobra.Command, flags *rootFlags, webhookID, dbPath string) ([]delivery, string, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err == nil {
		defer db.Close()
		rows, qerr := db.DB().QueryContext(cmd.Context(),
			`SELECT id, data FROM requests WHERE webhooks_id = ? ORDER BY synced_at DESC`, webhookID)
		if qerr == nil {
			defer rows.Close()
			out := []delivery{}
			for rows.Next() {
				var id string
				var data []byte
				if rows.Scan(&id, &data) != nil {
					continue
				}
				out = append(out, parseDelivery(id, data))
			}
			if len(out) > 0 {
				return out, "local", nil
			}
		}
	}

	// Live fallback.
	c, cerr := flags.newClient()
	if cerr != nil {
		return nil, "", cerr
	}
	raw, derr := c.Get("/v2/webhooks/"+webhookID+"/requests", nil)
	if derr != nil {
		return nil, "", classifyAPIError(derr, flags)
	}
	var env struct {
		Requests []map[string]any `json:"requests"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, "live", fmt.Errorf("parsing webhook requests: %w", err)
	}
	out := []delivery{}
	for _, r := range env.Requests {
		jb, _ := json.Marshal(r)
		id, _ := r["id"].(string)
		out = append(out, parseDelivery(id, jb))
	}
	return out, "live", nil
}

// parseDelivery extracts our minimal delivery shape from a request_log row.
// Figma's payload nests the original request under request_info.body.
func parseDelivery(id string, raw []byte) delivery {
	d := delivery{ID: id, Headers: map[string]string{}}
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	if id == "" {
		if v, ok := obj["id"].(string); ok {
			d.ID = v
		}
	}
	if ri, ok := obj["request_info"].(map[string]any); ok {
		if hh, ok := ri["headers"].(map[string]any); ok {
			for k, v := range hh {
				if s, ok := v.(string); ok {
					d.Headers[k] = s
				}
			}
		}
		switch body := ri["body"].(type) {
		case string:
			d.Body = []byte(body)
		case map[string]any:
			b, _ := json.Marshal(body)
			d.Body = b
		}
	}
	if rsp, ok := obj["response_info"].(map[string]any); ok {
		if s, ok := rsp["status"].(float64); ok {
			d.Status = int(s)
		}
	}
	if v, ok := obj["sent_at"].(string); ok {
		d.SentAt = v
	}
	if len(d.Body) == 0 {
		// Fallback: re-serialize the whole row so the replay carries something.
		d.Body = raw
	}
	_ = io.EOF
	return d
}

func filterFailed(in []delivery) []delivery {
	out := []delivery{}
	for _, d := range in {
		// Status == 0 is the zero value parseDelivery returns when the
		// response_info block was absent or unparseable. Treat unparsed
		// deliveries as failures rather than silently excluding them so
		// --replay-failed doesn't quietly drop real failure records.
		if d.Status == 0 || d.Status >= 400 {
			out = append(out, d)
		}
	}
	return out
}
