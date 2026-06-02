// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: weekly call digest grouped by deal.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type digestCall struct {
	ConversationID string `json:"conversationId"`
	Title          string `json:"title,omitempty"`
	StartTime      string `json:"startTime,omitempty"`
	Type           string `json:"type,omitempty"`
	Participants   int    `json:"participants"`
}

type digestGroup struct {
	DealID    string       `json:"dealId,omitempty"`
	DealName  string       `json:"dealName"`
	CallCount int          `json:"callCount"`
	Calls     []digestCall `json:"calls"`
	NextSteps []string     `json:"nextSteps,omitempty"`
	latest    time.Time
}

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var callType string
	var owner string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Pull every call in a window, grouped by deal, with next steps per deal.",
		Long: `Group every synced conversation in a time window by the deal it is linked to,
so you get a pipeline-shaped review instead of a flat call list. Next steps and
key takeaways are included when conversation detail has been synced (they live
in the conversation detail, not the list); otherwise each call is listed with
its title, time, and participant count.

Run 'sync' first. Calls not linked to a deal are grouped under "(no linked
deal)".`,
		Example: strings.Trim(`
  # This week's calls, grouped by deal
  sybill-pp-cli digest --since 7d

  # External calls only, as JSON for an agent
  sybill-pp-cli digest --since 7d --type external --agent

  # Last 24 hours
  sybill-pp-cli digest --since 24h
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := cmd.OutOrStdout()
			now := time.Now().UTC()
			cutoff, err := parseSince(since, now)
			if err != nil {
				return err
			}
			wantType := strings.ToUpper(strings.TrimSpace(callType))
			if wantType != "" && wantType != "INTERNAL" && wantType != "EXTERNAL" {
				return fmt.Errorf("--type must be internal or external (got %q)", callType)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("sybill-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sybill-pp-cli sync' first.", err)
			}
			defer db.Close()

			deals, err := loadRecords(db, "deals")
			if err != nil {
				return err
			}
			convs, err := loadRecords(db, "conversations")
			if err != nil {
				return err
			}
			if len(convs) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No conversations in the local store. Run 'sybill-pp-cli sync' first.")
			}

			// Index groups by deal id (and a sentinel for unlinked calls).
			groups := map[string]*digestGroup{}
			const noDeal = "\x00no-deal"

			for _, c := range convs {
				start, ok := convStart(c)
				if !ok || start.Before(cutoff) {
					continue
				}
				if wantType != "" && convType(c) != wantType {
					continue
				}

				// Find the deal this call links to.
				var matched map[string]any
				for _, d := range deals {
					if convMatchesDeal(c, d) {
						matched = d
						break
					}
				}
				if owner != "" {
					ownerHit := matched != nil && strings.Contains(strings.ToLower(dealOwner(matched)), strings.ToLower(owner))
					if !ownerHit {
						continue
					}
				}

				key := noDeal
				name := "(no linked deal)"
				did := ""
				if matched != nil {
					key = dealID(matched)
					if key == "" {
						key = dealName(matched)
					}
					name = dealName(matched)
					if name == "" {
						name = key
					}
					did = dealID(matched)
				} else if _, crmName, _ := convCRMNamed(c); crmName != "" {
					// Linked to a CRM object we didn't sync as a deal; still group it.
					key = "crm:" + strings.ToLower(crmName)
					name = crmName
				}

				g := groups[key]
				if g == nil {
					g = &digestGroup{DealID: did, DealName: name}
					groups[key] = g
				}
				g.CallCount++
				g.Calls = append(g.Calls, digestCall{
					ConversationID: convID(c),
					Title:          convTitle(c),
					StartTime:      start.Format(time.RFC3339),
					Type:           convType(c),
					Participants:   participantCount(c),
				})
				if start.After(g.latest) {
					g.latest = start
				}
				for _, ns := range extractNextSteps(c) {
					if !containsStr(g.NextSteps, ns) {
						g.NextSteps = append(g.NextSteps, ns)
					}
				}
			}

			results := make([]*digestGroup, 0, len(groups))
			for _, g := range groups {
				sort.SliceStable(g.Calls, func(i, j int) bool { return g.Calls[i].StartTime > g.Calls[j].StartTime })
				results = append(results, g)
			}
			// Most recently active deal first; unlinked group sinks to the bottom.
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].DealName == "(no linked deal)" {
					return false
				}
				if results[j].DealName == "(no linked deal)" {
					return true
				}
				return results[i].latest.After(results[j].latest)
			})

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No calls in the window since %s.\n", since)
				return nil
			}
			for _, g := range results {
				fmt.Fprintf(out, "\n%s  (%d call%s)\n", g.DealName, g.CallCount, plural(g.CallCount))
				for _, c := range g.Calls {
					when := c.StartTime
					if t, ok := parseTime(c.StartTime); ok {
						when = t.Format("2006-01-02 15:04")
					}
					fmt.Fprintf(out, "  - %s  %s  [%s]\n", when, truncate(orDash(c.Title), 60), strings.ToLower(c.Type))
				}
				for _, ns := range g.NextSteps {
					fmt.Fprintf(out, "    → %s\n", ns)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: 7d, 48h, 30m, or an RFC3339 timestamp")
	cmd.Flags().StringVar(&callType, "type", "", "Filter calls by type: internal or external")
	cmd.Flags().StringVar(&owner, "owner", "", "Only deals whose owner name/email contains this substring")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// convCRMNamed is a thin wrapper exposing crm id/name/type for digest grouping.
func convCRMNamed(c map[string]any) (id, name, ctype string) { return convCRM(c) }

// participantCount counts the participants array on a conversation.
func participantCount(c map[string]any) int {
	if p, ok := c["participants"].([]any); ok {
		return len(p)
	}
	return 0
}

// extractNextSteps pulls next-step / takeaway strings from a conversation's
// summary object when conversation detail has been synced. Returns nil when no
// summary is present (list-only sync).
func extractNextSteps(c map[string]any) []string {
	summary := nestedObj(c, "summary")
	if summary == nil {
		return nil
	}
	var out []string
	for _, key := range []string{"nextSteps", "next_steps", "actionItems", "action_items", "keyTakeaways", "key_takeaways"} {
		switch v := summary[key].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, s)
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
		}
	}
	return out
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func orDash(s string) string {
	if s == "" {
		return "(untitled)"
	}
	return s
}
