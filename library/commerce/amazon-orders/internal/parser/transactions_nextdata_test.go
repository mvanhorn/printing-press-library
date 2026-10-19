package parser

import (
	"testing"
)

const nextDataTransactionsHTML = `<html><body><div id="cpefront-mpo-widget">
<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"token":"tok-example",
"state":{"transactionResponseState":{"visibleTransactionResponse":{"transactionList":[
{"formattedAmount":"-$1,032.55","formattedDate":"Oct 19, 2026","orderData":[{"orderDisplayString":"Order #111-1111111-1111111"}],
 "statementDescriptor":"AMZN Mktp US","statusInfo":{"status":"Pending","label":"Pending"},
 "paymentMethodDisplayStringData":{"paymentMethodName":"Example Visa","paymentMethodNumber":{"prefix":"••••","lastDigits":"1234"}}},
{"formattedAmount":"+$24.97","formattedDate":"Oct 2, 2026","orderData":[{"orderDisplayString":"Order #111-2222222-2222222"},{"orderDisplayString":"Order #111-3333333-3333333"}],
 "statementDescriptor":"Amazon.com","statusInfo":{"label":"Refunded"},
 "paymentMethodDisplayStringData":{"paymentMethodName":"Example Visa","paymentMethodNumber":{"lastDigits":"1234"}}}
]},"hasMore":true,"lastEvaluatedPageKey":"key-page-2"}}}}}</script></div></body></html>`

func TestParseTransactionsNextData(t *testing.T) {
	page, err := ParseTransactions([]byte(nextDataTransactionsHTML))
	if err != nil {
		t.Fatal(err)
	}
	if page.Token != "tok-example" || page.NextPageKey != "key-page-2" {
		t.Fatalf("token/cursor = %q/%q", page.Token, page.NextPageKey)
	}
	if len(page.Transactions) != 2 {
		t.Fatalf("got %d transactions, want 2", len(page.Transactions))
	}
	c := page.Transactions[0]
	if c.Date != "2026-10-19" || c.Amount != -1032.55 || c.PaymentLast4 != "1234" || c.PaymentMethod != "Example Visa" ||
		c.OrderID != "111-1111111-1111111" || c.Merchant != "AMZN Mktp US" || c.Status != "Pending" {
		t.Errorf("charge parsed as %+v", c)
	}
	r := page.Transactions[1]
	if r.Amount != 24.97 || len(r.OrderIDs) != 2 || r.OrderIDs[1] != "111-3333333-3333333" || r.Date != "2026-10-02" {
		t.Errorf("refund parsed as %+v", r)
	}
}

func TestParseTransactionsNextDataLastPage(t *testing.T) {
	html := `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"token":"t","state":{"transactionResponseState":{"visibleTransactionResponse":{"transactionList":[]},"hasMore":false,"lastEvaluatedPageKey":"stale"}}}}}</script>`
	page, err := ParseTransactions([]byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageKey != "" || len(page.Transactions) != 0 {
		t.Errorf("hasMore=false should give no cursor and no rows, got %q / %d", page.NextPageKey, len(page.Transactions))
	}
}

func TestParseTransactionsXHR(t *testing.T) {
	body := []byte(`{"type":"GetTransactions","responseCode":"success","displayResponse":{"transactionsList":[
{"formattedAmount":"-$8.18","formattedDate":"Sep 3, 2026","orderData":[{"orderDisplayString":"Order #D01-1111111-1111111"}],
 "statementDescriptor":"Amazon Digital","statusInfo":{"label":"Charged"},
 "paymentMethodDisplayStringData":{"paymentMethodName":"Example Visa","paymentMethodNumber":{"lastDigits":"1234"}}}],
 "lastEvaluatedKey":"key-page-3"}}`)
	page, err := ParseTransactionsXHR(body)
	if err != nil {
		t.Fatal(err)
	}
	if page.NextPageKey != "key-page-3" || len(page.Transactions) != 1 {
		t.Fatalf("cursor %q, %d rows", page.NextPageKey, len(page.Transactions))
	}
	if tx := page.Transactions[0]; tx.Amount != -8.18 || tx.OrderID != "D01-1111111-1111111" || tx.Status != "Charged" {
		t.Errorf("parsed as %+v", tx)
	}
}

func TestParseTransactionsXHRErrorResponse(t *testing.T) {
	for _, body := range []string{
		`{"type":"GetTransactions","responseCode":"failure","errorMessage":"throttled"}`,
		`{"type":"GetTransactions","responseCode":"success"}`,
	} {
		if page, err := ParseTransactionsXHR([]byte(body)); err == nil {
			t.Errorf("ParseTransactionsXHR(%s) = %+v, want error", body, page)
		}
	}
}

func TestParseTransactionsLegacyFallback(t *testing.T) {
	// No __NEXT_DATA__: falls back to the legacy DOM scrape without error.
	page, err := ParseTransactions([]byte(`<html><body><div>nothing here</div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if page.Token != "" || page.NextPageKey != "" {
		t.Errorf("legacy page should not carry paging state")
	}
}
