package mcp

import (
	"reflect"
	"sort"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
)

func TestEtsyBuyerAllowlistCLIMirror(t *testing.T) {
	etsyWant := append([]string(nil), etsyNonBuyerResourceTypes...)
	sort.Strings(etsyWant)
	if etsyGot := cli.EtsyNonBuyerResourceTypes(); !reflect.DeepEqual(etsyGot, etsyWant) {
		t.Fatalf("CLI allowlist differs from MCP: got %v, want %v", etsyGot, etsyWant)
	}
}
