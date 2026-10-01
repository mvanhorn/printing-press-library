// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Ledger runs read-only queries against a mirrored Actual budget database.
//
// Actual's on-disk schema quirks handled here:
//   - transactions.acct is the account id; transactions.description is the
//     payee id; imported_description is the bank's raw payee text.
//   - merged payees/categories resolve through payee_mapping.targetId and
//     category_mapping.transferId.
//   - dates are integers YYYYMMDD, amounts are integer minor units.
//   - deletes are soft (tombstone = 1).
//   - split parents (isParent=1) carry the total; children carry the parts.
type Ledger struct {
	DB   *sql.DB
	cols map[string]map[string]bool
}

// interpolatedTables lists the only table names helpers may splice into SQL
// (SQLite cannot bind identifiers). Every caller passes a compile-time
// constant; this check keeps that true if a future caller passes input.
var interpolatedTables = map[string]bool{
	"payees": true, "categories": true, "accounts": true, "schedules": true,
	"category_groups": true, "zero_budgets": true, "reflect_budgets": true,
}

func checkTable(table string) error {
	if !interpolatedTables[table] {
		return fmt.Errorf("internal error: table %q is not allowed in an interpolated query", table)
	}
	return nil
}

// NewLedger wraps an open mirror database.
func NewLedger(db *sql.DB) *Ledger { return &Ledger{DB: db, cols: map[string]map[string]bool{}} }

// HasColumn reports whether table has column, so queries tolerate schema drift
// across Actual releases. A failed schema lookup reports false without being
// cached, so a transient error (e.g. a cancelled context) is retried next time.
func (l *Ledger) HasColumn(ctx context.Context, table, column string) bool {
	if cols, ok := l.cols[table]; ok {
		return cols[column]
	}
	rows, err := l.DB.QueryContext(ctx, "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return false
	}
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return false
		}
		cols[name] = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return false
	}
	l.cols[table] = cols
	return cols[column]
}

// HasTable reports whether a table or view exists.
func (l *Ledger) HasTable(ctx context.Context, table string) bool {
	var n int
	err := l.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name = ? AND type IN ('table','view')", table).Scan(&n)
	return err == nil && n > 0
}

// Txn is one live ledger transaction with names resolved.
type Txn struct {
	ID                  string `json:"id"`
	Date                string `json:"date"`
	Amount              int64  `json:"amount"`
	AccountID           string `json:"account_id"`
	Account             string `json:"account"`
	OffBudget           bool   `json:"offbudget"`
	PayeeID             string `json:"payee_id,omitempty"`
	Payee               string `json:"payee,omitempty"`
	CategoryID          string `json:"category_id,omitempty"`
	Category            string `json:"category,omitempty"`
	CategoryGroup       string `json:"category_group,omitempty"`
	IsIncome            bool   `json:"is_income,omitempty"`
	Notes               string `json:"notes,omitempty"`
	ImportedDescription string `json:"imported_payee,omitempty"`
	Cleared             bool   `json:"cleared"`
	IsParent            bool   `json:"is_parent,omitempty"`
	IsChild             bool   `json:"is_child,omitempty"`
	ParentID            string `json:"parent_id,omitempty"`
	TransferID          string `json:"transfer_id,omitempty"`
	TransferAccountID   string `json:"transfer_account_id,omitempty"`
	ScheduleID          string `json:"schedule_id,omitempty"`
	DateInt             int    `json:"-"`
}

// TxnFilter narrows TxnQuery results. Zero values mean "no filter".
type TxnFilter struct {
	From, To      int // inclusive YYYYMMDD
	AccountID     string
	CategoryID    string
	PayeeID       string
	OnBudgetOnly  bool
	Uncategorized bool
	// Leaves excludes split parents so every row is a real money movement
	// with its own category (the right view for spending reports).
	Leaves bool
	Limit  int
}

