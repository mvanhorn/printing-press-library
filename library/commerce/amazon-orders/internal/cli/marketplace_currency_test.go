package cli

import "testing"

func TestMarketplaceCurrency(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		// Dollar-sign marketplaces: currencyForMoneyToken returns "" for "$",
		// so these rely entirely on this fallback being correct.
		{"https://www.amazon.com", "USD"},
		{"https://www.amazon.ca", "CAD"},
		{"https://www.amazon.com.au", "AUD"},
		{"https://www.amazon.com.mx", "MXN"},
		{"https://www.amazon.sg", "SGD"},
		{"https://www.amazon.com.br", "BRL"},
		// Symbol-bearing marketplaces still resolve correctly.
		{"https://www.amazon.in", "INR"},
		{"https://www.amazon.co.uk", "GBP"},
		{"https://www.amazon.de", "EUR"},
		{"https://www.amazon.co.jp", "JPY"},
		// Host forms without scheme / with trailing path.
		{"amazon.com.au", "AUD"},
		{"https://www.amazon.in/gp/css/order-history", "INR"},
		// Unknown or malformed input falls back to USD rather than erroring.
		{"https://example.com", "USD"},
		{"", "USD"},
	}
	for _, tt := range tests {
		if got := marketplaceCurrency(tt.baseURL); got != tt.want {
			t.Errorf("marketplaceCurrency(%q) = %q, want %q", tt.baseURL, got, tt.want)
		}
	}
}

// Every marketplace the CLI accepts for cookie/auth purposes must have a
// currency mapping; otherwise its orders silently mis-tag as USD.
func TestEveryMarketplaceDomainHasACurrency(t *testing.T) {
	for domain := range amazonMarketplaceDomains {
		if _, ok := marketplaceCurrencies[domain]; !ok {
			t.Errorf("marketplaceCurrencies is missing an entry for %q", domain)
		}
	}
}

// Guard the inverse too, so the currency table doesn't drift to domains the
// auth allowlist would reject.
func TestCurrencyTableHasNoUnknownDomains(t *testing.T) {
	for domain := range marketplaceCurrencies {
		if _, ok := amazonMarketplaceDomains[domain]; !ok {
			t.Errorf("marketplaceCurrencies has %q, which is not an accepted Amazon marketplace domain", domain)
		}
	}
}
