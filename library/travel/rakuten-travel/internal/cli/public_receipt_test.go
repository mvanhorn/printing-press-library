// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
)

type publicReceiptCase struct {
	name string
	path []string
	args []string
}

func publicReceiptCases() []publicReceiptCase {
	in, _ := futureTravelDates()
	return []publicReceiptCase{
		{"areas list", []string{"areas", "list"}, []string{"--parent", "tokyo"}},
		{"hotels search", []string{"hotels", "search"}, []string{"--query", "品川"}},
		{"hotels show", []string{"hotels", "show"}, []string{"--hotel", "51870"}},
		{"offers search", []string{"offers", "search"}, offerCLIArgs("search")[2:]},
		{"offers show", []string{"offers", "show"}, append(offerCLIArgs("show")[2:], "--plan", "3951989", "--room", "s-double-")},
		{"compare", []string{"compare"}, []string{"--hotels", "51870", "--checkins", in, "--nights", "2", "--rooms", "1", "--adults-per-room", "2"}},
	}
}

func runReceiptRoot(root *cobra.Command, args []string) (string, string, error) {
	root.SetArgs(args)
	var out, diagnostics bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	err := root.Execute()
	return out.String(), diagnostics.String(), err
}

func assertNoReceiptArtifacts(t *testing.T, home string) {
	t.Helper()
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("receipt/audit artifacts were written: %v", entries)
	}
}

func TestFocusedTravelRejectsExplicitReceiptOptionsBeforeIO(t *testing.T) {
	for _, leaf := range publicReceiptCases() {
		for _, option := range []struct {
			name  string
			value func(string) []string
		}{
			{"receipt", func(string) []string { return []string{"--receipt"} }},
			{"receipt-file", func(home string) []string { return []string{"--receipt-file", filepath.Join(home, "receipt.json")} }},
			{"audit-dir", func(home string) []string { return []string{"--audit-dir", filepath.Join(home, "audit")} }},
		} {
			t.Run(leaf.name+"/"+option.name, func(t *testing.T) {
				testenv.Isolate(t)
				home := t.TempDir()
				original := publicTravelClientFactory
				clientCalls := 0
				publicTravelClientFactory = func(travel.Config) (travel.API, error) {
					clientCalls++
					return nil, fmt.Errorf("unexpected source client construction")
				}
				t.Cleanup(func() { publicTravelClientFactory = original })
				args := append(append(append([]string{}, leaf.path...), leaf.args...), "--home", home)
				args = append(args, option.value(home)...)
				out, diagnostics, err := runReceiptRoot(RootCmd(), args)
				if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--"+option.name+" is unsupported") {
					t.Fatalf("expected named usage error: err=%v stderr=%s", err, diagnostics)
				}
				if out != "" {
					t.Fatalf("unsupported receipt option produced stdout: %s", out)
				}
				if clientCalls != 0 {
					t.Fatalf("source client constructed %d times before receipt rejection", clientCalls)
				}
				if !strings.Contains(diagnostics, "--"+option.name) {
					t.Fatalf("named diagnostic absent from stderr: %s", diagnostics)
				}
				assertNoReceiptArtifacts(t, home)
			})
		}
	}
}

func TestFocusedTravelRejectsExplicitInactiveAndDryRunReceiptOptions(t *testing.T) {
	leaf := publicReceiptCases()[3]
	for _, option := range []struct {
		name string
		args []string
	}{
		{"receipt=false", []string{"--receipt=false"}},
		{"receipt-file=empty", []string{"--receipt-file="}},
		{"audit-dir=empty", []string{"--audit-dir="}},
		{"dry-run receipt", []string{"--dry-run", "--receipt"}},
		{"dry-run receipt-file", []string{"--dry-run", "--receipt-file="}},
		{"dry-run audit-dir", []string{"--dry-run", "--audit-dir="}},
	} {
		t.Run(option.name, func(t *testing.T) {
			testenv.Isolate(t)
			home := t.TempDir()
			original := publicTravelClientFactory
			calls := 0
			publicTravelClientFactory = func(travel.Config) (travel.API, error) {
				calls++
				return nil, fmt.Errorf("unexpected source client construction")
			}
			t.Cleanup(func() { publicTravelClientFactory = original })
			args := append(append(append([]string{}, leaf.path...), leaf.args...), "--home", home)
			args = append(args, option.args...)
			out, diagnostics, err := runReceiptRoot(RootCmd(), args)
			if err == nil || ExitCode(err) != 2 || out != "" || calls != 0 {
				t.Fatalf("receipt flag was silently accepted: err=%v stdout=%s stderr=%s client_calls=%d", err, out, diagnostics, calls)
			}
			assertNoReceiptArtifacts(t, home)
		})
	}
}

func TestFocusedTravelRejectsActiveReceiptProfileValues(t *testing.T) {
	leaf := publicReceiptCases()[2]
	for _, option := range []struct{ name, value string }{
		{"receipt", "true"}, {"receipt-file", "receipt.json"}, {"audit-dir", "audit"},
	} {
		t.Run(option.name, func(t *testing.T) {
			testenv.Isolate(t)
			home := t.TempDir()
			root := RootCmd()
			cmd, remaining, err := root.Find(leaf.path)
			if err != nil || len(remaining) != 0 {
				t.Fatalf("cannot find %s: %v %v", leaf.name, remaining, err)
			}
			// A run profile sets flag.Value but leaves Flag.Changed=false.
			value := option.value
			if option.name == "receipt-file" || option.name == "audit-dir" {
				value = filepath.Join(home, value)
			}
			if err := ApplyProfileToFlags(cmd, &Profile{Values: map[string]string{option.name: value}}); err != nil {
				t.Fatal(err)
			}
			if cmd.Flags().Changed(option.name) || cmd.InheritedFlags().Changed(option.name) {
				t.Fatalf("profile value marked --%s as explicitly changed", option.name)
			}
			original := publicTravelClientFactory
			calls := 0
			publicTravelClientFactory = func(travel.Config) (travel.API, error) {
				calls++
				return nil, fmt.Errorf("unexpected source client construction")
			}
			t.Cleanup(func() { publicTravelClientFactory = original })
			args := append(append(append([]string{}, leaf.path...), leaf.args...), "--home", home)
			out, diagnostics, err := runReceiptRoot(root, args)
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--"+option.name+" is unsupported") || out != "" || calls != 0 {
				t.Fatalf("active profile receipt option was accepted: err=%v stdout=%s stderr=%s client_calls=%d", err, out, diagnostics, calls)
			}
			assertNoReceiptArtifacts(t, home)
		})
	}
}

func TestReceiptHelpExplainsFocusedTravelLimitWithoutChangingRawCommands(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	for _, name := range []string{"receipt", "receipt-file", "audit-dir"} {
		flag := root.PersistentFlags().Lookup(name)
		if flag == nil || !strings.Contains(flag.Usage, "unsupported on focused Travel commands") {
			t.Fatalf("--%s help hides focused limit: %v", name, flag)
		}
	}
	for _, leaf := range publicReceiptCases() {
		cmd, remaining, err := root.Find(leaf.path)
		if err != nil || len(remaining) != 0 || cmd.PreRunE == nil {
			t.Fatalf("%s lacks pre-run guard: cmd=%v remaining=%v err=%v", leaf.name, cmd, remaining, err)
		}
	}
	raw, remaining, err := root.Find([]string{"hotel", "get-51870.html"})
	if err != nil || len(remaining) != 0 || raw.PreRunE != nil {
		t.Fatalf("raw route's pre-run behavior changed: cmd=%v remaining=%v err=%v", raw, remaining, err)
	}
}
