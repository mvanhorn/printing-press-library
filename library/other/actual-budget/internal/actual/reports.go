// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

// Read-only ledger queries and report aggregations used by the `ledger` and
// `report` command groups. Everything here works on leaf transactions (split
// parents excluded) so a split is never double-counted, and treats money as
// integer minor units.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// ---------------------------------------------------------------------------
// Month helpers

// YMFromDate returns the YYYYMM month of a YYYYMMDD date.
func YMFromDate(d int) int { return d / 100 }

// FormatYM renders a YYYYMM month as YYYY-MM.
func FormatYM(ym int) string { return fmt.Sprintf("%04d-%02d", ym/100, ym%100) }

func ymTime(ym int) time.Time {
	return time.Date(ym/100, time.Month(ym%100), 1, 0, 0, 0, 0, time.UTC)
}

// MonthEnd returns the last YYYYMMDD day of a YYYYMM month.
func MonthEnd(ym int) int { return DateInt(ymTime(ym).AddDate(0, 1, -1)) }

// MonthStart returns the first YYYYMMDD day of a YYYYMM month.
func MonthStart(ym int) int { return ym*100 + 1 }

// AddMonths shifts a YYYYMM month by n months.
func AddMonths(ym, n int) int {
	t := ymTime(ym).AddDate(0, n, 0)
	return t.Year()*100 + int(t.Month())
}

// MonthsBetween lists every YYYYMM month touched by the inclusive YYYYMMDD
// range, oldest first. Empty when either bound is missing or from > to.
func MonthsBetween(from, to int) []int {
	out := make([]int, 0)
	if from <= 0 || to <= 0 || from > to {
		return out
	}
	for ym := YMFromDate(from); ym <= YMFromDate(to); ym = AddMonths(ym, 1) {
		out = append(out, ym)
	}
	return out
}

// ---------------------------------------------------------------------------
// Shared report scope

// ReportScope narrows report inputs. From/To are inclusive YYYYMMDD bounds.
type ReportScope struct {
	From, To         int
	AccountID        string
	IncludeOffBudget bool
}

