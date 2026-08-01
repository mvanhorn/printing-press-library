// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/store"
	"github.com/spf13/cobra"
)

type syncResource struct {
	Type   string
	Field  string
	Fields string
}

var coreSyncResources = []syncResource{
	{Type: "apps", Field: "allApps", Fields: "id name slug isInstalled"},
	{Type: "spaces", Field: "allSpaces", Fields: "id name"},
	{Type: "app_instances", Field: "allAppInstances", Fields: "id appInstallId spaceId name version config status appId castedScreenByAppInstanceId(first: 500) { totalCount nodes { id name spaceId } } sharedSpacesByAppInstanceId(first: 500) { totalCount nodes { id name } }"},
	{Type: "app_installs", Field: "allAppInstalls", Fields: "id appId spaceId"},
	{Type: "app_versions", Field: "allAppVersions", Fields: "id appId version isLatest"},
}

var topologySyncResources = []syncResource{
	{Type: "channels", Field: "allChannels", Fields: "id name spaceId content"},
	{Type: "playlists", Field: "allPlaylists", Fields: "id name spaceId content"},
	{Type: "screens", Field: "allScreens", Fields: "id name spaceId castId status"},
	{Type: "associations", Field: "allAssociations", Fields: "fromCast fromScreen fromChannel fromPlaylist fromAppInstance toCast toFile toAppInstance toPlaylist toChannel toLink toCredential toSite"},
	{Type: "share_associations", Field: "allShareAssociations", Fields: "fromSpace shareChannel sharePlaylist shareFolder shareFile shareLink shareAppInstance shareCredential toSpace shareSite"},
}

