// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: live timing snapshot that tolerates an empty feed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "live",
		Short: "Fetch the current live-timing feed as an agent-friendly JSON snapshot.",
		Long: "Wraps the raw live-timing-lite endpoint. The feed is only populated during an\n" +
			"active session; between sessions this reports a clean 'no active session' state\n" +
			"instead of erroring.",
		Example:     "  motogp-pp-cli live --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Live timing is real-time only: there is no local copy to read or
			// fall back to, so --data-source local is an explicit unsupported
			// error. (auto/live proceed against the API.) The direct fetch below
			// keeps this command's special empty-body-between-sessions handling.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(cmd.Context(), "/timing-gateway/livetiming-lite", nil)
			if err != nil {
				// The endpoint returns HTTP 200 with an empty body between
				// sessions, which the client surfaces as an error. Treat an
				// empty/no-body feed as a defined "no active session" result.
				if isEmptyFeedErr(err) {
					return flags.printJSON(cmd, liveSnapshot{Active: false, Status: "no active session"})
				}
				return classifyAPIError(err, flags)
			}
			trimmed := bytes.TrimSpace(raw)
			if len(trimmed) == 0 || string(trimmed) == "null" {
				return flags.printJSON(cmd, liveSnapshot{Active: false, Status: "no active session"})
			}
			// Live feed present: emit it verbatim (already JSON) with an active flag.
			var feed json.RawMessage = trimmed
			return flags.printJSON(cmd, liveSnapshot{Active: true, Status: "live", Feed: feed})
		},
	}
	return cmd
}

type liveSnapshot struct {
	Active bool            `json:"active"`
	Status string          `json:"status"`
	Feed   json.RawMessage `json:"feed,omitempty"`
}

func isEmptyFeedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "empty response body") ||
		strings.Contains(msg, "unexpected end of json") ||
		strings.Contains(msg, "no content")
}