// Txns returns live transactions matching f, newest first.
func (l *Ledger) Txns(ctx context.Context, f TxnFilter) ([]Txn, error) {
	if f.Uncategorized {
		// Actual's "uncategorized" view only covers on-budget leaf rows.
		f.OnBudgetOnly, f.Leaves = true, true
	}
	var where []string
	var args []any
	where = append(where, "t.tombstone = 0", "COALESCE(a.tombstone, 0) = 0")
	if f.From > 0 {
		where = append(where, "t.date >= ?")
		args = append(args, f.From)
	}
	if f.To > 0 {
		where = append(where, "t.date <= ?")
		args = append(args, f.To)
	}
	if f.AccountID != "" {
		where = append(where, "t.acct = ?")
		args = append(args, f.AccountID)
	}
	if f.CategoryID != "" {
		where = append(where, "COALESCE(cm.transferId, t.category) = ?")
		args = append(args, f.CategoryID)
	}
	if f.PayeeID != "" {
		where = append(where, "COALESCE(pm.targetId, t.description) = ?")
		args = append(args, f.PayeeID)
	}
	if f.OnBudgetOnly {
		where = append(where, "COALESCE(a.offbudget, 0) = 0")
	}
	if f.Leaves {
		where = append(where, "COALESCE(t.isParent, 0) = 0")
	}
	if f.Uncategorized {
		// Matches Actual's own "uncategorized" filter: on-budget, not a
		// transfer, not a split parent, no category, not a starting balance.
		where = append(where,
			"COALESCE(cm.transferId, t.category) IS NULL",
			"p.transfer_acct IS NULL")
		if l.HasColumn(ctx, "transactions", "starting_balance_flag") {
			where = append(where, "COALESCE(t.starting_balance_flag, 0) = 0")
		}
	}
	// Only constant fragments are spliced in: optCol yields "t.schedule" or
	// "NULL", where holds literal clauses (values bind through args), and
	// LIMIT is an integer.
	// #nosec G202 -- no caller-controlled text reaches the SQL string
	q := `SELECT t.id, t.date, COALESCE(t.amount, 0), t.acct, COALESCE(a.name, ''), COALESCE(a.offbudget, 0),
		COALESCE(pm.targetId, t.description), p.name, COALESCE(cm.transferId, t.category), c.name, g.name, COALESCE(c.is_income, 0),
		t.notes, t.imported_description, COALESCE(t.cleared, 0), COALESCE(t.isParent, 0), COALESCE(t.isChild, 0),
		t.parent_id, t.transferred_id, p.transfer_acct, ` + l.optCol(ctx, "transactions", "schedule", "t") + `
	FROM transactions t
	LEFT JOIN accounts a ON a.id = t.acct
	LEFT JOIN payee_mapping pm ON pm.id = t.description
	LEFT JOIN payees p ON p.id = COALESCE(pm.targetId, t.description)
	LEFT JOIN category_mapping cm ON cm.id = t.category
	LEFT JOIN categories c ON c.id = COALESCE(cm.transferId, t.category)
	LEFT JOIN category_groups g ON g.id = c.cat_group
	WHERE ` + strings.Join(where, " AND ") + `
	ORDER BY t.date DESC, t.sort_order DESC, t.id`
	if f.Limit > 0 {
		q += " LIMIT " + strconv.Itoa(f.Limit)
	}
	rows, err := l.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying transactions: %w", err)
	}
	defer rows.Close()
	out := make([]Txn, 0)
	for rows.Next() {
		var t Txn
		var date sql.NullInt64
		var acct, payeeID, payee, catID, cat, group, notes, imported, parent, transfer, transferAcct, sched sql.NullString
		var off, income, cleared, isParent, isChild int64
		if err := rows.Scan(&t.ID, &date, &t.Amount, &acct, &t.Account, &off, &payeeID, &payee, &catID, &cat, &group, &income,
			&notes, &imported, &cleared, &isParent, &isChild, &parent, &transfer, &transferAcct, &sched); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		t.DateInt = int(date.Int64)
		t.Date = FormatDate(t.DateInt)
		t.AccountID, t.PayeeID, t.Payee = acct.String, payeeID.String, payee.String
		t.CategoryID, t.Category, t.CategoryGroup = catID.String, cat.String, group.String
		t.Notes, t.ImportedDescription = notes.String, imported.String
		t.ParentID, t.TransferID, t.TransferAccountID, t.ScheduleID = parent.String, transfer.String, transferAcct.String, sched.String
		t.OffBudget, t.IsIncome, t.Cleared, t.IsParent, t.IsChild = off != 0, income != 0, cleared != 0, isParent != 0, isChild != 0
		out = append(out, t)
	}
	return out, rows.Err()
}

