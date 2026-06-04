// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/client"
	"github.com/spf13/cobra"
)

type refundCascadeLicenseKey struct {
	KeyID           string `json:"key_id"`
	KeyShort        string `json:"key_short,omitempty"`
	Status          string `json:"status_before"`
	Disabled        bool   `json:"disabled_before"`
	ActivationLimit int    `json:"activation_limit,omitempty"`
	Activations     int    `json:"activations,omitempty"`
	Action          string `json:"action"`
	Error           string `json:"error,omitempty"`
}

type refundCascadeOrderItem struct {
	OrderItemID string                    `json:"order_item_id"`
	ProductID   string                    `json:"product_id,omitempty"`
	VariantID   string                    `json:"variant_id,omitempty"`
	ProductName string                    `json:"product_name,omitempty"`
	LicenseKeys []refundCascadeLicenseKey `json:"license_keys"`
}

type refundCascadeView struct {
	OrderID       string                   `json:"order_id"`
	OrderStatus   string                   `json:"order_status"`
	OrderRefunded bool                     `json:"order_refunded"`
	CustomerEmail string                   `json:"customer_email,omitempty"`
	OrderItems    []refundCascadeOrderItem `json:"order_items"`
	KeysDisabled  int                      `json:"keys_disabled"`
	KeysSkipped   int                      `json:"keys_skipped"`
	KeysFailed    int                      `json:"keys_failed"`
	Apply         bool                     `json:"apply"`
	FetchFailures []string                 `json:"fetch_failures,omitempty"`
	Note          string                   `json:"note,omitempty"`
}

func newNovelRefundCascadeCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "refund-cascade <order-id>",
		Short: "Walk order → items → license-keys → instances; with --apply disable every key tied to the refund",
		Long: `Walks the cascade for a single order:

  order -> order-items -> license-keys (filter by order_id) -> license-key-instances

By default prints a dry-run plan listing which keys would be disabled. Pass
--apply to actually call PATCH /v1/license-keys/{id} with data.attributes.disabled=true
for each key tied to the order.

Use this command for the post-refund disable cascade on a specific order. For
routine "find keys with abnormal seat counts" sweeps, use 'license-rollup' instead.

Data source: auto (live API for the cascade, plus local context).`,
		Example: "  # Dry-run preview\n  lemonsqueezy-pp-cli refund-cascade order_3aBc --json\n  # Actually disable the keys\n  lemonsqueezy-pp-cli refund-cascade order_3aBc --apply --json",
		Annotations: map[string]string{
			"mcp:read-only":  "false",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("order-id positional argument is required"))
			}
			orderID := args[0]
			// Global --dry-run gating: refund-cascade already has its own
			// --apply switch (default is plan-only). When the user passes
			// --dry-run together with --apply, force apply=false so the
			// cascade still computes and prints the plan instead of
			// silently emitting nothing. Without --apply, --dry-run is a
			// no-op (the default is already a plan).
			effectiveApply := apply
			if dryRunOK(flags) {
				effectiveApply = false
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := runRefundCascade(cmd.Context(), c, orderID, effectiveApply)
			if err != nil {
				return apiErr(err)
			}
			if dryRunOK(flags) && view.Note == "" {
				view.Note = "global --dry-run forced apply=false; this is a plan, not a mutation"
			}
			return emitRefundCascade(cmd, flags, view)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually disable license keys via PATCH (default: dry-run preview)")
	return cmd
}

