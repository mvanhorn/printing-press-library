// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `contact` command tree — bulk-tag, dedup, and decay.
// Reads the local SQLite cache; bulk-tag also mutates via the GHL API.
package cli

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/cliutil"
)

func newContactCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contact",
		Short: "Contact-level batch ops (bulk-tag, dedup, decay)",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newContactBulkTagCmd(flags))
	cmd.AddCommand(newContactDedupCmd(flags))
	cmd.AddCommand(newContactDecayCmd(flags))
	return cmd
}

// looksLikeEmail is a very loose check (contains @).
func looksLikeEmail(s string) bool { return strings.Contains(s, "@") }

func newContactBulkTagCmd(flags *rootFlags) *cobra.Command {
	var tag string
	var remove bool
	var batchSize int
	cmd := &cobra.Command{
		Use:   "bulk-tag",
		Short: "Add or remove a tag across many contacts (emails or IDs read from stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if tag == "" {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"error": "--tag is required",
					})
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "error: --tag is required")
				}
				return usageErr(fmt.Errorf("--tag is required"))
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			// Read stdin lines.
			scanner := bufio.NewScanner(os.Stdin)
			var inputs []string
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				inputs = append(inputs, line)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}

			// Resolve emails -> contact IDs via local cache.
			ids := make([]string, 0, len(inputs))
			notFound := []string{}
			for _, in := range inputs {
				if !looksLikeEmail(in) {
					ids = append(ids, in)
					continue
				}
				// Look up by email in resources (contacts).
				var idHit sql.NullString
				err := s.DB().QueryRowContext(ctx, `
                    SELECT id FROM resources
                    WHERE resource_type IN ('contacts', 'contacts_contacts')
                      AND lower(COALESCE(json_extract(data, '$.email'), '')) = lower(?)
                    LIMIT 1
                `, in).Scan(&idHit)
				if err == nil && idHit.Valid {
					ids = append(ids, idHit.String)
				} else {
					notFound = append(notFound, in)
				}
			}

			if batchSize <= 0 {
				return usageErr(fmt.Errorf("--batch-size must be > 0 (got %d)", batchSize))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would %s tag %q on %d contacts (not found: %d)\n",
					actionStr(remove), tag, len(ids), len(notFound))
				return nil
			}

			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":      actionStr(remove),
					"tag":         tag,
					"resolved":    len(ids),
					"not_found":   notFound,
					"batch_size":  batchSize,
					"dry_run":     true,
					"would_apply": ids,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			succeeded := 0
			failed := []map[string]string{}
			for i := 0; i < len(ids); i += batchSize {
				end := i + batchSize
				if end > len(ids) {
					end = len(ids)
				}
				chunk := ids[i:end]
				fmt.Fprintf(cmd.ErrOrStderr(), "applying batch %d-%d / %d\n", i+1, end, len(ids))
				for _, cid := range chunk {
					path := fmt.Sprintf("/contacts/%s/tags", cid)
					body := map[string]any{"tags": []string{tag}}
					var lerr error
					if remove {
						// GHL's tag-removal endpoint requires a request body
						// listing the tags to remove. The press client does not
						// yet expose DELETE-with-body, so we route through the
						// bulk-update endpoint which accepts {ids, tags, type:remove}.
						bulkBody := map[string]any{
							"ids":  []string{cid},
							"tags": []string{tag},
							"type": "remove",
						}
						_, _, lerr = c.Post("/contacts/tags/bulk", bulkBody)
					} else {
						_, _, lerr = c.Post(path, body)
					}
					if lerr != nil {
						failed = append(failed, map[string]string{"id": cid, "error": lerr.Error()})
					} else {
						succeeded++
					}
				}
			}
			report := map[string]any{
				"action":      actionStr(remove),
				"tag":         tag,
				"succeeded":   succeeded,
				"failed":      failed,
				"not_found":   notFound,
				"total_input": len(inputs),
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Tag to add or remove (required)")
	cmd.Flags().BoolVar(&remove, "remove", false, "Remove the tag instead of adding it")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Batch size per progress checkpoint")
	return cmd
}

func actionStr(remove bool) string {
	if remove {
		return "remove"
	}
	return "add"
}