// startingBalanceIDs returns ids of starting-balance transactions, which are
// neither income nor spending.
func (l *Ledger) startingBalanceIDs(ctx context.Context) (map[string]bool, error) {
	out := map[string]bool{}
	if !l.HasColumn(ctx, "transactions", "starting_balance_flag") {
		return out, nil
	}
	rows, err := l.DB.QueryContext(ctx, "SELECT id FROM transactions WHERE COALESCE(starting_balance_flag, 0) = 1")
	if err != nil {
		return nil, fmt.Errorf("querying starting balances: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// scopeTxns returns leaf transactions in scope plus the starting-balance set.
func (l *Ledger) scopeTxns(ctx context.Context, s ReportScope) ([]Txn, map[string]bool, error) {
	txns, err := l.Txns(ctx, TxnFilter{From: s.From, To: s.To, AccountID: s.AccountID, OnBudgetOnly: !s.IncludeOffBudget, Leaves: true})
	if err != nil {
		return nil, nil, err
	}
	sb, err := l.startingBalanceIDs(ctx)
	if err != nil {
		return nil, nil, err
	}
	return txns, sb, nil
}

// isSpend reports whether t is an outflow that counts as spending: negative,
// not in an income category, not a starting balance, and not an uncategorized
// transfer. Like Actual, a categorized transfer (on-budget -> off-budget) is
// budget activity and therefore spending; on-budget <-> on-budget transfers
// never carry a category.
func isSpend(t Txn, starting map[string]bool) bool {
	return t.Amount < 0 && !t.IsIncome && !starting[t.ID] && (t.TransferAccountID == "" || t.CategoryID != "")
}

// isRefund reports whether t is an inflow into an expense category (a refund
// or categorized transfer back from off-budget), which reduces net spending.
func isRefund(t Txn, starting map[string]bool) bool {
	return t.Amount > 0 && t.CategoryID != "" && !t.IsIncome && !starting[t.ID]
}

// spendAmount is t's contribution to net spending (positive = spent): the
// outflow for a spend, minus the inflow for a refund, and ok=false for
// anything else. Spending, Trends, Cashflow, BudgetVsActual and template
// status all count a category's spending this way.
func spendAmount(t Txn, starting map[string]bool) (int64, bool) {
	if isSpend(t, starting) || isRefund(t, starting) {
		return -t.Amount, true
	}
	return 0, false
}

// isIncomeTxn reports whether t counts as income: not a transfer or starting
// balance, and either in an income category (any sign, so a paycheck
// reversal reduces income) or an uncategorized inflow.
func isIncomeTxn(t Txn, starting map[string]bool) bool {
	if t.Amount == 0 || t.TransferAccountID != "" || starting[t.ID] {
		return false
	}
	return t.IsIncome || (t.CategoryID == "" && t.Amount > 0)
}

// UncategorizedName labels the bucket for transactions without a category.
const UncategorizedName = "Uncategorized"

// ---------------------------------------------------------------------------
// Search

// SearchTxns returns live leaf transactions whose payee, imported payee,
// notes, category or account contain every whitespace-separated term of query
// (case-insensitive). limit <= 0 means no limit.
func (l *Ledger) SearchTxns(ctx context.Context, query string, f TxnFilter, limit int) ([]Txn, error) {
	terms := strings.Fields(strings.ToLower(query))
	out := make([]Txn, 0)
	if len(terms) == 0 {
		return out, nil
	}
	f.Leaves = true
	f.Limit = 0
	txns, err := l.Txns(ctx, f)
	if err != nil {
		return nil, err
	}
	for _, t := range txns {
		hay := strings.ToLower(strings.Join([]string{t.Payee, t.ImportedDescription, t.Notes, t.Category, t.Account}, "\x00"))
		match := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				match = false
				break
			}
		}
		if match {
			out = append(out, t)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Duplicates

// DuplicateOptions configures FindDuplicates.
type DuplicateOptions struct {
	Days          int
	From, To      int
	AccountID     string
	MinConfidence float64
}

// DuplicatePair is one likely double-entered transaction pair.
type DuplicatePair struct {
	AID        string  `json:"a_id"`
	BID        string  `json:"b_id"`
	AccountID  string  `json:"account_id"`
	Account    string  `json:"account"`
	Amount     int64   `json:"amount"`
	ADate      string  `json:"a_date"`
	BDate      string  `json:"b_date"`
	Payee      string  `json:"payee"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// NormalizeText lowercases s and keeps only letters and digits.
func NormalizeText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TextSimilarity is 1 - normalized Levenshtein distance of the normalized
// forms of a and b (0..1). Two empty strings score 0.
func TextSimilarity(a, b string) float64 {
	x, y := NormalizeText(a), NormalizeText(b)
	if x == "" || y == "" {
		return 0
	}
	return LevenshteinSimilarity(x, y)
}

// daysBetween is the absolute number of days between two YYYYMMDD dates.
func daysBetween(a, b int) int {
	d := int(DateTime(b).Sub(DateTime(a)).Hours() / 24)
	if d < 0 {
		d = -d
	}
	return d
}

// Recurring-series detection for FindDuplicates: pairs at least
// minRecurringGapDays apart that repeat at the same interval lose
// recurringPenalty confidence, which drops them below the default threshold.
const (
	minRecurringGapDays = 5
	recurringPenalty    = 0.3
)

// recurringSlackDays absorbs calendar drift in a series (monthly charges are
// 28-31 days apart, weekly ones can slip a weekend).
const recurringSlackDays = 3

// chargeNear reports whether onDate has a charge within recurringSlackDays
// of day.
func chargeNear(onDate map[int]bool, day time.Time) bool {
	for d := -recurringSlackDays; d <= recurringSlackDays; d++ {
		if onDate[DateInt(day.AddDate(0, 0, d))] {
			return true
		}
	}
	return false
}

// FindDuplicates returns likely duplicate transaction pairs: same account,
// same amount, dates within opts.Days, and a matching payee (same payee id,
// equal normalized imported text, or payee-name similarity >= 0.8). Split
// parents/children, transfers and starting balances are never paired.
// Sorted by confidence descending.
func (l *Ledger) FindDuplicates(ctx context.Context, opts DuplicateOptions) ([]DuplicatePair, error) {
	out := make([]DuplicatePair, 0)
	if opts.Days < 0 {
		return out, nil
	}
	from, to := opts.From, opts.To
	// Widen the query window so pairs straddling the range edge still match.
	if from > 0 {
		from = DateInt(DateTime(from).AddDate(0, 0, -opts.Days))
	}
	if to > 0 {
		to = DateInt(DateTime(to).AddDate(0, 0, opts.Days))
	}
	txns, err := l.Txns(ctx, TxnFilter{From: from, To: to, AccountID: opts.AccountID, Leaves: true})
	if err != nil {
		return nil, err
	}
	starting, err := l.startingBalanceIDs(ctx)
	if err != nil {
		return nil, err
	}
	type key struct {
		acct string
		amt  int64
	}
	groups := map[key][]Txn{}
	for _, t := range txns {
		if t.IsChild || t.IsParent || t.TransferAccountID != "" || t.TransferID != "" || starting[t.ID] || t.Amount == 0 {
			continue
		}
		k := key{t.AccountID, t.Amount}
		groups[k] = append(groups[k], t)
	}
	inRange := func(d int) bool {
		return (opts.From <= 0 || d >= opts.From) && (opts.To <= 0 || d <= opts.To)
	}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		sort.Slice(g, func(i, j int) bool {
			if g[i].DateInt != g[j].DateInt {
				return g[i].DateInt < g[j].DateInt
			}
			return g[i].ID < g[j].ID
		})
		onDate := map[int]bool{}
		for _, t := range g {
			onDate[t.DateInt] = true
		}
		for i := 0; i < len(g); i++ {
			for j := i + 1; j < len(g); j++ {
				a, b := g[i], g[j]
				diff := daysBetween(a.DateInt, b.DateInt)
				if diff > opts.Days {
					break
				}
				if !inRange(a.DateInt) && !inRange(b.DateInt) {
					continue
				}
				var reasons []string
				payeeScore := 0.0
				if a.PayeeID != "" && a.PayeeID == b.PayeeID {
					payeeScore = 1
					reasons = append(reasons, "same payee")
				}
				na, nb := NormalizeText(a.ImportedDescription), NormalizeText(b.ImportedDescription)
				importedSame := na != "" && na == nb
				if importedSame {
					payeeScore = 1
					reasons = append(reasons, "same imported text")
				}
				if payeeScore == 0 {
					if s := TextSimilarity(a.Payee, b.Payee); s >= 0.8 {
						payeeScore = s
						reasons = append(reasons, fmt.Sprintf("similar payee names (%.2f)", s))
					}
				}
				if payeeScore == 0 {
					continue
				}
				conf := 0.5 + 0.3*payeeScore + 0.15*(1-float64(diff)/float64(opts.Days+1))
				if a.PayeeID != "" && a.PayeeID == b.PayeeID && importedSame {
					conf += 0.05
				}
				// A third charge at the same interval before a or after b
				// makes this a recurring series (weekly bill, standing
				// transfer), not a double entry.
				recurring := diff >= minRecurringGapDays &&
					(chargeNear(onDate, DateTime(a.DateInt).AddDate(0, 0, -diff)) || chargeNear(onDate, DateTime(b.DateInt).AddDate(0, 0, diff)) ||
						(diff >= 2*minRecurringGapDays && chargeNear(onDate, DateTime(a.DateInt).AddDate(0, 0, diff/2))))
				if recurring {
					conf -= recurringPenalty
					reasons = append(reasons, fmt.Sprintf("looks recurring (repeats every %d days)", diff))
				}
				conf = math.Min(1, math.Round(conf*100)/100)
				if conf < opts.MinConfidence {
					continue
				}
				if diff == 0 {
					reasons = append(reasons, "same date")
				} else {
					reasons = append(reasons, fmt.Sprintf("%d day(s) apart", diff))
				}
				payee := a.Payee
				if payee == "" {
					payee = a.ImportedDescription
				}
				out = append(out, DuplicatePair{AID: a.ID, BID: b.ID, AccountID: a.AccountID, Account: a.Account, Amount: a.Amount,
					ADate: a.Date, BDate: b.Date, Payee: payee, Confidence: conf, Reason: "same account and amount; " + strings.Join(reasons, "; ")})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if out[i].ADate != out[j].ADate {
			return out[i].ADate > out[j].ADate
		}
		return out[i].AID < out[j].AID
	})
	return out, nil
}

// DateTime converts a YYYYMMDD integer date to UTC midnight.
func DateTime(d int) time.Time {
	return time.Date(d/10000, time.Month(d/100%100), d%100, 0, 0, 0, 0, time.UTC)
}

// ---------------------------------------------------------------------------
// Read-only SQL

var (
	sqlPragmaInfo = regexp.MustCompile(`(?is)^pragma\s+table_x?info\s*\(`)
	sqlFirstWord  = regexp.MustCompile(`^[A-Za-z]+`)
)

// ErrNotReadOnlySQL marks SQL rejected by CheckReadOnlySQL.
var ErrNotReadOnlySQL = errors.New("only a single SELECT, WITH or PRAGMA table_info statement is allowed")

// sqlDeniedWord matches keywords that can write files, attach databases or
// flip connection state. query_only cannot be relied on alone because a
// PRAGMA statement can turn it off, so these are rejected anywhere outside
// literals and comments. (DML inside a single WITH statement is already
// stopped by the read-only connection.)
var sqlDeniedWord = regexp.MustCompile(`(?i)\b(attach|detach|vacuum|pragma|load_extension)\b`)

// CheckReadOnlySQL validates that q is a single read-only statement and
// returns the statement to run (comments removed, trailing semicolons
// trimmed) plus whether it is a SELECT/WITH query that can be wrapped with a
// LIMIT.
func CheckReadOnlySQL(q string) (string, bool, error) {
	run, scan, ok := splitSQL(q)
	if !ok {
		return "", false, fmt.Errorf("unterminated string, identifier or comment: %w", ErrNotReadOnlySQL)
	}
	scan = strings.TrimSpace(scan)
	run = strings.TrimSpace(run)
	for strings.HasSuffix(scan, ";") {
		scan = strings.TrimSpace(strings.TrimSuffix(scan, ";"))
		run = strings.TrimSpace(strings.TrimSuffix(run, ";"))
	}
	if scan == "" {
		return "", false, fmt.Errorf("empty query: %w", ErrNotReadOnlySQL)
	}
	if strings.Contains(scan, ";") {
		return "", false, fmt.Errorf("multiple statements: %w", ErrNotReadOnlySQL)
	}
	// SQLite reads $name(...), @name, :name, #name and ? as bind parameters,
	// and a $/@/:/# token runs to the next ')' or space, swallowing quotes
	// that splitSQL would take for a string. ledger sql never binds
	// arguments, so reject parameter syntax rather than model that rule.
	if strings.ContainsAny(scan, "$@:#?") {
		return "", false, fmt.Errorf("bind parameters ($, @, :, #, ?) are not supported: %w", ErrNotReadOnlySQL)
	}
	switch strings.ToLower(sqlFirstWord.FindString(scan)) {
	case "select", "with":
		if w := sqlDeniedWord.FindString(scan); w != "" {
			return "", false, fmt.Errorf("keyword %s is not allowed: %w", strings.ToUpper(w), ErrNotReadOnlySQL)
		}
		return run, true, nil
	case "pragma":
		rest := sqlPragmaInfo.ReplaceAllString(scan, "")
		if rest != scan && !sqlDeniedWord.MatchString(rest) {
			return run, false, nil
		}
	}
	return "", false, ErrNotReadOnlySQL
}

// splitSQL follows SQLite's lexer (-- and /* */ comments, single-quoted
// strings with doubled-quote escapes, "...", `...` and [...]) and returns two copies of s: run, with
// comments replaced by a space so the text that executes is exactly the text
// that was checked, and scan, which additionally blanks the inside of every
// literal and quoted identifier. ok is false when a literal or block comment
// is left open.
func splitSQL(s string) (run, scan string, ok bool) {
	var r, c strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '-' && i+1 < len(s) && s[i+1] == '-':
			j := strings.IndexByte(s[i:], '\n')
			if j < 0 {
				return r.String(), c.String(), true
			}
			r.WriteByte(' ')
			c.WriteByte(' ')
			i += j
		case ch == '/' && i+1 < len(s) && s[i+1] == '*':
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				return "", "", false
			}
			r.WriteByte(' ')
			c.WriteByte(' ')
			i += 2 + j + 1
		case ch == '\'' || ch == '"' || ch == '`' || ch == '[':
			closer := ch
			if ch == '[' {
				closer = ']'
			}
			j := i + 1
			for ; j < len(s); j++ {
				if s[j] != closer {
					continue
				}
				if closer != ']' && j+1 < len(s) && s[j+1] == closer {
					j++
					continue
				}
				break
			}
			if j >= len(s) {
				return "", "", false
			}
			r.WriteString(s[i : j+1])
			c.WriteByte(ch)
			c.WriteString(strings.Repeat(" ", j-i-1))
			c.WriteByte(closer)
			i = j
		default:
			r.WriteByte(ch)
			c.WriteByte(ch)
		}
	}
	return r.String(), c.String(), true
}

// QueryRows runs a read-only statement and returns each row as a
// column -> value map ([]byte converted to string), plus the column order.
func (l *Ledger) QueryRows(ctx context.Context, q string, args ...any) ([]map[string]any, []string, error) {
	rows, err := l.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	out := make([]map[string]any, 0)
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if bs, ok := vals[i].([]byte); ok {
				m[c] = string(bs)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out, cols, rows.Err()
}

// ---------------------------------------------------------------------------
// Spending

// SpendingRow is one bucket of a spending report.
type SpendingRow struct {
	Name  string  `json:"name"`
	ID    string  `json:"id"`
	Spent int64   `json:"spent"`
	Count int     `json:"count"`
	Share float64 `json:"share"`
}

// SpendingResult is a spending report: rows sorted by spent descending.
type SpendingResult struct {
	From  string        `json:"from"`
	To    string        `json:"to"`
	By    string        `json:"by"`
	Total int64         `json:"total"`
	Rows  []SpendingRow `json:"rows"`
}

// Spending groups net spending by "category" (default), "payee" or "group":
// outflows (negative leaf amounts, no uncategorized transfers, no income
// categories, no starting balances) minus refunds (inflows into an expense
// category), the same rule as Cashflow and BudgetVsActual. Spent is positive
// minor units; a bucket dominated by refunds can be negative. Count is the
// number of contributing transactions (outflows and refunds).
func (l *Ledger) Spending(ctx context.Context, s ReportScope, by string) (SpendingResult, error) {
	if by == "" {
		by = "category"
	}
	res := SpendingResult{From: FormatDate(s.From), To: FormatDate(s.To), By: by, Rows: make([]SpendingRow, 0)}
	if by != "category" && by != "payee" && by != "group" {
		return res, fmt.Errorf("invalid grouping %q (want category, payee or group)", by)
	}
	txns, starting, err := l.scopeTxns(ctx, s)
	if err != nil {
		return res, err
	}
	groupOf := map[string]string{}
	if by == "group" {
		cats, err := l.Categories(ctx)
		if err != nil {
			return res, err
		}
		for _, c := range cats {
			groupOf[c.ID] = c.GroupID
		}
	}
	idx := map[string]int{}
	for _, t := range txns {
		spent, ok := spendAmount(t, starting)
		if !ok {
			continue
		}
		var id, name string
		switch by {
		case "category":
			id, name = t.CategoryID, t.Category
			if id == "" {
				name = UncategorizedName
			}
		case "payee":
			id, name = t.PayeeID, t.Payee
			if name == "" {
				name = "(no payee)"
			}
		case "group":
			id, name = groupOf[t.CategoryID], t.CategoryGroup
			if t.CategoryID == "" {
				id, name = "", UncategorizedName
			}
		}
		i, ok := idx[id]
		if !ok {
			i = len(res.Rows)
			idx[id] = i
			res.Rows = append(res.Rows, SpendingRow{Name: name, ID: id})
		}
		res.Rows[i].Spent += spent
		res.Rows[i].Count++
		res.Total += spent
	}
	for i := range res.Rows {
		if res.Total > 0 {
			res.Rows[i].Share = math.Round(float64(res.Rows[i].Spent)/float64(res.Total)*10000) / 10000
		}
	}
	sort.SliceStable(res.Rows, func(i, j int) bool {
		if res.Rows[i].Spent != res.Rows[j].Spent {
			return res.Rows[i].Spent > res.Rows[j].Spent
		}
		return res.Rows[i].Name < res.Rows[j].Name
	})
	return res, nil
}

// ---------------------------------------------------------------------------
// Cashflow

// CashflowMonth is one month of income vs spending.
type CashflowMonth struct {
	Month    string `json:"month"`
	Income   int64  `json:"income"`
	Spending int64  `json:"spending"`
	Net      int64  `json:"net"`
}

// Cashflow returns income, spending and net per month across the scope range.
// Spending is net activity in expense categories: outflows minus refunds
// (positive amounts in a non-income category), so it can be negative in a
// month dominated by refunds.
func (l *Ledger) Cashflow(ctx context.Context, s ReportScope) ([]CashflowMonth, error) {
	months := MonthsBetween(s.From, s.To)
	out := make([]CashflowMonth, len(months))
	pos := map[int]int{}
	for i, ym := range months {
		out[i].Month = FormatYM(ym)
		pos[ym] = i
	}
	txns, starting, err := l.scopeTxns(ctx, s)
	if err != nil {
		return nil, err
	}
	for _, t := range txns {
		i, ok := pos[YMFromDate(t.DateInt)]
		if !ok {
			continue
		}
		if isIncomeTxn(t, starting) {
			out[i].Income += t.Amount
		} else if spent, ok := spendAmount(t, starting); ok {
			out[i].Spending += spent
		}
	}
	for i := range out {
		out[i].Net = out[i].Income - out[i].Spending
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Budget vs actual

// BudgetLine compares one category's budget with what was spent.
type BudgetLine struct {
	CategoryID string `json:"category_id"`
	Category   string `json:"category"`
	Group      string `json:"group"`
	Budgeted   int64  `json:"budgeted"`
	Spent      int64  `json:"spent"`
	Remaining  int64  `json:"remaining"`
	OverBudget bool   `json:"over_budget"`
}

// BudgetVsActualResult is the budget-vs-actual report for one month.
type BudgetVsActualResult struct {
	Month          string       `json:"month"`
	TotalBudgeted  int64        `json:"total_budgeted"`
	TotalSpent     int64        `json:"total_spent"`
	TotalRemaining int64        `json:"total_remaining"`
	Rows           []BudgetLine `json:"rows"`
}

// BudgetVsActual compares each expense category's budgeted amount for ym
// (YYYYMM) with its activity (spent = -sum of leaf amounts, so refunds reduce
// it) on on-budget accounts. Uncategorized rows (including on-budget <->
// on-budget transfers) and starting balances are excluded; categorized
// transfers to off-budget accounts count, as in Actual. Budgeted amounts are
// always budget-wide: accountID narrows only the activity side.
func (l *Ledger) BudgetVsActual(ctx context.Context, ym int, accountID string) (BudgetVsActualResult, error) {
	res := BudgetVsActualResult{Month: FormatYM(ym), Rows: make([]BudgetLine, 0)}
	budgeted, err := l.BudgetedAmounts(ctx, ym)
	if err != nil {
		return res, err
	}
	cats, err := l.Categories(ctx)
	if err != nil {
		return res, err
	}
	activity, err := l.categoryActivity(ctx, ym, accountID)
	if err != nil {
		return res, err
	}
	for _, c := range cats {
		if c.IsIncome {
			continue
		}
		b := budgeted[c.ID]
		spent := -activity[c.ID]
		if c.Hidden && b == 0 && spent == 0 {
			continue
		}
		line := BudgetLine{CategoryID: c.ID, Category: c.Name, Group: c.Group, Budgeted: b, Spent: spent, Remaining: b - spent, OverBudget: spent > b}
		res.Rows = append(res.Rows, line)
		res.TotalBudgeted += b
		res.TotalSpent += spent
	}
	res.TotalRemaining = res.TotalBudgeted - res.TotalSpent
	return res, nil
}

// categoryActivity sums leaf amounts per category for month ym (YYYYMM) on
// on-budget accounts, excluding uncategorized rows and starting balances —
// Actual's budget "activity" (negative = spent). accountID optionally narrows
// it to one account. Shared by BudgetVsActual and TemplateStatuses.
func (l *Ledger) categoryActivity(ctx context.Context, ym int, accountID string) (map[string]int64, error) {
	txns, starting, err := l.scopeTxns(ctx, ReportScope{From: MonthStart(ym), To: MonthEnd(ym), AccountID: accountID})
	if err != nil {
		return nil, err
	}
	activity := map[string]int64{}
	for _, t := range txns {
		if t.CategoryID == "" || starting[t.ID] {
			continue
		}
		activity[t.CategoryID] += t.Amount
	}
	return activity, nil
}

// ---------------------------------------------------------------------------
// Net worth

// AccountBalance is one account's balance at a point in time.
type AccountBalance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	OffBudget bool   `json:"offbudget"`
	Balance   int64  `json:"balance"`
}

// NetWorthPoint is the net worth at one month end.
type NetWorthPoint struct {
	Month    string           `json:"month"`
	AsOf     string           `json:"as_of"`
	NetWorth int64            `json:"net_worth"`
	Change   int64            `json:"change"`
	Accounts []AccountBalance `json:"accounts,omitempty"`
}

// NetWorth sums the balances of every live account (on- and off-budget) as of
// each month end in months (YYYYMM, oldest first). It reads the account list
// once and the per-account, per-date totals once, then accumulates balances
// month by month, matching Accounts(ctx, monthEnd) for every month.
func (l *Ledger) NetWorth(ctx context.Context, months []int, byAccount bool) ([]NetWorthPoint, error) {
	out := make([]NetWorthPoint, 0, len(months))
	if len(months) == 0 {
		return out, nil
	}
	ends := make([]int, len(months))
	maxEnd := 0
	for i, ym := range months {
		ends[i] = MonthEnd(ym)
		maxEnd = max(maxEnd, ends[i])
	}
	accts, err := l.Accounts(ctx, ends[0])
	if err != nil {
		return nil, err
	}
	deltas, err := l.accountDateTotals(ctx, maxEnd)
	if err != nil {
		return nil, err
	}
	// Visit month ends in ascending order, sweeping the date-sorted totals
	// into running per-account balances.
	order := make([]int, len(months))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return ends[order[a]] < ends[order[b]] })
	running := map[string]int64{}
	balances := make([]map[string]int64, len(months))
	next := 0
	for _, i := range order {
		for ; next < len(deltas) && deltas[next].date <= ends[i]; next++ {
			running[deltas[next].acct] += deltas[next].amount
		}
		balances[i] = maps.Clone(running)
	}
	var prev int64
	for i, ym := range months {
		p := NetWorthPoint{Month: FormatYM(ym), AsOf: FormatDate(ends[i])}
		if byAccount {
			p.Accounts = make([]AccountBalance, 0, len(accts))
		}
		for _, a := range accts {
			bal := balances[i][a.ID]
			p.NetWorth += bal
			if byAccount {
				p.Accounts = append(p.Accounts, AccountBalance{ID: a.ID, Name: a.Name, OffBudget: a.OffBudget, Balance: bal})
			}
		}
		if i > 0 {
			p.Change = p.NetWorth - prev
		}
		prev = p.NetWorth
		out = append(out, p)
	}
	return out, nil
}

type acctDateTotal struct {
	acct   string
	date   int
	amount int64
}

// accountDateTotals returns, sorted by date, the summed amount per account and
// date of the leaf, non-tombstoned transactions on or before asOf in live
// accounts (the rows Accounts aggregates).
func (l *Ledger) accountDateTotals(ctx context.Context, asOf int) ([]acctDateTotal, error) {
	rows, err := l.DB.QueryContext(ctx, `SELECT t.acct, t.date, COALESCE(SUM(t.amount), 0)
	FROM transactions t JOIN accounts a ON a.id = t.acct AND a.tombstone = 0
	WHERE t.tombstone = 0 AND COALESCE(t.isParent, 0) = 0 AND t.date <= ?
	GROUP BY t.acct, t.date
	ORDER BY t.date`, asOf)
	if err != nil {
		return nil, fmt.Errorf("querying account balances: %w", err)
	}
	defer rows.Close()
	out := make([]acctDateTotal, 0)
	for rows.Next() {
		var d acctDateTotal
		if err := rows.Scan(&d.acct, &d.date, &d.amount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Trends

// TrendPoint is one month of a category's spending.
type TrendPoint struct {
	Month string `json:"month"`
	Spent int64  `json:"spent"`
}

// TrendRow is one category's spending series.
type TrendRow struct {
	CategoryID string       `json:"category_id"`
	Category   string       `json:"category"`
	Total      int64        `json:"total"`
	Average    int64        `json:"average"`
	Latest     int64        `json:"latest"`
	Delta      int64        `json:"delta"`
	DeltaPct   float64      `json:"delta_pct"`
	Series     []TrendPoint `json:"series"`
}

// Trends returns per-category monthly net spending (outflows minus refunds,
// as in Spending) across the scope range, with
// the average month and the latest month's delta against it. categoryID
// limits output to one category ("" = all; uncategorized spending uses id "").
func (l *Ledger) Trends(ctx context.Context, s ReportScope, categoryID string) ([]TrendRow, error) {
	months := MonthsBetween(s.From, s.To)
	pos := map[int]int{}
	for i, ym := range months {
		pos[ym] = i
	}
	txns, starting, err := l.scopeTxns(ctx, s)
	if err != nil {
		return nil, err
	}
	rows := make([]TrendRow, 0)
	idx := map[string]int{}
	newRow := func(id, name string) int {
		r := TrendRow{CategoryID: id, Category: name, Series: make([]TrendPoint, len(months))}
		for i, ym := range months {
			r.Series[i].Month = FormatYM(ym)
		}
		rows = append(rows, r)
		idx[id] = len(rows) - 1
		return len(rows) - 1
	}
	for _, t := range txns {
		spent, ok := spendAmount(t, starting)
		if !ok {
			continue
		}
		if categoryID != "" && t.CategoryID != categoryID {
			continue
		}
		mi, ok := pos[YMFromDate(t.DateInt)]
		if !ok {
			continue
		}
		i, ok := idx[t.CategoryID]
		if !ok {
			name := t.Category
			if t.CategoryID == "" {
				name = UncategorizedName
			}
			i = newRow(t.CategoryID, name)
		}
		rows[i].Series[mi].Spent += spent
	}
	for i := range rows {
		r := &rows[i]
		for _, p := range r.Series {
			r.Total += p.Spent
		}
		if n := len(r.Series); n > 0 {
			r.Average = int64(math.Round(float64(r.Total) / float64(n)))
			r.Latest = r.Series[n-1].Spent
		}
		r.Delta = r.Latest - r.Average
		if r.Average != 0 {
			r.DeltaPct = math.Round(float64(r.Delta)/float64(r.Average)*10000) / 10000
			if r.DeltaPct == 0 {
				r.DeltaPct = 0 // normalize -0, which JSON would print as -0
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].Category < rows[j].Category
	})
	return rows, nil
}

// ---------------------------------------------------------------------------
// Recurring

// RecurringPayee is a payee that charges a similar amount month after month.
type RecurringPayee struct {
	PayeeID      string  `json:"payee_id"`
	Payee        string  `json:"payee"`
	Occurrences  int     `json:"occurrences"`
	Months       int     `json:"months"`
	AvgAmount    int64   `json:"avg_amount"`
	Variation    float64 `json:"variation"`
	CadenceDays  int     `json:"cadence_days"`
	LastDate     string  `json:"last_date"`
	NextExpected string  `json:"next_expected"`
	Scheduled    bool    `json:"scheduled"`
}

// RecurringMaxVariation is the largest coefficient of variation (stddev/mean)
// of a payee's amounts that still counts as "similar amounts".
const RecurringMaxVariation = 0.25

// Recurring detects payees with outflows in at least minOccurrences distinct
// months whose amounts vary by at most RecurringMaxVariation.
func (l *Ledger) Recurring(ctx context.Context, s ReportScope, minOccurrences int) ([]RecurringPayee, error) {
	if minOccurrences < 2 {
		minOccurrences = 2
	}
	txns, starting, err := l.scopeTxns(ctx, s)
	if err != nil {
		return nil, err
	}
	byPayee := map[string][]Txn{}
	var order []string
	for _, t := range txns {
		if !isSpend(t, starting) || t.PayeeID == "" {
			continue
		}
		if _, ok := byPayee[t.PayeeID]; !ok {
			order = append(order, t.PayeeID)
		}
		byPayee[t.PayeeID] = append(byPayee[t.PayeeID], t)
	}
	out := make([]RecurringPayee, 0)
	for _, pid := range order {
		g := byPayee[pid]
		months := map[int]bool{}
		var sum float64
		scheduled := false
		for _, t := range g {
			months[YMFromDate(t.DateInt)] = true
			sum += float64(-t.Amount)
			if t.ScheduleID != "" {
				scheduled = true
			}
		}
		if len(months) < minOccurrences {
			continue
		}
		mean := sum / float64(len(g))
		var ss float64
		for _, t := range g {
			d := float64(-t.Amount) - mean
			ss += d * d
		}
		cv := 0.0
		if mean > 0 {
			cv = math.Sqrt(ss/float64(len(g))) / mean
		}
		if cv > RecurringMaxVariation {
			continue
		}
		sort.Slice(g, func(i, j int) bool { return g[i].DateInt < g[j].DateInt })
		first, last := g[0].DateInt, g[len(g)-1].DateInt
		cadence := 0
		if len(g) > 1 {
			cadence = int(math.Round(float64(daysBetween(first, last)) / float64(len(g)-1)))
		}
		r := RecurringPayee{PayeeID: pid, Payee: g[0].Payee, Occurrences: len(g), Months: len(months), AvgAmount: int64(math.Round(mean)),
			Variation: math.Round(cv*1000) / 1000, CadenceDays: cadence, LastDate: FormatDate(last), Scheduled: scheduled}
		if cadence > 0 {
			r.NextExpected = FormatDate(DateInt(DateTime(last).AddDate(0, 0, cadence)))
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AvgAmount != out[j].AvgAmount {
			return out[i].AvgAmount > out[j].AvgAmount
		}
		return out[i].Payee < out[j].Payee
	})
	return out, nil
}

// ---------------------------------------------------------------------------
// Name resolution

// ErrAmbiguousName marks a name that matches more than one live record; the
// error message lists the matching ids so the caller can pass one instead.
var ErrAmbiguousName = errors.New("ambiguous name")

// ResolveAccountID matches a live account by exact id, else by
// case-insensitive name. Returns sql.ErrNoRows when nothing matches and
// ErrAmbiguousName when several accounts share the name.
func (l *Ledger) ResolveAccountID(ctx context.Context, nameOrID string) (string, error) {
	return l.resolveID(ctx, "account", "SELECT id, COALESCE(name, '') FROM accounts WHERE tombstone = 0 ORDER BY id", nameOrID)
}

// ResolveCategoryID matches a live category by exact id, else by
// case-insensitive name. Returns sql.ErrNoRows when nothing matches and
// ErrAmbiguousName when several categories share the name.
func (l *Ledger) ResolveCategoryID(ctx context.Context, nameOrID string) (string, error) {
	return l.resolveID(ctx, "category", "SELECT id, COALESCE(name, '') FROM categories WHERE tombstone = 0 ORDER BY id", nameOrID)
}

func (l *Ledger) resolveID(ctx context.Context, kind, q, nameOrID string) (string, error) {
	rows, err := l.DB.QueryContext(ctx, q)
	if err != nil {
		return "", fmt.Errorf("looking up %s: %w", kind, err)
	}
	type rec struct{ id, name string }
	var recs []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.name); err != nil {
			_ = rows.Close()
			return "", err
		}
		recs = append(recs, r)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return "", err
	}
	want := strings.TrimSpace(nameOrID)
	for _, r := range recs {
		if r.id == want {
			return r.id, nil
		}
	}
	var matches []string
	for _, r := range recs {
		if strings.EqualFold(strings.TrimSpace(r.name), want) {
			matches = append(matches, r.id)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no %s named %q: %w", kind, nameOrID, sql.ErrNoRows)
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("%w: %d %s records are named %q (ids: %s); pass one of the ids instead",
		ErrAmbiguousName, len(matches), kind, want, strings.Join(matches, ", "))
}
