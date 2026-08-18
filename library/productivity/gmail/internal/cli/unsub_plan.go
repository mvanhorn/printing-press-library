// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `unsub plan`: the preview half of the unsubscribe engine.
// Freezes an immutable sha-named plan (action kind "unsub": sender list +
// their exact https one-click URLs + samples) through the SAME
// buildAndWritePlan machinery as `cleanup plan`, and mints the same
// one-time token. `unsub run` accepts ONLY (plan sha, token) — never an
// ad-hoc sender list — so the confirm daemon can execute from exactly
// (sha, nonce).

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
	"github.com/spf13/cobra"
)

// unsubIneligible is one requested sender the plan refused to freeze.
type unsubIneligible struct {
	Sender string `json:"sender"`
	Reason string `json:"reason"`
}

// unsubThirdParty is one frozen sender whose one-click URL host lives
// outside the sender's registrable domain (ESP-hosted): 'unsub run'
// skips exactly these unless it is invoked with --allow-third-party.
type unsubThirdParty struct {
	Sender string `json:"sender"`
	Host   string `json:"host"`
}

// unsubPlanOutput wraps the shared plan output with the per-sender verdicts.
type unsubPlanOutput struct {
	*planOutput
	Ineligible     []unsubIneligible `json:"ineligible,omitempty"`
	ThirdParty     []unsubThirdParty `json:"third_party_hosts,omitempty"`
	ThirdPartyNote string            `json:"third_party_note,omitempty"`
	RunHint        string            `json:"run_hint,omitempty"`
}

func newNovelUnsubPlanCmd(flags *rootFlags) *cobra.Command {
	var sendersCSV string

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Freeze an immutable unsubscribe plan (sender list + exact one-click URLs, sha-named, 0600) plus a one-time run token — nothing posted",
		Long: `Freeze exactly which senders 'unsub run' may POST one-click unsubscribes
for, without posting any of them.

Each requested sender must classify as one-click-verified in the local
store (see 'unsub audit'): exactly one https URL, List-Unsubscribe-Post
equal to "List-Unsubscribe=One-Click", and Gmail's own
Authentication-Results carrying dmarc=pass. Senders that do not qualify are
listed under "ineligible" with the reason and are NOT frozen.

The plan freezes, per eligible sender: the sender address, the exact https
URL (the only URL run is authorized to POST to), and the sender's newest
unsubscribe-bearing message id (snapshot + sample). Frozen senders whose
one-click URL host lives outside the sender's own registrable domain
(ESP-hosted) are listed under "third_party_hosts": 'unsub run' skips
exactly those unless it is invoked with --allow-third-party, so confirm
this plan with that list in hand. The file is written to
<auth-dir>/plans/<sha>.json (0600) where the name IS the sha256 of the
content, and a one-time 128-bit token bound to (plan, account) is minted,
valid 10 minutes — the token flow is identical to 'cleanup plan'.

'unsub run --plan <sha> --token <nonce>' re-verifies every condition live
(plus the DKIM coverage check) before any POST; it never accepts a sender
list of its own.

Typed exits: 0 plan written (or nothing eligible) / 2 usage / 4 identity
refusal / 5 auth or API failure.`,
		Example: `  # Audit first, then freeze the senders you want gone
  gmail-pp-cli unsub audit --account personal
  gmail-pp-cli unsub plan --account personal --senders deals@shop.example,news@letters.example

  # Then, within 10 minutes:
  gmail-pp-cli unsub run --plan <sha> --token <nonce>`,
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
			senders, err := cliutil.ParseStringList(sendersCSV)
			if err != nil {
				return usageErr(fmt.Errorf("parsing --senders: %w", err))
			}
			if len(senders) == 0 {
				return usageErr(fmt.Errorf("--senders is required: a comma-separated list of sender addresses from 'unsub audit'"))
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			ctx := cmd.Context()

			// Identity preflight binds the plan to the live mailbox at plan
			// time, exactly like `cleanup plan`.
			if err := verifyLiveIdentity(ctx, c, flags, account); err != nil {
				return classifyEngineAPIError(err)
			}
			prof, err := gauthProfile(flags, account)
			if err != nil {
				return err
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var reqs []planGroupRequest
			var ineligible []unsubIneligible
			seen := map[string]bool{}
			for _, raw := range senders {
				sender := strings.ToLower(strings.TrimSpace(raw))
				if sender == "" || seen[sender] {
					continue
				}
				seen[sender] = true
				newest, err := db.NewestUnsubMeta(account, sender)
				if errors.Is(err, sql.ErrNoRows) {
					ineligible = append(ineligible, unsubIneligible{Sender: sender,
						Reason: "no stored message from this sender carries a List-Unsubscribe header — run 'sync' then 'unsub audit' first"})
					continue
				}
				if err != nil {
					return fmt.Errorf("reading newest unsubscribe message for %s: %w", sender, err)
				}
				cls := classifyUnsubSender(newest.ListUnsubscribe, newest.ListUnsubscribePost, newest.AuthResults)
				if cls.Class != classUnsubOneClick {
					ineligible = append(ineligible, unsubIneligible{Sender: sender,
						Reason: fmt.Sprintf("classified %s: %s", cls.Class, cls.Reason)})
					continue
				}
				reqs = append(reqs, planGroupRequest{
					Rule:     sender,
					IDs:      []string{newest.ID},
					Action:   cleanupPlanAction{Type: "unsub"},
					Sender:   sender,
					UnsubURL: cls.URL,
				})
			}

			// Destination alignment is surfaced at plan time so the operator
			// confirms with the host list in hand; run re-derives it live.
			var thirdParty []unsubThirdParty
			for _, r := range reqs {
				if !unsubHostAligned(r.UnsubURL, r.Sender) {
					thirdParty = append(thirdParty, unsubThirdParty{Sender: r.Sender, Host: unsubURLHost(r.UnsubURL)})
				}
			}

			if len(reqs) == 0 {
				out := &unsubPlanOutput{
					planOutput: &planOutput{
						Account: account, AccountEmail: prof.Email, Source: "unsub plan",
						Samples: []cleanupPlanSample{}, PerGroup: []ruleCount{},
						Note: "no eligible one-click senders — no plan file written, no token minted",
					},
					Ineligible: ineligible,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			planOut, err := buildAndWritePlan(ctx, c, db, flags, account, prof.Email, "unsub plan",
				reqs, cleanupPlanDefaultLimit, true, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := &unsubPlanOutput{planOutput: planOut, Ineligible: ineligible, ThirdParty: thirdParty}
			if len(thirdParty) > 0 {
				// The hint stays default-deny on purpose: hints get executed
				// verbatim, and adding the override here would turn disclosure
				// into silent consent. The operator types the flag themselves.
				out.ThirdPartyNote = fmt.Sprintf("%d frozen sender(s) post to third-party (ESP) hosts — see third_party_hosts; 'unsub run' skips exactly those unless the operator explicitly adds --allow-third-party", len(thirdParty))
			}
			if planOut.PlanSha != "" && planOut.Nonce != "" {
				out.RunHint = fmt.Sprintf("gmail-pp-cli unsub run --plan %s --token %s", planOut.PlanSha, planOut.Nonce)
				out.ApplyHint = "" // the cleanup-apply hint does not apply to unsub plans
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&sendersCSV, "senders", "", "Comma-separated sender addresses to freeze (each must classify one-click-verified in 'unsub audit')")
	return cmd
}
