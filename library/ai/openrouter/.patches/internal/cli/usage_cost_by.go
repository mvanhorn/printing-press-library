// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type toolCallEntry struct {
	Tool      string  `json:"tool"`
	Caller    string  `json:"caller"`
	CronName  string  `json:"cron_name"`
	Cron      string  `json:"cron"`
	Model     string  `json:"model"`
	Provider  string  `json:"provider"`
	CostUSD   float64 `json:"cost_usd"`
	Cost      float64 `json:"cost"`
	Timestamp string  `json:"timestamp"`
	Ts        string  `json:"ts"`
	Time      string  `json:"time"`
}

func (e *toolCallEntry) groupValue(group string) string {
	switch group {
	case "cron":
		if e.CronName != "" {
			return e.CronName
		}
		if e.Cron != "" {
			return e.Cron
		}
		if e.Caller != "" {
			return e.Caller
		}
		return "unknown"
	case "caller":
		if e.Caller != "" {
			return e.Caller
		}
		return "unknown"
	case "model":
		if e.Model != "" {
			return e.Model
		}
		return "unknown"
	case "provider":
		if e.Provider != "" {
			return e.Provider
		}
		return "unknown"
	}
	return "unknown"
}

func (e *toolCallEntry) cost() float64 {
	if e.CostUSD != 0 {
		return e.CostUSD
	}
	return e.Cost
}

func (e *toolCallEntry) when() (time.Time, bool) {
	for _, s := range []string{e.Timestamp, e.Ts, e.Time} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, true
		}
		if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func toolCallLogDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".openclaw", "tool-call-log")
}

func loadToolCallEntries(since time.Time) ([]toolCallEntry, error) {
	dir := toolCallLogDir()
	out := []toolCallEntry{}
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return out, nil
	}
	for _, path := range matches {
		// Skip files older than since (filename is YYYY-MM-DD.jsonl)
		base := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if dt, err := time.Parse("2006-01-02", base); err == nil {
			if dt.Add(24 * time.Hour).Before(since) {
				continue
			}
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 || line[0] != '{' {
				continue
			}
			var e toolCallEntry
			if json.Unmarshal(line, &e) != nil {
				continue
			}
			if !since.IsZero() {
				if t, ok := e.when(); ok && t.Before(since) {
					continue
				}
			}
			out = append(out, e)
		}
		f.Close()
	}
	return out, nil
}

func newUsageCostByCmd(flags *rootFlags) *cobra.Command {
	var group, since string
	var llm bool
	var top int

	cmd := &cobra.Command{
		Use:         "cost-by",
		Short:       "Group OpenRouter spend by cron/caller/model/provider from local tool-call log",
		Example:     "  openrouter-pp-cli usage cost-by --group cron --since 7d --llm",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			validGroups := map[string]bool{"cron": true, "caller": true, "model": true, "provider": true}
			if !validGroups[group] {
				return usageErr(fmt.Errorf("invalid --group %q (cron|caller|model|provider)", group))
			}
			sinceT := time.Time{}
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(err)
				}
				sinceT = t
			}
			entries, err := loadToolCallEntries(sinceT)
			if err != nil {
				return err
			}

			type agg struct {
				Group string  `json:"group"`
				Cost  float64 `json:"cost_usd"`
				Calls int     `json:"calls"`
			}
			byKey := map[string]*agg{}
			for _, e := range entries {
				k := e.groupValue(group)
				a := byKey[k]
				if a == nil {
					a = &agg{Group: k}
					byKey[k] = a
				}
				a.Cost += e.cost()
				a.Calls++
			}
			rows := make([]agg, 0, len(byKey))
			for _, a := range byKey {
				rows = append(rows, *a)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Cost > rows[j].Cost })
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}

			if flags.asJSON {
				if rows == nil {
					rows = []agg{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if llm {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no data")
					return nil
				}
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s cost=$%.4f calls=%d\n", group, r.Group, r.Cost, r.Calls)
				}
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no data")
				return nil
			}
			headers := []string{strings.ToUpper(group), "COST", "CALLS"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.Group, fmt.Sprintf("$%.4f", r.Cost), fmt.Sprintf("%d", r.Calls)})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&group, "group", "cron", "Group by: cron|caller|model|provider")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window (e.g. 7d, 24h, 1w)")
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output for agents")
	cmd.Flags().IntVar(&top, "top", 0, "Limit to top N groups by cost")
	return cmd
}
