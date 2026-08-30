// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelBatchValidateHelpWires smoke-tests that the batch validate command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBatchValidateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"batch", "validate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("batch validate --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "validate"} {
		if !strings.Contains(help, want) {
			t.Fatalf("batch validate --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestValidateBatchLine(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		valid   bool
		endpoint string
	}{
		{"valid chat completion", `{"custom_id":"req-1","model":"openai/gpt-oss-20b","messages":[{"role":"user","content":"hello"}]}`, true, "/v1/chat/completions"},
		{"valid embedding", `{"custom_id":"emb-1","model":"openai/gpt-oss-20b","input":"hello world"}`, true, "/v1/embeddings"},
		{"invalid JSON", `{not json`, false, ""},
		{"missing model", `{"custom_id":"req-2","messages":[{"role":"user","content":"hi"}]}`, false, "/v1/chat/completions"},
		{"missing custom_id and messages", `{"model":"openai/gpt-oss-20b"}`, false, "/v1/responses"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := validateBatchLine(1, c.line)
			if res.Valid != c.valid {
				t.Fatalf("valid = %v, want %v (issues: %v)", res.Valid, c.valid, res.Issues)
			}
			if res.Endpoint != c.endpoint {
				t.Fatalf("endpoint = %q, want %q", res.Endpoint, c.endpoint)
			}
		})
	}
}
