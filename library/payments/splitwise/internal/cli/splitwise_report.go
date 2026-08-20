package cli

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type reportOpts struct {
	GroupInput string
	Since      string
	Until      string
	Currency   string
	Limit      int
}

type reportPerson struct {
	UserID int     `json:"user_id"`
	Name   string  `json:"name"`
	Paid   float64 `json:"paid"`
	Owed   float64 `json:"owed"`
	Net    float64 `json:"net"`
}

type reportCategory struct {
	Name  string  `json:"name"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

type reportExpenseRow struct {
	ID           int     `json:"id"`
	Date         string  `json:"date"`
	Description  string  `json:"description"`
	Cost         float64 `json:"cost"`
	CurrencyCode string  `json:"currency_code"`
	Payer        string  `json:"payer"`
}

type reportResult struct {
	Scope                 string             `json:"scope"`
	Currency              string             `json:"currency"`
	PeriodStart           string             `json:"period_start"`
	PeriodEnd             string             `json:"period_end"`
	ExpenseCount          int                `json:"expense_count"`
	ExcludedOtherCurrency int                `json:"excluded_other_currency"`
	TotalCost             float64            `json:"total_cost"`
	YourPaid              float64            `json:"your_paid"`
	YourOwed              float64            `json:"your_owed"`
	YourNet               float64            `json:"your_net"`
	People                []reportPerson     `json:"people"`
	Categories            []reportCategory   `json:"categories"`
	Expenses              []reportExpenseRow `json:"expenses"`
	Truncated             bool               `json:"truncated"`
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	var groupRef string
	var since string
	var until string
	var currency string
	var format string
	limit := 100

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Export an offline trip/period spend report (md/csv/json; PDF out of scope in v1)",
		Example: "  splitwise-pp-cli report --group \"Tahoe Trip\" --format md\n" +
			"  splitwise-pp-cli report --since 2025-01-01 --until 2025-12-31 --format csv > 2025.csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute report")
				return nil
			}

			format = strings.ToLower(strings.TrimSpace(format))
			if format != "" && format != "md" && format != "csv" && format != "json" {
				return usageErr(fmt.Errorf("invalid --format value %q: must be md, csv, or json", format))
			}

			if strings.TrimSpace(since) != "" {
				if _, err := time.Parse("2006-01-02", strings.TrimSpace(since)); err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
			}
			if strings.TrimSpace(until) != "" {
				if _, err := time.Parse("2006-01-02", strings.TrimSpace(until)); err != nil {
					return usageErr(fmt.Errorf("invalid --until %q: %w", until, err))
				}
			}
			if strings.TrimSpace(since) != "" && strings.TrimSpace(until) != "" {
				sv, _ := time.Parse("2006-01-02", strings.TrimSpace(since))
				uv, _ := time.Parse("2006-01-02", strings.TrimSpace(until))
				if uv.Before(sv) {
					return usageErr(fmt.Errorf("--until must be on/after --since"))
				}
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-expenses")
			hintIfUnsynced(cmd, db, "get-groups")
			hintIfStale(cmd, db, "get-expenses", flags.maxAge)
			hintIfStale(cmd, db, "get-groups", flags.maxAge)

			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			groups, err := loadGroups(db)
			if err != nil {
				return err
			}
			youID := loadCurrentUserID(db)

			if strings.TrimSpace(groupRef) != "" {
				if _, _, ok := resolveReportGroup(groupRef, groups); !ok {
					return usageErr(fmt.Errorf("no group matches %q; run sync first", groupRef))
				}
			}

			res := computeReport(expenses, groups, youID, reportOpts{
				GroupInput: groupRef,
				Since:      since,
				Until:      until,
				Currency:   currency,
				Limit:      limit,
			})

			switch format {
			case "csv":
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderReportCSV(res))
				if res.Truncated {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
						"note: CSV truncated to %d of %d expense row(s); use --limit 0 for all rows\n",
						len(res.Expenses), res.ExpenseCount)
				}
				return nil
			case "md":
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderReportMarkdown(res))
				return nil
			case "json":
				return flags.printJSON(cmd, res)
			default:
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, res)
				}
				return printReportSummary(cmd, res)
			}
		},
	}

	cmd.Flags().StringVar(&groupRef, "group", "", "Group name or id")
	cmd.Flags().StringVar(&since, "since", "", "Include expenses on/after YYYY-MM-DD")
	cmd.Flags().StringVar(&until, "until", "", "Include expenses on/before YYYY-MM-DD")
	cmd.Flags().StringVar(&currency, "currency", "", "Single currency code (default picks most common and excludes others)")
	cmd.Flags().StringVar(&format, "format", "", "Output format: md|csv|json (PDF is out of scope in v1)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max expense rows in output (<=0 means all)")
	return cmd
}

// resolveReportGroup maps a --group reference (numeric id or exact, case-insensitive
// name) to its group id and display name. Self-contained so `report` does not depend on
// any other novel command's resolver; prefers an exact match and never substring-guesses.
func resolveReportGroup(input string, groups []Group) (int, string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", false
	}
	if isAllDigits(trimmed) {
		id, _ := strconv.Atoi(trimmed)
		for _, g := range groups {
			if g.ID == id {
				return g.ID, strings.TrimSpace(g.Name), true
			}
		}
		return 0, "", false
	}
	for _, g := range groups {
		if strings.EqualFold(strings.TrimSpace(g.Name), trimmed) {
			return g.ID, strings.TrimSpace(g.Name), true
		}
	}
	return 0, "", false
}

func computeReport(expenses []Expense, groups []Group, youID int, opts reportOpts) reportResult {
	res := reportResult{
		Scope:      "all",
		People:     make([]reportPerson, 0),
		Categories: make([]reportCategory, 0),
		Expenses:   make([]reportExpenseRow, 0),
	}

	var groupID int
	groupFilter := false
	if strings.TrimSpace(opts.GroupInput) != "" {
		id, name, ok := resolveReportGroup(opts.GroupInput, groups)
		groupFilter = true
		if ok {
			groupID = id
			res.Scope = "group:" + name
		} else {
			// GroupInput was given but matches no group. Filter to nothing rather
			// than silently falling through to an unfiltered report — no expense has
			// a negative group id, so this yields an empty, correctly-scoped report
			// even if this pure function is called without the command's
			// pre-validation. (Greptile #971)
			groupID = -1
			res.Scope = "group:" + strings.TrimSpace(opts.GroupInput) + " (no match)"
		}
	}

	var sinceT time.Time
	hasSince := false
	if strings.TrimSpace(opts.Since) != "" {
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(opts.Since)); err == nil {
			sinceT = t
			hasSince = true
		}
	}
	var untilT time.Time
	hasUntil := false
	if strings.TrimSpace(opts.Until) != "" {
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(opts.Until)); err == nil {
			untilT = t
			hasUntil = true
		}
	}

	filtered := make([]Expense, 0)
	for _, e := range expenses {
		if e.Payment || expenseDeleted(e.DeletedAt) {
			continue
		}
		if groupFilter && e.GroupID != groupID {
			continue
		}
		t, ok := parseSplitwiseDate(e.Date)
		if hasSince {
			if !ok || t.Before(sinceT) {
				continue
			}
		}
		if hasUntil {
			// --until is inclusive of the whole named day: extend the bound to just
			// before the next midnight so a same-day timestamp with a time component
			// (e.g. "2025-12-31T18:00") still matches.
			endOfUntil := untilT.Add(24*time.Hour - time.Nanosecond)
			if !ok || t.After(endOfUntil) {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	currency := strings.ToUpper(strings.TrimSpace(opts.Currency))
	if currency == "" {
		freq := make(map[string]int)
		for _, e := range filtered {
			cc := strings.ToUpper(strings.TrimSpace(e.CurrencyCode))
			if cc == "" {
				continue
			}
			freq[cc]++
		}
		maxCount := 0
		for cc, n := range freq {
			if n > maxCount || (n == maxCount && (currency == "" || cc < currency)) {
				currency = cc
				maxCount = n
			}
		}
	}
	res.Currency = currency

	kept := make([]Expense, 0, len(filtered))
	for _, e := range filtered {
		cc := strings.ToUpper(strings.TrimSpace(e.CurrencyCode))
		if currency != "" && cc != currency {
			res.ExcludedOtherCurrency++
			continue
		}
		kept = append(kept, e)
	}

	type personAgg struct {
		name string
		paid float64
		owed float64
	}
	type catAgg struct {
		total float64
		count int
	}
	people := make(map[int]*personAgg)
	cats := make(map[string]*catAgg)

	var minDate *time.Time
	var maxDate *time.Time

	for _, e := range kept {
		cost := round2(parseAmount(e.Cost))
		res.TotalCost = round2(res.TotalCost + cost)
		res.ExpenseCount++

		if t, ok := parseSplitwiseDate(e.Date); ok {
			td := t.UTC()
			if minDate == nil || td.Before(*minDate) {
				cpy := td
				minDate = &cpy
			}
			if maxDate == nil || td.After(*maxDate) {
				cpy := td
				maxDate = &cpy
			}
		}

		catName := strings.TrimSpace(e.Category.Name)
		if catName == "" {
			catName = "Uncategorized"
		}
		if _, ok := cats[catName]; !ok {
			cats[catName] = &catAgg{}
		}
		cats[catName].total = round2(cats[catName].total + cost)
		cats[catName].count++

		payerName := ""
		paidPositive := 0
		maxPaid := 0.0
		for _, u := range e.Users {
			paid := round2(parseAmount(u.PaidShare))
			owed := round2(parseAmount(u.OwedShare))
			name := strings.TrimSpace(strings.TrimSpace(u.User.FirstName) + " " + strings.TrimSpace(u.User.LastName))
			if name == "" {
				name = fmt.Sprintf("user %d", u.UserID)
			}
			if _, ok := people[u.UserID]; !ok {
				people[u.UserID] = &personAgg{name: name}
			}
			if people[u.UserID].name == "" {
				people[u.UserID].name = name
			}
			people[u.UserID].paid = round2(people[u.UserID].paid + paid)
			people[u.UserID].owed = round2(people[u.UserID].owed + owed)

			if u.UserID == youID {
				res.YourPaid = round2(res.YourPaid + paid)
				res.YourOwed = round2(res.YourOwed + owed)
			}

			if paid > 0 {
				paidPositive++
			}
			if paid > maxPaid {
				maxPaid = paid
				payerName = name
			}
		}
		if paidPositive >= 2 {
			payerName = "multiple"
		}
		if payerName == "" {
			// No participant has a positive paid_share — distinct from a genuine
			// 2+-payer split, so don't mislabel it "multiple".
			payerName = "-"
		}

		ds := strings.TrimSpace(e.Date)
		if t, ok := parseSplitwiseDate(e.Date); ok {
			ds = t.UTC().Format("2006-01-02")
		}
		res.Expenses = append(res.Expenses, reportExpenseRow{
			ID:           e.ID,
			Date:         ds,
			Description:  strings.TrimSpace(e.Description),
			Cost:         cost,
			CurrencyCode: strings.ToUpper(strings.TrimSpace(e.CurrencyCode)),
			Payer:        payerName,
		})
	}
	res.YourNet = round2(res.YourPaid - res.YourOwed)

	if minDate != nil {
		res.PeriodStart = minDate.Format("2006-01-02")
	}
	if maxDate != nil {
		res.PeriodEnd = maxDate.Format("2006-01-02")
	}

	for id, a := range people {
		name := strings.TrimSpace(a.name)
		if name == "" {
			name = fmt.Sprintf("user %d", id)
		}
		res.People = append(res.People, reportPerson{
			UserID: id,
			Name:   name,
			Paid:   round2(a.paid),
			Owed:   round2(a.owed),
			Net:    round2(a.paid - a.owed),
		})
	}
	sort.Slice(res.People, func(i, j int) bool {
		if res.People[i].Net != res.People[j].Net {
			return res.People[i].Net > res.People[j].Net
		}
		if res.People[i].Name != res.People[j].Name {
			return res.People[i].Name < res.People[j].Name
		}
		return res.People[i].UserID < res.People[j].UserID
	})

	for name, a := range cats {
		res.Categories = append(res.Categories, reportCategory{Name: name, Total: round2(a.total), Count: a.count})
	}
	sort.Slice(res.Categories, func(i, j int) bool {
		if res.Categories[i].Total != res.Categories[j].Total {
			return res.Categories[i].Total > res.Categories[j].Total
		}
		return res.Categories[i].Name < res.Categories[j].Name
	})

	sort.Slice(res.Expenses, func(i, j int) bool {
		if res.Expenses[i].Date != res.Expenses[j].Date {
			return res.Expenses[i].Date < res.Expenses[j].Date
		}
		return res.Expenses[i].ID < res.Expenses[j].ID
	})

	if opts.Limit > 0 && len(res.Expenses) > opts.Limit {
		res.Expenses = res.Expenses[:opts.Limit]
		res.Truncated = true
	}

	return res
}

func renderReportMarkdown(r reportResult) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "# Report — %s\n\n", r.Scope)
	_, _ = fmt.Fprintf(&b, "Period: %s to %s  | Currency: %s  | Expenses: %d  | Total: %.2f  | Your net: %.2f\n\n", valueOrDash(r.PeriodStart), valueOrDash(r.PeriodEnd), valueOrDash(r.Currency), r.ExpenseCount, r.TotalCost, r.YourNet)
	if r.ExcludedOtherCurrency > 0 {
		_, _ = fmt.Fprintf(&b, "Excluded %d expense(s) in other currencies.\n\n", r.ExcludedOtherCurrency)
	}

	b.WriteString("## By person\n\n")
	b.WriteString("| Name | Paid | Owed | Net |\n")
	b.WriteString("| --- | ---: | ---: | ---: |\n")
	for _, p := range r.People {
		_, _ = fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f |\n", mdCell(p.Name), p.Paid, p.Owed, p.Net)
	}
	b.WriteString("\n## By category\n\n")
	b.WriteString("| Category | Total | Count |\n")
	b.WriteString("| --- | ---: | ---: |\n")
	for _, c := range r.Categories {
		_, _ = fmt.Fprintf(&b, "| %s | %.2f | %d |\n", mdCell(c.Name), c.Total, c.Count)
	}
	b.WriteString("\n## Expenses\n\n")
	b.WriteString("| ID | Date | Description | Cost | Currency | Payer |\n")
	b.WriteString("| ---: | --- | --- | ---: | --- | --- |\n")
	for _, e := range r.Expenses {
		_, _ = fmt.Fprintf(&b, "| %d | %s | %s | %.2f | %s | %s |\n", e.ID, e.Date, mdCell(e.Description), e.Cost, e.CurrencyCode, mdCell(e.Payer))
	}
	if r.Truncated {
		_, _ = fmt.Fprintf(&b, "\nShowing first %d expense row(s) of %d.\n", len(r.Expenses), r.ExpenseCount)
	}
	return b.String()
}

// mdCell escapes a user-controlled value for a Markdown table cell: a literal
// pipe would inject an extra column and a newline would split the row. Splitwise
// descriptions frequently contain '|', so this keeps the rendered table well-formed.
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func renderReportCSV(r reportResult) string {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"id", "date", "description", "cost", "currency", "payer"})
	for _, e := range r.Expenses {
		_ = w.Write([]string{
			strconv.Itoa(e.ID),
			e.Date,
			e.Description,
			fmt.Sprintf("%.2f", e.Cost),
			e.CurrencyCode,
			e.Payer,
		})
	}
	w.Flush()
	return b.String()
}

func printReportSummary(cmd *cobra.Command, r reportResult) error {
	out := cmd.OutOrStdout()
	tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "FIELD\tVALUE")
	_, _ = fmt.Fprintf(tw, "scope\t%s\n", r.Scope)
	_, _ = fmt.Fprintf(tw, "period\t%s to %s\n", valueOrDash(r.PeriodStart), valueOrDash(r.PeriodEnd))
	_, _ = fmt.Fprintf(tw, "currency\t%s\n", valueOrDash(r.Currency))
	_, _ = fmt.Fprintf(tw, "expense_count\t%d\n", r.ExpenseCount)
	_, _ = fmt.Fprintf(tw, "total\t%.2f\n", r.TotalCost)
	_, _ = fmt.Fprintf(tw, "your_net\t%.2f\n", r.YourNet)
	if r.ExcludedOtherCurrency > 0 {
		_, _ = fmt.Fprintf(tw, "excluded_other_currency\t%d\n", r.ExcludedOtherCurrency)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(out)
	twPeople := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(twPeople, "PERSON\tPAID\tOWED\tNET")
	for i, p := range r.People {
		if i >= 8 {
			break
		}
		_, _ = fmt.Fprintf(twPeople, "%s\t%.2f\t%.2f\t%.2f\n", p.Name, p.Paid, p.Owed, p.Net)
	}
	if err := twPeople.Flush(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(out)
	twCat := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(twCat, "CATEGORY\tTOTAL\tCOUNT")
	for i, c := range r.Categories {
		if i >= 8 {
			break
		}
		_, _ = fmt.Fprintf(twCat, "%s\t%.2f\t%d\n", c.Name, c.Total, c.Count)
	}
	if err := twCat.Flush(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "\n%d expenses — use --format md for the full report\n", r.ExpenseCount)
	if r.Truncated {
		_, _ = fmt.Fprintf(out, "rows capped at %d; use --limit 0 for all rows\n", len(r.Expenses))
	}
	if r.ExcludedOtherCurrency > 0 {
		_, _ = fmt.Fprintf(out, "note: excluded %d expense(s) in other currencies\n", r.ExcludedOtherCurrency)
	}
	return nil
}

func valueOrDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}