func newScreenCloudSyncCmd(flags *rootFlags) *cobra.Command {
	var first int
	var includeTopology bool
	var resourcesCSV string
	var maxPages int
	cmd := &cobra.Command{
		Use: "sync", Short: "Synchronize bounded, sanitized Studio metadata into the local SQLite mirror",
		Example:     "  screencloud-pp-cli sync --first 100 --include-topology --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if first < 1 || first > 500 {
				return usageErr(fmt.Errorf("--first must be between 1 and 500"))
			}
			if maxPages < 1 || maxPages > 20 {
				return usageErr(fmt.Errorf("--max-pages must be between 1 and 20"))
			}
			resources := append([]syncResource{}, coreSyncResources...)
			if includeTopology {
				resources = append(resources, topologySyncResources...)
			}
			if strings.TrimSpace(resourcesCSV) != "" {
				wanted := map[string]struct{}{}
				for _, name := range strings.Split(resourcesCSV, ",") {
					normalized := strings.ReplaceAll(strings.TrimSpace(name), "-", "_")
					if normalized != "" {
						wanted[normalized] = struct{}{}
					}
				}
				filtered := make([]syncResource, 0, len(wanted))
				seen := map[string]struct{}{}
				for _, resource := range append(append([]syncResource{}, coreSyncResources...), topologySyncResources...) {
					if _, ok := wanted[resource.Type]; ok {
						filtered = append(filtered, resource)
						seen[resource.Type] = struct{}{}
					}
				}
				if len(seen) != len(wanted) {
					unknown := []string{}
					for name := range wanted {
						if _, ok := seen[name]; !ok {
							unknown = append(unknown, strings.ReplaceAll(name, "_", "-"))
						}
					}
					return usageErr(fmt.Errorf("unknown --resources value(s): %s", strings.Join(unknown, ", ")))
				}
				resources = filtered
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}
			if flags.dryRun {
				names := make([]string, 0, len(resources))
				for _, resource := range resources {
					names = append(names, resource.Type)
				}
				return printValue(cmd, flags, map[string]any{"operation": "read-only metadata sync", "resources": names, "per_page_limit": first, "max_pages": maxPages, "private_playgrounds_content": "excluded", "sent": false})
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("screencloud-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			counts := map[string]int{}
			failures := map[string]string{}
			states := map[string]string{}
			queryCosts := map[string]float64{}
			pruned := map[string]int{}
			for _, resource := range resources {
				count := 0
				failed := false
				exhausted := false
				seenIDs := []string{}
				seenIDSet := map[string]struct{}{}
				// Invalidate any prior complete marker before the first page is
				// written so concurrent analyses and crash recovery fail closed.
				if err := s.SaveSyncState(resource.Type, "in_progress", 0); err != nil {
					failures[resource.Type] = fmt.Sprintf("persisting in-progress sync state: %v", err)
					states[resource.Type] = "failed"
					continue
				}
				for page := 0; page < maxPages; page++ {
					query := fmt.Sprintf("query CLISync($first: Int!, $offset: Int!) { %s(first: $first, offset: $offset) { totalCount nodes { %s } } }", resource.Field, resource.Fields)
					data, meta, queryErr := runGraphQL(cmd.Context(), flags, query, map[string]any{"first": first, "offset": page * first})
					if queryErr != nil {
						failures[resource.Type] = queryErr.Error()
						failed = true
						break
					}
					if cost, ok := meta["graphqlQueryCost"].(float64); ok {
						queryCosts[resource.Type] += cost
					}
					var root map[string]json.RawMessage
					if err := json.Unmarshal(data, &root); err != nil {
						failures[resource.Type] = err.Error()
						failed = true
						break
					}
					rawConnection, ok := root[resource.Field]
					if !ok || len(rawConnection) == 0 || string(rawConnection) == "null" {
						failures[resource.Type] = fmt.Sprintf("GraphQL response omitted %s connection", resource.Field)
						failed = true
						break
					}
					var connection struct {
						TotalCount *int               `json:"totalCount"`
						Nodes      *[]json.RawMessage `json:"nodes"`
					}
					if err := json.Unmarshal(rawConnection, &connection); err != nil {
						failures[resource.Type] = fmt.Sprintf("decoding %s connection: %v", resource.Field, err)
						failed = true
						break
					}
					if connection.TotalCount == nil || connection.Nodes == nil || *connection.TotalCount < 0 {
						failures[resource.Type] = fmt.Sprintf("GraphQL %s connection omitted a usable totalCount or nodes list", resource.Field)
						failed = true
						break
					}
					rawNodes := *connection.Nodes
					if page*first+len(rawNodes) > *connection.TotalCount {
						failures[resource.Type] = fmt.Sprintf("GraphQL %s connection returned more rows than totalCount", resource.Field)
						failed = true
						break
					}
					nodes, sanitizeErr := sanitizeSyncNodes(resource.Type, rawNodes)
					if sanitizeErr != nil {
						failures[resource.Type] = sanitizeErr.Error()
						failed = true
						break
					}
					for _, raw := range nodes {
						var object map[string]any
						if err := json.Unmarshal(raw, &object); err != nil {
							failures[resource.Type] = fmt.Sprintf("decoding sanitized row: %v", err)
							failed = true
							break
						}
						id := firstString(object, "id", "uuid")
						if id == "" {
							failures[resource.Type] = "sanitized row omitted an id"
							failed = true
							break
						}
						if _, duplicate := seenIDSet[id]; duplicate {
							failures[resource.Type] = fmt.Sprintf("GraphQL %s connection repeated id %q across the traversal", resource.Field, id)
							failed = true
							break
						}
						seenIDSet[id] = struct{}{}
						seenIDs = append(seenIDs, id)
					}
					if failed {
						break
					}
					stored, extractFailures, err := s.UpsertBatch(resource.Type, nodes)
					if err != nil {
						failures[resource.Type] = err.Error()
						failed = true
						break
					}
					if extractFailures > 0 || stored != len(nodes) {
						failures[resource.Type] = fmt.Sprintf("%d of %d rows could not be stored", len(nodes)-stored, len(nodes))
						failed = true
						break
					}
					count += stored
					observedEnd := page*first + len(rawNodes)
					if observedEnd >= *connection.TotalCount {
						exhausted = true
						break
					}
					if len(rawNodes) < first {
						failures[resource.Type] = fmt.Sprintf("GraphQL %s connection ended early at %d of %d rows", resource.Field, observedEnd, *connection.TotalCount)
						failed = true
						break
					}
				}
				counts[resource.Type] = count
				state := "truncated"
				if failed {
					state = "failed"
				} else if exhausted {
					state = "complete"
				}
				if state == "complete" {
					deleted, reconcileErr := s.ReconcileAll(resource.Type, seenIDs)
					if reconcileErr != nil {
						failures[resource.Type] = reconcileErr.Error()
						state = "failed"
					} else {
						pruned[resource.Type] = deleted
					}
				}
				if stateErr := s.SaveSyncState(resource.Type, state, count); stateErr != nil {
					failures[resource.Type] = fmt.Sprintf("persisting sync state: %v", stateErr)
					state = "failed"
				}
				states[resource.Type] = state
			}
			complete := len(failures) == 0
			for _, state := range states {
				if state != "complete" {
					complete = false
				}
			}
			out := map[string]any{"database": s.Path(), "counts": counts, "pruned": pruned, "failures": failures, "resource_states": states, "graphql_query_cost_by_resource": queryCosts, "private_playgrounds_content": "excluded", "complete": complete}
			if err := printValue(cmd, flags, out); err != nil {
				return err
			}
			if len(failures) > 0 {
				return apiErr(fmt.Errorf("metadata sync completed with %d failed resource(s)", len(failures)))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&first, "first", 100, "Maximum records per resource (1-500)")
	cmd.Flags().BoolVar(&includeTopology, "include-topology", true, "Include channels, playlists, screens, and placement edges")
	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to sync, such as apps,spaces,app-instances")
	cmd.Flags().IntVar(&maxPages, "max-pages", 10, "Maximum pages per selected resource (1-20)")
	return cmd
}

func sanitizeSyncNodes(resourceType string, nodes []json.RawMessage) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(nodes))
	for _, raw := range nodes {
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("decoding %s row: %w", resourceType, err)
		}
		switch resourceType {
		case "app_instances":
			if config, ok := object["config"]; ok {
				if appUUID := findIdentifier(config, "appuuid", "app_uuid"); appUUID != "" {
					object["playgroundsAppUuid"] = appUUID
				}
				object["config"] = sanitizeStructure(config)
			}
		case "channels", "playlists":
			if content, ok := object["content"]; ok {
				ids := []string{}
				collectIdentifierValues("content", content, &ids)
				object["reference_ids"] = uniqueStrings(ids)
				delete(object, "content")
			}
		case "associations", "share_associations":
			// These edge tables have composite keys and expose no scalar id.
			// Derive a stable local identifier from the canonical JSON fields.
			canonical, err := json.Marshal(object)
			if err != nil {
				return nil, fmt.Errorf("encoding %s composite key: %w", resourceType, err)
			}
			sum := sha256.Sum256(canonical)
			object["id"] = "edge:" + hex.EncodeToString(sum[:16])
		}
		encoded, err := json.Marshal(object)
		if err != nil {
			return nil, fmt.Errorf("encoding sanitized %s row: %w", resourceType, err)
		}
		result = append(result, encoded)
	}
	return result, nil
}

