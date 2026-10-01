// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/config"
	"github.com/spf13/cobra"
)

type auditPage struct {
	Events  []json.RawMessage `json:"events"`
	HasMore bool              `json:"hasMore"`
	Cursor  string            `json:"cursor"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Read workspace audit events with a dedicated audit API key",
	}
	cmd.AddCommand(newAuditListCmd(flags))
	return cmd
}

func newAuditListCmd(flags *rootFlags) *cobra.Command {
	var action, occurredBefore, occurredAfter, cursor string
	var pageSize int
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List audit events",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if pageSize <= 0 || pageSize > 30 {
				return usageErr(fmt.Errorf("--page-size must be between 1 and 30"))
			}
			var err error
			if occurredBefore != "" {
				occurredBefore, err = normalizeAuditTimestamp("occurred-before", occurredBefore)
				if err != nil {
					return err
				}
			}
			if occurredAfter != "" {
				occurredAfter, err = normalizeAuditTimestamp("occurred-after", occurredAfter)
				if err != nil {
					return err
				}
			}
			c, err := newAuditClient(flags)
			if err != nil {
				return err
			}
			params := map[string]string{"page_size": fmt.Sprintf("%d", pageSize)}
			if action != "" {
				params["action"] = action
			}
			if occurredBefore != "" {
				params["occurred_before"] = occurredBefore
			}
			if occurredAfter != "" {
				params["occurred_after"] = occurredAfter
			}
			if cursor != "" {
				params["cursor"] = cursor
			}
			if !all {
				raw, err := c.Get("/v1/audit", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			var events []json.RawMessage
			seen := map[string]bool{}
			for {
				raw, err := c.Get("/v1/audit", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page auditPage
				if err := json.Unmarshal(raw, &page); err != nil {
					return fmt.Errorf("decode audit response: %w", err)
				}
				events = append(events, page.Events...)
				if !page.HasMore {
					break
				}
				if page.Cursor == "" || seen[page.Cursor] {
					return fmt.Errorf("audit API returned hasMore with a missing or repeated cursor")
				}
				seen[page.Cursor] = true
				params["cursor"] = page.Cursor
			}
			raw, _ := json.Marshal(events)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "Exact action or dotted action prefix (for example, workspace)")
	cmd.Flags().StringVar(&occurredBefore, "occurred-before", "", "Only events before this date or RFC3339 timestamp")
	cmd.Flags().StringVar(&occurredAfter, "occurred-after", "", "Only events after this date or RFC3339 timestamp")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Cursor returned by the previous page")
	cmd.Flags().IntVar(&pageSize, "page-size", 10, "Events per page (1-30)")
	cmd.Flags().BoolVar(&all, "all", false, "Fetch all pages serially")
	return cmd
}

func normalizeAuditTimestamp(flagName, value string) (string, error) {
	return normalizeAuditTimestampAt(flagName, value, time.Now())
}

func normalizeAuditTimestampAt(flagName, value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	dateOnly := len(value) == len("2006-01-02")
	var (
		parsed time.Time
		err    error
	)
	if dateOnly {
		parsed, err = time.Parse("2006-01-02", value)
	} else {
		parsed, err = time.Parse(time.RFC3339Nano, value)
	}
	if err != nil {
		return "", usageErr(fmt.Errorf("invalid --%s %q: expected YYYY-MM-DD or RFC3339 timestamp", flagName, value))
	}
	cutoff := now.UTC().AddDate(-1, 0, 0)
	if dateOnly {
		year, month, day := cutoff.Date()
		cutoff = time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
	if parsed.Before(cutoff) {
		return "", usageErr(fmt.Errorf("invalid --%s %q: Granola audit events are retained for one year; earliest available timestamp is %s", flagName, value, cutoff.Format(time.RFC3339)))
	}
	return granolaAPITimestamp(parsed), nil
}

func newAuditClient(flags *rootFlags) (*client.Client, error) {
	key := os.Getenv("GRANOLA_AUDIT_API_KEY")
	if key == "" {
		return nil, authErr(fmt.Errorf("GRANOLA_AUDIT_API_KEY is required for the audit API; audit keys are separate from regular Granola API keys"))
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	cfg.GranolaApiKey = ""
	cfg.AccessToken = ""
	cfg.AuthHeaderVal = "Bearer " + key
	cfg.AuthSource = "env:GRANOLA_AUDIT_API_KEY"
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.DryRun = flags.dryRun
	// Audit payloads can contain sensitive workspace activity and should not
	// be written to the generated GET cache (which is shared and mode 0644).
	c.NoCache = true
	return c, nil
}
