// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `rules run`: replays every enabled rule through the SAME
// plan machinery as `cleanup plan` — one incremental sync, one merged
// immutable plan (deltas grouped per rule), one one-time token — and then
// STOPS. It never auto-applies; applying is always an explicit
// `cleanup apply --plan <sha> --token <nonce>`.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

func newNovelRulesRunCmd(flags *rootFlags) *cobra.Command {
	var planOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Preview every enabled rule as ONE merged plan (per-rule counts + total) and mint the apply token — never auto-applies",
		Long: `Merge all enabled rules into one combined preview.

An incremental sync runs first; then each enabled rule's query resolves
against the LIVE Gmail API (capped at --limit ids per rule, default 2000)
into one frozen, immutable plan file — deltas grouped per rule, an id
matched by several rules going to the first. A message id is frozen at
most once. The output shows per-rule counts, the total, a 10-row sample
digest, the plan sha, and (unless --plan-only) a one-time apply token.

This command ALWAYS stops at the plan. Applying is a separate, explicit
'cleanup apply --plan <sha> --token <nonce>'.

--plan-only mints NO token — the preview and plan file are produced, but
nothing applyable exists afterwards. This is the unattended-sweep mode: a
scheduled 'rules run --plan-only' can report what WOULD change without
ever leaving a live credential-to-mutate lying around.

Typed exits: 0 plan written (or nothing matched) / 2 usage / 4 identity
refusal / 5 auth or API failure.`,
		Example: `  # Preview all enabled rules, get a plan + token
  gmail-pp-cli rules run --account personal

  # Unattended sweep: preview only, no token minted
  gmail-pp-cli rules run --account personal --plan-only --agent`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"mcp:local-write":     "true",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive, got %d", limit))
			}
			authDir := gauthConfigDirFrom(flags.authDir)
			rb, err := loadRulebook(authDir)
			if err != nil {
				return err
			}
			var enabled []mailRule
			for _, r := range rb.Rules {
				if r.Enabled {
					enabled = append(enabled, r)
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(enabled) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"account": account, "rules": 0, "total": 0,
					"note": "no enabled rules in " + rulebookPath(authDir) + " — add one with 'rules add'",
				}, flags)
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if err := verifyLiveIdentity(ctx, c, flags, account); err != nil {
				return classifyEngineAPIError(err)
			}
			prof, err := gauthProfile(flags, account)
			if err != nil {
				return err
			}

			// Freeze each rule's action, resolving label refs live so the
			// merged plan only ever names labels that exist right now.
			reqs := make([]planGroupRequest, 0, len(enabled))
			for _, r := range enabled {
				act := cleanupPlanAction{Type: r.Action, Add: r.Add, Remove: r.Remove}
				if act.Type == "label" {
					if act.Add, err = resolveLabelRefs(ctx, c, act.Add); err != nil {
						return fmt.Errorf("rule %q: %w", r.Name, err)
					}
					if act.Remove, err = resolveLabelRefs(ctx, c, act.Remove); err != nil {
						return fmt.Errorf("rule %q: %w", r.Name, err)
					}
				}
				reqs = append(reqs, planGroupRequest{Rule: r.Name, Query: r.Query, Action: act})
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			out, err := buildAndWritePlan(ctx, c, db, flags, account, prof.Email, "rules run",
				reqs, limit, !planOnly, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&planOnly, "plan-only", false, "Produce the preview + plan file but mint NO apply token (unattended-sweep mode)")
	cmd.Flags().IntVar(&limit, "limit", cleanupPlanDefaultLimit, "Maximum ids each rule's query may freeze")
	return cmd
}
