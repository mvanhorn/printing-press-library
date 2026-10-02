// Copyright 2026 klubieniecki and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/learn"
)

var ashbyPlaybookPlaceholderRE = regexp.MustCompile(`^\{[A-Za-z][A-Za-z0-9_.-]*\}$`)

// validateAshbyPlaybook keeps persisted choreography declarative and
// read-only. Stored playbooks are still untrusted historical data at recall;
// this allowlist prevents them from carrying an arbitrary executable or shell
// program across sessions in the first place.
func validateAshbyPlaybook(playbook learn.Playbook) error {
	if len(playbook.Steps) == 0 {
		return fmt.Errorf("playbook must contain at least one step")
	}
	for i, step := range playbook.Steps {
		hasCommand := strings.TrimSpace(step.Cmd) != ""
		hasClientStep := strings.TrimSpace(step.ClientSide) != ""
		if hasCommand == hasClientStep {
			return fmt.Errorf("playbook step %d must set exactly one of cmd or client_side", i+1)
		}
		if hasCommand {
			if err := validateAshbyPlaybookCommand(step.Cmd); err != nil {
				return fmt.Errorf("playbook step %d: %w", i+1, err)
			}
			continue
		}
		switch step.ClientSide {
		case "dedupe", "filter", "rank_by", "select", "sort_by":
		default:
			return fmt.Errorf("playbook step %d: unsupported client_side operation %q", i+1, step.ClientSide)
		}
	}
	return nil
}

func validateAshbyPlaybookCommand(command string) error {
	if strings.ContainsAny(command, "\r\n\t;&|`$\\\"'") {
		return fmt.Errorf("cmd contains shell syntax or quoting")
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("cmd is empty")
	}
	if fields[0] == "ashby-pp-cli" {
		fields = fields[1:]
	}
	if len(fields) == 1 && (fields[0] == "--help" || fields[0] == "--version") {
		return nil
	}

	var path string
	var positionalCount int
	switch {
	case len(fields) >= 2 && fields[0] == "postings" && fields[1] == "list":
		path, positionalCount, fields = "postings list", 1, fields[2:]
	case len(fields) >= 2 && fields[0] == "postings" && fields[1] == "get":
		path, positionalCount, fields = "postings get", 2, fields[2:]
	case len(fields) >= 1 && fields[0] == "search":
		path, positionalCount, fields = "search", 1, fields[1:]
	default:
		return fmt.Errorf("cmd must use an Ashby read-only command (postings list, postings get, or search)")
	}
	return validateAshbyPlaybookArguments(path, positionalCount, fields)
}

func validateAshbyPlaybookArguments(path string, positionalCount int, fields []string) error {
	booleanFlags := map[string]bool{
		"--agent": true, "--compact": true, "--csv": true, "--dry-run": true,
		"--human-friendly": true, "--json": true, "--no-cache": true,
		"--no-color": true, "--no-input": true, "--no-learn": true,
		"--plain": true, "--quiet": true,
	}
	valueFlags := map[string]bool{"--data-source": true, "--select": true}
	switch path {
	case "postings list":
		for _, flag := range []string{"--has-compensation", "--include-compensation", "--remote"} {
			booleanFlags[flag] = true
		}
		for _, flag := range []string{"--currency", "--department", "--employment-type", "--limit", "--location", "--published-since", "--query", "-q", "--salary-max", "--salary-min", "--team", "--workplace"} {
			valueFlags[flag] = true
		}
	case "postings get":
		booleanFlags["--include-compensation"] = true
	case "search":
		valueFlags["--board"] = true
		valueFlags["--limit"] = true
	}

	positionals := 0
	for len(fields) > 0 {
		field := fields[0]
		fields = fields[1:]
		if !strings.HasPrefix(field, "-") {
			if err := validateAshbyPlaybookValue(field, false); err != nil {
				return fmt.Errorf("invalid %s argument: %w", path, err)
			}
			positionals++
			continue
		}
		name, inlineValue, hasInlineValue := strings.Cut(field, "=")
		if booleanFlags[name] {
			if hasInlineValue && inlineValue != "true" && inlineValue != "false" {
				return fmt.Errorf("flag %s requires a boolean value", name)
			}
			continue
		}
		if !valueFlags[name] {
			return fmt.Errorf("flag %q is not allowed for %s", name, path)
		}
		value := inlineValue
		if !hasInlineValue {
			if len(fields) == 0 {
				return fmt.Errorf("flag %s requires a value", name)
			}
			value, fields = fields[0], fields[1:]
		}
		if err := validateAshbyPlaybookValue(value, true); err != nil {
			return fmt.Errorf("invalid value for %s: %w", name, err)
		}
		if name == "--data-source" && value != "auto" && value != "live" && value != "local" && value != "<str>" {
			return fmt.Errorf("invalid value for --data-source")
		}
	}
	if positionals != positionalCount {
		return fmt.Errorf("%s requires %d positional argument(s)", path, positionalCount)
	}
	return nil
}

func validateAshbyPlaybookValue(value string, allowSynthSlot bool) error {
	if value == "" {
		return fmt.Errorf("value is empty")
	}
	if allowSynthSlot && (value == "<str>" || value == "<int>") {
		return nil
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("value must not be parsed as a flag")
	}
	if strings.ContainsAny(value, "\r\n\t;&|><`$\\\"'") {
		return fmt.Errorf("value contains shell syntax or quoting")
	}
	if strings.ContainsAny(value, "{}") && !ashbyPlaybookPlaceholderRE.MatchString(value) {
		return fmt.Errorf("placeholder must have the form {slot.field}")
	}
	return nil
}
