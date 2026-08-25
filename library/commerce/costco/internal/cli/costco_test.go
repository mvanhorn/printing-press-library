package cli

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestNumUnmarshalTolerant(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{`12.5`, 12.5},
		{`"12.5"`, 12.5},
		{`"$0"`, 0}, // unparseable stays zero, no decode error
		{`null`, 0},
		{`0`, 0},
		{`"1000"`, 1000},
	}
	for _, tc := range cases {
		var n num
		if err := json.Unmarshal([]byte(tc.in), &n); err != nil {
			t.Fatalf("Unmarshal(%s) error = %v", tc.in, err)
		}
		if n.float() != tc.want {
			t.Fatalf("num(%s) = %v, want %v", tc.in, n.float(), tc.want)
		}
	}
}

func TestNumInsideStruct(t *testing.T) {
	// A quoted amount must not drop the whole receipt.
	var r costcoReceipt
	if err := json.Unmarshal([]byte(`{"total":"42.00","totalItemCount":3}`), &r); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if r.Total.float() != 42 || int(r.TotalItemCount.float()) != 3 {
		t.Fatalf("total=%v count=%v, want 42 and 3", r.Total.float(), r.TotalItemCount.float())
	}
}

func TestParseLooseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"24h", 24 * time.Hour},
		{"7d", 7 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"6mo", 6 * 30 * 24 * time.Hour},
		{"1y", 365 * 24 * time.Hour},
	}
	for _, tc := range cases {
		got, err := parseLooseDuration(tc.in)
		if err != nil {
			t.Fatalf("parseLooseDuration(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseLooseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseLooseDuration("garbage"); err == nil {
		t.Fatal("expected error for unparseable duration")
	}
}

func TestResolveRange(t *testing.T) {
	t.Run("explicit dates pass through", func(t *testing.T) {
		s, e, err := resolveRange("2024-01-01", "2024-06-30", 2)
		if err != nil || s != "2024-01-01" || e != "2024-06-30" {
			t.Fatalf("got %s..%s err=%v", s, e, err)
		}
	})
	t.Run("empty since uses years lookback", func(t *testing.T) {
		s, _, err := resolveRange("", "", 3)
		if err != nil {
			t.Fatal(err)
		}
		want := time.Now().AddDate(-3, 0, 0).Format(dateLayout)
		if s != want {
			t.Fatalf("start = %s, want %s", s, want)
		}
	})
	t.Run("bad until errors", func(t *testing.T) {
		if _, _, err := resolveRange("", "nope", 2); err == nil {
			t.Fatal("expected error for bad --until")
		}
	})
}

func TestChannelMapping(t *testing.T) {
	cases := map[string]string{
		"warehouse":     "warehouse",
		"inWarehouse":   "warehouse",
		"gasStation":    "gas",
		"carwash":       "carwash",
		"gasAndCarWash": "gas+carwash",
	}
	for dt, want := range cases {
		r := costcoReceipt{DocumentType: dt}
		if got := r.channel(); got != want {
			t.Fatalf("channel(%q) = %q, want %q", dt, got, want)
		}
	}
}

func TestMatchesType(t *testing.T) {
	gas := costcoReceipt{DocumentType: "gasStation"}
	wh := costcoReceipt{DocumentType: "warehouse"}
	if !matchesType(gas, "gas") || matchesType(gas, "warehouse") {
		t.Fatal("gas filter mismatch")
	}
	if !matchesType(wh, "all") || !matchesType(gas, "") {
		t.Fatal("all/empty filter should include everything")
	}
}

func TestDecodeJWTExp(t *testing.T) {
	exp := time.Now().Add(15 * time.Minute).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":` + strconv.FormatInt(exp, 10) + `}`))
	token := "aGVhZGVy." + payload + ".c2ln"
	got, ok := decodeJWTExp(token)
	if !ok {
		t.Fatal("expected exp to decode")
	}
	if got.Unix() != exp {
		t.Fatalf("exp = %d, want %d", got.Unix(), exp)
	}
	if _, ok := decodeJWTExp("not-a-jwt"); ok {
		t.Fatal("expected non-JWT to fail")
	}
}
