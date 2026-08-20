// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runVerify runs the root command under PRINTING_PRESS_VERIFY=1 with
// the given args, returning the captured stdout buffer and the
// Execute error. Centralised so each subtest stays a single
// assertion-shaped block.
//
// Stdout is also wired into stderr's slot so cobra's usage output (if
// a flag parse fails) does not leak into the JSON buffer.
func runVerify(t *testing.T, args []string) (*bytes.Buffer, error) {
	t.Helper()
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs(args)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	return buf, cmd.Execute()
}

// assertVerifyEnvelope asserts the JSON in buf is a verify
// short-circuit envelope with the canonical __pp_verify_synthetic__
// + verify_noop = true shape. Returns the parsed map so subtests
// can make additional command-specific assertions.
func assertVerifyEnvelope(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env), "envelope must be valid JSON: %s", buf.String())
	assert.Equal(t, true, env["__pp_verify_synthetic__"], "envelope must carry __pp_verify_synthetic__=true")
	assert.Equal(t, true, env["verify_noop"], "envelope must carry verify_noop=true")
	assert.Equal(t, "verify_short_circuit", env["reason"], "envelope reason must be verify_short_circuit")
	return env
}

// TestVerifyEnvelope_ReadOnlyCommands covers the eight read-only
// novel commands that opt into the verify-env short-circuit when
// invoked without --json (the default human-shaped rendering path
// is what the short-circuit replaces). The envelope must parse and
// carry the canonical synthetic + noop markers so naive validators
// do not mistake the rendered output for a real result.
//
// Commands that take positional args (similar, compare, artist) get
// dummy IDs that would otherwise hit the store; the short-circuit
// fires before any DB or network access so the args are never
// dereferenced beyond cobra's arg-count validation.
func TestVerifyEnvelope_ReadOnlyCommands(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantCommand string
	}{
		{"presence", []string{"presence"}, "presence"},
		{"random", []string{"random"}, "random"},
		{"browse", []string{"browse"}, "browse"},
		{"similar", []string{"similar", "aic:1"}, "similar"},
		{"compare", []string{"compare", "aic:1", "aic:2"}, "compare"},
		{"artist", []string{"artist", "Hokusai"}, "artist"},
		{"coverage", []string{"coverage"}, "coverage"},
		{"gaps", []string{"gaps"}, "gaps"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			buf, err := runVerify(t, tc.args)
			require.NoError(t, err)
			env := assertVerifyEnvelope(t, buf)
			assert.Equal(t, tc.wantCommand, env["command"], "envelope command should match invoked command")
		})
	}
}

// TestVerifyEnvelope_HiddenJournalCommands covers the three
// mcp:hidden journal subcommands (export, opt-in, compact). Unlike
// the read-only commands, these short-circuit unconditionally under
// PRINTING_PRESS_VERIFY=1 regardless of --json so the verifier never
// triggers a file write, preference-file mutation, or VACUUM. The
// `journal compact` command requires --confirm to clear cobra's
// pre-run validation gate.
func TestVerifyEnvelope_HiddenJournalCommands(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantCommand string
	}{
		{"journal export", []string{"journal", "export"}, "journal export"},
		{"journal opt-in", []string{"journal", "opt-in"}, "journal opt-in"},
		// --confirm is MarkFlagRequired; cobra rejects the run before
		// RunE fires if it is missing, so pass it even though the
		// verify short-circuit will swallow the VACUUM.
		{"journal compact", []string{"journal", "compact", "--confirm"}, "journal compact"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			buf, err := runVerify(t, tc.args)
			require.NoError(t, err)
			env := assertVerifyEnvelope(t, buf)
			assert.Equal(t, tc.wantCommand, env["command"], "envelope command should match invoked subcommand path")
		})
	}
}

// TestVerifyEnvelope_HiddenJournalCommands_JSON confirms the
// mcp:hidden journal subcommands also short-circuit when --json is
// present. The read-only commands intentionally do NOT short-circuit
// with --json (they let the data path run because it is non-mutating
// terminal output), but the hidden commands must short-circuit
// unconditionally because their data path mutates user state.
func TestVerifyEnvelope_HiddenJournalCommands_JSON(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"journal export --json", []string{"journal", "export", "--json"}},
		{"journal opt-in --json", []string{"journal", "opt-in", "--json"}},
		{"journal compact --json", []string{"journal", "compact", "--confirm", "--json"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			buf, err := runVerify(t, tc.args)
			require.NoError(t, err)
			assertVerifyEnvelope(t, buf)
		})
	}
}
