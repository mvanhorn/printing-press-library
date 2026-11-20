package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
	"github.com/spf13/cobra"
)

// evaluateBudget checks a planned request count against a known daily balance.
func evaluateBudget(q client.EtsyQuota, known bool, plannedCalls int) error {
	if plannedCalls <= 0 || !known {
		return nil
	}
	if plannedCalls > q.RemainingToday {
		return rateLimitErr(fmt.Errorf("planned %d Etsy calls exceeds %d remaining today; lower --max-calls, wait for available quota, or use --pages smaller; pass --ignore-budget to override", plannedCalls, q.RemainingToday))
	}
	// Division avoids overflow while preserving remaining*20 < limit.
	if q.LimitPerDay > 0 && q.RemainingToday <= (q.LimitPerDay-1)/20 {
		return rateLimitErr(fmt.Errorf("daily budget nearly exhausted (%d of %d left); pass --ignore-budget to override", q.RemainingToday, q.LimitPerDay))
	}
	return nil
}

// checkBudget refuses plans exceeding the last observed daily Etsy budget.
func checkBudget(cmd *cobra.Command, plannedCalls int, ignoreBudget bool) error {
	if plannedCalls <= 0 || ignoreBudget {
		return nil
	}
	q, known := client.LoadEtsyQuota(etsyQuotaKey(cmd))
	if err := evaluateBudget(q, known, plannedCalls); err != nil {
		return err
	}
	if known && q.LimitPerDay > 0 && q.RemainingToday <= (q.LimitPerDay-1)/5 && cmd != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: %d of %d daily Etsy calls left\n", q.RemainingToday, q.LimitPerDay)
	}
	return nil
}

// etsy-ext: always expose the selected key's quota status.
func etsyQuotaSummary(key string) (string, bool) {
	q, ok := client.LoadEtsyQuota(key)
	if err := client.EtsyQuotaPersistError(key); err != nil {
		return "unwritable: " + err.Error(), false
	}
	if !ok {
		return "unknown: no observation in the last 24h for this key", false
	}
	return fmt.Sprintf("known: remaining %d of %d (observed %s ago)", q.RemainingToday, q.LimitPerDay, time.Since(q.ObservedAt).Round(time.Second)), true
}

// etsy-ext: Shared daily-quota classification for sync and other Etsy batches.
func isDailyQuotaErr(err error) bool { return client.IsDailyQuotaExhausted(err) }

// etsy-ext: shared ceilings for untrusted MCP callers.
const (
	etsyMCPMaxCalls         = 500
	etsyMCPMaxSyncPages     = 50
	etsyMCPMaxSnapshotPages = 3
)

func etsyMCPClamp(cmd *cobra.Command, name string, value *int, ceiling int, zeroUnlimited bool) {
	if (*value <= 0 && zeroUnlimited) || *value > ceiling {
		fmt.Fprintf(cmd.ErrOrStderr(), "MCP: --%s clamped from %d to %d\n", name, *value, ceiling)
		*value = ceiling
	}
}

// etsyQuotaKey uses the selected config, not the store's token-bearing identity.
type etsyQuotaContextKey struct{}

func etsyQuotaKey(cmd *cobra.Command) string {
	if cmd != nil && cmd.Context() != nil && cmd.Root().PersistentFlags().Lookup("config") != nil {
		if key, ok := cmd.Context().Value(etsyQuotaContextKey{}).(string); ok {
			return key
		}
	}
	path := ""
	if cmd != nil {
		path, _ = cmd.Flags().GetString("config")
		if flag := cmd.Root().PersistentFlags().Lookup("config"); flag != nil {
			path = flag.Value.String()
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return ""
	}
	if cmd != nil && cmd.Context() != nil && cmd.Root().PersistentFlags().Lookup("config") != nil {
		cmd.SetContext(context.WithValue(cmd.Context(), etsyQuotaContextKey{}, cfg.EtsyApiKey))
	}
	return cfg.EtsyApiKey
}
