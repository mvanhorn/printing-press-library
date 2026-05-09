// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH models-query-tests: hand-built unit tests for the DSL parser.

import "testing"

func TestParseSizeSuffix(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"64k", 64_000, false},
		{"200000", 200_000, false},
		{"1m", 1_000_000, false},
		{"1.5m", 1_500_000, false},
		{"  3K  ", 3_000, false},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := parseSizeSuffix(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseSizeSuffix(%q) = %v, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSizeSuffix(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseSizeSuffix(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestQueryTokenRegex(t *testing.T) {
	cases := []struct {
		in           string
		matches      bool
		key, op, val string
	}{
		{"tools=true", true, "tools", "=", "true"},
		{"ctx>=64k", true, "ctx", ">=", "64k"},
		{"cost.completion<1", true, "cost.completion", "<", "1"},
		{"provider!=anthropic", true, "provider", "!=", "anthropic"},
		{"name=gpt-5", true, "name", "=", "gpt-5"},
		{"justaword", false, "", "", ""},
		{"two words", false, "", "", ""}, // no = either side
	}
	for _, c := range cases {
		m := queryTokenRe.FindStringSubmatch(c.in)
		if c.matches {
			if m == nil {
				t.Errorf("queryTokenRe(%q) = nil, want match", c.in)
				continue
			}
			if m[1] != c.key || m[2] != c.op || m[3] != c.val {
				t.Errorf("queryTokenRe(%q) = (%q,%q,%q), want (%q,%q,%q)",
					c.in, m[1], m[2], m[3], c.key, c.op, c.val)
			}
		} else if m != nil {
			t.Errorf("queryTokenRe(%q) matched %v, want no match", c.in, m)
		}
	}
}

func TestJsonPathExpr(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"pricing.completion", "json_extract(data, '$.pricing.completion')"},
		{"architecture.modality", "json_extract(data, '$.architecture.modality')"},
		{"id", "json_extract(data, '$.id')"},
	}
	for _, c := range cases {
		got := jsonPathExpr(c.path)
		if got != c.want {
			t.Errorf("jsonPathExpr(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
