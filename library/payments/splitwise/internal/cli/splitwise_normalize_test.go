package cli

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

func TestComputeNormalize(t *testing.T) {
	tests := []struct {
		name      string
		friends   []Friend
		expenses  []Expense
		youID     int
		rates     map[string]float64
		base      string
		assertion func(t *testing.T, got normalizeResult)
	}{
		{
			name: "multi-currency net converts and totals",
			friends: []Friend{
				{Balance: []Balance{{CurrencyCode: "USD", Amount: "10"}, {CurrencyCode: "EUR", Amount: "5"}}},
				{Balance: []Balance{{CurrencyCode: "GBP", Amount: "2"}}},
			},
			rates: map[string]float64{"EUR": 1.08, "GBP": 1.27},
			base:  "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if got.Net.TotalBase != 17.94 {
					t.Fatalf("Net.TotalBase = %.2f, want 17.94", got.Net.TotalBase)
				}
				if len(got.Net.Rows) != 3 {
					t.Fatalf("len(Net.Rows) = %d, want 3", len(got.Net.Rows))
				}
			},
		},
		{
			name: "missing rate is unconverted and excluded from total",
			friends: []Friend{
				{Balance: []Balance{{CurrencyCode: "USD", Amount: "10"}, {CurrencyCode: "JPY", Amount: "1200"}}},
			},
			rates: map[string]float64{},
			base:  "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if got.Net.TotalBase != 10 {
					t.Fatalf("Net.TotalBase = %.2f, want 10.00", got.Net.TotalBase)
				}
				if len(got.Net.Unconverted) != 1 {
					t.Fatalf("len(Net.Unconverted) = %d, want 1", len(got.Net.Unconverted))
				}
				if got.Net.Unconverted[0].CurrencyCode != "JPY" || got.Net.Unconverted[0].Original != 1200 {
					t.Fatalf("unexpected unconverted row: %+v", got.Net.Unconverted[0])
				}
			},
		},
		{
			name: "base currency passes through at rate one",
			friends: []Friend{
				{Balance: []Balance{{CurrencyCode: "usd", Amount: "3.33"}}},
			},
			rates: map[string]float64{"EUR": 1.08},
			base:  "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if len(got.Net.Rows) != 1 {
					t.Fatalf("len(Net.Rows) = %d, want 1", len(got.Net.Rows))
				}
				row := got.Net.Rows[0]
				if row.CurrencyCode != "USD" || row.Rate != 1 || row.Converted != 3.33 {
					t.Fatalf("unexpected row: %+v", row)
				}
			},
		},
		{
			name: "spend counts only non-payment non-deleted owed_share for you",
			expenses: []Expense{
				{
					CurrencyCode: "USD",
					Users:        []ExpenseUser{{UserID: 9, OwedShare: "10.00"}, {UserID: 1, OwedShare: "10.00"}},
				},
				{
					CurrencyCode: "USD",
					Payment:      true,
					Users:        []ExpenseUser{{UserID: 1, OwedShare: "50.00"}},
				},
				{
					CurrencyCode: "EUR",
					DeletedAt: func() *string {
						s := "2026-01-01"
						return &s
					}(),
					Users: []ExpenseUser{{UserID: 1, OwedShare: "20.00"}},
				},
			},
			youID: 1,
			rates: map[string]float64{},
			base:  "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if len(got.Spend.Rows) != 1 {
					t.Fatalf("len(Spend.Rows) = %d, want 1", len(got.Spend.Rows))
				}
				if got.Spend.Rows[0].Original != 10 {
					t.Fatalf("Spend USD original = %.2f, want 10.00", got.Spend.Rows[0].Original)
				}
			},
		},
		{
			name:     "empty inputs return zero totals",
			friends:  []Friend{},
			expenses: []Expense{},
			rates:    map[string]float64{},
			base:     "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if got.Net.TotalBase != 0 || got.Spend.TotalBase != 0 {
					t.Fatalf("totals not zero: net=%.2f spend=%.2f", got.Net.TotalBase, got.Spend.TotalBase)
				}
				if got.Net.Rows == nil || got.Spend.Rows == nil {
					t.Fatalf("rows should be non-nil")
				}
			},
		},
		{
			name: "rounding to two decimals",
			friends: []Friend{
				{Balance: []Balance{{CurrencyCode: "EUR", Amount: "1.006"}}},
			},
			rates: map[string]float64{"EUR": 1.3333},
			base:  "USD",
			assertion: func(t *testing.T, got normalizeResult) {
				t.Helper()
				if len(got.Net.Rows) != 1 {
					t.Fatalf("len(Net.Rows) = %d, want 1", len(got.Net.Rows))
				}
				row := got.Net.Rows[0]
				if row.Original != 1.01 {
					t.Fatalf("Original = %.2f, want 1.01", row.Original)
				}
				if math.Abs(row.Converted-1.35) > 1e-9 {
					t.Fatalf("Converted = %.2f, want 1.35", row.Converted)
				}
				if got.Net.TotalBase != 1.35 {
					t.Fatalf("Net.TotalBase = %.2f, want 1.35", got.Net.TotalBase)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNormalize(tt.friends, tt.expenses, tt.youID, tt.rates, tt.base)
			tt.assertion(t, got)
		})
	}
}

