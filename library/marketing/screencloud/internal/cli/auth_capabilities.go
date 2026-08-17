// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelAuthCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source live
	var flagFor string

	cmd := &cobra.Command{
		Use:         "capabilities",
		Short:       "Explain whether the current identity appears able to run a supported mapped command without exposing token material or raw grants",
		Example:     "  screencloud-pp-cli auth capabilities --for 'playgrounds files put' --agent --select summary,capabilities,partial_visibility,authorization_proof,visibility",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "inspect currentToken, currentUser, and permissionsList", "for": flagFor, "sent": false})
			}
			root := map[string]any{}
			visibility := map[string]bool{}
			var queryCost float64
			queries := []struct {
				name  string
				query string
			}{
				{"current_token", `query CLICurrentToken { currentToken { permissions } }`},
				{"current_user", `query CLICurrentUser { currentUser { permissions status } }`},
				{"permission_catalog", `query CLIPermissionsCatalog { permissionsList }`},
			}
			for _, item := range queries {
				data, meta, err := runGraphQL(cmd.Context(), flags, item.query, nil)
				if err != nil {
					if expectedPermissionVisibilityError(err) {
						visibility[item.name] = false
						continue
					}
					return err
				}
				visibility[item.name] = true
				var partial map[string]any
				if err := json.Unmarshal(data, &partial); err != nil {
					return err
				}
				for key, value := range partial {
					root[key] = value
				}
				if cost, ok := meta["graphqlQueryCost"].(float64); ok {
					queryCost += cost
				}
			}
			grants := []string{}
			collectPermissionStrings(root["currentToken"], &grants)
			catalog := []string{}
			collectPermissionCatalog(root["permissionsList"], "", &catalog)
			required := requiredCapabilities(flagFor)
			capabilities := []map[string]any{}
			available, missing, unknown := 0, 0, 0
			grantVisibilityComplete := visibility["current_token"]
			for _, need := range required {
				state := "unknown"
				if grantVisibilityComplete {
					if capabilityMatches(grants, need) {
						state = "available"
						available++
					} else {
						state = "missing"
						missing++
					}
				} else {
					unknown++
				}
				capabilities = append(capabilities, map[string]any{"capability": need, "state": state, "cataloged": capabilityMatches(catalog, need)})
			}
			decision := "unknown"
			if len(required) == 0 {
				decision = "unknown_command"
			} else if missing > 0 {
				decision = "missing"
			} else if unknown > 0 {
				decision = "unknown"
			} else {
				decision = "appears_available"
			}
			out := map[string]any{"for": normalizedCommandPath(flagFor), "summary": map[string]any{"decision": decision, "available": available, "missing": missing, "unknown": unknown}, "capabilities": capabilities, "catalog_domain_count": countPermissionDomains(root["permissionsList"]), "visibility": visibility, "partial_visibility": !grantVisibilityComplete || !visibility["permission_catalog"], "raw_grants_included": false, "authorization_proof": false}
			if queryCost > 0 {
				out["graphql_query_cost"] = queryCost
			}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagFor, "for", "", "CLI command path to evaluate, for example 'playgrounds files put'")
	return cmd
}

var commandCapabilityMap = map[string][]string{
	"playgrounds files get":      {"app_instance:read", "token:create"},
	"playgrounds files put":      {"app_instance:update", "token:create"},
	"playgrounds data get":       {"app_instance:read", "token:create"},
	"playgrounds data put":       {"app_instance:update", "token:create"},
	"playgrounds contract-check": {"app_instance:read", "token:create"},
	"app-instances create":       {"app_instance:create"},
	"sync":                       {"app:read", "app_instance:read", "app_install:read", "app_version:read", "space:read", "channel:read", "playlist:read", "screen:read", "association:read", "share_association:read"},
}

func requiredCapabilities(command string) []string {
	normalized := normalizedCommandPath(strings.ToLower(command))
	if exact, ok := commandCapabilityMap[normalized]; ok {
		return exact
	}
	return nil
}

func collectPermissionStrings(value any, destination *[]string) {
	switch typed := value.(type) {
	case string:
		*destination = append(*destination, strings.ToLower(typed))
	case []any:
		for _, item := range typed {
			collectPermissionStrings(item, destination)
		}
	case map[string]any:
		for _, item := range typed {
			collectPermissionStrings(item, destination)
		}
	}
}

func collectPermissionCatalog(value any, domain string, destination *[]string) {
	switch typed := value.(type) {
	case string:
		if domain == "" {
			*destination = append(*destination, typed)
		} else {
			*destination = append(*destination, domain+":"+typed)
		}
	case []any:
		for _, item := range typed {
			collectPermissionCatalog(item, domain, destination)
		}
	case map[string]any:
		for key, item := range typed {
			collectPermissionCatalog(item, key, destination)
		}
	}
}

func capabilityMatches(values []string, capability string) bool {
	wantDomain, wantAction, ok := normalizeCapability(capability)
	if !ok {
		return false
	}
	for _, value := range values {
		domain, action, valid := normalizeCapability(value)
		if valid && (domain == wantDomain || domain == "*") && (action == wantAction || action == "*") {
			return true
		}
	}
	return false
}

func normalizeCapability(value string) (string, string, bool) {
	normalized := strings.NewReplacer(".", ":", "/", ":").Replace(strings.ToLower(strings.TrimSpace(value)))
	parts := strings.Split(normalized, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	domain := strings.ReplaceAll(parts[0], "-", "_")
	action := strings.ReplaceAll(parts[1], "-", "_")
	return domain, action, true
}

func expectedPermissionVisibilityError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "forbidden") ||
		strings.Contains(message, "permission denied") ||
		strings.Contains(message, "not authorized") ||
		strings.Contains(message, "access denied")
}

func countPermissionDomains(catalog any) int {
	switch typed := catalog.(type) {
	case map[string]any:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return 0
	}
}
