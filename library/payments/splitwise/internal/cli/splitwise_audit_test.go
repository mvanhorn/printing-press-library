package cli

import "testing"

func TestRunAudit(t *testing.T) {
	sp := func(s string) *string { return &s }

	expenses := []Expense{
		{ID: 1, GroupID: 10, Description: "Settle all balances", Cost: "12.50", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 2, GroupID: 10, Description: "  settle ALL   balances ", Cost: "12.50", CurrencyCode: "USD", Date: "2026-05-20T19:30:00Z", Category: Category{Name: "Meals"}},
		{ID: 3, GroupID: 10, Description: "Coffee", Cost: "9.90", CurrencyCode: "USD", Date: "2026-05-21T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 4, GroupID: 10, Description: "Snack", Cost: "9.95", CurrencyCode: "USD", Date: "2026-05-22T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 5, GroupID: 10, Description: "Brunch", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-23T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 6, GroupID: 10, Description: "Lunch", Cost: "10.05", CurrencyCode: "USD", Date: "2026-05-24T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 7, GroupID: 10, Description: "Dinner", Cost: "10.10", CurrencyCode: "USD", Date: "2026-05-25T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 8, GroupID: 10, Description: "Groceries", Cost: "10.15", CurrencyCode: "USD", Date: "2026-05-26T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 9, GroupID: 10, Description: "Tea", Cost: "9.85", CurrencyCode: "USD", Date: "2026-05-27T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 10, GroupID: 10, Description: "Sandwich", Cost: "9.80", CurrencyCode: "USD", Date: "2026-05-28T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 11, GroupID: 10, Description: "Pizza", Cost: "10.20", CurrencyCode: "USD", Date: "2026-05-29T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 12, GroupID: 10, Description: "Soup", Cost: "10.25", CurrencyCode: "USD", Date: "2026-05-30T10:00:00Z", Category: Category{Name: "Meals"}},
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
	if res.DuplicatesTotal != 1 {
		t.Fatalf("duplicates total = %d, want 1", res.DuplicatesTotal)
	}
	dup := res.Duplicates[0]
	if dup.Count != 2 {
		t.Fatalf("duplicate count = %d, want 2", dup.Count)
	}
	if len(dup.ExpenseIDs) != 2 || dup.ExpenseIDs[0] != 1 || dup.ExpenseIDs[1] != 2 {
		t.Fatalf("duplicate IDs = %v, want [1 2]", dup.ExpenseIDs)
	}

	if len(res.Outliers) == 0 {
		t.Fatalf("outliers len = %d, want >= 1", len(res.Outliers))
	}
	if res.OutliersTotal < 1 {
		t.Fatalf("outliers total = %d, want >= 1", res.OutliersTotal)
	}

	foundWhale := false
	for _, o := range res.Outliers {
		if o.ExpenseID == 13 {
			foundWhale = true
		}
		if o.ExpenseID == 3 || o.ExpenseID == 4 || o.ExpenseID == 5 || o.ExpenseID == 6 || o.ExpenseID == 7 || o.ExpenseID == 8 || o.ExpenseID == 9 || o.ExpenseID == 10 || o.ExpenseID == 11 || o.ExpenseID == 12 {
			t.Fatalf("normal meal expense was incorrectly flagged as outlier: %d", o.ExpenseID)
		}
		if o.Category == "Travel" {
			t.Fatalf("small-category travel outlier should have been skipped: %+v", o)
		}
	}
	if !foundWhale {
		t.Fatalf("expected whale expense 13 to be flagged, got outliers: %+v", res.Outliers)
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

func TestDetectDuplicateClustersCurrencyIsolation(t *testing.T) {
	expenses := []Expense{
		{ID: 1, GroupID: 42, Description: "Team lunch", Cost: "50.00", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z"},
		{ID: 2, GroupID: 42, Description: "team   lunch", Cost: "50.00", CurrencyCode: "EUR", Date: "2026-05-20T19:00:00Z"},
	}

	clusters := detectDuplicateClusters(expenses)
	if len(clusters) != 0 {
		t.Fatalf("clusters len = %d, want 0 for mixed currency", len(clusters))
	}
}

func TestDetectDuplicateClustersSameCurrencyStillClusters(t *testing.T) {
	expenses := []Expense{
		{ID: 10, GroupID: 42, Description: "Team lunch", Cost: "50.00", CurrencyCode: "USD", Date: "2026-05-20T10:00:00Z"},
		{ID: 11, GroupID: 42, Description: "team   lunch", Cost: "50.00", CurrencyCode: "USD", Date: "2026-05-20T19:00:00Z"},
	}

	clusters := detectDuplicateClusters(expenses)
	if len(clusters) != 1 {
		t.Fatalf("clusters len = %d, want 1", len(clusters))
	}
	if clusters[0].CurrencyCode != "USD" {
		t.Fatalf("cluster currency = %q, want USD", clusters[0].CurrencyCode)
	}
	if clusters[0].Count != 2 {
		t.Fatalf("cluster count = %d, want 2", clusters[0].Count)
	}
}

func TestDetectCostOutliersFlagsWhaleInSmallCategory(t *testing.T) {
	expenses := []Expense{
		{ID: 1, Description: "A", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-01T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 2, Description: "B", Cost: "10.25", CurrencyCode: "USD", Date: "2026-05-02T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 3, Description: "C", Cost: "9.95", CurrencyCode: "USD", Date: "2026-05-03T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 4, Description: "D", Cost: "10.10", CurrencyCode: "USD", Date: "2026-05-04T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 5, Description: "E", Cost: "10.05", CurrencyCode: "USD", Date: "2026-05-05T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 6, Description: "F", Cost: "10.15", CurrencyCode: "USD", Date: "2026-05-06T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 7, Description: "G", Cost: "9.90", CurrencyCode: "USD", Date: "2026-05-07T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 8, Description: "H", Cost: "10.20", CurrencyCode: "USD", Date: "2026-05-08T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 9, Description: "I", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-09T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 10, Description: "Whale", Cost: "100000.00", CurrencyCode: "USD", Date: "2026-05-10T10:00:00Z", Category: Category{Name: "Meals"}},
	}
	outliers := detectCostOutliers(expenses)
	if len(outliers) != 1 {
		t.Fatalf("outliers len = %d, want 1", len(outliers))
	}
	if outliers[0].ExpenseID != 10 {
		t.Fatalf("outlier expense_id = %d, want 10", outliers[0].ExpenseID)
	}
}

func TestDetectCostOutliersFlagsUnusuallyCheapItem(t *testing.T) {
	expenses := []Expense{
		{ID: 1, Description: "A", Cost: "80.00", CurrencyCode: "USD", Date: "2026-05-01T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 2, Description: "B", Cost: "82.00", CurrencyCode: "USD", Date: "2026-05-02T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 3, Description: "C", Cost: "78.00", CurrencyCode: "USD", Date: "2026-05-03T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 4, Description: "D", Cost: "81.00", CurrencyCode: "USD", Date: "2026-05-04T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 5, Description: "E", Cost: "79.00", CurrencyCode: "USD", Date: "2026-05-05T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 6, Description: "F", Cost: "80.50", CurrencyCode: "USD", Date: "2026-05-06T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 7, Description: "G", Cost: "79.50", CurrencyCode: "USD", Date: "2026-05-07T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 8, Description: "H", Cost: "80.00", CurrencyCode: "USD", Date: "2026-05-08T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 9, Description: "Misfiled $1 dinner", Cost: "1.00", CurrencyCode: "USD", Date: "2026-05-09T10:00:00Z", Category: Category{Name: "Meals"}},
	}
	outliers := detectCostOutliers(expenses)
	if len(outliers) != 1 {
		t.Fatalf("outliers len = %d, want 1 (the $1 misfiled item)", len(outliers))
	}
	if outliers[0].ExpenseID != 9 {
		t.Fatalf("outlier expense_id = %d, want 9", outliers[0].ExpenseID)
	}
}

func TestDetectCostOutliersSkipsMADZero(t *testing.T) {
	expenses := []Expense{
		{ID: 1, Description: "A", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-01T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 2, Description: "B", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-02T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 3, Description: "C", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-03T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 4, Description: "D", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-04T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 5, Description: "E", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-05T10:00:00Z", Category: Category{Name: "Meals"}},
		{ID: 6, Description: "F", Cost: "100000.00", CurrencyCode: "USD", Date: "2026-05-06T10:00:00Z", Category: Category{Name: "Meals"}},
	}
	outliers := detectCostOutliers(expenses)
	if len(outliers) != 0 {
		t.Fatalf("outliers len = %d, want 0 when MAD == 0", len(outliers))
	}
}

func TestMedian(t *testing.T) {
	odd := median([]float64{5, 1, 9})
	if odd != 5 {
		t.Fatalf("median odd = %v, want 5", odd)
	}
	even := median([]float64{10, 2, 6, 4})
	if even != 5 {
		t.Fatalf("median even = %v, want 5", even)
	}
}