func (l *Ledger) optCol(ctx context.Context, table, col, alias string) string {
	if l.HasColumn(ctx, table, col) {
		return alias + "." + col
	}
	return "NULL"
}

// Account is one live account with its balances.
type Account struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	OffBudget      bool   `json:"offbudget"`
	Closed         bool   `json:"closed"`
	Balance        int64  `json:"balance"`
	ClearedBalance int64  `json:"cleared_balance"`
	TxnCount       int    `json:"transaction_count"`
}

// Accounts returns live accounts with balances computed from leaf transactions
// dated on or before asOf (0 = all).
func (l *Ledger) Accounts(ctx context.Context, asOf int) ([]Account, error) {
	cond := ""
	var args []any
	if asOf > 0 {
		cond = " AND t.date <= ?"
		args = append(args, asOf)
	}
	q := `SELECT a.id, COALESCE(a.name, ''), COALESCE(a.offbudget, 0), COALESCE(a.closed, 0),
		COALESCE(SUM(t.amount), 0), COALESCE(SUM(CASE WHEN t.cleared = 1 THEN t.amount ELSE 0 END), 0), COUNT(t.id)
	FROM accounts a
	LEFT JOIN transactions t ON t.acct = a.id AND t.tombstone = 0 AND COALESCE(t.isParent, 0) = 0` + cond + `
	WHERE a.tombstone = 0
	GROUP BY a.id
	ORDER BY COALESCE(a.offbudget, 0), COALESCE(a.closed, 0), a.sort_order, a.name`
	rows, err := l.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying accounts: %w", err)
	}
	defer rows.Close()
	out := make([]Account, 0)
	for rows.Next() {
		var a Account
		var off, closed int64
		if err := rows.Scan(&a.ID, &a.Name, &off, &closed, &a.Balance, &a.ClearedBalance, &a.TxnCount); err != nil {
			return nil, err
		}
		a.OffBudget, a.Closed = off != 0, closed != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

// Category is one live category with its group.
type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	GroupID  string `json:"group_id"`
	Group    string `json:"group"`
	IsIncome bool   `json:"is_income"`
	Hidden   bool   `json:"hidden"`
	GoalDef  string `json:"-"`
}

// Categories returns live categories in display order.
func (l *Ledger) Categories(ctx context.Context) ([]Category, error) {
	goal := l.optCol(ctx, "categories", "goal_def", "c")
	// #nosec G202 -- goal is the constant "c.goal_def" or "NULL"
	q := `SELECT c.id, COALESCE(c.name, ''), COALESCE(c.cat_group, ''), COALESCE(g.name, ''), COALESCE(c.is_income, 0),
		COALESCE(c.hidden, 0) OR COALESCE(g.hidden, 0), ` + goal + `
	FROM categories c LEFT JOIN category_groups g ON g.id = c.cat_group
	WHERE c.tombstone = 0
	ORDER BY COALESCE(g.is_income, 0), g.sort_order, c.sort_order, c.name`
	rows, err := l.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		var c Category
		var income, hidden int64
		var goalDef sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.GroupID, &c.Group, &income, &hidden, &goalDef); err != nil {
			return nil, err
		}
		c.IsIncome, c.Hidden, c.GoalDef = income != 0, hidden != 0, goalDef.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// Payee is one live payee.
