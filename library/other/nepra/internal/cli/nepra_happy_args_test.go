// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. One invariant over the WHOLE command tree, rather than a
// per-command literal.
//
// WHY THIS FILE EXISTS. Six commands shipped a pp:happy-args value in the bare
// `label=value` form — capacity, gen, conflicts, events, fleet and sources —
// while verify and disco used the correct `--flag=value` form. Cobra parses a
// bare label=value as a POSITIONAL argument, so on a flag-driven command the
// probe's argument is swallowed and the command runs as if it had been given
// nothing. Only `capacity` failed visibly, because it is the one that refuses
// to run without its flag; the other five fell into a help or catalogue branch
// and exited 0, so the Sample Output Probe reported them as PASSING while
// never exercising them at all.
//
// Four separate per-command literal assertions existed and three of them
// PINNED THE BUG. A single tree-wide invariant is the right shape: it cannot
// be satisfied one command at a time, and a new command inherits it for free.

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHappyArgsUseTheFlagFormOnFlagDrivenCommands walks every command that
// declares pp:happy-args and requires the --flag=value form unless the
// command's Use line actually declares a positional.
func TestHappyArgsUseTheFlagFormOnFlagDrivenCommands(t *testing.T) {
	checked := 0
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			walk(child)
		}
		happy := cmd.Annotations["pp:happy-args"]
		if happy == "" {
			return
		}
		checked++
		// A command whose Use line declares a positional (e.g.
		// "reliability <path>") legitimately takes a bare value; the harness
		// maps label=value onto that positional. Everything else is
		// flag-driven.
		if strings.Contains(cmd.Use, "<") {
			return
		}
		for _, arg := range strings.Split(happy, ";") {
			arg = strings.TrimSpace(arg)
			if arg == "" {
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				t.Errorf("%s: pp:happy-args %q uses the bare label=value form. %s takes no positional "+
					"(Use: %q), so cobra parses that token as a positional argument and the flag is never "+
					"set — the command runs as if it had been given nothing, and a command with a "+
					"catalogue or help-only branch then exits 0 and reports as passing without ever "+
					"being exercised. Use the --flag=value form.",
					cmd.CommandPath(), happy, cmd.Name(), cmd.Use)
				continue
			}
			// The flag it names must actually exist on the command, or the
			// probe drives an unknown flag and exits 2.
			name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
			if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
				t.Errorf("%s: pp:happy-args names --%s, which is not declared on the command",
					cmd.CommandPath(), name)
			}
		}
	}
	walk(RootCmd())
	if checked == 0 {
		t.Fatal("no command declares pp:happy-args; this test would assert nothing")
	}
	t.Logf("checked pp:happy-args on %d commands", checked)
}

// TestHappyArgsFlagFormIsWhatTheProbeNeeds demonstrates the mechanism the test
// above protects against, so the reason is not merely asserted in a comment.
//
// It builds a flag-driven command in both shapes and shows that the bare form
// leaves the flag unset while the value lands in args.
func TestHappyArgsFlagFormIsWhatTheProbeNeeds(t *testing.T) {
	newCmd := func() (*cobra.Command, *string, *[]string) {
		var got string
		var positional []string
		cmd := &cobra.Command{
			Use: "probe",
			RunE: func(c *cobra.Command, args []string) error {
				positional = args
				return nil
			},
		}
		cmd.Flags().StringVar(&got, "as-of", "", "")
		return cmd, &got, &positional
	}

	// The bare form: swallowed as a positional, flag left empty.
	cmd, got, positional := newCmd()
	cmd.SetArgs([]string{"as-of=2024-06-30"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bare form: %v", err)
	}
	if *got != "" {
		t.Fatalf("bare form set --as-of to %q; it should have been swallowed as a positional", *got)
	}
	if len(*positional) != 1 || (*positional)[0] != "as-of=2024-06-30" {
		t.Fatalf("bare form positional = %v, want the whole token", *positional)
	}

	// The flag form: the flag is actually set and nothing lands in args.
	cmd, got, positional = newCmd()
	cmd.SetArgs([]string{"--as-of=2024-06-30"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("flag form: %v", err)
	}
	if *got != "2024-06-30" {
		t.Fatalf("flag form set --as-of to %q, want 2024-06-30", *got)
	}
	if len(*positional) != 0 {
		t.Fatalf("flag form left positional %v", *positional)
	}
}
