// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// slackWebCookieHeader resolves the Slack `d` session cookie without ever
// printing or persisting cookie values. Order:
//  1. SLACK_COOKIE_D env var (either bare xoxd-... value or "d=..." pair)
//  2. `agentcookie cookies --domain .slack.com` CLI if installed on PATH
func slackWebCookieHeader(ctx context.Context) (string, error) {
	if raw := strings.TrimSpace(os.Getenv("SLACK_COOKIE_D")); raw != "" {
		if strings.HasPrefix(raw, "d=") {
			return raw, nil
		}
		return "d=" + raw, nil
	}
	bin, err := exec.LookPath("agentcookie")
	if err != nil {
		return "", authErr(fmt.Errorf("missing Slack session cookie: set SLACK_COOKIE_D or install agentcookie with a logged-in .slack.com session"))
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, bin, "cookies", "--domain", ".slack.com").Output()
	if err != nil {
		return "", authErr(fmt.Errorf("failed to resolve .slack.com cookies via agentcookie: %w", err))
	}
	header := strings.TrimSpace(string(out))
	if header == "" {
		return "", authErr(fmt.Errorf("agentcookie returned no active .slack.com cookies; sign into Slack in Chrome or set SLACK_COOKIE_D"))
	}
	return header, nil
}

func slackWebSessionToken() (string, error) {
	tok := strings.TrimSpace(os.Getenv("SLACK_SESSION_TOKEN"))
	if tok == "" {
		return "", authErr(fmt.Errorf("SLACK_SESSION_TOKEN (xoxc-...) is required for web-client internal endpoints"))
	}
	return tok, nil
}

func slackWebBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("SLACK_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://slack.com/api"
}

// runSlackWebClientPost calls an internal Slack desktop/web RPC method using
// xoxc session token + d cookie authentication. These endpoints reject OAuth
// xoxb-/xoxp- tokens with `not_allowed_token_type`.
func runSlackWebClientPost(cmd *cobra.Command, flags *rootFlags, path string, body map[string]any) error {
	if flags.dataSource == "local" {
		return notFoundErr(fmt.Errorf("%s requires live Slack web-client access", path))
	}
	if flags.dryRun {
		return emitSlackReadJSON(cmd, flags, json.RawMessage(fmt.Sprintf(`{"dry_run":true,"method":"POST","path":%q,"auth":"xoxc+cookie"}`, path)))
	}

	token, err := slackWebSessionToken()
	if err != nil {
		return err
	}
	cookieHeader, err := slackWebCookieHeader(cmd.Context())
	if err != nil {
		return err
	}

	if body == nil {
		body = map[string]any{}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	timeout := flags.timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, slackWebBaseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return apiErr(fmt.Errorf("slack web-client %s: %w", path, err))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return apiErr(fmt.Errorf("reading slack web-client %s response: %w", path, err))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return authErr(fmt.Errorf("slack web-client %s returned HTTP %d", path, resp.StatusCode))
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return rateLimitErr(fmt.Errorf("slack web-client %s rate limited (HTTP 429)", path))
	}
	if resp.StatusCode >= 400 {
		return apiErr(fmt.Errorf("slack web-client %s returned HTTP %d", path, resp.StatusCode))
	}
	if slackErr := checkSlackAPIError(json.RawMessage(raw)); slackErr != nil {
		return slackErr
	}
	return emitSlackReadJSON(cmd, flags, json.RawMessage(raw))
}

func newWebClientCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web-client",
		Short: "Read-only Slack desktop/web internal endpoints (requires SLACK_SESSION_TOKEN xoxc + d cookie/agentcookie)",
		Long: `Calls read-only Slack web/desktop client RPC endpoints discovered from the Slack
client bundle (client.counts, saved.list, drafts.list, activity.feed,
subscriptions.thread.getView).

These endpoints require a browser/desktop xoxc session token (SLACK_SESSION_TOKEN)
paired with Slack's d session cookie (SLACK_COOKIE_D, or resolved dynamically via
agentcookie cookies --domain .slack.com). Standard xoxb-/xoxp- OAuth tokens are
rejected by Slack with not_allowed_token_type.`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2,4"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebClientUnreadsCmd(flags))
	cmd.AddCommand(newWebClientLaterCmd(flags))
	cmd.AddCommand(newWebClientDraftsCmd(flags))
	cmd.AddCommand(newWebClientActivityCmd(flags))
	cmd.AddCommand(newWebClientThreadsCmd(flags))
	return cmd
}

func newWebClientUnreadsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "unreads",
		Short:       "Fetch live channel, DM, and thread unread counts (POST /client.counts)",
		Example:     "  slack-pp-cli web-client unreads --json",
		Annotations: map[string]string{"pp:endpoint": "web_client.unreads", "pp:method": "POST", "pp:path": "/client.counts", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSlackWebClientPost(cmd, flags, "/client.counts", map[string]any{
				"thread_counts_by_channel": true,
				"org_wide_aware":           true,
			})
		},
	}
}

func newWebClientLaterCmd(flags *rootFlags) *cobra.Command {
	var filter string
	var limit int
	cmd := &cobra.Command{
		Use:         "later",
		Short:       "List Slack Later / saved items (POST /saved.list)",
		Example:     "  slack-pp-cli web-client later --filter saved --limit 25 --json",
		Annotations: map[string]string{"pp:endpoint": "web_client.later", "pp:method": "POST", "pp:path": "/saved.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if strings.TrimSpace(filter) != "" {
				body["filter"] = strings.TrimSpace(filter)
			}
			if limit > 0 {
				body["limit"] = limit
			}
			return runSlackWebClientPost(cmd, flags, "/saved.list", body)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "saved", "Saved items filter (saved, in_progress, completed)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum saved items to return")
	return cmd
}

func newWebClientDraftsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "drafts",
		Short:       "List unsent message drafts (POST /drafts.list)",
		Example:     "  slack-pp-cli web-client drafts --json",
		Annotations: map[string]string{"pp:endpoint": "web_client.drafts", "pp:method": "POST", "pp:path": "/drafts.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if limit > 0 {
				body["limit"] = limit
			}
			return runSlackWebClientPost(cmd, flags, "/drafts.list", body)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum drafts to return")
	return cmd
}

func newWebClientActivityCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "activity",
		Short:       "Read the Slack Activity feed (mentions, reactions, replies) via POST /activity.feed",
		Example:     "  slack-pp-cli web-client activity --limit 25 --json",
		Annotations: map[string]string{"pp:endpoint": "web_client.activity", "pp:method": "POST", "pp:path": "/activity.feed", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if limit > 0 {
				body["limit"] = limit
			}
			return runSlackWebClientPost(cmd, flags, "/activity.feed", body)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum activity items to return")
	return cmd
}

func newWebClientThreadsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "threads",
		Short:       "Read subscribed threads view (POST /subscriptions.thread.getView)",
		Example:     "  slack-pp-cli web-client threads --limit 20 --json",
		Annotations: map[string]string{"pp:endpoint": "web_client.threads", "pp:method": "POST", "pp:path": "/subscriptions.thread.getView", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if limit > 0 {
				body["limit"] = limit
			}
			return runSlackWebClientPost(cmd, flags, "/subscriptions.thread.getView", body)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum subscribed threads to return")
	return cmd
}
