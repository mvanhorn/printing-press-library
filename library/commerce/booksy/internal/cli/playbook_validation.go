// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/learn"
)

// validateBooksyPlaybook converts legacy command strings into argument vectors.
// Only read-only provider commands are approved. No shell or command handler is
// invoked while checking the real Cobra flag types and required positionals.
func validateBooksyPlaybook(pb *learn.Playbook) error {
	// RootCmd binds its --no-color flag to this package variable. Parsing a
	// recalled step must not change formatting for the real invocation.
	previousNoColor := noColor
	defer func() { noColor = previousNoColor }()
	for i := range pb.Steps {
		step := &pb.Steps[i]
		if step.ClientSide != "" || len(step.Args) != 0 {
			return fmt.Errorf("playbook step %d: client_side/args operations are not approved; perform post-processing separately", i+1)
		}
		argv := append([]string(nil), step.Argv...)
		legacy := step.Cmd != ""
		if step.Cmd != "" {
			if len(argv) != 0 || strings.ContainsAny(step.Cmd, "\"'\\") {
				return fmt.Errorf("playbook step %d: use argv instead of ambiguous command text", i+1)
			}
			argv = strings.Fields(step.Cmd)
			if strings.ContainsAny(step.Cmd, "\n\r") {
				return fmt.Errorf("playbook step %d: command must be a single line", i+1)
			}
		}
		if len(argv) > 0 && argv[0] == "booksy-pp-cli" {
			argv = argv[1:]
		}
		if len(argv) == 0 {
			return fmt.Errorf("playbook step %d: command is required", i+1)
		}
		for _, arg := range argv {
			if strings.IndexFunc(arg, unicode.IsControl) >= 0 {
				return fmt.Errorf("playbook step %d: control characters are not allowed", i+1)
			}
			if legacy && (strings.ContainsAny(arg, ";|&`$\\") ||
				(strings.ContainsAny(arg, "<>") && arg != "<str>" && arg != "<int>")) {
				return fmt.Errorf("playbook step %d: shell syntax and control characters are not allowed", i+1)
			}
		}
		root := RootCmd()
		command, remaining, err := root.Find(argv)
		if err != nil {
			return fmt.Errorf("playbook step %d: %w", i+1, err)
		}
		approved := command.Annotations["mcp:read-only"] == "true" && command.Annotations["pp:method"] == "GET"
		approved = approved || command.CommandPath() == "booksy-pp-cli services" || command.CommandPath() == "booksy-pp-cli availability" || command.CommandPath() == "booksy-pp-cli earliest"
		if !approved {
			return fmt.Errorf("playbook step %d: command is not an approved Booksy read operation", i+1)
		}
		// Root flags include credentials, arbitrary destinations and file paths.
		// Permit only these output controls in addition to the endpoint schema.
		outputFlags := map[string]bool{"agent": true, "json": true, "pretty": true, "compact": true, "no-color": true}
		for _, arg := range remaining {
			if !strings.HasPrefix(arg, "-") {
				continue
			}
			if !strings.HasPrefix(arg, "--") {
				return fmt.Errorf("playbook step %d: use long flag names", i+1)
			}
			name, _, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			if command.LocalNonPersistentFlags().Lookup(name) == nil && !outputFlags[name] {
				return fmt.Errorf("playbook step %d: flag --%s is not approved", i+1, name)
			}
		}
		// Synthesis stores only value classes, never the original flag values.
		// Type-check those slots with harmless sample values while retaining
		// the placeholders in the returned argv for explicit substitution.
		parseArgs := make([]string, len(remaining))
		for j, arg := range remaining {
			if name, value, found := strings.Cut(arg, "="); found {
				parseArgs[j] = name + "=" + playbookSampleValue(value)
			} else {
				parseArgs[j] = playbookSampleValue(arg)
			}
			if strings.Contains(parseArgs[j], "<redacted>") {
				return fmt.Errorf("playbook step %d: redacted credential slots are not approved", i+1)
			}
		}
		if err := command.ParseFlags(parseArgs); err != nil {
			return fmt.Errorf("playbook step %d: %w", i+1, err)
		}
		positionals := command.Flags().Args()
		required := strings.Count(command.Use, "<")
		if len(positionals) != required {
			return fmt.Errorf("playbook step %d: expected %d positional arguments", i+1, required)
		}
		for _, arg := range positionals {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("playbook step %d: positional arguments cannot be empty", i+1)
			}
		}
		if err := command.ValidateRequiredFlags(); err != nil {
			return fmt.Errorf("playbook step %d: %w", i+1, err)
		}
		if command.CommandPath() == "booksy-pp-cli availability" || command.CommandPath() == "booksy-pp-cli earliest" {
			variant, err := command.Flags().GetString("service-variant")
			if err != nil {
				return fmt.Errorf("playbook step %d: %w", i+1, err)
			}
			id, err := strconv.ParseInt(variant, 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("playbook step %d: --service-variant requires a positive ID", i+1)
			}
		}
		step.Argv = argv
		step.Cmd = ""
	}
	return nil
}

func playbookSampleValue(value string) string {
	switch value {
	case "<str>":
		return "example"
	case "<int>":
		return "1"
	default:
		return value
	}
}
