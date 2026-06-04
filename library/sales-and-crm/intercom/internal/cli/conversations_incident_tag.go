// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/cliutil"
	"github.com/spf13/cobra"
)

func newConversationsIncidentTagCmd(flags *rootFlags) *cobra.Command {
	var mentions string
	var since string
	var tagName string
	var apply bool
	var limit int

	cmd := &cobra.Command{
		Use:   "incident-tag",
		Short: "Search conversations by body substring + time window and bulk-apply a tag (dry-run by default)",
		Example: strings.Trim(`
  # Dry-run: list conversations that mention "checkout 500" in the last 24h
  intercom-pp-cli conversations incident-tag --mentions "checkout 500" --tag incident-2026-05-24

  # Apply the tag
  intercom-pp-cli conversations incident-tag --mentions "checkout 500" --tag incident-2026-05-24 --apply
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return cmd.Help()
			}
			if mentions == "" || tagName == "" {
				return cmd.Help()
			}

			dur, err := parseSlaDuration(since)
			if err != nil {
				return usageErr(err)
			}
			sinceEpoch := time.Now().Add(-dur).Unix()
			if limit <= 0 {
				limit = 100
			}

			body := map[string]any{
				"query": map[string]any{
					"operator": "AND",
					"value": []map[string]any{
						{"field": "source.body", "operator": "~", "value": mentions},
						{"field": "updated_at", "operator": ">", "value": sinceEpoch},
					},
				},
				// per_page is Intercom's max page size (150). The user-supplied --limit
				// is the across-all-pages safety cap enforced in the pagination loop
				// below; previously --limit was wired in as per_page and the apply
				// path silently stopped at the first page, leaving conversations
				// 101..N un-tagged on large incident windows.
				"pagination": map[string]any{"per_page": perPageMax(limit)},
			}

			// Verify-env short-circuit BEFORE any network call (apply or not).
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would apply (verify mode)")
				return nil
			}

			if !apply {
				// Dry-run path: emit the search body + a would-run summary
				// without dialing out. The destructive surface lives behind
				// --apply; the default surfaces enough so an operator can
				// review the query before flipping it on.
				envelope := map[string]any{
					"dry_run":     true,
					"mentions":    mentions,
					"tag":         tagName,
					"since":       since,
					"since_epoch": sinceEpoch,
					"limit":       limit,
					"search_body": body,
					"hint":        "re-run with --apply to actually search and tag",
				}
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve admin id from /me — needed for the tag attach call.
			meRaw, err := c.Get(cmd.Context(), "/me", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var me struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(meRaw, &me)
			if me.ID == "" {
				return apiErr(fmt.Errorf("could not resolve admin id from /me"))
			}

			// Resolve tag: if non-numeric, look up by name to get id.
			tagID := tagName
			if !isAllDigits(tagName) {
				tagsRaw, terr := c.Get(cmd.Context(), "/tags", nil)
				if terr != nil {
					return classifyAPIError(terr, flags)
				}
				resolved, found := findTagIDByName(tagsRaw, tagName)
				if !found {
					return notFoundErr(fmt.Errorf("tag %q not found; create it first with 'intercom-pp-cli tags create'", tagName))
				}
				tagID = resolved
			}

			// Paginated search + tag loop. Intercom's /conversations/search
			// response carries pages.next.starting_after; iterate until the
			// cursor is empty OR we hit --limit conversations OR we exhaust
			// what the search returned. The earlier shipped version only ran
			// one page, which silently under-tagged whenever total_count
			// exceeded the per_page cap (~150). The user-supplied --limit is
			// the all-pages safety cap.
			type convRow struct {
				ID        string `json:"id"`
				State     string `json:"state"`
				UpdatedAt int64  `json:"updated_at"`
			}
			type searchResponse struct {
				Conversations []convRow `json:"conversations"`
				TotalCount    int       `json:"total_count"`
				Pages         struct {
					Next struct {
						StartingAfter string `json:"starting_after"`
					} `json:"next"`
				} `json:"pages"`
			}

			tagged := 0
			failed := 0
			matched := 0
			totalHint := 0
			tagBody := map[string]any{"id": tagID, "admin_id": me.ID}
			cursor := ""
			for {
				if cursor != "" {
					// Re-shape pagination on subsequent pages: keep per_page,
					// add starting_after.
					body["pagination"] = map[string]any{
						"per_page":       perPageMax(limit),
						"starting_after": cursor,
					}
				}
				searchData, _, err := c.Post(cmd.Context(), "/conversations/search", body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var sr searchResponse
				if err := json.Unmarshal(searchData, &sr); err != nil {
					return apiErr(fmt.Errorf("parsing search response: %w", err))
				}
				if totalHint == 0 {
					totalHint = sr.TotalCount
				}
				matched += len(sr.Conversations)
				for _, conv := range sr.Conversations {
					if tagged+failed >= limit {
						break
					}
					path := "/conversations/" + conv.ID + "/tags"
					_, _, tagErr := c.Post(cmd.Context(), path, tagBody)
					if tagErr != nil {
						failed++
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: tag %s failed: %v\n", conv.ID, tagErr)
						continue
					}
					tagged++
					fmt.Fprintf(cmd.ErrOrStderr(), "tagged: %s\n", conv.ID)
				}
				cursor = sr.Pages.Next.StartingAfter
				if cursor == "" || tagged+failed >= limit {
					break
				}
			}

			summary := map[string]any{
				"tag":              tagName,
				"matched":          matched,
				"tagged":           tagged,
				"failed":           failed,
				"total_count_hint": totalHint,
				"limit_reached":    (tagged+failed) >= limit && totalHint > matched,
			}
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}

	cmd.Flags().StringVar(&mentions, "mentions", "", "Body substring to search for (required)")
	cmd.Flags().StringVar(&since, "since", "24h", "Time window (e.g. 24h, 7d)")
	cmd.Flags().StringVar(&tagName, "tag", "", "Tag id or name to apply (required)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually apply the tag; default is dry-run")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max conversations to consider")
	return cmd
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func findTagIDByName(raw json.RawMessage, name string) (string, bool) {
	var env struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
		Tags []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"tags"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return "", false
	}
	candidates := append(env.Data, env.Tags...)
	for _, t := range candidates {
		if t.Name == name {
			return t.ID, true
		}
	}
	return "", false
}

// perPageMax returns the Intercom search per_page cap, bounded by the user's
// --limit so we don't over-fetch on small queries. Intercom's documented max
// is 150; anything higher 400s. A --limit of 0 (which Cobra never produces but
// is safe to guard) falls back to the cap.
func perPageMax(limit int) int {
	const intercomSearchPerPageMax = 150
	if limit <= 0 || limit > intercomSearchPerPageMax {
		return intercomSearchPerPageMax
	}
	return limit
}