func TestParseNormalizeRates(t *testing.T) {
	tests := []struct {
		name    string
		rateArg []string
		want    map[string]float64
		wantErr bool
	}{
		{
			name:    "valid map",
			rateArg: []string{"EUR=1.08", "gbp=1.27", "USD=1"},
			want:    map[string]float64{"EUR": 1.08, "GBP": 1.27, "USD": 1},
		},
		{name: "missing equals", rateArg: []string{"EUR"}, wantErr: true},
		{name: "empty currency", rateArg: []string{"=1.2"}, wantErr: true},
		{name: "non-positive", rateArg: []string{"JPY=0"}, wantErr: true},
		{name: "bad factor", rateArg: []string{"CAD=abc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNormalizeRates(tt.rateArg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("rates = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadNormalizeRatesFile(t *testing.T) {
	dir := t.TempDir()

	// empty path → empty map, no error
	if got, err := loadNormalizeRatesFile(""); err != nil || len(got) != 0 {
		t.Fatalf("empty path = (%v, %v), want (empty map, nil)", got, err)
	}

	// valid JSON object, currency codes upper-cased
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"eur":1.08,"GBP":1.27}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadNormalizeRatesFile(good)
	if err != nil {
		t.Fatalf("valid file: unexpected error %v", err)
	}
	if !reflect.DeepEqual(got, map[string]float64{"EUR": 1.08, "GBP": 1.27}) {
		t.Fatalf("valid file rates = %#v", got)
	}

	// non-positive factor → error
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"EUR":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNormalizeRatesFile(bad); err == nil {
		t.Fatalf("non-positive factor: expected error, got nil")
	}

	// empty currency code → error
	emptyCode := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyCode, []byte(`{"  ":1.1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNormalizeRatesFile(emptyCode); err == nil {
		t.Fatalf("empty currency code: expected error, got nil")
	}

	// malformed JSON → error
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNormalizeRatesFile(malformed); err == nil {
		t.Fatalf("malformed JSON: expected error, got nil")
	}

	// missing file → error
	if _, err := loadNormalizeRatesFile(filepath.Join(dir, "nope.json")); err == nil {
		t.Fatalf("missing file: expected error, got nil")
	}
}

func TestComputeNormalize_RatePrecision(t *testing.T) {
	// The Rate field must carry the FULL-precision rate used in the conversion,
	// not a rounded display value, so a reader can reproduce original*rate≈converted.
	friends := []Friend{{Balance: []Balance{{CurrencyCode: "EUR", Amount: "10.00"}}}}
	got := computeNormalize(friends, nil, 1, map[string]float64{"EUR": 1.3333}, "USD")
	if len(got.Net.Rows) != 1 {
		t.Fatalf("len(Net.Rows) = %d, want 1", len(got.Net.Rows))
	}
	row := got.Net.Rows[0]
	if row.Rate != 1.3333 {
		t.Fatalf("Rate = %v, want 1.3333 (full precision, not rounded)", row.Rate)
	}
	// converted = round2(10 * 1.3333) = round2(13.333) = 13.33
	if row.Converted != 13.33 {
		t.Fatalf("Converted = %v, want 13.33", row.Converted)
	}
}

// TestNormalizeUnsyncedUserNote guards that normalize emits the
// "current user not synced" stderr note when get-current-user is unsynced
// (youID == 0), so a silently-zero Spend total is explained rather than
// mistaken for real data. Mirrors balances --by-group.
func TestNormalizeUnsyncedUserNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbPath := defaultDBPath("splitwise-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// An expense is present, but get-current-user is NOT synced -> youID == 0.
	if err := s.Upsert("get-expenses", "1", []byte(`{"id":1,"currency_code":"USD","cost":"10.00","users":[{"user_id":42,"owed_share":"10.00"}]}`)); err != nil {
		t.Fatalf("seed expense: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	flags := &rootFlags{agent: true} // structured output path
	cmd := newNormalizeCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	// --base is required to bypass the "no flags → help" guard (NFlag()==0 check).
	cmd.SetArgs([]string{"--base=USD"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (stderr: %s)", err, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "current user not synced") {
		t.Errorf("expected unsynced-current-user note on stderr, got: %q", errBuf.String())
	}
}
