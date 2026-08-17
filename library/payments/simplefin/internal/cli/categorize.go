// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: rule-based transaction categorization. SimpleFIN has no native
// categories, so we assign them deterministically from user regex rules.
//
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

type catRule struct {
	Pattern  string `json:"pattern"`
	Field    string `json:"field"`
	Category string `json:"category"`
	Priority int    `json:"priority"`
	re       *regexp.Regexp
}

func newNovelCategorizeCmd(flags *rootFlags) *cobra.Command {
	var dbPath, rulesFile, add, field string
	var apply, listRules bool

	cmd := &cobra.Command{
		Use:   "categorize",
		Short: "Assign categories to transactions with deterministic keyword/regex rules (the protocol has none).",
		Long: "SimpleFIN ships no transaction categories. Define regex rules that map a transaction's\n" +
			"description/payee/memo to a category, then apply them across the local store. This is NOT\n" +
			"spending math — 'cashflow' reads the categories this command writes. Rules also run\n" +
			"automatically during 'sync' for new transactions.\n\n" +
			"  categorize                                show the current category breakdown\n" +
			"  categorize --add 'WHOLEFOODS=Groceries'   add a rule (regex matched against description)\n" +
			"  categorize --rules rules.json             load rules from a JSON file\n" +
			"  categorize --apply                        re-apply all rules to every transaction\n" +
			"  categorize --list-rules                   list configured rules",
		Example:     "  simplefin-pp-cli categorize --apply --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("simplefin-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureSimpleFINSchema(ctx); err != nil {
				return fmt.Errorf("ensuring schema: %w", err)
			}

			if add != "" {
				pat, cat, ok := strings.Cut(add, "=")
				pat = strings.TrimSpace(pat)
				cat = strings.TrimSpace(cat)
				if !ok || pat == "" || cat == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--add expects 'PATTERN=Category'"))
				}
				if _, err := regexp.Compile(pat); err != nil {
					return usageErr(fmt.Errorf("--add pattern is not a valid regex: %w", err))
				}
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO categorization_rules (pattern, field, category, priority, created_at) VALUES (?, ?, ?, ?, ?)`,
					pat, field, cat, 100, time.Now().Unix()); err != nil {
					return fmt.Errorf("adding rule: %w", err)
				}
				// Status lines go to stderr so stdout stays pure JSON in --json/--agent mode.
				fmt.Fprintf(cmd.ErrOrStderr(), "added rule: /%s/ on %s -> %s\n", pat, field, cat)
			}

			if rulesFile != "" {
				n, err := loadRulesFile(ctx, db, rulesFile)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "loaded %d rules from %s\n", n, rulesFile)
			}

			if listRules {
				rules, err := loadRules(ctx, db)
				if err != nil {
					return err
				}
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, rules)
				}
				if len(rules) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no rules configured (add one: categorize --add 'PATTERN=Category')")
					return nil
				}
				for _, r := range rules {
					fmt.Fprintf(cmd.OutOrStdout(), "[%d] /%s/ on %s -> %s\n", r.Priority, r.Pattern, r.Field, r.Category)
				}
				return nil
			}

			if apply {
				n, err := reapplyCategorizationRules(ctx, db)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "categorized %d transactions\n", n)
			}

			return printCategoryBreakdown(ctx, cmd, flags, db)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&add, "add", "", "Add a rule as 'PATTERN=Category' (PATTERN is a regex)")
	cmd.Flags().StringVar(&field, "field", "description", "Field a new rule matches: description, payee, memo, or any")
	cmd.Flags().StringVar(&rulesFile, "rules", "", "Load rules from a JSON file ([{pattern,field,category,priority}])")
	cmd.Flags().BoolVar(&apply, "apply", false, "Re-apply all rules to every transaction (keeps manual categories)")
	cmd.Flags().BoolVar(&listRules, "list-rules", false, "List configured categorization rules")
	return cmd
}

func loadRules(ctx context.Context, db *store.Store) ([]catRule, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT pattern, field, category, priority FROM categorization_rules ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	out := make([]catRule, 0)
	for rows.Next() {
		var r catRule
		if err := rows.Scan(&r.Pattern, &r.Field, &r.Category, &r.Priority); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if re, err := regexp.Compile("(?i)" + r.Pattern); err == nil {
			r.re = re
			out = append(out, r)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

func loadRulesFile(ctx context.Context, db *store.Store, path string) (int, error) {
	// #nosec G304 -- path is the user-supplied --rules-file argument; reading a
	// caller-chosen local file is the documented purpose of this command.
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading rules file: %w", err)
	}
	var rules []catRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return 0, fmt.Errorf("parsing rules file (expected JSON array of {pattern,field,category,priority}): %w", err)
	}
	n := 0
	for _, r := range rules {
		if strings.TrimSpace(r.Pattern) == "" || strings.TrimSpace(r.Category) == "" {
			continue
		}
		if _, err := regexp.Compile(r.Pattern); err != nil {
			continue
		}
		if r.Field == "" {
			r.Field = "description"
		}
		if r.Priority == 0 {
			r.Priority = 100
		}
		if _, err := db.DB().ExecContext(ctx,
			`INSERT INTO categorization_rules (pattern, field, category, priority, created_at) VALUES (?, ?, ?, ?, ?)`,
			r.Pattern, r.Field, r.Category, r.Priority, time.Now().Unix()); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func ruleMatchText(t storedTxn, field string) string {
	switch field {
	case "payee":
		return t.Payee
	case "memo":
		return t.Memo
	case "any":
		return t.Description + " " + t.Payee + " " + t.Memo
	default:
		return t.Description
	}
}

// applyCategorizationRules categorizes only transactions that have no category
// yet. Returns the number newly categorized. Called by sync.
func applyCategorizationRules(ctx context.Context, db *store.Store) (int, error) {
	return categorizeInternal(ctx, db, false)
}

// reapplyCategorizationRules re-applies rules to every transaction whose
// category is rule-sourced or absent, leaving manual categories untouched.
func reapplyCategorizationRules(ctx context.Context, db *store.Store) (int, error) {
	return categorizeInternal(ctx, db, true)
}

func categorizeInternal(ctx context.Context, db *store.Store, reapply bool) (int, error) {
	rules, err := loadRules(ctx, db)
	if err != nil {
		return 0, err
	}
	if len(rules) == 0 {
		return 0, nil
	}
	txns, err := loadTransactions(ctx, db, "", 0, 0, true)
	if err != nil {
		return 0, err
	}
	// Existing assignments + their source (drain-first; closed before writes).
	existing := map[string]string{}
	rows, err := db.DB().QueryContext(ctx, `SELECT txn_id, source FROM transaction_categories`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id, src string
		if err := rows.Scan(&id, &src); err != nil {
			_ = rows.Close()
			return 0, err
		}
		existing[id] = src
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	now := time.Now().Unix()
	n := 0
	for _, t := range txns {
		src, has := existing[t.ResourceID]
		if has {
			if !reapply || src == "manual" {
				continue
			}
		}
		for _, r := range rules {
			if r.re.MatchString(ruleMatchText(t, r.Field)) {
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO transaction_categories (txn_id, category, source, updated_at) VALUES (?, ?, 'rule', ?)
					 ON CONFLICT(txn_id) DO UPDATE SET category=excluded.category, source='rule', updated_at=excluded.updated_at`,
					t.ResourceID, r.Category, now); err != nil {
					return n, err
				}
				n++
				break
			}
		}
	}
	return n, nil
}

