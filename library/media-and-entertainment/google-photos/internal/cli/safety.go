// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type commandPolicy struct {
	ProfileName string
	Allow       []string
	Deny        []string
}

func bakedCommandPolicy() commandPolicy {
	return commandPolicy{
		ProfileName: bakedSafetyProfileName,
		Allow:       normalizeCommandRules(bakedAllowCommands),
		Deny:        normalizeCommandRules(bakedDenyCommands),
	}
}

func enforceCommandPolicyForCobra(cmd *cobra.Command, flags *rootFlags) error {
	path := cobraCommandPath(cmd)
	if path == "" {
		return nil
	}
	if err := EnforceCommandPolicy(path, "", ""); err != nil {
		return err
	}
	if flags == nil {
		return nil
	}
	return EnforceCommandPolicy(path, flags.enableCommands, flags.disableCommands)
}

// EnforceCommandPolicy checks a dotted command path against the baked build
// profile plus the optional runtime allow/deny lists. It is exported so MCP
// handlers can enforce the same policy before issuing API calls.
func EnforceCommandPolicy(commandPath, runtimeAllow, runtimeDeny string) error {
	commandPath = normalizeCommandPath(commandPath)
	if commandPath == "" {
		return nil
	}

	if err := enforcePolicy(commandPath, bakedCommandPolicy()); err != nil {
		return err
	}
	return enforcePolicy(commandPath, commandPolicy{
		ProfileName: "runtime",
		Allow:       parseCommandRules(runtimeAllow),
		Deny:        parseCommandRules(runtimeDeny),
	})
}

func enforcePolicy(commandPath string, policy commandPolicy) error {
	for _, deny := range policy.Deny {
		if commandRuleMatches(deny, commandPath) {
			return usageErr(fmt.Errorf("command %q is blocked by %s command policy", commandPath, policyName(policy)))
		}
	}
	if len(policy.Allow) == 0 {
		return nil
	}
	for _, allow := range policy.Allow {
		if commandRuleMatches(allow, commandPath) {
			return nil
		}
	}
	return usageErr(fmt.Errorf("command %q is not allowed by %s command policy", commandPath, policyName(policy)))
}

func policyName(policy commandPolicy) string {
	if policy.ProfileName == "" {
		return "baked"
	}
	return policy.ProfileName
}

func cobraCommandPath(cmd *cobra.Command) string {
	parts := []string{}
	for c := cmd; c != nil; c = c.Parent() {
		if c.Parent() == nil {
			break
		}
		parts = append(parts, c.Name())
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return normalizeCommandPath(strings.Join(parts, "."))
}

func parseCommandRules(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})
	return normalizeCommandRules(parts)
}

func normalizeCommandRules(rules []string) []string {
	out := make([]string, 0, len(rules))
	seen := map[string]bool{}
	for _, rule := range rules {
		normalized := normalizeCommandPath(rule)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func normalizeCommandPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, ".")
	path = strings.ReplaceAll(path, " ", ".")
	for strings.Contains(path, "..") {
		path = strings.ReplaceAll(path, "..", ".")
	}
	return path
}

func commandRuleMatches(rule, commandPath string) bool {
	return commandPath == rule || strings.HasPrefix(commandPath, rule+".")
}
