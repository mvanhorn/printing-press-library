package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// costcoGraphQLPath is the single endpoint every Costco orders query rides.
const costcoGraphQLPath = "/orders/graphql"

// orDefault returns v unless it is blank, in which case it returns def.
func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// num tolerates Costco numeric fields arriving as either JSON numbers or
// JSON-encoded strings (some money/amount fields are quoted upstream).
// Decoding a quoted number into a bare float64 would error and drop the whole
// receipt, so accept both shapes; absent/null/unparseable stays 0.
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		return nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // leave zero rather than fail the whole decode
	}
	*n = num(f)
	return nil
}

func (n num) float() float64 { return float64(n) }
// flexStr tolerates Costco string-typed fields arriving as either JSON strings
// or bare JSON numbers (e.g. itemUPCNumber, warehouseNumber). Without this,
// json.Unmarshal rejects a bare 847 into a string field and drops the receipt.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		return nil
	}
	s = strings.Trim(s, `"`)
	*f = flexStr(s)
	return nil
}

func (f flexStr) String() string { return string(f) }

// costcoItem is one line item on a receipt.
type costcoItem struct {
	ItemNumber       flexStr `json:"itemNumber"`
	UPC              flexStr `json:"itemUPCNumber"`
	Description      string  `json:"itemDescription01"`
	Description2     string  `json:"itemDescription02"`
	Amount           num     `json:"amount"`
	Unit             num     `json:"unit"`
	UnitPriceAmount  num     `json:"itemUnitPriceAmount"`
	DepartmentNumber flexStr `json:"itemDepartmentNumber"`
	TaxFlag          string `json:"taxFlag"`
	FuelGradeCode    string `json:"fuelGradeCode"`
	FuelGradeDesc    string `json:"fuelGradeDescription"`
}

// costcoCoupon is one coupon/discount on a receipt.
type costcoCoupon struct {
	CouponNumber flexStr `json:"couponNumber"`
	Amount       num    `json:"amountCoupon"`
}

// costcoTender is one payment method on a receipt. displayAccountNumber is
// already masked by Costco (last 4); never expand it.
type costcoTender struct {
	TenderTypeName       string `json:"tenderTypeName"`
	Amount               num    `json:"amountTender"`
	DisplayAccountNumber string `json:"displayAccountNumber"`
}

// costcoReceipt is one transaction (in-warehouse, gas, or carwash).
type costcoReceipt struct {
	DocumentType        string         `json:"documentType"`
	ReceiptType         string         `json:"receiptType"`
	MembershipNumber    flexStr        `json:"membershipNumber"`
	TransactionType     string         `json:"transactionType"`
	TransactionDate     string         `json:"transactionDate"`
	TransactionDateTime string         `json:"transactionDateTime"`
	WarehouseName       string         `json:"warehouseName"`
	WarehouseNumber     flexStr        `json:"warehouseNumber"`
	WarehouseShortName  string         `json:"warehouseShortName"`
	Total               num            `json:"total"`
	SubTotal            num            `json:"subTotal"`
	Taxes               num            `json:"taxes"`
	TotalItemCount      num            `json:"totalItemCount"`
	InstantSavings      num            `json:"instantSavings"`
	TransactionBarcode  string         `json:"transactionBarcode"`
	ItemArray           []costcoItem   `json:"itemArray"`
	CouponArray         []costcoCoupon `json:"couponArray"`
	TenderArray         []costcoTender `json:"tenderArray"`
}

// channel maps documentType to a human channel label.
func (r costcoReceipt) channel() string {
	switch strings.ToLower(r.DocumentType) {
	case "warehouse", "inwarehouse":
		return "warehouse"
	case "gas", "gasstation", "fuel":
		return "gas"
	case "carwash":
		return "carwash"
	case "gasandcarwash":
		return "gas+carwash"
	default:
		if r.DocumentType != "" {
			return strings.ToLower(r.DocumentType)
		}
		return "warehouse"
	}
}

