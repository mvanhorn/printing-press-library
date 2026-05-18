// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// newWebhooksTailCmd polls the webhook message inbox at a fixed interval and
// emits new messages as JSON to stdout. Used to confirm webhook delivery
// during local development without spinning up a public tunnel.
func newWebhooksTailCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var once bool
	var follow bool
	cmd := &cobra.Command{
		Use:         "tail",
		Short:       "Poll the webhook inbox and emit new messages as JSON",
		Example:     "  tella-pp-cli webhooks tail --once --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			// Initial snapshot of the inbox.
			data, err := c.Get("/v1/webhooks/messages", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			seen := map[string]bool{}
			messages := extractMessageObjects(data)
			for _, m := range messages {
				if id, ok := m["id"].(string); ok {
					seen[id] = true
				}
			}
			if once {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"messages": messages,
					"count":    len(messages),
				}, flags)
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			enc := json.NewEncoder(cmd.OutOrStdout())
			// Emit initial snapshot.
			for _, m := range messages {
				_ = enc.Encode(m)
			}
			// --follow=false: NDJSON single-shot. Same exit contract as
			// tail.go's --follow guard; --once already covered the
			// envelope-shaped single shot above.
			if !follow {
				return nil
			}
			for {
				select {
				case <-sig:
					return nil
				case <-ticker.C:
					data, err := c.Get("/v1/webhooks/messages", nil)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
						continue
					}
					for _, m := range extractMessageObjects(data) {
						id, _ := m["id"].(string)
						if id == "" || seen[id] {
							continue
						}
						seen[id] = true
						_ = enc.Encode(m)
					}
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Polling interval")
	cmd.Flags().BoolVar(&once, "once", false, "Take a single inbox snapshot and exit")
	cmd.Flags().BoolVar(&follow, "follow", true, "Keep running and stream new messages (set --follow=false to emit the initial snapshot as NDJSON and exit)")
	return cmd
}

// newWebhooksReplayCmd fetches a stored webhook message and POSTs it to a
// chosen URL, signing with the endpoint secret using HMAC-SHA256. With
// --dry-run (the default unless --to is set), the planned POST is printed
// without sending.
func newWebhooksReplayCmd(flags *rootFlags) *cobra.Command {
	var to string
	var endpoint string
	cmd := &cobra.Command{
		Use:     "replay <msg-id>",
		Short:   "Replay a stored webhook message to a target URL with HMAC signature",
		Example: "  tella-pp-cli webhooks replay msg_abc --to http://localhost:8080/webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				return usageErr(fmt.Errorf("missing required positional argument"))
			}
			if dryRunOK(flags) {
				return nil
			}
			msgID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err := c.Get(fmt.Sprintf("/v1/webhooks/messages/%s", msgID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			signature := ""
			if endpoint != "" {
				// Surface secret-fetch failure instead of silently
				// firing an unsigned POST. A receiving server with
				// HMAC verification will reject an unsigned replay,
				// and without this error the CLI would otherwise
				// report success (status from the upstream rejection)
				// with no indication that signing was attempted and
				// failed. classifyAPIError preserves 401/404/network
				// distinctions for the user.
				secret, sErr := fetchEndpointSecret(c, endpoint)
				if sErr != nil {
					return classifyAPIError(fmt.Errorf("fetching endpoint secret for %q: %w", endpoint, sErr), flags)
				}
				if secret == "" {
					return apiErr(fmt.Errorf("endpoint %q returned an empty secret; cannot sign replay", endpoint))
				}
				signature = signHMAC(secret, body)
			}
			plan := map[string]any{
				"message_id":    msgID,
				"target":        to,
				"endpoint":      endpoint,
				"has_signature": signature != "",
				"body_size":     len(body),
				"dry_run":       to == "",
			}
			if to == "" {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, to, bytes.NewReader(body))
			if err != nil {
				return apiErr(err)
			}
			req.Header.Set("Content-Type", "application/json")
			if signature != "" {
				req.Header.Set("X-Tella-Signature", "sha256="+signature)
			}
			httpClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := httpClient.Do(req)
			if err != nil {
				return apiErr(err)
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			plan["status"] = resp.StatusCode
			plan["response_size"] = len(respBody)
			plan["dry_run"] = false
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Destination URL to POST the message body to (omit to print plan only)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Webhook endpoint ID to fetch HMAC secret from")
	return cmd
}

// fetchEndpointSecret resolves the HMAC signing secret for a webhook endpoint.
// The Tella endpoint returns either {secret: "..."} or a raw string.
func fetchEndpointSecret(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, endpointID string) (string, error) {
	data, err := c.Get(fmt.Sprintf("/v1/webhooks/endpoints/%s/secret", endpointID), nil)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, k := range []string{"secret", "signing_secret", "signingSecret"} {
			if s, ok := obj[k].(string); ok && s != "" {
				return s, nil
			}
		}
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil && s != "" {
		return s, nil
	}
	return "", nil
}

func signHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
