// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestPlanBudgetBuildExpenseShape(t *testing.T) {
	target := map[string]any{
		"userId": 42,
		"itinerary": map[string]any{
			"budget": map[string]any{"amount": map[string]any{"amount": 500, "currencyCode": "SGD"}},
			"sections": []any{
				map[string]any{"blocks": []any{map[string]any{"id": 123, "type": "place"}}},
			},
		},
	}
	exp, err := buildBudgetExpense(budgetExpenseFlags{Description: "Lunch", Amount: 12.5, Category: "food", Date: "2026-08-30", SplitWith: "noOne", BlockID: 123}, target)
	if err != nil {
		t.Fatalf("buildBudgetExpense: %v", err)
	}
	if stringField(exp, "description") != "Lunch" || stringField(exp, "category") != "food" || intAny(exp["paidByUserId"]) != 42 {
		t.Fatalf("expense = %#v", exp)
	}
	amount := mapField(exp, "amount")
	if amount["amount"] != 12.5 || stringField(amount, "currencyCode") != "SGD" {
		t.Fatalf("amount = %#v", amount)
	}
	if intAny(exp["blockId"]) != 123 || exp["date"] != "2026-08-30" || exp["associatedDate"] != "2026-08-30" {
		t.Fatalf("date/block = %#v", exp)
	}
	split := mapField(exp, "splitWith")
	users, _ := split["users"].([]any)
	if got := split["type"]; got != "noOne" || len(users) != 0 {
		t.Fatalf("split = %#v", exp["splitWith"])
	}
}

func TestPlanBudgetBuildExpenseRejectsMissingBlockID(t *testing.T) {
	target := map[string]any{
		"userId": 42,
		"itinerary": map[string]any{
			"budget":   map[string]any{"amount": map[string]any{"amount": 500, "currencyCode": "SGD"}},
			"sections": []any{map[string]any{"blocks": []any{map[string]any{"id": 456, "type": "place"}}}},
		},
	}
	_, err := buildBudgetExpense(budgetExpenseFlags{Description: "Lunch", Amount: 12.5, Category: "food", Date: "2026-08-30", SplitWith: "noOne", BlockID: 123}, target)
	if err == nil {
		t.Fatalf("missing block id accepted")
	}
}

func TestPlanBudgetSummarizeExpensePreservesBlockID(t *testing.T) {
	expense := map[string]any{
		"id":          99,
		"description": "Snack",
		"amount":      map[string]any{"amount": 500, "currencyCode": "JPY"},
		"blockId":     123,
	}
	summary := summarizeBudgetExpense(expense)
	if intAny(summary["blockId"]) != 123 || stringField(summary, "description") != "Snack" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestPlanBudgetSplitWithIndividuals(t *testing.T) {
	split, err := budgetSplitWith("individuals", []int{1, 2})
	if err != nil {
		t.Fatalf("budgetSplitWith: %v", err)
	}
	users, _ := split["users"].([]any)
	if split["type"] != "individuals" || len(users) != 2 {
		t.Fatalf("split = %#v", split)
	}
	first, _ := users[0].(map[string]any)
	if first["type"] != "registered" || intAny(first["id"]) != 1 {
		t.Fatalf("first user = %#v", first)
	}
	if _, err := budgetSplitWith("bad", nil); err == nil {
		t.Fatalf("budgetSplitWith bad mode succeeded")
	}
}

func TestPlanBudgetAppendOpsCreatesMissingBudgetOrList(t *testing.T) {
	item := map[string]any{"id": 1}
	ops := budgetAppendOps(defaultBudget(), false, "expenses", item)
	if len(ops) != 1 || ops[0]["oi"] == nil {
		t.Fatalf("missing budget ops = %#v", ops)
	}
	created := ops[0]["oi"].(map[string]any)
	expenses, _ := created["expenses"].([]any)
	if len(expenses) != 1 {
		t.Fatalf("created budget = %#v", created)
	}
	ops = budgetAppendOps(map[string]any{"amount": map[string]any{"currencyCode": "USD"}}, true, "payments", item)
	if len(ops) != 1 || ops[0]["oi"] == nil || ops[0]["od"] != nil {
		t.Fatalf("missing list ops = %#v", ops)
	}
}

func TestPlanBudgetSummarizeBudget(t *testing.T) {
	budget := map[string]any{
		"amount": map[string]any{"amount": 100, "currencyCode": "SGD"},
		"expenses": []any{
			map[string]any{"amount": map[string]any{"amount": 10.0, "currencyCode": "SGD"}, "category": "food"},
			map[string]any{"amount": map[string]any{"amount": 20.0, "currencyCode": "SGD"}, "category": "lodging"},
		},
		"payments": []any{map[string]any{"id": 7}},
	}
	summary := summarizeBudget(budget)
	if summary["expense_count"] != 2 || summary["payment_count"] != 1 {
		t.Fatalf("summary counts = %#v", summary)
	}
	byCategory := summary["totals_by_category"].(map[string]float64)
	byCurrency := summary["totals_by_currency"].(map[string]float64)
	if byCategory["food"] != 10 || byCategory["lodging"] != 20 || byCurrency["SGD"] != 30 {
		t.Fatalf("summary totals = %#v", summary)
	}
}
