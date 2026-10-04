// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"github.com/spf13/cobra"
	"reflect"
	"testing"
)

func TestNamedVariadicUsesAdvertisedWordListAndRejectsFlags(t *testing.T) {
	positionals := positionalArgsForCommand(&cobra.Command{Use: "compare [park-id...]"}, nil)
	got, err := positionalArgsFromMCP(map[string]any{"park-id": `"rvpark/1086" yypark/213`}, positionals, true, nil)
	if err != nil || !reflect.DeepEqual(got, []string{"rvpark/1086", "yypark/213"}) {
		t.Fatalf("named variadic got=%#v err=%v", got, err)
	}
	for _, value := range []string{"rvpark/1086 --config=elsewhere", "rvpark/1086 --no-learn=false", "rvpark/1086 --deliver=webhook:https://invalid.example"} {
		if _, err := positionalArgsFromMCP(map[string]any{"park-id": value}, positionals, true, nil); err == nil {
			t.Fatalf("injected flag accepted %q", value)
		}
	}
	for _, value := range []any{[]any{"rvpark/1086", "yypark/213"}, map[string]any{"id": "rvpark/1086"}, true, float64(1)} {
		if _, err := positionalArgsFromMCP(map[string]any{"park-id": value}, positionals, true, nil); err == nil {
			t.Fatalf("nonscalar variadic accepted %#v", value)
		}
	}
	scalar := positionalArgsForCommand(&cobra.Command{Use: "recall <query>"}, nil)
	got, err = positionalArgsFromMCP(map[string]any{"query": "find public parks"}, scalar, false, nil)
	if err != nil || !reflect.DeepEqual(got, []string{"find public parks"}) {
		t.Fatal("scalar text was split", got, err)
	}
}
