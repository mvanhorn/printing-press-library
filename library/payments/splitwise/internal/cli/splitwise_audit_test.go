package cli

import "testing"

func TestRunAudit(t *testing.T) {
	sp := func(s string) *string { return &s }

	expenses := []Expense{
		{ID: 1, GroupID: 10, Description: "Settle all balances", Cost: "12.50", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 2, GroupID: 10, Description: "  settle ALL   balances ", Cost: "12.50", CurrencyCode: "USD", Date: "2026-05-20T19:30:00Z", Category: Category{Name: "Meals"}},
		{ID: 3, GroupID: 10, Description: "Coffee", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-21T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 4, GroupID: 10, Description: "Snack", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-22T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 5, GroupID: 10, Description: "Brunch", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-23T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 6, GroupID: 10, Description: "Lunch", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-24T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 7, GroupID: 10, Description: "Dinner", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-25T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 8, GroupID: 10, Description: "Groceries", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-26T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 9, GroupID: 10, Description: "Tea", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-27T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 10, GroupID: 10, Description: "Sandwich", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-28T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 11, GroupID: 10, Description: "Pizza", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-29T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 12, GroupID: 10, Description: "Soup", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-30T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 13, GroupID: 10, Description: "Luxury tasting", Cost: "10000.00", CurrencyCode: "USD", Date: "2026-05-31T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 14, GroupID: 20, Description: "Small one", Cost: "1.00", CurrencyCode: "USD", Date: "2026-05-01T10:00:00Z", Category: Category{Name: "Travel"}},
		{ID: 15, GroupID: 20, Description: "Small two", Cost: "1.00", CurrencyCode: "USD", Date: "2026-05-02T10:00:00Z", Category: Category{Name: "Travel"}},
		{ID: 16, GroupID: 20, Description: "Small three", Cost: "1.00", CurrencyCode: "USD", Date: "2026-05-03T10:00:00Z", Category: Category{Name: "Travel"}},
		{ID: 17, GroupID: 20, Description: "Huge travel", Cost: "1000000.00", CurrencyCode: "USD", Date: "2026-05-04T10:00:00Z", Category: Category{Name: "Travel"}},
		{ID: 18, GroupID: 10, Description: "Payment record", Cost: "999.00", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z", Payment: true, Category: Category{Name: "Meals"}},
		{ID: 19, GroupID: 10, Description: "Deleted record", Cost: "999.00", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z", DeletedAt: sp("2026-01-01"), Category: Category{Name: "Meals"}},
	}

	res := runAudit(expenses, 50)

	if res.ScannedExpenses != 17 {
		t.Fatalf("ScannedExpenses = %d, want 17", res.ScannedExpenses)
	}

	if len(res.Duplicates) != 1 {
		t.Fatalf("duplicates len = %d, want 1", len(res.Duplicates))
	}
	dup := res.Duplicates[0]
	if dup.Count != 2 {
		t.Fatalf("duplicate count = %d, want 2", dup.Count)
	}
	if len(dup.ExpenseIDs) != 2 || dup.ExpenseIDs[0] != 1 || dup.ExpenseIDs[1] != 2 {
		t.Fatalf("duplicate IDs = %v, want [1 2]", dup.ExpenseIDs)
	}

	if len(res.Outliers) != 1 {
		t.Fatalf("outliers len = %d, want 1", len(res.Outliers))
	}
	if res.Outliers[0].ExpenseID != 13 {
		t.Fatalf("outlier expense_id = %d, want 13", res.Outliers[0].ExpenseID)
	}

	for _, o := range res.Outliers {
		if o.ExpenseID == 3 || o.ExpenseID == 4 || o.ExpenseID == 5 || o.ExpenseID == 6 || o.ExpenseID == 7 || o.ExpenseID == 8 || o.ExpenseID == 9 || o.ExpenseID == 10 || o.ExpenseID == 11 || o.ExpenseID == 12 {
			t.Fatalf("normal meal expense was incorrectly flagged as outlier: %d", o.ExpenseID)
		}
		if o.Category == "Travel" {
			t.Fatalf("small-category travel outlier should have been skipped: %+v", o)
		}
	}
}

func TestRunAuditSingletonNotDuplicate(t *testing.T) {
	expenses := []Expense{
		{ID: 100, GroupID: 1, Description: "Solo expense", Cost: "8.00", CurrencyCode: "USD", Date: "2026-05-01T10:00:00Z", Category: Category{Name: "Misc"}},
	}
	res := runAudit(expenses, 50)
	if len(res.Duplicates) != 0 {
		t.Fatalf("duplicates len = %d, want 0", len(res.Duplicates))
	}
}
