package cli

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestArrayQueryParamsCommaJoined(t *testing.T) {
	cases := []struct {
		name, param, path string
		args              []string
	}{
		{"batch_ids", "listing_ids", "/listings/batch", []string{"listings", "get-by-ids"}},
		{"batch_includes", "includes", "/listings/batch", []string{"listings", "get-by-ids", "--listing-ids", "1000000001"}},
		{"listing_includes", "includes", "/listings/1000000001", []string{"listings", "get", "1000000001"}},
		{"shop_includes", "includes", "/shops/9000001/listings", []string{"listings", "get-by-shop", "9000001"}},
		{"shop_sections", "shop_section_ids", "/shops/9000001/shop-sections/listings", []string{"listings", "get-by-shop-section-id", "9000001"}},
		{"shipping_ids", "listing_ids", "/listings/batch/shipping", []string{"listings", "get-shipping-by-ids"}},
		{"inventory_ids", "listing_ids", "/listings/batch/inventory", []string{"listing-inventory", "get-listings-inventory-by-listing-ids"}},
		{"payment_ids", "payment_ids", "/shops/9000001/payments", []string{"payments", "get", "9000001"}},
		{"ledger_ids", "ledger_entry_ids", "/shops/9000001/payment-account/ledger-entries/payments", []string{"payments", "get-account-ledger-entry", "9000001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempLearnHome(t)
			for key, value := range map[string]string{
				"ETSY_API_KEY":       "synthetic-key:synthetic-secret",
				"ETSY_SHARED_SECRET": "", "ETSY_OAUTH2": "synthetic-token",
				"ETSY_SHOP_ID": "9000001", "ETSY_NO_LEARN": "true",
			} {
				t.Setenv(key, value)
			}
			for _, jsonInput := range []bool{false, true} {
				input, want, token := "1000000001,1000000002,1000000003", "1000000001,1000000002,1000000003", "ARRAYQ_CSV_VALUE"
				if jsonInput {
					input, want, token = `["1000000001","1000000002"]`, "1000000001,1000000002", "ARRAYQ_JSON_VALUE"
				}
				if tc.param == "includes" {
					input, want = "Images,Shop", "Images,Shop"
					if jsonInput {
						input = `["Images","Shop"]`
					}
				}
				queries := make(chan string, 8)
				fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != tc.path {
						t.Errorf("unexpected path: %s", r.URL.Path)
						http.NotFound(w, r)
						return
					}
					queries <- r.URL.RawQuery
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
				}))
				t.Setenv("ETSY_BASE_URL", fake.URL)
				args := append([]string{}, tc.args...)
				args = append(args, "--"+strings.ReplaceAll(tc.param, "_", "-"), input, "--data-source", "live", "--json", "--no-cache", "--no-learn")
				stdout, stderr, err := runRootArgs(t, args...)
				fake.Close()
				if err != nil {
					t.Fatalf("command: %v stdout=%s stderr=%s", err, stdout, stderr)
				}
				select {
				case raw := <-queries:
					parsed, err := url.ParseQuery(raw)
					if err != nil {
						t.Fatal(err)
					}
					if !jsonInput && !strings.Contains(raw, tc.param+"="+url.QueryEscape(want)) {
						t.Errorf("ARRAYQ_WIRE_ENCODING: %s", raw)
					}
					values := parsed[tc.param]
					if len(values) != 1 {
						t.Errorf("ARRAYQ_SINGLE_KEY: %s = %q", tc.param, values)
					}
					if len(values) != 1 || values[0] != want {
						t.Errorf("%s: got %q want %q", token, values, want)
					}
					if strings.Count(raw, tc.param+"=") != 1 {
						t.Errorf("ARRAYQ_NO_REPEAT: %s", raw)
					}
				default:
					t.Fatal("ARRAYQ_NO_REQUEST")
				}
			}
		})
	}
}

func TestArrayQueryHelperExplodeStillRepeats(t *testing.T) {
	path := appendArrayQueryParam("/listings", "x", "1,2", "form", true)
	parsed, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	values := parsed.Query()["x"]
	if len(values) != 2 || values[0] != "1" || values[1] != "2" {
		t.Fatalf("explode=true should repeat keys: %s", path)
	}
}

func TestArrayQueryShopSectionIdsPaginationKeepsCommaJoined(t *testing.T) {
	withTempLearnHome(t)
	for key, value := range map[string]string{
		"ETSY_API_KEY": "synthetic-key:synthetic-secret", "ETSY_SHARED_SECRET": "",
		"ETSY_OAUTH2": "synthetic-token", "ETSY_SHOP_ID": "9000001", "ETSY_NO_LEARN": "true",
	} {
		t.Setenv(key, value)
	}
	queries := make(chan string, 16)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shops/9000001/shop-sections/listings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		queries <- r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "" || r.URL.Query().Get("offset") == "0" {
			_, _ = w.Write([]byte(`{"count":3,"results":[{"listing_id":1000000001},{"listing_id":1000000002}]}`))
		} else {
			_, _ = w.Write([]byte(`{"count":3,"results":[{"listing_id":1000000003}]}`))
		}
	}))
	defer fake.Close()
	t.Setenv("ETSY_BASE_URL", fake.URL)
	stdout, stderr, err := runRootArgs(t, "listings", "get-by-shop-section-id", "9000001", "--shop-section-ids", "11,22", "--limit", "2", "--all", "--data-source", "live", "--json", "--no-cache", "--no-learn")
	if err != nil {
		t.Fatalf("command: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	count := len(queries)
	if count < 2 {
		t.Fatalf("ARRAYQ_PAGE_COUNT: %d", count)
	}
	for i := 0; i < count; i++ {
		raw := <-queries
		parsed, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		values := parsed["shop_section_ids"]
		if len(values) != 1 || values[0] != "11,22" || strings.Count(raw, "shop_section_ids=") != 1 || !strings.Contains(raw, "shop_section_ids=11%2C22") {
			t.Errorf("ARRAYQ_PAGE_COMMA: %s", raw)
		}
	}
}
