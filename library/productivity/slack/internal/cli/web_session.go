// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/spf13/cobra"
)

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

	raw, err := client.PostWebSession(cmd.Context(), path, body, flags.timeout)
	if err != nil {
		var wsErr *client.WebSessionError
		if errors.As(err, &wsErr) {
			switch wsErr.Kind {
			case client.WebSessionErrAuth:
				return authErr(wsErr)
			case client.WebSessionErrRateLimit:
				return rateLimitErr(wsErr)
			default:
				return apiErr(wsErr)
			}
		}
		return apiErr(err)
	}
	return emitSlackReadJSON(cmd, flags, raw)
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
