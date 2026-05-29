// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/cliutil"
	"github.com/spf13/cobra"
)

type refundEntitlementLoss struct {
	EntitlementID string `json:"entitlement_id"`
	DisplayName   string `json:"display_name,omitempty"`
	LookupKey     string `json:"lookup_key,omitempty"`
}

type refundCascadeView struct {
	ProjectID        string                  `json:"project_id"`
	SubscriptionID   string                  `json:"subscription_id"`
	CustomerID       string                  `json:"customer_id,omitempty"`
	Status           string                  `json:"status"`
	StatusAfter      string                  `json:"status_after,omitempty"`
	TotalRevenueUSD  float64                 `json:"total_revenue_usd"`
	GivesAccess      bool                    `json:"gives_access"`
	EntitlementsLost []refundEntitlementLoss `json:"entitlements_at_risk"`
	Apply            bool                    `json:"apply"`
	Refunded         bool                    `json:"refunded"`
	Note             string                  `json:"note,omitempty"`
}

func newNovelRefundCascadeCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var apply bool
	cmd := &cobra.Command{
		Use:   "refund-cascade <subscription-id>",
		Short: "Trace a subscription's revenue and entitlement fallout; with --apply issue the live refund",
		Long: `Traces one subscription:

  subscription -> total revenue -> granted entitlements (the access lost on refund)

By default this only traces and prints what WOULD be refunded. Pass --apply to
issue the live refund via POST /projects/{project_id}/subscriptions/{id}/actions/refund,
which immediately revokes the customer's access to the listed entitlements.

--apply is force-disabled under --dry-run and under the verifier environment so
the trace runs without mutating.

Use this command to trace or issue a refund for one subscription and see the
entitlement fallout. Do NOT use it for aggregate refund-rate trends; use
'charts get refund_rate' instead.

Data source: auto (live API for the subscription detail and the refund action).`,
		Example: "  # Trace only (default)\n  revenuecat-pp-cli refund-cascade sub1a2b3c4d5e --project proj1ab2c3d4 --json\n  # Issue the live refund\n  revenuecat-pp-cli refund-cascade sub1a2b3c4d5e --project proj1ab2c3d4 --apply",
		Annotations: map[string]string{
			"mcp:read-only":  "false",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}
			// dry-run short-circuit BEFORE requiring args/project so verify
			// probes succeed with no input.
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would trace the subscription and (with --apply) issue a live refund")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a <subscription-id> positional argument is required"))
			}
			subID := args[0]
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			// Force trace-only under the verifier env so no mutation fires.
			effectiveApply := apply
			if cliutil.IsVerifyEnv() {
				effectiveApply = false
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := runRefundCascade(cmd.Context(), c, projectID, subID, effectiveApply)
			if err != nil {
				return apiErr(err)
			}
			if apply && !effectiveApply && view.Note == "" {
				view.Note = "verifier environment detected; forced trace-only (no refund issued)"
			}
			return emitRefundCascade(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Issue the live refund via POST actions/refund (default: trace only)")
	return cmd
}

func emitRefundCascade(cmd *cobra.Command, flags *rootFlags, view refundCascadeView) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(view.EntitlementsLost) > 0 {
			items := make([]map[string]any, 0, len(view.EntitlementsLost))
			for _, e := range view.EntitlementsLost {
				items = append(items, map[string]any{
					"entitlement_id": e.EntitlementID,
					"display_name":   e.DisplayName,
					"lookup_key":     e.LookupKey,
				})
			}
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				return err
			}
		}
		verb := "would refund"
		if view.Refunded {
			verb = "refunded"
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"\nSubscription %s  status=%s  revenue=$%.2f  gives_access=%v\n%s (apply=%v); %d entitlement(s) at risk.\n",
			view.SubscriptionID, view.Status, view.TotalRevenueUSD, view.GivesAccess,
			verb, view.Apply, len(view.EntitlementsLost))
		if view.StatusAfter != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Status after refund: %s\n", view.StatusAfter)
		}
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// subscriptionDetail is the slice of the Subscription object refund-cascade
// reads. RevenueCat subscriptions are flat with a nested entitlements list.
type subscriptionDetail struct {
	ID           string `json:"id"`
	CustomerID   string `json:"customer_id"`
	Status       string `json:"status"`
	GivesAccess  bool   `json:"gives_access"`
	TotalRevenue any    `json:"total_revenue_in_usd"`
	ProductID    string `json:"product_id"`
	Entitlements struct {
		Items []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			LookupKey   string `json:"lookup_key"`
		} `json:"items"`
	} `json:"entitlements"`
}

// traceFromSubscription maps the fetched subscription detail onto the cascade
// view: customer, status, gives-access, revenue at risk, and the entitlements
// that would be revoked. Pure function for unit-testing.
func traceFromSubscription(view *refundCascadeView, sub subscriptionDetail) {
	view.CustomerID = sub.CustomerID
	view.Status = sub.Status
	view.GivesAccess = sub.GivesAccess
	view.TotalRevenueUSD = monetaryGrossUSD(sub.TotalRevenue)
	for _, e := range sub.Entitlements.Items {
		view.EntitlementsLost = append(view.EntitlementsLost, refundEntitlementLoss{
			EntitlementID: e.ID,
			DisplayName:   e.DisplayName,
			LookupKey:     e.LookupKey,
		})
	}
}

// runRefundCascade fetches the subscription, traces its entitlement fallout,
// and (when apply) issues the live refund.
//
// TODO(verify): confirm the subscription detail includes the entitlements list
// inline against live data; if not, fetch /subscriptions/{id}/entitlements.
func runRefundCascade(ctx context.Context, c *client.Client, projectID, subID string, apply bool) (refundCascadeView, error) {
	view := refundCascadeView{
		ProjectID:        projectID,
		SubscriptionID:   subID,
		Apply:            apply,
		EntitlementsLost: []refundEntitlementLoss{},
	}

	path := replacePathParam("/projects/{project_id}/subscriptions/{subscription_id}", "project_id", projectID)
	path = replacePathParam(path, "subscription_id", subID)
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return view, fmt.Errorf("fetching subscription %s: %w", subID, err)
	}
	var sub subscriptionDetail
	if err := json.Unmarshal(raw, &sub); err != nil {
		return view, fmt.Errorf("parsing subscription %s: %w", subID, err)
	}
	traceFromSubscription(&view, sub)

	if !apply {
		view.Note = "trace only; rerun with --apply to issue the live refund (this revokes access immediately)"
		return view, nil
	}

	// Issue the live refund. The client gates mutating verbs under the
	// verifier env; the caller has already forced apply=false there.
	refundPath := replacePathParam("/projects/{project_id}/subscriptions/{subscription_id}/actions/refund", "project_id", projectID)
	refundPath = replacePathParam(refundPath, "subscription_id", subID)
	respRaw, status, perr := c.Post(ctx, refundPath, nil)
	if perr != nil {
		return view, fmt.Errorf("refunding subscription %s: %w", subID, perr)
	}
	// Guard against transports that return a non-2xx envelope without an error:
	// never report a refund as issued on a 4xx/5xx status.
	if status >= 400 {
		return view, fmt.Errorf("refunding subscription %s: API returned HTTP %d", subID, status)
	}
	view.Refunded = true
	// The refund action returns the updated Subscription; surface its new status.
	var after struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(respRaw, &after) == nil {
		view.StatusAfter = after.Status
	}
	view.Note = fmt.Sprintf("refund issued; %d entitlement(s) revoked", len(view.EntitlementsLost))
	return view, nil
}
