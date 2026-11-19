package mcp

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestEtsyCanonicalBuyerTypesMatchMCPRefusal(t *testing.T) {
	if !reflect.DeepEqual(etsyBuyerResourceTypes, store.EtsyBuyerTypes()) {
		t.Fatal("MCP buyer classification drifted from storage policy")
	}
	// Compare against the registered resource universe, not just the canonical
	// list itself: an extra refusal or a missing dependent must fail too.
	var refused []string
	for _, typ := range cli.EtsyWritableResourceTypes() {
		if buyerSQLBypass.MatchString("SELECT * FROM " + typ) {
			refused = append(refused, typ)
		}
	}
	// The payment-check ledger is lazy, not a generated sync resource.
	if buyerSQLBypass.MatchString("SELECT * FROM " + store.EtsyPaymentChecksTable) {
		refused = append(refused, store.EtsyPaymentChecksTable)
	}
	sort.Strings(refused)
	if !reflect.DeepEqual(refused, store.EtsyBuyerTypes()) {
		t.Fatalf("refusal set %v; canonical %v", refused, store.EtsyBuyerTypes())
	}
	for _, typ := range store.EtsyBuyerTypes() {
		for _, allowed := range etsyNonBuyerResourceTypes {
			if typ == allowed {
				t.Fatalf("buyer type allowlisted: %s", typ)
			}
		}
		if !buyerSQLBypass.MatchString("SELECT * FROM " + strings.ToUpper(typ)) {
			t.Fatalf("case bypass: %s", typ)
		}
	}
}
