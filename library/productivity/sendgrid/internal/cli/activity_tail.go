// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/sendgrid/internal/store"
	"github.com/spf13/cobra"
)

func newActivityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "activity",
		Short:  "Email activity streaming and search",
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newActivityTailCmd(flags))
	return cmd
}

func newActivityTailCmd(flags *rootFlags) *cobra.Command {
	var flagFollow bool
	var flagRateAware bool
	var flagFilters []string

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream Email Activity events with rate-aware polling",
		Long: `Polls GET /v3/messages repeatedly, deduplicates by msg_id, and upserts each
new event into the local SQLite store under email_activity. Respects SendGrid's
6/min Email Activity cap by sleeping 10s between polls when --rate-aware is set
(default: true). Use --filter key:value (repeatable) to post-fetch filter events.
Without --follow, does one pass and exits.`,
		Example:     "  sendgrid-pp-cli activity tail --follow --filter status:bounce",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("sendgrid-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer func() { _ = db.Close() }()

			seen := map[string]bool{}
			filters := parseActivityFilters(flagFilters)

			for {
				// Filter by msg_id dedup only; SendGrid's last_event_time DSL is
				// unreliable across regions, so we lean on the seen-map for
				// incremental polling — both first and subsequent polls issue the
				// same request shape and dedup via `seen`.
				params := map[string]string{"limit": "100"}
				data, err := c.Get("/v3/messages", params)
				if err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warn: poll error: %v\n", err)
				} else {
					newEvents := extractNewMessages(data, seen, filters)

					for _, ev := range newEvents {
						msgID, _ := ev["msg_id"].(string)
						if msgID == "" {
							msgID, _ = ev["id"].(string)
						}
						if msgID != "" {
							seen[msgID] = true
							raw, _ := json.Marshal(ev)
							_ = db.Upsert("email_activity", msgID, raw)
						}

						if flags.asJSON {
							raw, _ := json.Marshal(ev)
							_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(raw))
						} else {
							printActivityEvent(cmd, ev)
						}
					}
				}

				if !flagFollow {
					break
				}

				if flagRateAware {
					select {
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					case <-time.After(10 * time.Second):
					}
				} else {
					select {
					case <-cmd.Context().Done():
						return cmd.Context().Err()
					case <-time.After(2 * time.Second):
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flagFollow, "follow", false, "Keep polling for new events (Ctrl+C to stop)")
	cmd.Flags().BoolVar(&flagRateAware, "rate-aware", true, "Sleep 10s between polls to respect SendGrid's 6/min Email Activity cap")
	cmd.Flags().StringArrayVar(&flagFilters, "filter", nil, "Post-fetch filter in key:value format (repeatable; e.g. --filter status:bounce)")

	return cmd
}

func parseActivityFilters(filterStrs []string) map[string]string {
	filters := map[string]string{}
	for _, f := range filterStrs {
		k, v, ok := strings.Cut(f, ":")
		if ok && k != "" {
			filters[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return filters
}

func extractNewMessages(data json.RawMessage, seen map[string]bool, filters map[string]string) []map[string]any {
	// Messages API returns {"messages":[...]}
	var envelope map[string]json.RawMessage
	var msgs []map[string]any

	if err := json.Unmarshal(data, &envelope); err == nil {
		if msgsRaw, ok := envelope["messages"]; ok {
			_ = json.Unmarshal(msgsRaw, &msgs)
		}
	}
	if msgs == nil {
		_ = json.Unmarshal(data, &msgs)
	}

	var out []map[string]any
	for _, msg := range msgs {
		msgID, _ := msg["msg_id"].(string)
		if msgID == "" {
			msgID, _ = msg["id"].(string)
		}
		if seen[msgID] {
			continue
		}
		if !matchesFilters(msg, filters) {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func matchesFilters(msg map[string]any, filters map[string]string) bool {
	for k, v := range filters {
		field, ok := msg[k]
		if !ok {
			return false
		}
		fieldStr := fmt.Sprintf("%v", field)
		if !strings.EqualFold(fieldStr, v) {
			return false
		}
	}
	return true
}

func printActivityEvent(cmd *cobra.Command, ev map[string]any) {
	msgID, _ := ev["msg_id"].(string)
	status, _ := ev["status"].(string)
	toEmail, _ := ev["to_email"].(string)
	fromEmail, _ := ev["from_email"].(string)
	subject, _ := ev["subject"].(string)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s -> %s | %s | %s\n",
		status, fromEmail, toEmail, subject, msgID)
}
