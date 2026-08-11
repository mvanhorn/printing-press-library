package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"

	"github.com/spf13/cobra"
)

// rateLimitWindow is the rendered projection of one RateLimitResultPayload.
// Linear reports period and reset in milliseconds. reset_at is the derived
// RFC3339 form so an agent planning a batch does not have to do epoch math.
type rateLimitWindow struct {
	Type      string  `json:"type"`
	Requested float64 `json:"requested_amount"`
	Allowed   float64 `json:"allowed_amount"`
	Remaining float64 `json:"remaining_amount"`
	PeriodMS  float64 `json:"period_ms"`
	ResetMS   float64 `json:"reset_ms"`
	ResetAt   string  `json:"reset_at"`
	ResetIn   string  `json:"reset_in"`
}

// rateLimitStatus is the rendered projection of RateLimitPayload.
type rateLimitStatus struct {
	Identifier string            `json:"identifier"`
	Kind       string            `json:"kind"`
	Limits     []rateLimitWindow `json:"limits"`
}

// pp:data-source live
// newRateLimitCmd exposes Linear's rateLimitStatus query. Exit code 7 has
// always existed for rate limiting, but until now the only way to discover
// exhaustion was to hit it. This command reads the budget before spending it.
func newRateLimitCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rate-limit",
		Short:       "Show the current Linear API rate limit budget",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,4,5,7"},
		Long: `Query Linear's rateLimitStatus for the authenticated credential and report
each limit window: the allowed quota, how much this request consumed, what is
left, and when the window replenishes.

Read this before a full sync or a large batch of mutations so the run can be
paced instead of discovering exhaustion by failing with exit code 7.`,
		Example: `  linear-pp-cli rate-limit
  linear-pp-cli rate-limit --agent`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var payload struct {
				RateLimitStatus struct {
					Identifier string `json:"identifier"`
					Kind       string `json:"kind"`
					Limits     []struct {
						Type            string  `json:"type"`
						RequestedAmount float64 `json:"requestedAmount"`
						AllowedAmount   float64 `json:"allowedAmount"`
						Period          float64 `json:"period"`
						RemainingAmount float64 `json:"remainingAmount"`
						Reset           float64 `json:"reset"`
					} `json:"limits"`
				} `json:"rateLimitStatus"`
			}
			if err := c.QueryInto(client.RateLimitStatusQuery, nil, &payload); err != nil {
				return classifyLiveReadError(err, flags)
			}

			now := time.Now()
			status := rateLimitStatus{
				Identifier: payload.RateLimitStatus.Identifier,
				Kind:       payload.RateLimitStatus.Kind,
				Limits:     make([]rateLimitWindow, 0, len(payload.RateLimitStatus.Limits)),
			}
			for _, l := range payload.RateLimitStatus.Limits {
				resetAt := time.UnixMilli(int64(l.Reset)).UTC()
				window := rateLimitWindow{
					Type:      l.Type,
					Requested: l.RequestedAmount,
					Allowed:   l.AllowedAmount,
					Remaining: l.RemainingAmount,
					PeriodMS:  l.Period,
					ResetMS:   l.Reset,
					ResetAt:   resetAt.Format(time.RFC3339),
					ResetIn:   resetAt.Sub(now).Truncate(time.Second).String(),
				}
				if resetAt.Before(now) {
					window.ResetIn = "0s"
				}
				status.Limits = append(status.Limits, window)
			}

			prov := attachFreshness(DataProvenance{
				Source:       "live",
				ResourceType: "rate_limit",
				Reason:       "user_requested",
			}, flags)

			out, err := json.Marshal(status)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if flags.selectFields != "" {
					out = filterFields(out, flags.selectFields)
				}
				wrapped, wrapErr := wrapWithProvenance(out, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
			}

			printProvenance(cmd, len(status.Limits), prov)
			fmt.Fprintf(cmd.OutOrStdout(), "Identifier: %s\nKind:       %s\n\n", status.Identifier, status.Kind)
			if len(status.Limits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Linear reported no rate limit windows for this credential.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-28s %12s %12s %12s %10s %s\n", "TYPE", "LIMIT", "USED", "REMAINING", "WINDOW", "RESETS IN")
			for _, l := range status.Limits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %12.0f %12.0f %12.0f %10s %s\n",
					l.Type, l.Allowed, l.Requested, l.Remaining,
					time.Duration(l.PeriodMS)*time.Millisecond, l.ResetIn)
			}
			return nil
		},
	}
	return cmd
}
