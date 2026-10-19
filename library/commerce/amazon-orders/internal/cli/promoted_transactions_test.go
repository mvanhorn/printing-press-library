package cli

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
)

type fakeTxnPoster struct {
	pages map[string]string // exclusiveStartKey -> response body
	calls []map[string]any
	token string
}

func (f *fakeTxnPoster) PostWithHeaders(path string, body any, headers map[string]string) (json.RawMessage, int, error) {
	if path != transactionsXHRPath {
		return nil, 0, fmt.Errorf("unexpected path %s", path)
	}
	f.token = headers["x-amzn-upx-token"]
	b := body.(map[string]any)
	f.calls = append(f.calls, b)
	resp, ok := f.pages[b["exclusiveStartKey"].(string)]
	if !ok {
		return nil, 0, fmt.Errorf("unexpected key %v", b["exclusiveStartKey"])
	}
	return json.RawMessage(resp), 200, nil
}

func xhrPage(next string, dates ...string) string {
	var rows []map[string]any
	for _, d := range dates {
		rows = append(rows, map[string]any{"formattedAmount": "-$1.00", "formattedDate": d,
			"statusInfo": map[string]any{"label": "Charged"}})
	}
	b, _ := json.Marshal(map[string]any{"responseCode": "success",
		"displayResponse": map[string]any{"transactionsList": rows, "lastEvaluatedKey": next}})
	return string(b)
}

func firstTxnPage() *parser.TransactionsPage {
	return &parser.TransactionsPage{Token: "tok", NextPageKey: "k2",
		Transactions: []parser.Transaction{{Date: "2026-10-01", Amount: -1}}}
}

func TestFetchOlderTransactionsStopsAtSince(t *testing.T) {
	f := &fakeTxnPoster{pages: map[string]string{
		"k2": xhrPage("k3", "Sep 20, 2026", "Sep 2, 2026"),
		"k3": xhrPage("k4", "Aug 30, 2026", "Aug 10, 2026"),
		"k4": xhrPage("", "Jul 1, 2026"),
	}}
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rows, err := fetchOlderTransactions(f, firstTxnPage(), since, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || f.token != "tok" {
		t.Fatalf("calls=%d token=%q, want 2 calls with the page token", len(f.calls), f.token)
	}
	if len(rows) != 3 || rows[len(rows)-1].Date != "2026-09-02" {
		t.Fatalf("rows = %+v, want the 3 rows on or after 2026-09-01", rows)
	}
}

func TestFetchOlderTransactionsMaxPagesOneMakesNoRequests(t *testing.T) {
	f := &fakeTxnPoster{}
	since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rows, err := fetchOlderTransactions(f, firstTxnPage(), since, 1)
	if err != nil || len(f.calls) != 0 || len(rows) != 1 {
		t.Fatalf("rows=%d calls=%d err=%v, want first page only and no requests", len(rows), len(f.calls), err)
	}
}

func TestFetchOlderTransactionsSurfacesErrorResponse(t *testing.T) {
	f := &fakeTxnPoster{pages: map[string]string{"k2": `{"responseCode":"failure"}`}}
	if _, err := fetchOlderTransactions(f, firstTxnPage(), time.Time{}, 5); err == nil {
		t.Fatal("want error for a failed get-transactions page, got nil")
	}
}
