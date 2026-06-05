// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: regression test for null/empty resource arrays in list envelopes
// (Durianpay returns {"data":{"orders":null,"total":0}} for zero records).
package cli

import (
	"encoding/json"
	"testing"
)

func TestIsEmptyPageNullListWithZeroTotal(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{`{"data":{"orders":null,"total":0}}`, true},
		{`{"data":{"orders":[],"total":0}}`, true},
		{`{"data":{"orders":[{"id":"ord_1"}],"total":1}}`, false},
		// single-object response with incidental null field must NOT be empty-page
		{`{"data":{"id":"ord_1","fees":null}}`, false},
	}
	for _, c := range cases {
		if got := isEmptyPageResponse(json.RawMessage(c.body)); got != c.want {
			t.Errorf("isEmptyPageResponse(%s) = %v, want %v", c.body, got, c.want)
		}
	}
}

func TestExtractPageItemsNullResourceArray(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"null array", `{"data":{"orders":null,"total":0}}`, 0},
		{"empty array", `{"data":{"orders":[],"total":0}}`, 0},
		{"populated array", `{"data":{"orders":[{"id":"ord_1"},{"id":"ord_2"}],"total":2}}`, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			items, _, _ := extractPageItems(json.RawMessage(c.body), "skip")
			if items == nil && c.want == 0 {
				t.Fatalf("extractPageItems(%s) returned nil items; want empty page extraction", c.body)
			}
			if len(items) != c.want {
				t.Errorf("extractPageItems(%s) = %d items, want %d", c.body, len(items), c.want)
			}
		})
	}
}
