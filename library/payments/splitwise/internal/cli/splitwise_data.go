package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"github.com/spf13/cobra"
)

// novelErr gives hand-authored commands the same typed JSON error contract as
// generated API commands while avoiding a duplicate root-level error message.
func novelErr(cmd *cobra.Command, flags *rootFlags, err error) error {
	if err == nil {
		return nil
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		return err
	}
	code := ExitCode(err)
	_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"error": err.Error(), "code": code})
	cmd.SilenceErrors = true
	return silentCodeErr(code)
}

// splitwiseMutationError inspects a Splitwise write response body. Splitwise
// returns HTTP 200 even when a create/update fails, signaling the failure only
// via a non-empty "errors" object in the body. Callers that check only the HTTP
// status would treat such a failure as success, so write paths must also run
// the response body through this check. Returns nil when the body reports no
// errors (or is not a recognizable JSON object — the HTTP status is the
// caller's first line of defense in that case).
func splitwiseMutationError(data json.RawMessage) error {
	var env struct {
		Errors json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	s := strings.TrimSpace(string(env.Errors))
	if s == "" || s == "{}" || s == "[]" || s == "null" {
		return nil
	}
	return fmt.Errorf("splitwise rejected the request: %s", s)
}

func upsertCreatedExpenses(db *store.Store, data json.RawMessage) error {
	var env struct {
		Expenses []json.RawMessage `json:"expenses"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	for _, raw := range env.Expenses {
		var expense struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(raw, &expense); err != nil {
			return err
		}
		if expense.ID == 0 {
			return errors.New("create_expense response contains an expense without an id")
		}
		if err := db.Upsert("get-expenses", strconv.Itoa(expense.ID), raw); err != nil {
			return err
		}
	}
	return nil
}

// openSplitwiseStore opens (creating if absent) the local SQLite store. Novel
// read commands use this instead of a store-must-exist open so that running a
// command before `sync` yields an empty result plus the unsynced stderr hint
// rather than a hard error — matching the framework's store-query convention.
func openSplitwiseStore(ctx context.Context, path string) (*store.Store, error) {
	if path == "" {
		path = defaultDBPath("splitwise-pp-cli")
	}
	return store.OpenWithContext(ctx, path)
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// stripHTML removes HTML tags from Splitwise notification content (which the
// API returns as markup like "<strong>Alex</strong> paid you<br>...") and then
// decodes HTML entities and collapses whitespace. cliutil.CleanText only
// unescapes entities; it does not remove tags, so notification content needs
// this extra pass before it is shown to a user or agent.
func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br />", " ")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = cliutil.CleanText(s)
	return strings.Join(strings.Fields(s), " ")
}

type Balance struct {
	CurrencyCode string `json:"currency_code"`
	Amount       string `json:"amount"`
}

type Friend struct {
	ID        int           `json:"id"`
	FirstName string        `json:"first_name"`
	LastName  string        `json:"last_name"`
	Balance   []Balance     `json:"balance"`
	Groups    []FriendGroup `json:"groups"`
}

type FriendGroup struct {
	GroupID int       `json:"group_id"`
	Balance []Balance `json:"balance"`
}

type GroupMember struct {
	ID        int       `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Balance   []Balance `json:"balance"`
}

type SimplifiedDebt struct {
	From         int    `json:"from"`
	To           int    `json:"to"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
}

type Group struct {
	ID              int              `json:"id"`
	Name            string           `json:"name"`
	Members         []GroupMember    `json:"members"`
	SimplifiedDebts []SimplifiedDebt `json:"simplified_debts"`
}

type NestedUser struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type ExpenseUser struct {
	UserID    int        `json:"user_id"`
	PaidShare string     `json:"paid_share"`
	OwedShare string     `json:"owed_share"`
	User      NestedUser `json:"user"`
}

type Category struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	Subcategories []Category `json:"subcategories"`
}

type Expense struct {
	ID           int           `json:"id"`
	GroupID      int           `json:"group_id"`
	Description  string        `json:"description"`
	Cost         string        `json:"cost"`
	CurrencyCode string        `json:"currency_code"`
	Date         string        `json:"date"`
	UpdatedAt    string        `json:"updated_at"`
	Payment      bool          `json:"payment"`
	DeletedAt    *string       `json:"deleted_at"`
	Category     Category      `json:"category"`
	Users        []ExpenseUser `json:"users"`
}

type expenseWindow struct {
	since    time.Time
	until    time.Time
	hasSince bool
	hasUntil bool
}

func parseExpenseWindow(since, until string, now time.Time) (expenseWindow, error) {
	var window expenseWindow
	var err error
	if strings.TrimSpace(since) != "" {
		window.since, err = parseExpenseWindowBound(since, now, false)
		if err != nil {
			return window, fmt.Errorf("invalid --since %q: %w", since, err)
		}
		window.hasSince = true
	}
	if strings.TrimSpace(until) != "" {
		window.until, err = parseExpenseWindowBound(until, now, true)
		if err != nil {
			return window, fmt.Errorf("invalid --until %q: %w", until, err)
		}
		window.hasUntil = true
	}
	if window.hasSince && window.hasUntil && window.until.Before(window.since) {
		return window, fmt.Errorf("--until must be on/after --since")
	}
	return window, nil
}

func parseExpenseWindowBound(value string, now time.Time, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return t, nil
	}
	dur, err := cliutil.ParseDurationLoose(value)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-dur), nil
}

func expenseInWindow(e Expense, window expenseWindow) bool {
	if !window.hasSince && !window.hasUntil {
		return true
	}
	t, ok := parseSplitwiseDate(e.Date)
	if !ok {
		return false
	}
	return (!window.hasSince || !t.Before(window.since)) && (!window.hasUntil || !t.After(window.until))
}

func parseAmount(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func friendDisplayName(f Friend) string {
	return strings.TrimSpace(strings.TrimSpace(f.FirstName) + " " + strings.TrimSpace(f.LastName))
}

// loadCurrentUserID returns the authenticated user's numeric id from the synced
// current-user resource, or 0 if it cannot be determined. get_current_user is
// stored as {"user":{"id":...}}, but some sync shapes store the user object
// directly, so both are attempted. Needed by settle-up to populate the "you"
// side of a friend payment.
func loadCurrentUserID(db *store.Store) int {
	rows, err := listResourceRows(db, "get-current-user", "current_user", "users")
	if err != nil {
		return 0
	}
	for _, row := range rows {
		var wrap struct {
			User struct {
				ID int `json:"id"`
			} `json:"user"`
		}
		if json.Unmarshal(row, &wrap) == nil && wrap.User.ID != 0 {
			return wrap.User.ID
		}
		var direct struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(row, &direct) == nil && direct.ID != 0 {
			return direct.ID
		}
	}
	return 0
}

// listResourceRows returns rows for the first candidate resource_type that has
// any. Sync keys resources by the promoted endpoint name (e.g. "get-friends"),
// but a bare name ("friends") may be used by other sync shapes — try both so
// the loaders are robust to either naming and never silently return zero.
func listResourceRows(db *store.Store, candidates ...string) ([]json.RawMessage, error) {
	var firstErr error
	for _, rt := range candidates {
		rows, err := db.List(rt, 0)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(rows) > 0 {
			return rows, nil
		}
	}
	return make([]json.RawMessage, 0), firstErr
}

func loadFriends(db *store.Store) ([]Friend, error) {
	out := make([]Friend, 0)
	rows, err := listResourceRows(db, "get-friends", "friends")
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		var f Friend
		if err := json.Unmarshal(row, &f); err != nil {
			continue
		}
		if f.Balance == nil {
			f.Balance = make([]Balance, 0)
		}
		if f.Groups == nil {
			f.Groups = make([]FriendGroup, 0)
		}
		out = append(out, f)
	}
	return out, nil
}

func loadGroups(db *store.Store) ([]Group, error) {
	out := make([]Group, 0)
	rows, err := listResourceRows(db, "get-groups", "groups")
	if err != nil {
		return out, err
	}
	// Dedupe by group id. A store synced by more than one binary can hold the
	// same group under two keys (the id-0 "Non-group expenses" group is stored
	// as key "0" by one generation and by name by another), which surfaced as a
	// duplicated row in `balances --by-group`. Rows arrive newest-first
	// (store.List orders by updated_at DESC), so the first occurrence wins.
	seen := make(map[int]bool)
	for _, row := range rows {
		var g Group
		if err := json.Unmarshal(row, &g); err != nil {
			continue
		}
		if seen[g.ID] {
			continue
		}
		seen[g.ID] = true
		if g.Members == nil {
			g.Members = make([]GroupMember, 0)
		}
		if g.SimplifiedDebts == nil {
			g.SimplifiedDebts = make([]SimplifiedDebt, 0)
		}
		out = append(out, g)
	}
	return out, nil
}

func loadExpenses(db *store.Store) ([]Expense, error) {
	out := make([]Expense, 0)
	rows, err := listResourceRows(db, "get-expenses", "expenses")
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		var e Expense
		if err := json.Unmarshal(row, &e); err != nil {
			continue
		}
		if e.Users == nil {
			e.Users = make([]ExpenseUser, 0)
		}
		if e.Category.Subcategories == nil {
			e.Category.Subcategories = make([]Category, 0)
		}
		out = append(out, e)
	}
	return out, nil
}