// receiptsQuery is the in-warehouse/gas/carwash receipts query. Field set is
// the union the hand-built commands consume.
const receiptsQuery = `query receipts($startDate: String!, $endDate: String!) {
  receipts(startDate: $startDate, endDate: $endDate) {
    documentType
    receiptType
    membershipNumber
    transactionType
    transactionDate
    transactionDateTime
    warehouseName
    warehouseNumber
    warehouseShortName
    total
    subTotal
    taxes
    totalItemCount
    instantSavings
    transactionBarcode
    itemArray {
      itemNumber
      itemUPCNumber
      itemDescription01
      itemDescription02
      amount
      unit
      itemUnitPriceAmount
      itemDepartmentNumber
      taxFlag
      fuelGradeCode
      fuelGradeDescription
    }
    couponArray {
      couponNumber
      amountCoupon
    }
    tenderArray {
      tenderTypeName
      amountTender
      displayAccountNumber
    }
  }
}`

type graphQLError struct {
	Message string `json:"message"`
}

type receiptsEnvelope struct {
	Data struct {
		Receipts []costcoReceipt `json:"receipts"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// fetchReceipts runs the receipts query for [startDate, endDate] (YYYY-MM-DD)
// and returns the receipts. It surfaces GraphQL-level errors as Go errors so an
// empty result is never confused with an API failure.
func fetchReceipts(ctx context.Context, flags *rootFlags, startDate, endDate string) ([]costcoReceipt, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"query":     receiptsQuery,
		"variables": map[string]string{"startDate": startDate, "endDate": endDate},
	}
	data, _, err := c.PostQueryWithParams(ctx, costcoGraphQLPath, nil, body)
	if err != nil {
		return nil, err
	}
	var env receiptsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding receipts response: %w", err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("costco API error: %s", env.Errors[0].Message)
	}
	return env.Data.Receipts, nil
}

// --- date helpers ---

const dateLayout = "2006-01-02"

// todayDate returns today's date in YYYY-MM-DD (UTC-stable enough for a date
// range query; Costco treats the bound as a calendar date).
func todayDate() string {
	return time.Now().Format(dateLayout)
}

// resolveRange turns --since/--until/--years inputs into concrete YYYY-MM-DD
// bounds. since may be a date (YYYY-MM-DD) or a loose duration (e.g. 30d, 1y).
// When since is empty, years controls the lookback (default applied by caller).
func resolveRange(since, until string, years int) (string, string, error) {
	end := strings.TrimSpace(until)
	if end == "" {
		end = todayDate()
	} else if _, err := time.Parse(dateLayout, end); err != nil {
		return "", "", fmt.Errorf("--until must be YYYY-MM-DD: %q", until)
	}
	start := strings.TrimSpace(since)
	if start == "" {
		start = startFromYears(years)
		return start, end, nil
	}
	if _, err := time.Parse(dateLayout, start); err == nil {
		return start, end, nil
	}
	// Treat as a loose duration before "now".
	d, err := parseLooseDuration(start)
	if err != nil {
		return "", "", fmt.Errorf("--since must be YYYY-MM-DD or a duration like 30d/6mo/1y: %q", since)
	}
	return time.Now().Add(-d).Format(dateLayout), end, nil
}

func startFromYears(years int) string {
	if years <= 0 {
		years = 2
	}
	return time.Now().AddDate(-years, 0, 0).Format(dateLayout)
}

// parseLooseDuration accepts Go durations plus d (day), w (week), mo (month),
// y (year) suffixes that time.ParseDuration rejects.
func parseLooseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	mult := map[string]time.Duration{
		"mo": 30 * 24 * time.Hour,
		"y":  365 * 24 * time.Hour,
		"w":  7 * 24 * time.Hour,
		"d":  24 * time.Hour,
	}
	for _, suf := range []string{"mo", "y", "w", "d"} {
		if strings.HasSuffix(s, suf) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(s, suf), 64)
			if err != nil {
				return 0, err
			}
			return time.Duration(n * float64(mult[suf])), nil
		}
	}
	return time.ParseDuration(s)
}

// --- JWT expiry decode (for doctor) ---

// decodeJWTExp extracts the exp claim from a JWT without verifying the
// signature (the CLI never mints tokens; it only reports staleness).
func decodeJWTExp(token string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
