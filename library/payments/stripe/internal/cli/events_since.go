// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newEventsSinceCmd(flags *rootFlags) *cobra.Command {
	var typePattern string
	var limit int
	var profile string

	cmd := &cobra.Command{
		Use:   "events-since [cursor]",
		Short: "Fetch events since cursor; persist new cursor in profile (one-shot, no daemon)",
		Long: `Pull every Stripe event newer than the cursor (or the last persisted
cursor for the named profile when no positional is given). The new cursor is
saved to ~/.config/stripe-pp-cli/cursors/<profile>.json so the next run picks
up where this one left off — a one-shot replacement for stripe-cli's listen
daemon.`,
		Example: `  # First run uses cursor from default profile (or starts fresh)
  stripe-pp-cli events-since --json --limit 50

  # Just charge.* and invoice.* events
  stripe-pp-cli events-since --type charge. --json
  stripe-pp-cli events-since --type invoice. --json

  # Multiple profiles for separate event streams
  stripe-pp-cli events-since --profile billing-pipeline --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			cursor := ""
			if len(args) > 0 {
				cursor = args[0]
			} else {
				saved, err := readCursor(profile)
				if err != nil {
					return configErr(err)
				}
				cursor = saved
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			collected := make([]json.RawMessage, 0, limit)
			lastID := ""
			for {
				params := map[string]string{"limit": strconv.Itoa(min(100, limit-len(collected)))}
				if cursor != "" {
					params["starting_after"] = cursor
				}
				data, err := c.Get("/v1/events", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				items, hasMore, lastSeen := decodeEventsPage(data)
				for _, it := range items {
					if typePattern != "" && !eventMatchesPattern(it, typePattern) {
						continue
					}
					collected = append(collected, it)
					if len(collected) >= limit {
						break
					}
				}
				if lastSeen != "" {
					lastID = lastSeen
					cursor = lastSeen
				}
				if !hasMore || len(collected) >= limit || len(items) == 0 {
					break
				}
			}

			// Persist the new cursor (latest event id seen) before printing.
			if lastID != "" {
				if err := writeCursor(profile, lastID); err != nil {
					return configErr(fmt.Errorf("persisting cursor: %w", err))
				}
				fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"cursor_saved","profile":%q,"cursor":%q}`+"\n",
					nonEmpty(profile, "default"), lastID)
			}

			return printJSONFiltered(cmd.OutOrStdout(), collected, flags)
		},
	}

	cmd.Flags().StringVar(&typePattern, "type", "", "Substring match against event.type (e.g. 'charge.', 'invoice.payment_failed')")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum events to return (default 100)")
	cmd.Flags().StringVar(&profile, "profile", "default", "Cursor namespace; lets multiple consumers track separately")

	return cmd
}

func decodeEventsPage(raw json.RawMessage) ([]json.RawMessage, bool, string) {
	var env struct {
		Data    []json.RawMessage `json:"data"`
		HasMore bool              `json:"has_more"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false, ""
	}
	last := ""
	for _, item := range env.Data {
		if id, ok := jsonGet(item, "id"); ok && id != "" {
			last = id
		}
	}
	return env.Data, env.HasMore, last
}

func eventMatchesPattern(item json.RawMessage, pattern string) bool {
	t, _ := jsonGet(item, "type")
	if strings.Contains(t, pattern) {
		return true
	}
	// Also check data.object.object since the spec says it's substring-matched.
	var ev map[string]json.RawMessage
	if err := json.Unmarshal(item, &ev); err == nil {
		if d, ok := ev["data"]; ok {
			var data map[string]json.RawMessage
			if err := json.Unmarshal(d, &data); err == nil {
				if o, ok := data["object"]; ok {
					if obj, ok := jsonGet(o, "object"); ok && strings.Contains(obj, pattern) {
						return true
					}
				}
			}
		}
	}
	return false
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
