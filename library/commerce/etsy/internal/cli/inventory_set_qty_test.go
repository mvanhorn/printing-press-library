package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func s2PutOf(srv *s2Server) *s2Req {
	for _, r := range srv.all() {
		r := r
		if r.Method == "PUT" {
			return &r
		}
	}
	return nil
}

func TestSetQtyPreviewIsDiffOnly(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	out, _, err := s2Run(t, "inventory", "set-qty", "1000000001", "--product", "MUG-SPECKLE-S", "--qty", "12", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if s2PutOf(srv) != nil {
		t.Fatal("preview wrote the inventory")
	}
	var res s2QtyResult
	s2Decode(t, out, &res)
	if res.Mode != "preview" || res.Applied || (res.Diff.OldQuantity == nil || *res.Diff.OldQuantity != 5) || res.Diff.NewQuantity != 12 ||
		res.Diff.ProductID != "4000000001" || res.Diff.OfferingID != "5000000001" || res.Diff.OfferingsUnchaged != 1 {
		t.Fatalf("diff = %s", out)
	}
}

func TestSetQtyYesPutsFullInventoryChangingOnlyTarget(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	_, _, err := s2Run(t, "inventory", "set-qty", "1000000001", "--product", "4000000001", "--qty", "0", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	put := s2PutOf(srv)
	if put == nil || put.Path != "/listings/1000000001/inventory" {
		t.Fatalf("no PUT: %+v", srv.all())
	}
	// expected = the normalized GET inventory with only the target quantity changed
	want, err := s2InventoryPutBody([]byte(s2Inventory), nil)
	s2Must(t, err)
	wantProds := want["products"].([]any)
	wantProds[0].(map[string]any)["offerings"].([]any)[0].(map[string]any)["quantity"] = 0
	var got map[string]json.RawMessage
	s2Decode(t, put.Body, &got)
	var gotProds []json.RawMessage
	s2Decode(t, string(got["products"]), &gotProds)
	if len(gotProds) != 2 {
		t.Fatalf("products = %s", put.Body)
	}
	for i, p := range wantProds {
		wb, _ := json.Marshal(p)
		var a, b any
		s2Must(t, json.Unmarshal(wb, &a))
		s2Must(t, json.Unmarshal(gotProds[i], &b))
		if i == 1 {
			// untouched product: byte-identical to the normalized read
			var compact strings.Builder
			_ = compact
			cb, _ := json.Marshal(b)
			if string(cb) != string(wb) {
				t.Fatalf("untouched product changed:\n got %s\nwant %s", cb, wb)
			}
		}
	}
	if !strings.Contains(string(gotProds[0]), `"quantity":0`) || !strings.Contains(string(gotProds[1]), `"quantity":2`) {
		t.Fatalf("quantities wrong: %s", put.Body)
	}
	if string(got["price_on_property"]) != "[200]" || strings.Contains(put.Body, "offering_id") || strings.Contains(put.Body, "product_id") || strings.Contains(put.Body, "MUG-OLD") {
		t.Fatalf("PUT shape wrong: %s", put.Body)
	}
	if !strings.Contains(string(gotProds[0]), `"price":24`) || !strings.Contains(string(gotProds[1]), `"price":30`) {
		t.Fatalf("prices not preserved as decimals: %s", put.Body)
	}
}

func TestSetQtyErrors(t *testing.T) {
	srv := s2NewServer(t, s2ShopHandler(nil))
	args := []string{"--no-cache", "--rate-limit", "1000"}
	for name, tc := range map[string]struct {
		args []string
		exit int
	}{
		"unknown product": {[]string{"inventory", "set-qty", "1000000001", "--product", "NOPE", "--qty", "1"}, 3},
		"deleted product": {[]string{"inventory", "set-qty", "1000000001", "--product", "MUG-OLD", "--qty", "1", "--yes"}, 3},
		"no qty":          {[]string{"inventory", "set-qty", "1000000001", "--product", "MUG-SPECKLE-S"}, 2},
		"negative qty":    {[]string{"inventory", "set-qty", "1000000001", "--product", "MUG-SPECKLE-S", "--qty", "-1"}, 2},
		"no product":      {[]string{"inventory", "set-qty", "1000000001", "--qty", "1"}, 2},
		"bad listing":     {[]string{"inventory", "set-qty", "abc", "--product", "x", "--qty", "1"}, 2},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := s2Run(t, append(tc.args, args...)...)
			if err == nil || ExitCode(err) != tc.exit {
				t.Fatalf("err = %v (exit %d), want exit %d", err, ExitCode(err), tc.exit)
			}
		})
	}
	if s2PutOf(srv) != nil {
		t.Fatal("an error path wrote the inventory")
	}
}

func TestSetQtyAmbiguousOfferingNeedsFlag(t *testing.T) {
	inv := `{"products":[{"product_id":9,"sku":"MULTI","offerings":[
	 {"offering_id":71,"quantity":1,"is_enabled":true,"price":{"amount":1000,"divisor":100}},
	 {"offering_id":72,"quantity":2,"is_enabled":true,"price":{"amount":1000,"divisor":100}}]}]}`
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.Method == "GET" {
			w.Write([]byte(inv))
			return true
		}
		return false
	})
	base := []string{"inventory", "set-qty", "1000000001", "--product", "MULTI", "--qty", "7", "--no-cache", "--rate-limit", "1000"}
	_, _, err := s2Run(t, base...)
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--offering") {
		t.Fatalf("err = %v", err)
	}
	out, _, err := s2Run(t, append(base, "--offering", "72", "--json")...)
	s2Must(t, err)
	if !strings.Contains(out, `"offering_id": "72"`) || s2PutOf(srv) != nil {
		t.Fatalf("out = %s", out)
	}
}
