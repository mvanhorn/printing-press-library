// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense quick <prompt>` — one-line natural-language expense filing.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

// quickParsed is the intermediate representation pulled from a prompt.
type quickParsed struct {
	Amount   int // cents
	Merchant string
	Date     string // YYYY-MM-DD
	Comment  string
}

func newExpenseQuickCmd(flags *rootFlags) *cobra.Command {
	var overrideAmount int
	var overrideMerchant, overrideCategory, overridePolicy, overrideDate, overrideCurrency string
	var previewOnly bool
	cmd := &cobra.Command{
		Use:   "quick <prompt>",
		Short: "File an expense from a natural-language prompt",
		Long: `Parses a short phrase like "Dinner at Maya $42.50" or "Uber $24 this morning"
into amount / merchant / date, auto-fills category from prior expenses at the
same merchant, and calls RequestMoney.`,
		Example: `  expensify-pp-cli expense quick "Dinner at Maya $42.50"
  expensify-pp-cli expense quick "Uber $24 yesterday" --category Travel`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")
			parsed := parseQuickPrompt(prompt)

			// Apply overrides
			if overrideAmount > 0 {
				parsed.Amount = overrideAmount
			}
			if overrideMerchant != "" {
				parsed.Merchant = overrideMerchant
			}
			if overrideDate != "" {
				parsed.Date = overrideDate
			}
			if parsed.Date == "" {
				parsed.Date = time.Now().Format("2006-01-02")
			}
			if parsed.Amount == 0 {
				return usageErr(fmt.Errorf("could not parse amount from %q — pass --amount cents", prompt))
			}
			if parsed.Merchant == "" {
				return usageErr(fmt.Errorf("could not parse merchant from %q — pass --merchant", prompt))
			}

			// Category resolution order:
			//   1. --category override
			//   2. prior expense at same merchant (learns from your history)
			//   3. built-in merchant heuristics (Uber -> Transportation, etc.)
			//   4. Uncategorized
			category := overrideCategory
			if category == "" {
				st, err := store.Open("")
				if err == nil {
					defer st.Close()
					if prev, _ := st.LastCategoryForMerchant(parsed.Merchant); prev != "" {
						category = prev
					}
				}
			}
			if category == "" {
				category = suggestCategoryForMerchant(parsed.Merchant)
			}
			if category == "" {
				category = "Uncategorized"
			}

			currency := overrideCurrency
			if currency == "" {
				currency = "USD"
			}

			body := map[string]any{
				"amount":   parsed.Amount,
				"merchant": parsed.Merchant,
				"category": category,
				"currency": currency,
				"created":  parsed.Date,
			}
			if overridePolicy != "" {
				body["policyID"] = overridePolicy
			}
			if parsed.Comment != "" {
				body["comment"] = parsed.Comment
			}

			if previewOnly || flags.dryRun {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "DRY RUN: would create expense\n")
				fmt.Fprintf(w, "  merchant: %s\n", parsed.Merchant)
				fmt.Fprintf(w, "  amount:   %d cents (%s %.2f)\n", parsed.Amount, currency, float64(parsed.Amount)/100)
				fmt.Fprintf(w, "  category: %s (auto-suggested)\n", category)
				fmt.Fprintf(w, "  date:     %s\n", parsed.Date)
				if overridePolicy != "" {
					fmt.Fprintf(w, "  policyID: %s\n", overridePolicy)
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(cmd.Context(), "/RequestMoney", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			id := extractTransactionID(data)
			w := cmd.OutOrStdout()
			if id != "" {
				fmt.Fprintf(w, "Created expense %s (HTTP %d) — %s $%.2f, category=%s\n",
					id, status, parsed.Merchant, float64(parsed.Amount)/100, category)
			} else {
				fmt.Fprintf(w, "Created expense (HTTP %d) — %s $%.2f, category=%s\n",
					status, parsed.Merchant, float64(parsed.Amount)/100, category)
			}
			if flags.asJSON {
				fmt.Fprintln(w, string(data))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&overrideAmount, "amount", 0, "Amount in cents (overrides prompt)")
	cmd.Flags().StringVar(&overrideMerchant, "merchant", "", "Merchant (overrides prompt)")
	cmd.Flags().StringVar(&overrideCategory, "category", "", "Category (overrides lookup)")
	cmd.Flags().StringVar(&overridePolicy, "policy", "", "Workspace/policy ID")
	cmd.Flags().StringVar(&overrideDate, "date", "", "Date YYYY-MM-DD")
	cmd.Flags().StringVar(&overrideCurrency, "currency", "", "Currency (default USD)")
	cmd.Flags().BoolVar(&previewOnly, "dry-run", false, "Preview the parsed expense without sending")
	return cmd
}

var (
	reQuickAmount   = regexp.MustCompile(`\$?(\d+(?:\.\d{1,2})?)`)
	reQuickAt       = regexp.MustCompile(`(?i)\b(?:at|from)\s+([A-Z][A-Za-z0-9&'\-\. ]{1,40}?)(?:\s+\$|\s+on|\s+for|\s+yesterday|\s+today|\s+this|\s+last|\s+[\d\$]|$)`)
	reQuickCapWords = regexp.MustCompile(`\b([A-Z][A-Za-z0-9&'\-\.]+(?:\s+[A-Z][A-Za-z0-9&'\-\.]+){0,3})\b`)
)

// parseQuickPrompt extracts amount / merchant / date / comment from a prompt.
func parseQuickPrompt(s string) quickParsed {
	var p quickParsed
	p.Comment = s

	// Amount: look for "$42.50" or "42" — prefer dollars; if no decimal, assume whole dollars.
	if m := reQuickAmount.FindStringSubmatch(s); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			p.Amount = int(v*100 + 0.5)
		}
	}

	// Merchant: "at <Words>" wins; else first capitalized cluster.
	if m := reQuickAt.FindStringSubmatch(s); m != nil {
		p.Merchant = strings.TrimSpace(m[1])
	}
	if p.Merchant == "" {
		for _, m := range reQuickCapWords.FindAllStringSubmatch(s, -1) {
			candidate := strings.TrimSpace(m[1])
			lower := strings.ToLower(candidate)
			// Skip day-of-week / time words
			if isCommonTimeWord(lower) {
				continue
			}
			p.Merchant = candidate
			break
		}
	}

	// Date: parse "yesterday" / "today" / "last tuesday" / ISO date.
	lower := strings.ToLower(s)
	now := time.Now()
	switch {
	case strings.Contains(lower, "yesterday"):
		p.Date = now.AddDate(0, 0, -1).Format("2006-01-02")
	case strings.Contains(lower, "today") || strings.Contains(lower, "this morning") || strings.Contains(lower, "tonight") || strings.Contains(lower, "this afternoon") || strings.Contains(lower, "this evening"):
		p.Date = now.Format("2006-01-02")
	case strings.Contains(lower, "last "):
		for i := 0; i < 7; i++ {
			candidate := now.AddDate(0, 0, -i)
			name := strings.ToLower(candidate.Weekday().String())
			if strings.Contains(lower, "last "+name) {
				p.Date = candidate.AddDate(0, 0, -7).Format("2006-01-02")
				break
			}
		}
	}
	// ISO date anywhere in text
	if p.Date == "" {
		if m := regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`).FindStringSubmatch(s); m != nil {
			p.Date = m[1]
		}
	}
	return p
}

// merchantCategoryHints maps common merchant substrings to reasonable default
// categories. Purely local — no network call, no ML. Order matters: first
// matching substring wins. These serve only as a cold-start fallback when the
// local expense history has no prior entry for the merchant.
var merchantCategoryHints = []struct {
	match    string
	category string
}{
	// Transportation
	{"uber", "Transportation"},
	{"lyft", "Transportation"},
	{"taxi", "Transportation"},
	{"amtrak", "Transportation"},
	{"delta", "Travel"},
	{"united", "Travel"},
	{"american air", "Travel"},
	{"southwest", "Travel"},
	{"alaska air", "Travel"},
	{"jetblue", "Travel"},
	// Lodging
	{"marriott", "Lodging"},
	{"hilton", "Lodging"},
	{"hyatt", "Lodging"},
	{"airbnb", "Lodging"},
	{"four seasons", "Lodging"},
	// Meals
	{"starbucks", "Meals"},
	{"dunkin", "Meals"},
	{"blue bottle", "Meals"},
	{"doordash", "Meals"},
	{"grubhub", "Meals"},
	{"ubereats", "Meals"},
	{"uber eats", "Meals"},
	{"chipotle", "Meals"},
	{"shake shack", "Meals"},
	{"mcdonald", "Meals"},
	// Software / SaaS
	{"github", "Software"},
	{"vercel", "Software"},
	{"openai", "Software"},
	{"anthropic", "Software"},
	{"aws", "Software"},
	{"google cloud", "Software"},
	{"datadog", "Software"},
	{"notion", "Software"},
	{"slack", "Software"},
	// Office
	{"staples", "Office Supplies"},
	{"office depot", "Office Supplies"},
	{"amazon", "Office Supplies"},
}

// suggestCategoryForMerchant returns a built-in category guess for a merchant
// name, or "" if no hint matches. Case-insensitive substring match.
func suggestCategoryForMerchant(merchant string) string {
	if merchant == "" {
		return ""
	}
	lower := strings.ToLower(merchant)
	for _, h := range merchantCategoryHints {
		if strings.Contains(lower, h.match) {
			return h.category
		}
	}
	return ""
}

func isCommonTimeWord(w string) bool {
	switch w {
	case "yesterday", "today", "tonight", "tomorrow", "this", "last", "morning", "afternoon", "evening":
		return true
	case "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		return true
	}
	return false
}

func extractTransactionID(data json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	for _, k := range []string{"transactionID", "transaction_id", "id"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
		if v, ok := m[k].(float64); ok && v != 0 {
			return fmt.Sprintf("%d", int64(v))
		}
	}
	return ""
}

// keep os import used
var _ = os.Stderr
