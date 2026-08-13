// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: pre-flight credit estimator for planned runs.
// pp:data-source live
// pp:novel-static-reference credit costs: standard endpoints charge 1 credit
// per call; /v2/instagram/post/comments with include_replies=true charges a
// 15-credit flat rate (vendor-documented, docs.scrapecreators.com, 2026-08).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	creditsPerStandardCall = 1
	creditsIncludeReplies  = 15
)

type estimateEnvelope struct {
	PlannedCalls     int64  `json:"planned_calls"`
	ProjectedCredits int64  `json:"projected_credits"`
	Balance          int64  `json:"balance"`
	Remaining        int64  `json:"remaining_after_run"`
	OverBudget       bool   `json:"over_budget"`
	Breakdown        string `json:"breakdown"`
}

// estimatePlan is the validated input of one estimate: what the caller plans
// to run. The command's RunE validates flag shapes; projection() and
// runAccountEstimate() own the arithmetic and the exit-7 decision input, so
// the credit-spend contract is testable against a fake client.
type estimatePlan struct {
	posts          int64
	calls          int64
	creditsPerCall int64
	withReplies    string // "", "flat", or "per-comment" (validated by the caller)
}

// projection computes the planned call count, the projected credit cost, and
// the human breakdown string for a plan.
func (p estimatePlan) projection() (planned, projected int64, breakdown string) {
	var parts []string
	if p.posts > 0 {
		perPost := int64(creditsPerStandardCall)
		label := "comments page"
		switch p.withReplies {
		case "flat":
			perPost = creditsIncludeReplies
			label = "include_replies flat"
		case "per-comment":
			// Cost is comment-count dependent; use the flat rate as the
			// honest upper bound the router would never exceed.
			perPost = creditsIncludeReplies
			label = "per-comment upper bound (router caps at the flat rate)"
		}
		projected += p.posts * perPost
		planned += p.posts
		parts = append(parts, fmt.Sprintf("%d posts x %d cr (%s)", p.posts, perPost, label))
	}
	if p.calls > 0 {
		cpc := p.creditsPerCall
		if cpc <= 0 {
			cpc = creditsPerStandardCall
		}
		projected += p.calls * cpc
		planned += p.calls
		parts = append(parts, fmt.Sprintf("%d calls x %d cr", p.calls, cpc))
	}
	return planned, projected, strings.Join(parts, " + ")
}

// parseCreditBalance extracts the live credit balance from the
// /v1/account/credit-balance envelope. The live shape carries creditCount
// (the same field account budget reads); balance/credits are kept for fixture
// and legacy shapes. A zero balance is indistinguishable from a shape we
// failed to parse, so it is an error rather than a spurious over-budget
// exit 7.
func parseCreditBalance(raw json.RawMessage) (int64, error) {
	var bal struct {
		CreditCount json.Number `json:"creditCount"`
		Balance     int64       `json:"balance"`
		Credits     int64       `json:"credits"`
	}
	_ = json.Unmarshal(raw, &bal)
	balance, _ := toInt64(bal.CreditCount)
	if balance == 0 && bal.Balance > 0 {
		balance = bal.Balance
	}
	if balance == 0 && bal.Credits > 0 {
		balance = bal.Credits
	}
	if balance == 0 {
		return 0, fmt.Errorf("could not read a credit balance from /v1/account/credit-balance; run 'account budget' to inspect the raw balance")
	}
	return balance, nil
}

// runAccountEstimate projects a plan's credit cost against the live balance
// fetched through the injectable client seam. OverBudget is true only when
// the projection strictly exceeds the balance — a plan that lands exactly on
// the balance is allowed.
func runAccountEstimate(ctx context.Context, c apiGetter, p estimatePlan) (estimateEnvelope, error) {
	planned, projected, breakdown := p.projection()
	balRaw, err := c.Get(ctx, "/v1/account/credit-balance", nil)
	if err != nil {
		return estimateEnvelope{}, fmt.Errorf("fetching credit balance: %w", err)
	}
	balance, err := parseCreditBalance(balRaw)
	if err != nil {
		return estimateEnvelope{}, err
	}
	return estimateEnvelope{
		PlannedCalls:     planned,
		ProjectedCredits: projected,
		Balance:          balance,
		Remaining:        balance - projected,
		OverBudget:       projected > balance,
		Breakdown:        breakdown,
	}, nil
}

func newNovelAccountEstimateCmd(flags *rootFlags) *cobra.Command {
	var posts int64
	var withReplies string
	var calls int64
	var creditsPerCall int64

	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Project the credit cost of a planned run against the live balance",
		Long: strings.Trim(`
Use this command to project the credit cost of a planned run before making it:
it multiplies the planned calls by the per-endpoint credit costs and compares
the total against the live credit balance, exiting non-zero when the run would
exhaust it. Do NOT use it for historical burn rate or days-remaining runway;
use 'account budget' instead.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli account estimate --posts 40 --with-replies flat --agent
  scrape-creators-pp-cli account estimate --calls 200 --credits-per-call 2`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would project credit cost against the live balance")
				return nil
			}
			if posts <= 0 && calls <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("plan something: --posts N and/or --calls N"))
			}
			if withReplies != "" && withReplies != "flat" && withReplies != "per-comment" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--with-replies must be flat or per-comment"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, err := runAccountEstimate(ctx, c, estimatePlan{
				posts:          posts,
				calls:          calls,
				creditsPerCall: creditsPerCall,
				withReplies:    withReplies,
			})
			if err != nil {
				return err
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
				return err
			}
			if out.OverBudget {
				return &cliError{code: 7, err: fmt.Errorf("planned run needs %d credits but balance is %d", out.ProjectedCredits, out.Balance)}
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&posts, "posts", 0, "Planned number of posts to pull comments for")
	cmd.Flags().StringVar(&withReplies, "with-replies", "", "Reply strategy for --posts: flat (15 cr/post) or per-comment (upper bound 15 cr/post)")
	cmd.Flags().Int64Var(&calls, "calls", 0, "Planned number of generic API calls")
	cmd.Flags().Int64Var(&creditsPerCall, "credits-per-call", 1, "Credit cost per generic call (default 1)")
	return cmd
}
