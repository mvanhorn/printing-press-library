// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func budgetsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "openrouter-pp-cli", "budgets.json")
}

func loadBudgets() (map[string]float64, error) {
	out := map[string]float64{}
	raw, err := os.ReadFile(budgetsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]float64{}, err
	}
	return out, nil
}

func saveBudgets(b map[string]float64) error {
	p := budgetsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, buf, 0o644)
}

// parseBudgetUSD accepts "5usd", "5", "5.50", "$5".
func parseBudgetUSD(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "usd")
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Set and check per-cron weekly USD budgets against tool-call log spend",
	}
	cmd.AddCommand(newBudgetSetCmd(flags))
	cmd.AddCommand(newBudgetCheckCmd(flags))
	return cmd
}