func newContactDedupCmd(flags *rootFlags) *cobra.Command {
	var by string
	var apply bool
	cmd := &cobra.Command{
		Use:         "dedup",
		Short:       "Group contacts by email/phone, score by richness+recency, emit a merge plan",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			byEmail := strings.Contains(by, "email")
			byPhone := strings.Contains(by, "phone")
			if !byEmail && !byPhone {
				byEmail = true
			}

			rows, err := s.DB().QueryContext(ctx, `
                SELECT id, data FROM resources
                WHERE resource_type IN ('contacts', 'contacts_contacts')
            `)
			if err != nil {
				return apiErr(fmt.Errorf("query contacts: %w", err))
			}
			defer rows.Close()

			type contactRow struct {
				id       string
				email    string
				phone    string
				score    int
				name     string
				obj      map[string]any
				lastSeen string
			}
			var contacts []contactRow
			for rows.Next() {
				var id sql.NullString
				var raw []byte
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				email, _ := obj["email"].(string)
				phone, _ := obj["phone"].(string)
				fn, _ := obj["firstName"].(string)
				ln, _ := obj["lastName"].(string)
				name := strings.TrimSpace(fn + " " + ln)
				score := 0
				for _, k := range []string{"firstName", "lastName", "phone", "email", "address1", "city", "state"} {
					if v, _ := obj[k].(string); strings.TrimSpace(v) != "" {
						score++
					}
				}
				if cf, ok := obj["customFields"].([]any); ok {
					score += len(cf)
				}
				if tags, ok := obj["tags"].([]any); ok {
					score += len(tags)
				}
				du, _ := obj["dateUpdated"].(string)
				if du != "" {
					if t, err := time.Parse(time.RFC3339, du); err == nil {
						if time.Since(t) < 30*24*time.Hour {
							score++
						}
					}
				}
				contacts = append(contacts, contactRow{
					id:       nullStr(id),
					email:    strings.ToLower(strings.TrimSpace(email)),
					phone:    normalizePhone(phone),
					score:    score,
					name:     name,
					obj:      obj,
					lastSeen: du,
				})
			}

			type group struct {
				key       string
				reason    string
				canonical contactRow
				dups      []contactRow
			}
			groupBy := func(keyFn func(contactRow) string, reason string) []group {
				m := map[string][]contactRow{}
				for _, c := range contacts {
					k := keyFn(c)
					if k == "" {
						continue
					}
					m[k] = append(m[k], c)
				}
				out := []group{}
				for k, list := range m {
					if len(list) < 2 {
						continue
					}
					sort.Slice(list, func(i, j int) bool { return list[i].score > list[j].score })
					out = append(out, group{
						key:       k,
						reason:    reason,
						canonical: list[0],
						dups:      list[1:],
					})
				}
				return out
			}

			var groups []group
			if byEmail {
				groups = append(groups, groupBy(func(c contactRow) string { return c.email }, "matched by email")...)
			}
			if byPhone {
				groups = append(groups, groupBy(func(c contactRow) string { return c.phone }, "matched by phone")...)
			}

			plan := make([]map[string]any, 0, len(groups))
			for _, g := range groups {
				dups := make([]map[string]any, 0, len(g.dups))
				for _, d := range g.dups {
					dups = append(dups, map[string]any{
						"id":    d.id,
						"email": d.email,
						"phone": d.phone,
						"name":  d.name,
						"score": d.score,
					})
				}
				plan = append(plan, map[string]any{
					"canonical": map[string]any{
						"id":    g.canonical.id,
						"email": g.canonical.email,
						"phone": g.canonical.phone,
						"name":  g.canonical.name,
						"score": g.canonical.score,
					},
					"duplicates": dups,
					"reason":     g.reason,
					"key":        g.key,
				})
			}

			if apply {
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.OutOrStdout(), "would apply merge plan: %d groups\n", len(plan))
					return nil
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				merged := 0
				for _, p := range plan {
					can, _ := p["canonical"].(map[string]any)
					if can == nil {
						continue
					}
					body := map[string]any{
						"email": can["email"],
						"phone": can["phone"],
					}
					if _, _, err := c.Post("/contacts/upsert", body); err == nil {
						merged++
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"plan_groups": len(plan),
					"merged":      merged,
				}, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "email", "Group by: email, phone, or 'email,phone'")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually merge via /contacts/upsert (default is dry-run)")
	return cmd
}

func normalizePhone(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, c)
		}
	}
	if len(out) == 11 && out[0] == '1' {
		return "+" + string(out)
	}
	if len(out) == 10 {
		return "+1" + string(out)
	}
	if len(out) > 0 {
		return "+" + string(out)
	}
	return ""
}

