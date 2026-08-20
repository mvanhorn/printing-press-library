package zohotools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/client"
)

// AutoscanResult is the polled state of an in-progress Zoho autoscan.
type AutoscanResult struct {
	ExpenseID string         `json:"expense_id"`
	Status    string         `json:"autoscan_status"`
	Expense   map[string]any `json:"expense,omitempty"`
}

// PollAutoscan polls GET /expenses/{id} with 2/4/8/16s backoff until the
// autoscan completes (status "Processed" or "Failed") or the timeout
// elapses. Always returns a result on success — even when the terminal
// status is "Failed" — so callers can distinguish "didn't finish" (err)
// from "finished badly" (result.Status).
func PollAutoscan(ctx context.Context, c *client.Client, expenseID string, timeout time.Duration) (*AutoscanResult, error) {
	if expenseID == "" {
		return nil, fmt.Errorf("expense_id required")
	}
	deadline := time.Now().Add(timeout)
	backoff := 2 * time.Second
	const maxBackoff = 16 * time.Second
	for {
		res, err := fetchExpense(ctx, c, expenseID)
		if err != nil {
			return nil, err
		}
		status := strings.ToLower(strings.TrimSpace(res.Status))
		switch status {
		case "processed", "failed", "scanned", "scan_failed":
			return res, nil
		}
		if time.Now().After(deadline) {
			return res, fmt.Errorf("autoscan poll timed out after %s (last status: %q)", timeout, res.Status)
		}
		sleep := backoff
		if remaining := time.Until(deadline); remaining < sleep {
			sleep = remaining
		}
		if sleep <= 0 {
			return res, fmt.Errorf("autoscan poll timed out after %s (last status: %q)", timeout, res.Status)
		}
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(sleep):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// fetchExpense hits GET /expenses/{id} bypassing the response cache so the
// poll loop sees fresh autoscan_status on each tick. Unwraps Zoho's
// envelope {"expense": {...}} when present.
func fetchExpense(ctx context.Context, c *client.Client, expenseID string) (*AutoscanResult, error) {
	path := "/expenses/" + expenseID
	raw, err := c.GetNoCache(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", path, err)
	}
	expense, status := extractExpenseStatus(raw)
	return &AutoscanResult{
		ExpenseID: expenseID,
		Status:    status,
		Expense:   expense,
	}, nil
}

// extractExpenseStatus parses Zoho's two common shapes: the canonical
// {"expense": {...}} envelope and the bare object. Returns the expense
// object and the autoscan_status (empty when missing).
func extractExpenseStatus(raw json.RawMessage) (map[string]any, string) {
	var env struct {
		Expense map[string]any `json:"expense"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Expense != nil {
		return env.Expense, asString(env.Expense["autoscan_status"])
	}
	var bare map[string]any
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare, asString(bare["autoscan_status"])
	}
	return nil, ""
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
