package cli

import "testing"

func TestComputeNetPlan_PerCurrencyNetting(t *testing.T) {
	friends := []Friend{
		{
			ID:        1,
			FirstName: "Alex",
			LastName:  "Kim",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "10.00"},
				{CurrencyCode: "EUR", Amount: "-5.00"},
			},
		},
		{
			ID:        2,
			FirstName: "Sam",
			LastName:  "Lee",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "-3.25"},
				{CurrencyCode: "EUR", Amount: "2.00"},
			},
		},
	}

	got := computeNetPlan(friends, 123)
	if got.YouID != 123 {
		t.Fatalf("YouID = %d, want 123", got.YouID)
	}
	if len(got.ByCurrency) != 2 {
		t.Fatalf("len(ByCurrency) = %d, want 2", len(got.ByCurrency))
	}

	usd := got.ByCurrency[1]
	eur := got.ByCurrency[0]
	if eur.CurrencyCode != "EUR" {
		t.Fatalf("ByCurrency[0].CurrencyCode = %q, want EUR", eur.CurrencyCode)
	}
	if usd.CurrencyCode != "USD" {
		t.Fatalf("ByCurrency[1].CurrencyCode = %q, want USD", usd.CurrencyCode)
	}

	if eur.OwedToYou != 2.00 || eur.YouOwe != 5.00 || eur.Net != -3.00 {
		t.Fatalf("EUR summary = %+v, want owed_to_you=2 you_owe=5 net=-3", eur)
	}
	if usd.OwedToYou != 10.00 || usd.YouOwe != 3.25 || usd.Net != 6.75 {
		t.Fatalf("USD summary = %+v, want owed_to_you=10 you_owe=3.25 net=6.75", usd)
	}
}

func TestComputeNetPlan_SavingsCount(t *testing.T) {
	friends := []Friend{
		{
			ID:        7,
			FirstName: "Chris",
			LastName:  "Park",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "15.00"},
			},
			Groups: []FriendGroup{
				{GroupID: 1, Balance: []Balance{{CurrencyCode: "USD", Amount: "4.00"}}},
				{GroupID: 2, Balance: []Balance{{CurrencyCode: "USD", Amount: "6.00"}}},
				{GroupID: 3, Balance: []Balance{{CurrencyCode: "USD", Amount: "5.00"}}},
			},
		},
	}

	got := computeNetPlan(friends, 0)
	if len(got.Savings) != 1 {
		t.Fatalf("len(Savings) = %d, want 1", len(got.Savings))
	}
	s := got.Savings[0]
	if s.CurrencyCode != "USD" {
		t.Fatalf("Savings currency = %q, want USD", s.CurrencyCode)
	}
	if s.PerGroupTransfers != 3 || s.NettedTransfers != 1 || s.Saved != 2 {
		t.Fatalf("Savings = %+v, want per_group=3 netted=1 saved=2", s)
	}
}

func TestComputeNetPlan_DirectionAndZeroExclusion(t *testing.T) {
	friends := []Friend{
		{
			ID:        10,
			FirstName: "Pat",
			LastName:  "Ng",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "8.00"},
			},
			Groups: []FriendGroup{{GroupID: 1, Balance: []Balance{{CurrencyCode: "USD", Amount: "8.00"}}}},
		},
		{
			ID:        11,
			FirstName: "Jordan",
			LastName:  "Wu",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "-4.50"},
			},
			Groups: []FriendGroup{{GroupID: 2, Balance: []Balance{{CurrencyCode: "USD", Amount: "-4.50"}}}},
		},
		{
			ID:        12,
			FirstName: "Taylor",
			LastName:  "Li",
			Balance: []Balance{
				{CurrencyCode: "USD", Amount: "0"},
				{CurrencyCode: "EUR", Amount: ""},
			},
			Groups: []FriendGroup{{GroupID: 3, Balance: []Balance{{CurrencyCode: "USD", Amount: "2.00"}}}},
		},
	}

	got := computeNetPlan(friends, 0)
	if len(got.Plan) != 2 {
		t.Fatalf("len(Plan) = %d, want 2 (zero-net friend excluded)", len(got.Plan))
	}

	if got.Plan[0].Direction != "they_pay" {
		t.Fatalf("first transfer direction = %q, want they_pay", got.Plan[0].Direction)
	}
	if got.Plan[1].Direction != "you_pay" {
		t.Fatalf("second transfer direction = %q, want you_pay", got.Plan[1].Direction)
	}

	if len(got.Savings) != 2 {
		t.Fatalf("len(Savings) = %d, want 2 (EUR and USD)", len(got.Savings))
	}
	for _, s := range got.Savings {
		if s.CurrencyCode == "USD" {
			if s.NettedTransfers != 2 {
				t.Fatalf("USD netted_transfers = %d, want 2 (zero-net friend excluded)", s.NettedTransfers)
			}
		}
	}
}
