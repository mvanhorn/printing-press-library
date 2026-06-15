package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newCostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Estimate the credit cost of a planned generation",
	}
	cmd.AddCommand(newCostEstimateCmd(flags))
	return cmd
}

func newCostEstimateCmd(flags *rootFlags) *cobra.Command {
	var (
		modelInput string
		duration   int
		count      int
		resolution string
		liveCheck  bool
	)
	cmd := &cobra.Command{
		Use:   "estimate",
		Short: "Project the credit cost (and remaining balance) for a planned generation",
		Long: `Estimate the credit cost of one or more generations BEFORE you submit.

Uses the local model catalog (credits_per_video x duration_scale x
resolution_multiplier x count). With --live, also calls
/suite/api/topaz/estimate when available for an authoritative server-side
quote, plus /suite/api/user/my-info to project remaining balance.

This is a read-only operation; no credits are spent.`,
		Example: `  # Local-only estimate (no auth required)
  openart-pp-cli cost estimate --model byte-plus-seedance-2 --duration 10 --count 4

  # Live: also fetch authoritative quote + show projected remaining balance
  openart-pp-cli cost estimate --model seedance2 --duration 10 --count 2 --live`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if modelInput == "" {
				return fmt.Errorf("--model is required (e.g. --model seedance2)")
			}
			model := openartmodels.Resolve(modelInput)
			if model == nil {
				return fmt.Errorf("unknown model %q. Run 'openart-pp-cli models list' to see options.", modelInput)
			}
			isImage := model.Family == openartmodels.FamilyImage
			if duration <= 0 {
				if isImage {
					duration = 0
				} else {
					duration = (model.DurationMinSec + model.DurationMaxSec) / 2
					if duration == 0 {
						duration = 5
					}
				}
			}
			if count <= 0 {
				count = 1
			}
			estimate := model.EstimateCredits(duration, count, resolution)

			result := map[string]any{
				"model":            model.Slug,
				"display_name":     model.DisplayName,
				"vendor":           model.Vendor,
				"duration_seconds": duration,
				"count":            count,
				"resolution":       resolution,
				"estimate_credits": estimate,
				"per_video":        estimate / count,
				"basis":            "local model catalog (credits_per_video × duration_scale × resolution_multiplier × count)",
			}

			if cliutil.IsVerifyEnv() || flags.dryRun || !liveCheck {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				// Live check is best-effort; fall back to local estimate on auth failure.
				result["live_error"] = err.Error()
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Get user balance. OpenArt has multiple credit pools; sum the
			// three that actually fund generations (subscription + free +
			// trial). dalle2 credits are siloed to one tool family and
			// excluded here.
			if myInfo, _, err := c.Post("/user/my-info", map[string]any{}); err == nil {
				var u struct {
					FreeCreditBalance         int `json:"free_credit_balance"`
					SubscriptionMonthlyCredit int `json:"subscription_monthly_credit"`
					TrialCreditBalance        int `json:"trial_credit_balance"`
				}
				if json.Unmarshal(myInfo, &u) == nil {
					balance := u.SubscriptionMonthlyCredit + u.FreeCreditBalance + u.TrialCreditBalance
					if balance > 0 {
						result["balance_before"] = balance
						result["balance_after_projection"] = balance - estimate
						if balance < estimate {
							result["warning"] = fmt.Sprintf("estimate (%d) exceeds your balance (%d)", estimate, balance)
						}
					}
				}
			} else {
				result["balance_error"] = err.Error()
			}

			// Try /topaz/estimate (best-effort). Build a family-shaped body
			// so the live quote actually reaches the right endpoint for
			// image-family models.
			formType := openartmodels.FormText2Video
			topazInput := map[string]any{
				"prompt":      "estimate-only",
				"duration":    duration,
				"videoCount":  count,
				"resolution":  resolution,
				"aspectRatio": "16:9",
			}
			if isImage {
				formType = openartmodels.FormText2Image
				topazInput = map[string]any{
					"prompt":      "estimate-only",
					"imageCount":  count,
					"aspectRatio": "1:1",
				}
				if cmd.Flags().Changed("resolution") && resolution != "" {
					topazInput["resolution"] = resolution
				}
			}
			topazBody := map[string]any{
				"capabilityId": model.Capability(formType),
				"input":        topazInput,
			}
			if raw, status, err := c.Post("/topaz/estimate", topazBody); err == nil && status < 400 {
				var t struct {
					TotalCredits int `json:"totalCredits"`
					Cost         int `json:"cost"`
				}
				if json.Unmarshal(raw, &t) == nil {
					if t.TotalCredits > 0 {
						result["topaz_credits"] = t.TotalCredits
					} else if t.Cost > 0 {
						result["topaz_credits"] = t.Cost
					} else {
						result["topaz_raw"] = string(raw)
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&modelInput, "model", "", "Model slug or shorthand")
	cmd.Flags().IntVar(&duration, "duration", 0, "Duration in seconds")
	cmd.Flags().IntVar(&count, "count", 1, "Number of videos")
	cmd.Flags().StringVar(&resolution, "resolution", "720p", "Output resolution")
	cmd.Flags().BoolVar(&liveCheck, "live", false, "Also query the live balance + topaz/estimate endpoint")
	return cmd
}

func costSummaryString(estimate, balance int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "estimate=%d", estimate)
	if balance > 0 {
		fmt.Fprintf(&b, " balance=%d after≈%d", balance, balance-estimate)
	}
	return b.String()
}
