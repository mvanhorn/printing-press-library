// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// messages_query.go implements `superhuman-pp-cli messages query "<text>"`.
// Mirrors the email half of the Superhuman MCP's
// query_email_and_calendar tool. Calendar is out of scope for v1.1
// (plan 2026-05-14-003 Scope Boundaries -> deferred to v1.2).
//
// Implementation-time unknown (plan 2026-05-14-003 U12): the exact
// `scope` parameter name. The hypothesis (chosen here) is `scope: "email"`
// to constrain the underlying ai.askAIProxy call away from calendar
// results. If a bundle probe shows a different filter shape (e.g.,
// `filters: {include_calendar: false}`), swap the body in queryBody —
// the command's output shape stays the same.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newMessagesQueryCmd registers `messages query <text>`.
func newMessagesQueryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "query <text>",
		Short: "Semantic email search (no calendar)",
		Long: `Semantic email search via Superhuman's /v3/ai.askAIProxy
endpoint with an email-only scope. Calendar results are intentionally
excluded — for joint email+calendar search, use the 'ai' command.

The query argument may be a multi-word natural-language phrase; wrap
in quotes so the shell doesn't split it.`,
		Example: `  superhuman-pp-cli messages query "invoices from last month"
  superhuman-pp-cli messages query "what did Alice say about the Q2 plan?" --limit 5`,
		Annotations: map[string]string{
			"pp:endpoint":   "messages.query",
			"pp:method":     "POST",
			"pp:path":       "/v3/ai.askAIProxy",
			"mcp:read-only": "true",
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("messages query: requires exactly one <text> argument (wrap multi-word queries in quotes)"))
			}
			if args[0] == "" {
				return usageErr(fmt.Errorf("messages query: <text> cannot be empty"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessagesQuery(cmd, flags, args[0], limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results to return")
	return cmd
}

// queryBody returns the ai.askAIProxy body for an email-only search.
// Centralized so a bundle-driven scope rename is a one-liner.
func queryBody(text string, limit int) map[string]any {
	return map[string]any{
		"query": text,
		"scope": "email",
		"limit": limit,
	}
}

func runMessagesQuery(cmd *cobra.Command, flags *rootFlags, text string, limit int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	body := queryBody(text, limit)
	data, statusCode, err := c.Post("/v3/ai.askAIProxy", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		if flags.quiet {
			return nil
		}
		var parsed any
		_ = json.Unmarshal(data, &parsed)
		envelope := map[string]any{
			"action":   "messages.query",
			"resource": "messages",
			"path":     "/v3/ai.askAIProxy",
			"query":    text,
			"limit":    limit,
			"status":   statusCode,
			"success":  statusCode >= 200 && statusCode < 300,
			"data":     parsed,
		}
		envelopeJSON, jerr := json.Marshal(envelope)
		if jerr != nil {
			return jerr
		}
		return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
	}

	// Human: best-effort row rendering, fall through to raw JSON if the
	// response shape doesn't match the expected ranked-results array.
	data = extractResponseData(data)
	var items []map[string]any
	if jerr := json.Unmarshal(data, &items); jerr == nil && len(items) > 0 {
		if perr := printAutoTable(cmd.OutOrStdout(), items); perr == nil {
			return nil
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "No matches.")
	return nil
}