type Payee struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TransferAcct string `json:"transfer_account_id,omitempty"`
	TxnCount     int    `json:"transaction_count"`
	LastUsed     string `json:"last_used,omitempty"`
}

// Payees returns live, non-transfer payees with usage counts (resolving merges).
func (l *Ledger) Payees(ctx context.Context) ([]Payee, error) {
	q := `WITH used AS (
		SELECT COALESCE(pm.targetId, t.description) AS pid, COUNT(*) AS n, MAX(t.date) AS last
		FROM transactions t LEFT JOIN payee_mapping pm ON pm.id = t.description
		WHERE t.tombstone = 0 AND COALESCE(t.isChild, 0) = 0 AND t.description IS NOT NULL
		GROUP BY 1)
	SELECT p.id, COALESCE(p.name, ''), p.transfer_acct, COALESCE(u.n, 0), u.last
	FROM payees p LEFT JOIN used u ON u.pid = p.id
	WHERE p.tombstone = 0 AND p.transfer_acct IS NULL
	ORDER BY COALESCE(u.n, 0) DESC, p.name`
	rows, err := l.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("querying payees: %w", err)
	}
	defer rows.Close()
	out := make([]Payee, 0)
	for rows.Next() {
		var p Payee
		var transfer sql.NullString
		var last sql.NullInt64
		if err := rows.Scan(&p.ID, &p.Name, &transfer, &p.TxnCount, &last); err != nil {
			return nil, err
		}
		p.TransferAcct = transfer.String
		if last.Valid {
			p.LastUsed = FormatDate(int(last.Int64))
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// BudgetedAmounts returns category id -> budgeted amount for a month (YYYYMM).
// The table follows the budget's type when the synced `budgetType` preference
// is present ("rollover"/"envelope" -> zero_budgets, "report"/"tracking" ->
// reflect_budgets). Without it, the first of zero_budgets / reflect_budgets
// that has rows for the month wins.
func (l *Ledger) BudgetedAmounts(ctx context.Context, month int) (map[string]int64, error) {
	if table := l.preferredBudgetTable(ctx); table != "" && l.HasTable(ctx, table) {
		return l.budgetRows(ctx, table, month)
	}
	for _, table := range []string{"zero_budgets", "reflect_budgets"} {
		if !l.HasTable(ctx, table) {
			continue
		}
		out, err := l.budgetRows(ctx, table, month)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return map[string]int64{}, nil
}

// preferredBudgetTable maps the synced budgetType preference to its budget
// table, or "" when the preference is missing or unrecognised.
func (l *Ledger) preferredBudgetTable(ctx context.Context) string {
	if !l.HasTable(ctx, "preferences") {
		return ""
	}
	var v sql.NullString
	if err := l.DB.QueryRowContext(ctx, "SELECT value FROM preferences WHERE id = 'budgetType'").Scan(&v); err != nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(v.String)) {
	case "rollover", "envelope":
		return "zero_budgets"
	case "report", "tracking":
		return "reflect_budgets"
	}
	return ""
}

func (l *Ledger) budgetRows(ctx context.Context, table string, month int) (map[string]int64, error) {
	out := map[string]int64{}
	if err := checkTable(table); err != nil {
		return nil, err
	}
	rows, err := l.DB.QueryContext(ctx, "SELECT category, COALESCE(amount, 0) FROM "+table+" WHERE month = ?", month) // #nosec G202 -- table passed checkTable allowlist
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cat sql.NullString
		var amt int64
		if err := rows.Scan(&cat, &amt); err != nil {
			return nil, err
		}
		if cat.Valid {
			out[cat.String] += amt
		}
	}
	return out, rows.Err()
}

// Notes returns entity id -> note text for all non-empty notes.
func (l *Ledger) Notes(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	if !l.HasTable(ctx, "notes") {
		return out, nil
	}
	rows, err := l.DB.QueryContext(ctx, "SELECT id, note FROM notes WHERE note IS NOT NULL AND note != ''")
	if err != nil {
		return nil, fmt.Errorf("querying notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, note string
		if err := rows.Scan(&id, &note); err != nil {
			return nil, err
		}
		out[id] = note
	}
	return out, rows.Err()
}

// FormatDate converts Actual's integer YYYYMMDD date to YYYY-MM-DD.
func FormatDate(d int) string {
	if d <= 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d/10000, d/100%100, d%100)
}

// ParseDate accepts YYYY-MM-DD or YYYYMMDD and returns Actual's integer form.
func ParseDate(s string) (int, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return DateInt(t), nil
	}
	if len(s) == 8 {
		if n, err := strconv.Atoi(s); err == nil {
			if _, err := time.Parse("20060102", s); err == nil {
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
}

// DateInt converts a time to Actual's integer YYYYMMDD form.
func DateInt(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }

// MonthRange parses YYYY-MM and returns the first/last YYYYMMDD of the month
// plus the YYYYMM integer Actual uses for budget months.
func MonthRange(month string) (from, to, ym int, err error) {
	t, err := time.Parse("2006-01", strings.TrimSpace(month))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid month %q (want YYYY-MM)", month)
	}
	last := t.AddDate(0, 1, -1)
	return DateInt(t), DateInt(last), t.Year()*100 + int(t.Month()), nil
}

// FormatAmount renders integer minor units as a decimal string (12030 -> 120.30).
func FormatAmount(v int64) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// ParseAmount converts a decimal string to minor units. Accepted forms:
// "120.30", "-5", "+5", "$7.05", "-$5.00", "$-5", "(12.00)" (negative) and
// "1,234.56" (comma thousands separators in groups of three). Fractional
// digits beyond the second are accepted only when they are zeros ("12.340").
// Other extra precision ("12.345"), signs inside the number or inside
// parentheses ("(-5)") and European "1.234,56" are rejected rather than
// silently misread.
func ParseAmount(s string) (int64, error) {
	orig := s
	bad := func(why string) (int64, error) {
		return 0, fmt.Errorf("invalid amount %q: %s", orig, why)
	}
	s = strings.TrimSpace(s)
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, strings.TrimSpace(s[1:len(s)-1])
		if strings.HasPrefix(strings.TrimPrefix(s, "$"), "-") || strings.HasPrefix(strings.TrimPrefix(s, "$"), "+") {
			return bad("sign inside parentheses; use (12.00) or -12.00")
		}
	}
	signed := false
	takeSign := func() {
		if strings.HasPrefix(s, "-") {
			neg, s, signed = !neg, s[1:], true
		} else if strings.HasPrefix(s, "+") {
			s, signed = s[1:], true
		}
	}
	takeSign()
	if strings.HasPrefix(s, "$") {
		s = s[1:]
		if !signed {
			takeSign()
		}
	}
	if s == "" {
		return 0, fmt.Errorf("empty amount %q", orig)
	}
	whole, frac, _ := strings.Cut(s, ".")
	if strings.Contains(frac, ",") {
		return bad("comma after the decimal point (European format?); use 1234.56")
	}
	if strings.Contains(whole, ",") {
		groups := strings.Split(whole, ",")
		for i, g := range groups {
			if (i == 0 && (len(g) == 0 || len(g) > 3)) || (i > 0 && len(g) != 3) {
				return bad("misplaced thousands separator; use 1,234.56 or 1234.56")
			}
		}
		whole = strings.Join(groups, "")
	}
	if whole == "" && frac == "" {
		return bad("no digits")
	}
	if !allDigits(whole) || !allDigits(frac) {
		return bad("want digits with an optional leading sign or $")
	}
	if len(frac) > 2 {
		if strings.Trim(frac[2:], "0") != "" {
			return bad("more than 2 decimal places")
		}
		frac = frac[:2]
	}
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w > (1<<63-1)/100-1 {
		return bad("out of range")
	}
	f, _ := strconv.ParseInt((frac + "00")[:2], 10, 64)
	v := w*100 + f
	if neg {
		v = -v
	}
	return v, nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
