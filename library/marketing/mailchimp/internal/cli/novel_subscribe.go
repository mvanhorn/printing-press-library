// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
//
// PATCH mailchimp-novel-workflow-commands: hand-authored novel command
// (one of 8). Composes the spec-emitted client; not generator-emitted.

package cli

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// subscriberHash returns the lowercase-email MD5 that Mailchimp uses as the
// subscriber_hash path parameter on every /lists/{id}/members/{hash} route.
// Surfaces in the help text so users don't have to know the trick.
func subscriberHash(email string) string {
	h := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(h[:])
}

func newSubscribeCmd(flags *rootFlags) *cobra.Command {
	var listID string
	var tags string
	var merges []string
	var status string

	cmd := &cobra.Command{
		Use:   "subscribe <email>",
		Short: "Upsert a member and apply tags in one call. Auto-MD5 the email; uses PUT not POST so a re-run is idempotent.",
		Long: `Compose the three-step Mailchimp subscribe workflow into one command:

  1. Compute the MD5 subscriber_hash of the lowercased email
  2. PUT /lists/{list-id}/members/{hash} with status + merge fields (creates or updates)
  3. POST /lists/{list-id}/members/{hash}/tags to apply the tag set

The PUT path is upsert — re-running with the same email is safe. The legacy
'lists members create' POST returns 400 'Member Exists' on the second run; this
command never does.`,
		Example: `  mailchimp-pp-cli subscribe alice@example.com --list b7661f2918 --tags vip,newsletter
  mailchimp-pp-cli subscribe alice@example.com --list b7661f2918 --merge FNAME=Alice --merge LNAME=Smith
  mailchimp-pp-cli subscribe alice@example.com --list b7661f2918 --status pending  # double opt-in`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if listID == "" {
				return fmt.Errorf("--list is required (audience/list id, e.g. b7661f2918)")
			}
			email := strings.TrimSpace(args[0])
			if email == "" || !strings.Contains(email, "@") {
				return fmt.Errorf("invalid email address: %q", args[0])
			}
			hash := subscriberHash(email)

			// Build merge_fields object from --merge KEY=VALUE pairs
			mergeFields := map[string]string{}
			for _, m := range merges {
				kv := strings.SplitN(m, "=", 2)
				if len(kv) != 2 || kv[0] == "" {
					return fmt.Errorf("invalid --merge value %q; expected KEY=VALUE", m)
				}
				mergeFields[strings.ToUpper(kv[0])] = kv[1]
			}

			// Build tag list
			var tagList []map[string]string
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tagList = append(tagList, map[string]string{"name": t, "status": "active"})
					}
				}
			}

			if dryRunOK(flags) {
				out := map[string]any{
					"would_upsert_member": map[string]any{
						"PUT":             fmt.Sprintf("/lists/%s/members/%s", listID, hash),
						"email_address":   email,
						"subscriber_hash": hash,
						"status_if_new":   status,
						"merge_fields":    mergeFields,
					},
				}
				if len(tagList) > 0 {
					out["would_apply_tags"] = map[string]any{
						"POST": fmt.Sprintf("/lists/%s/members/%s/tags", listID, hash),
						"tags": tagList,
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: PUT upsert. status_if_new applies to brand-new members only.
			// We deliberately omit "status" from the PUT body: including it would
			// force-flip the status of every existing member (e.g. re-subscribe
			// someone who previously opted out — a GDPR / consent violation
			// waiting to happen). Callers who explicitly want to change an
			// existing member's status should use the raw
			// `lists members patch-lists-id-id` endpoint, not the upsert helper.
			memberBody := map[string]any{
				"email_address": email,
				"status_if_new": status,
			}
			if len(mergeFields) > 0 {
				memberBody["merge_fields"] = mergeFields
			}
			memberData, _, err := c.Put(fmt.Sprintf("/lists/%s/members/%s", listID, hash), memberBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]json.RawMessage{
				"member": memberData,
			}

			// Step 2: POST tags (only if any specified; an empty tag set would clear all tags)
			if len(tagList) > 0 {
				tagsBody := map[string]any{"tags": tagList}
				tagsData, _, err := c.Post(fmt.Sprintf("/lists/%s/members/%s/tags", listID, hash), tagsBody)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if len(tagsData) > 0 {
					result["tags"] = tagsData
				} else {
					// Mailchimp returns 204 No Content with empty body on success.
					result["tags"] = json.RawMessage(`{"status":"applied","count":` + fmt.Sprintf("%d", len(tagList)) + `}`)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&listID, "list", "", "Audience (list) ID — find via 'mailchimp-pp-cli audiences get-contacts'")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tag names to apply (e.g. 'vip,newsletter')")
	cmd.Flags().StringArrayVar(&merges, "merge", nil, "Repeatable KEY=VALUE merge fields (e.g. --merge FNAME=Alice). Keys are uppercased.")
	cmd.Flags().StringVar(&status, "status", "subscribed", "Initial status: subscribed, pending (double opt-in), cleaned, unsubscribed")
	return cmd
}
