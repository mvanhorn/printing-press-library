// Copyright 2026 adrian-horning. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// runWizardIfEligible fires when the user ran the bare binary in a real TTY
// with no agent-shaped flags. It walks platform -> action -> required params
// and then invokes the resolved cobra command in-process. Returns true when
// the wizard ran (success or user abort); false when help should print.
func runWizardIfEligible(rootCmd *cobra.Command, flags *rootFlags, args []string) (bool, error) {
	if len(args) > 0 {
		return false, nil
	}
	if flags.noInput || flags.agent || flags.yes {
		return false, nil
	}
	if !wizardTTY() {
		return false, nil
	}
	return true, runWizard(rootCmd)
}

func wizardTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func runWizard(rootCmd *cobra.Command) error {
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout

	platforms := collectPlatforms(rootCmd)
	if len(platforms) == 0 {
		return fmt.Errorf("wizard: no platforms registered")
	}

	fmt.Fprintln(out, "scrape-creators interactive mode")
	fmt.Fprintln(out, "Press Ctrl+C to exit. Select by number.")
	fmt.Fprintln(out)

	platform, err := pickOne(in, out, "platform", platforms)
	if err != nil {
		return err
	}

	platformCmd, _, err := rootCmd.Find([]string{platform})
	if err != nil || platformCmd == rootCmd {
		return fmt.Errorf("wizard: platform %q not found", platform)
	}

	actions := collectActions(platformCmd)
	if len(actions) == 0 {
		return fmt.Errorf("wizard: platform %q has no actions", platform)
	}

	action, err := pickOne(in, out, "action", actions)
	if err != nil {
		return err
	}

	actionCmd, _, err := platformCmd.Find([]string{action})
	if err != nil || actionCmd == platformCmd {
		return fmt.Errorf("wizard: action %q not found", action)
	}

	assembled := []string{platform, action}
	flagNames := stringFlags(actionCmd)
	for _, name := range flagNames {
		fmt.Fprintf(out, "%s (leave blank to skip): ", name)
		line, err := in.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		assembled = append(assembled, "--"+name, value)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "running: scrape-creators-pp-cli %s\n\n", strings.Join(assembled, " "))
	rootCmd.SetArgs(assembled)
	return rootCmd.Execute()
}

func pickOne(in *bufio.Reader, out io.Writer, label string, options []string) (string, error) {
	fmt.Fprintf(out, "%s:\n", label)
	for i, opt := range options {
		fmt.Fprintf(out, "  [%d] %s\n", i+1, opt)
	}
	for {
		fmt.Fprintf(out, "choose 1-%d: ", len(options))
		line, err := in.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Allow name lookup as a power-user shortcut.
		for _, opt := range options {
			if opt == trimmed {
				return opt, nil
			}
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil || n < 1 || n > len(options) {
			fmt.Fprintf(out, "invalid selection %q\n", trimmed)
			continue
		}
		return options[n-1], nil
	}
}

// collectPlatforms returns sorted names of platform-level commands, skipping
// top-level utilities that are not "scrape an API" surfaces.
func collectPlatforms(rootCmd *cobra.Command) []string {
	skip := map[string]bool{
		"doctor": true, "auth": true, "export": true, "import": true,
		"search": true, "sync": true, "tail": true, "analytics": true,
		"workflow": true, "api": true, "version": true, "help": true,
		"completion": true, "agent": true,
	}
	var names []string
	for _, c := range rootCmd.Commands() {
		if c.Hidden || skip[c.Name()] {
			continue
		}
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}

func collectActions(platformCmd *cobra.Command) []string {
	var names []string
	for _, c := range platformCmd.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}

// stringFlags returns the flag names on the action command, excluding
// persistent flags inherited from root and the help flag.
func stringFlags(actionCmd *cobra.Command) []string {
	var names []string
	actionCmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		names = append(names, f.Name)
	})
	sort.Strings(names)
	return names
}
