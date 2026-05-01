// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/internal/client"
)

// StackResult is the output of `rewards stack`. RecommendedCouponID is null
// when no coupon was applicable. Currency values are USD.
type StackResult struct {
	RecommendedCouponID any     `json:"recommended_coupon_id"`
	CouponValue         float64 `json:"coupon_value"`
	PointsRedeemed      int     `json:"points_redeemed"`
	CreditUsed          float64 `json:"credit_used"`
	TotalSavings        float64 `json:"total_savings"`
	FinalTotal          float64 `json:"final_total"`
	Warning             string  `json:"warning,omitempty"`
}

// stackable holds the parsed inputs the optimizer works against.
type stackable struct {
	Coupons []couponShape
	Credit  float64
}

type couponShape struct {
	ID       any
	Value    float64
	MinOrder float64
}

// pickBestCoupon returns the index of the highest-value coupon whose MinOrder
// is met by the order total, or -1 when none qualifies.
func pickBestCoupon(coupons []couponShape, orderTotal float64) int {
	best := -1
	bestVal := 0.0
	for i, c := range coupons {
		if c.MinOrder > orderTotal {
			continue
		}
		if c.Value > bestVal {
			bestVal = c.Value
			best = i
		}
	}
	return best
}

// computeStack picks the best coupon, applies stored credit to whatever
// remains, and returns the structured result. Heuristic only — Pagliacci's
// checkout enforces its own rules; this is a recommendation.
func computeStack(s stackable, orderTotal float64, experimental bool) StackResult {
	res := StackResult{
		FinalTotal:          orderTotal,
		RecommendedCouponID: nil,
	}

	if len(s.Coupons) > 0 {
		idx := pickBestCoupon(s.Coupons, orderTotal)
		if idx >= 0 {
			c := s.Coupons[idx]
			// Coupon value is capped at the order total (no negative remainders).
			val := c.Value
			if val > orderTotal {
				val = orderTotal
			}
			res.RecommendedCouponID = c.ID
			res.CouponValue = val
			res.FinalTotal = orderTotal - val
		}
	}

	// Credit covers whatever is left, capped at remaining balance.
	if s.Credit > 0 && res.FinalTotal > 0 {
		use := s.Credit
		if use > res.FinalTotal {
			use = res.FinalTotal
		}
		res.CreditUsed = use
		res.FinalTotal -= use
	}

	// Defensive rounding to avoid float drift in JSON.
	res.FinalTotal = round2(res.FinalTotal)
	res.CouponValue = round2(res.CouponValue)
	res.CreditUsed = round2(res.CreditUsed)
	res.TotalSavings = round2(res.CouponValue + res.CreditUsed)

	if experimental {
		res.Warning = "multi-coupon stacking is heuristic; checkout may reject"
	}
	return res
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// fetchCoupons returns the structured coupon list from /StoredCoupons.
// Empty slice (not error) when the user has none.
func fetchCoupons(c *client.Client) ([]couponShape, error) {
	raw, err := c.Get("/StoredCoupons", nil)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil {
		return nil, nil
	}
	out := make([]couponShape, 0, len(arr))
	for _, c := range arr {
		var id any = c["ID"]
		if id == nil {
			id = c["Id"]
		}
		if id == nil {
			id = c["Code"]
		}
		out = append(out, couponShape{
			ID:       id,
			Value:    extractFloat(c, "Value", "Amount", "FaceValue"),
			MinOrder: extractFloat(c, "MinOrder", "Minimum", "MinimumOrder"),
		})
	}
	return out, nil
}

// fetchStoredCredit returns the user's StoredCredit balance.
func fetchStoredCredit(c *client.Client) (float64, error) {
	raw, err := c.Get("/StoredCredit", nil)
	if err != nil {
		return 0, err
	}
	// /StoredCredit may be either a single object {"Balance": ...} or
	// an array of credit lines.
	var single map[string]any
	if json.Unmarshal(raw, &single) == nil && single != nil {
		if v := extractFloat(single, "Balance", "Amount", "Total"); v > 0 {
			return v, nil
		}
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) == nil {
		var sum float64
		for _, e := range arr {
			sum += extractFloat(e, "Balance", "Amount", "Value")
		}
		return sum, nil
	}
	return 0, nil
}

func newRewardsStackCmd(flags *rootFlags) *cobra.Command {
	var orderTotal float64
	var experimental bool

	cmd := &cobra.Command{
		Use:   "stack",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Pick the best coupon + stored credit combination for a given order total",
		Example: `  pagliacci-pizza-pp-cli rewards stack --order-total 45.00
  pagliacci-pizza-pp-cli rewards stack --order-total 45.00 --experimental --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if orderTotal <= 0 {
				return usageErr(fmt.Errorf("--order-total must be > 0 (e.g., --order-total 45.00)"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			coupons, cerr := fetchCoupons(c)
			if cerr != nil {
				// Auth-required; surface the error verbatim.
				return classifyAPIError(cerr)
			}
			credit, _ := fetchStoredCredit(c) // best-effort; absence is ok

			res := computeStack(stackable{Coupons: coupons, Credit: credit}, orderTotal, experimental)

			out, err := json.Marshal(res)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&orderTotal, "order-total", 0, "Pre-discount cart total (required)")
	cmd.Flags().BoolVar(&experimental, "experimental", false, "Try multi-coupon stacking (heuristic; may be rejected at checkout)")
	return cmd
}