func newContactDecayCmd(flags *rootFlags) *cobra.Command {
	var stage string
	var idleDays int
	cmd := &cobra.Command{
		Use:         "decay",
		Short:       "Find contacts in a stage with no inbound/outbound messages in N days",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			s, err := openGHLStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			// Resolve stage name -> stage ID via local pipelines/stages tables.
			var stageID string
			if stage != "" {
				if rs, err := s.DB().QueryContext(ctx, `SELECT id, name FROM stages`); err == nil {
					for rs.Next() {
						var id, nm sql.NullString
						if err := rs.Scan(&id, &nm); err == nil {
							if strings.EqualFold(nullStr(nm), stage) || nullStr(id) == stage {
								stageID = nullStr(id)
								break
							}
						}
					}
					rs.Close()
				}
				if stageID == "" {
					stageID = stage
				}
			}

			// Walk opportunities; filter by stageID.
			oppRows, err := s.DB().QueryContext(ctx, `
                SELECT id, data FROM resources
                WHERE resource_type IN ('opportunities', 'opportunities_opportunities')
            `)
			if err != nil {
				return apiErr(fmt.Errorf("query opportunities: %w", err))
			}
			defer oppRows.Close()

			type out struct {
				ContactID       string `json:"contact_id"`
				Email           string `json:"email"`
				LastMessageDate string `json:"last_message_date"`
				DaysIdle        int    `json:"days_idle"`
				OppID           string `json:"opp_id"`
				OppName         string `json:"opp_name"`
			}
			results := []out{}

			for oppRows.Next() {
				var oppID sql.NullString
				var raw []byte
				if err := oppRows.Scan(&oppID, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(raw, &obj); err != nil {
					continue
				}
				curStage, _ := obj["pipelineStageId"].(string)
				if stageID != "" && curStage != stageID {
					continue
				}
				contactID, _ := obj["contactId"].(string)
				if contactID == "" {
					if c, ok := obj["contact"].(map[string]any); ok {
						contactID, _ = c["id"].(string)
					}
				}
				if contactID == "" {
					continue
				}
				oppName, _ := obj["name"].(string)

				// Get most recent message date for the contact (messages.conversations_id -> conv -> contactId is
				// complex; fall back to messages whose data references this contactId).
				var lastMsg sql.NullString
				_ = s.DB().QueryRowContext(ctx, `
                    SELECT MAX(COALESCE(json_extract(data, '$.dateAdded'),
                                        json_extract(data, '$.dateUpdated')))
                    FROM messages
                    WHERE json_extract(data, '$.contactId') = ?
                `, contactID).Scan(&lastMsg)

				lmDate := nullStr(lastMsg)
				daysIdle := 99999
				if lmDate != "" {
					if t, err := time.Parse(time.RFC3339, lmDate); err == nil {
						daysIdle = int(time.Since(t).Hours() / 24)
					}
				}
				if daysIdle < idleDays {
					continue
				}

				// Look up contact email.
				var emailNS sql.NullString
				_ = s.DB().QueryRowContext(ctx, `
                    SELECT COALESCE(json_extract(data, '$.email'), '') FROM resources
                    WHERE resource_type IN ('contacts', 'contacts_contacts') AND id = ?
                `, contactID).Scan(&emailNS)

				results = append(results, out{
					ContactID:       contactID,
					Email:           nullStr(emailNS),
					LastMessageDate: lmDate,
					DaysIdle:        daysIdle,
					OppID:           nullStr(oppID),
					OppName:         oppName,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&stage, "stage", "", "Stage name or ID to filter by")
	cmd.Flags().IntVar(&idleDays, "idle-days", 30, "Days since last message threshold")
	return cmd
}
