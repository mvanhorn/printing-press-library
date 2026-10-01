// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

// A negative transaction in an income category (paycheck reversal) reduces
// cashflow income instead of disappearing from every report.
func TestCashflowCountsIncomeReversal(t *testing.T) {
	ctx := context.Background()
	sept := actual.ReportScope{From: 20260901, To: 20260930}
	base, err := fixtureLedger(t).Cashflow(ctx, sept)
	if err != nil {
		t.Fatal(err)
	}
	l := mutatedFixtureLedger(t,
		`INSERT INTO transactions (id,acct,category,amount,description,date,sort_order,cleared) VALUES
			('txn-pay-rev','acct-checking','cat-salary',-5000,'pay-employer',20260922,30,1)`)
	got, err := l.Cashflow(ctx, sept)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Income != base[0].Income-5000 || got[0].Spending != base[0].Spending || got[0].Net != base[0].Net-5000 {
		t.Fatalf("cashflow after reversal = %+v, base %+v", got[0], base[0])
	}
}

// Monthly charges on the same day are 28-31 days apart; the series check
// must tolerate that drift so a wide --days window doesn't flag them.
func TestDuplicatesSkipMonthlySeries(t *testing.T) {
	l := mutatedFixtureLedger(t,
		`INSERT INTO transactions (id,acct,category,amount,description,date,sort_order,cleared) VALUES
			('txn-m1','acct-card','cat-groceries',-1599,'pay-netflix',20260115,40,1),
			('txn-m2','acct-card','cat-groceries',-1599,'pay-netflix',20260215,41,1),
			('txn-m3','acct-card','cat-groceries',-1599,'pay-netflix',20260315,42,1),
			('txn-m4','acct-card','cat-groceries',-1599,'pay-netflix',20260415,43,1)`)
	pairs, err := l.FindDuplicates(context.Background(), actual.DuplicateOptions{Days: 31, From: 20260101, To: 20260430, MinConfidence: 0.6})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pairs {
		if strings.HasPrefix(p.AID, "txn-m") {
			t.Errorf("monthly series flagged as duplicate: %+v", p)
		}
	}
}

// The sync-server client must not follow a redirect to another host: it
// would resend X-ACTUAL-TOKEN (and, on 307/308, the login password).
func TestClientRefusesCrossHostRedirect(t *testing.T) {
	var leaked bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := actual.New(srv.URL, 5*time.Second)
	if err := c.Login(context.Background(), "hunter2"); err == nil {
		t.Fatal("login through a cross-host redirect succeeded")
	}
	if leaked {
		t.Fatal("request reached the redirect target")
	}
}