func emitRefundCascade(cmd *cobra.Command, flags *rootFlags, view refundCascadeView) error {
	// Flatten the order-items / license-keys tree into one row per key for
	// the human table. JSON output keeps the nested shape.
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0)
		for _, oi := range view.OrderItems {
			for _, k := range oi.LicenseKeys {
				items = append(items, map[string]any{
					"order_item_id":    oi.OrderItemID,
					"key_id":           k.KeyID,
					"key_short":        k.KeyShort,
					"status_before":    k.Status,
					"disabled_before":  k.Disabled,
					"activation_limit": k.ActivationLimit,
					"activations":      k.Activations,
					"action":           k.Action,
					"error":            k.Error,
				})
			}
		}
		if len(items) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"\nOrder %s  status=%s  refunded=%v\nKeys: %d disabled, %d skipped, %d failed  (apply=%v)\n",
			view.OrderID, view.OrderStatus, view.OrderRefunded,
			view.KeysDisabled, view.KeysSkipped, view.KeysFailed, view.Apply)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		for _, f := range view.FetchFailures {
			fmt.Fprintf(cmd.OutOrStdout(), "Fetch failure: %s\n", f)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

func runRefundCascade(ctx context.Context, c *client.Client, orderID string, apply bool) (refundCascadeView, error) {
	view := refundCascadeView{OrderID: orderID, Apply: apply, OrderItems: []refundCascadeOrderItem{}}

	orderData, err := c.Get(ctx, "/v1/orders/"+url.PathEscape(orderID), map[string]string{
		"include": "order-items",
	})
	if err != nil {
		return view, fmt.Errorf("fetching order %s: %w", orderID, err)
	}
	var orderEnv struct {
		Data struct {
			Attributes struct {
				Status    string `json:"status"`
				Refunded  any    `json:"refunded"`
				UserEmail string `json:"user_email"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(orderData, &orderEnv); err != nil {
		return view, fmt.Errorf("parsing order %s response: %w", orderID, err)
	}
	view.OrderStatus = orderEnv.Data.Attributes.Status
	view.OrderRefunded = toBoolLS(orderEnv.Data.Attributes.Refunded)
	view.CustomerEmail = orderEnv.Data.Attributes.UserEmail

	// Safety: only --apply for orders the API itself reports as refunded.
	// Prevents disabling keys on a typo'd order ID that returns a non-refunded
	// order envelope.
	if apply && !view.OrderRefunded {
		view.Apply = false
		view.Note = "order is not marked refunded; refusing to --apply. Re-run with --apply only after the order is actually refunded in Lemon Squeezy."
		apply = false
	}

	// Fetch ALL license keys for the order — paginate via JSON:API links.next
	// until exhausted. A single page of 100 is not enough for large orders;
	// silently leaving keys enabled after --apply would defeat the cascade.
	type licenseKeyRec struct {
		ID         string `json:"id"`
		Attributes struct {
			KeyShort        string `json:"key_short"`
			Status          string `json:"status"`
			Disabled        any    `json:"disabled"`
			ActivationLimit any    `json:"activation_limit"`
			ActivationUsage any    `json:"activation_usage"`
			ProductID       any    `json:"product_id"`
			VariantID       any    `json:"variant_id"`
			OrderItemID     any    `json:"order_item_id"`
		} `json:"attributes"`
	}
	type keysPage struct {
		Data  []licenseKeyRec `json:"data"`
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
	}

	var allKeys []licenseKeyRec
	const refundCascadeMaxPages = 50 // 50 * 100 = 5000 keys ceiling; refuses to silently truncate
	page := 1
	for {
		params := map[string]string{
			"filter[order_id]": orderID,
			"page[size]":       "100",
			"page[number]":     strconv.Itoa(page),
		}
		raw, err := c.Get(ctx, "/v1/license-keys", params)
		if err != nil {
			view.FetchFailures = append(view.FetchFailures, "license-keys page "+strconv.Itoa(page)+": "+err.Error())
			return view, nil
		}
		var pg keysPage
		if err := json.Unmarshal(raw, &pg); err != nil {
			return view, fmt.Errorf("parsing license-keys page %d: %w", page, err)
		}
		allKeys = append(allKeys, pg.Data...)
		if pg.Links.Next == "" || len(pg.Data) == 0 {
			break
		}
		if page >= refundCascadeMaxPages {
			// Refuse to silently truncate — bail out and surface to the caller.
			view.FetchFailures = append(view.FetchFailures, fmt.Sprintf("license-keys: reached pagination ceiling of %d pages (%d keys); refusing to --apply on a partial set", refundCascadeMaxPages, len(allKeys)))
			if apply {
				view.Apply = false
				apply = false
			}
			break
		}
		page++
	}
	keysEnv := struct {
		Data []licenseKeyRec
	}{Data: allKeys}

	// Group keys by order-item-id for nicer output.
	byItem := map[string]*refundCascadeOrderItem{}
	for _, k := range keysEnv.Data {
		itemID := toStringLS(k.Attributes.OrderItemID)
		if itemID == "" {
			itemID = "(no order_item_id)"
		}
		oi, ok := byItem[itemID]
		if !ok {
			oi = &refundCascadeOrderItem{
				OrderItemID: itemID,
				ProductID:   toStringLS(k.Attributes.ProductID),
				VariantID:   toStringLS(k.Attributes.VariantID),
			}
			byItem[itemID] = oi
		}
		row := refundCascadeLicenseKey{
			KeyID:           k.ID,
			KeyShort:        k.Attributes.KeyShort,
			Status:          k.Attributes.Status,
			Disabled:        toBoolLS(k.Attributes.Disabled),
			ActivationLimit: int(toFloatLS(k.Attributes.ActivationLimit)),
			Activations:     int(toFloatLS(k.Attributes.ActivationUsage)),
		}
		if row.Disabled {
			row.Action = "skip (already disabled)"
			view.KeysSkipped++
		} else if !apply {
			row.Action = "would disable"
		} else {
			body := map[string]any{
				"data": map[string]any{
					"type": "license-keys",
					"id":   row.KeyID,
					"attributes": map[string]any{
						"disabled": true,
					},
				},
			}
			if _, _, err := c.Patch(ctx, "/v1/license-keys/"+url.PathEscape(row.KeyID), body); err != nil {
				row.Action = "FAILED"
				row.Error = err.Error()
				view.KeysFailed++
			} else {
				row.Action = "disabled"
				view.KeysDisabled++
			}
		}
		oi.LicenseKeys = append(oi.LicenseKeys, row)
	}
	// Sort items for stable output across runs (map iteration is random).
	itemKeys := make([]string, 0, len(byItem))
	for k := range byItem {
		itemKeys = append(itemKeys, k)
	}
	sort.Strings(itemKeys)
	for _, k := range itemKeys {
		view.OrderItems = append(view.OrderItems, *byItem[k])
	}
	if len(view.OrderItems) == 0 && view.Note == "" {
		view.Note = "no license keys found for this order"
	} else if !apply && view.Note == "" {
		view.Note = "dry-run preview; rerun with --apply to actually disable the keys"
	}
	return view, nil
}
