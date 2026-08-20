// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type RiskLevel string

const (
	RiskRead             RiskLevel = "read"
	RiskWriteSafe        RiskLevel = "write-safe"
	RiskWriteMutate      RiskLevel = "write-mutate"
	RiskWriteDestructive RiskLevel = "write-destructive"
)

const robinhoodWriteBarrier = "requires_ROBINHOOD_PP_ALLOW_WRITES"

func annotateRobinhoodRisk(root *cobra.Command) {
	for _, cmd := range allCommands(root) {
		risk, ok := classifyRobinhoodRisk(cmd)
		if !ok {
			continue
		}
		if cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		cmd.Annotations["pp:risk"] = string(risk)
		cmd.Annotations["mcp:risk"] = string(risk)
		if risk == RiskRead {
			cmd.Annotations["mcp:read-only"] = "true"
			cmd.Annotations["pp:barrier"] = "none"
			continue
		}
		cmd.Annotations["mcp:read-only"] = "false"
		cmd.Annotations["pp:barrier"] = robinhoodWriteBarrier
		appendRiskWarning(cmd, "[WRITES TO LIVE ROBINHOOD] Write route: may modify your Robinhood account. Default is --dry-run; live execution also requires ROBINHOOD_PP_ALLOW_WRITES=1.")
	}
}

func allCommands(root *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		out = append(out, cmd)
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
	return out
}

func classifyRobinhoodRisk(cmd *cobra.Command) (RiskLevel, bool) {
	method, path := endpointMetadata(cmd)
	if method == "" || path == "" {
		return RiskRead, false
	}
	if method == "GET" || cmd.Annotations["mcp:read-only"] == "true" {
		return RiskRead, true
	}
	lower := strings.ToLower(strings.Join([]string{method, path, cmd.CommandPath(), cmd.Short}, " "))
	if method == "DELETE" || containsAny(lower, "cancel", "delete", "remove", "unlink", "disable", "revoke") {
		return RiskWriteDestructive, true
	}
	if containsAny(lower, "estimate", "validate", "preview") {
		return RiskWriteSafe, true
	}
	return RiskWriteMutate, true
}

func commandRiskLevel(cmd *cobra.Command) RiskLevel {
	if cmd == nil {
		return RiskRead
	}
	if cmd.Annotations != nil {
		if cmd.Annotations["pp:dynamic-risk"] == "true" {
			return RiskRead
		}
		if raw := cmd.Annotations["pp:risk"]; raw != "" {
			return RiskLevel(raw)
		}
	}
	risk, ok := classifyRobinhoodRisk(cmd)
	if !ok {
		return RiskRead
	}
	return risk
}

func endpointMetadata(cmd *cobra.Command) (method, path string) {
	if cmd == nil || cmd.Annotations == nil {
		return "", ""
	}
	return strings.ToUpper(cmd.Annotations["pp:method"]), cmd.Annotations["pp:path"]
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func appendRiskWarning(cmd *cobra.Command, warning string) {
	if warning == "" {
		return
	}
	block := "\n\nRisk warning: " + warning
	if strings.Contains(cmd.Long, warning) {
		return
	}
	if cmd.Long == "" {
		cmd.Long = cmd.Short + block
		return
	}
	cmd.Long += block
}