func sanitizeStructure(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = sanitizeStructure(item)
		}
		return out
	case []any:
		if len(typed) == 0 {
			return []any{}
		}
		// Preserve the union of array element shapes while discarding values.
		// The stable JSON encoding makes the result deterministic and avoids
		// leaking array length through repeated identical shapes.
		unique := map[string]any{}
		for _, item := range typed {
			sanitized := sanitizeStructure(item)
			encoded, err := json.Marshal(sanitized)
			if err == nil {
				unique[string(encoded)] = sanitized
			}
		}
		keys := make([]string, 0, len(unique))
		for key := range unique {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]any, 0, len(keys))
		for _, key := range keys {
			out = append(out, unique[key])
		}
		return out
	case nil:
		return nil
	case bool:
		return false
	case float64, json.Number:
		return float64(0)
	default:
		return ""
	}
}

func findIdentifier(value any, keys ...string) string {
	wanted := map[string]struct{}{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = struct{}{}
	}
	var found string
	var walk func(any)
	walk = func(current any) {
		if found != "" {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			for key, item := range typed {
				if _, ok := wanted[strings.ToLower(key)]; ok {
					if text, ok := item.(string); ok {
						found = text
						return
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func collectIdentifierValues(parentKey string, value any, destination *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			collectIdentifierValues(key, item, destination)
		}
	case []any:
		for _, item := range typed {
			collectIdentifierValues(parentKey, item, destination)
		}
	case string:
		key := strings.ToLower(parentKey)
		if key == "id" || strings.HasSuffix(key, "id") || strings.HasSuffix(key, "uuid") || strings.Contains(key, "reference") {
			*destination = append(*destination, typed)
		}
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func newScreenCloudSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var typesCSV string
	cmd := &cobra.Command{
		Use: "search <query>", Short: "Search the sanitized local Studio mirror",
		Example:     "  screencloud-pp-cli search Playgrounds --types app_instances,channels --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<query>=Playgrounds"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "search local mirror", "limit": limit, "sent": false})
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("exactly one <query> is required"))
			}
			if limit < 1 || limit > 500 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 500"))
			}
			s, err := loadStoreReadOnly()
			if err != nil {
				return printValue(cmd, flags, map[string]any{"items": []any{}, "hint": "Run screencloud-pp-cli sync first.", "source": "local"})
			}
			defer s.Close()
			var types []string
			for _, item := range strings.Split(typesCSV, ",") {
				if strings.TrimSpace(item) != "" {
					types = append(types, strings.TrimSpace(item))
				}
			}
			rows, err := s.Search(args[0], limit, types...)
			if err != nil {
				return err
			}
			items := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				var item map[string]any
				if json.Unmarshal(row, &item) == nil {
					items = append(items, item)
				}
			}
			return printValue(cmd, flags, items)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum local matches")
	cmd.Flags().StringVar(&typesCSV, "types", "", "Comma-separated resource types to search")
	return cmd
}