type categoryCount struct {
	Category string  `json:"category"`
	Count    int     `json:"count"`
	Total    float64 `json:"total"`
}

func printCategoryBreakdown(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *store.Store) error {
	txns, err := loadTransactions(ctx, db, "", 0, 0, true)
	if err != nil {
		return err
	}
	cats, err := loadCategoryMap(ctx, db)
	if err != nil {
		return err
	}
	agg := map[string]*categoryCount{}
	uncategorized := 0
	for _, t := range txns {
		cat, ok := cats[t.ResourceID]
		if !ok {
			uncategorized++
			cat = "(uncategorized)"
		}
		c := agg[cat]
		if c == nil {
			c = &categoryCount{Category: cat}
			agg[cat] = c
		}
		c.Count++
		if amt, ok2 := simplefin.ParseAmount(t.Amount); ok2 {
			c.Total += amt
		}
	}
	out := make([]categoryCount, 0, len(agg))
	for _, c := range agg {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })

	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, map[string]any{
			"categories":    out,
			"uncategorized": uncategorized,
			"total_txns":    len(txns),
		})
	}
	if len(txns) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no transactions in the local store — run: simplefin-pp-cli sync")
		return nil
	}
	for _, c := range out {
		fmt.Fprintf(cmd.OutOrStdout(), "%-24s %5d txns  %12.2f\n", c.Category, c.Count, c.Total)
	}
	return nil
}
