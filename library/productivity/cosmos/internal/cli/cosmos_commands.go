// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Cosmos command layer preserved across Printing Press regenerations.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/platform"
	"github.com/spf13/cobra"
)

type cosmosGraphQLError struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

type cosmosGraphQLEnvelope struct {
	Data   map[string]any       `json:"data"`
	Errors []cosmosGraphQLError `json:"errors"`
}

type cosmosEndpointBuilder func(*rootFlags) *cobra.Command

func cosmosCapturedQuery(flags *rootFlags, builder cosmosEndpointBuilder) (string, string, map[string]any, error) {
	probe := builder(flags)
	opFlag := probe.Flags().Lookup("operation-name")
	queryFlag := probe.Flags().Lookup("query")
	if opFlag == nil || queryFlag == nil {
		return "", "", nil, fmt.Errorf("captured operation metadata is unavailable")
	}
	extensions := map[string]any{}
	if extFlag := probe.Flags().Lookup("extensions"); extFlag != nil && strings.TrimSpace(extFlag.DefValue) != "" {
		if err := json.Unmarshal([]byte(extFlag.DefValue), &extensions); err != nil {
			return "", "", nil, fmt.Errorf("decode captured GraphQL extensions: %w", err)
		}
	}
	return opFlag.DefValue, queryFlag.DefValue, extensions, nil
}

func cosmosCaptured(flags *rootFlags, cmd *cobra.Command, builder cosmosEndpointBuilder, variables map[string]any) (map[string]any, error) {
	op, query, extensions, err := cosmosCapturedQuery(flags, builder)
	if err != nil {
		return nil, err
	}
	return cosmosGraphQL(flags, cmd, op, query, variables, extensions)
}

func cosmosGraphQL(flags *rootFlags, cmd *cobra.Command, operation, query string, variables map[string]any, extensions map[string]any) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"operationName": operation,
		"query":         query,
		"variables":     variables,
	}
	if len(extensions) > 0 {
		body["extensions"] = extensions
	}
	operationKind, err := cosmosGraphQLOperationKind(query)
	if err != nil {
		return nil, usageErr(fmt.Errorf("%s: %w", operation, err))
	}
	var raw json.RawMessage
	if operationKind == "query" {
		raw, _, err = c.PostQueryWithParamsAndHeaders(cmd.Context(), "/graphql", map[string]string{"q": operation}, body, map[string]string{"x-client-name": "cosmos-web"})
	} else {
		raw, _, err = c.PostWithParamsAndHeaders(cmd.Context(), "/graphql", map[string]string{"q": operation}, body, map[string]string{"x-client-name": "cosmos-web"})
	}
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if flags.dryRun {
		return map[string]any{"dry_run": true, "operation": operation}, nil
	}
	var envelope cosmosGraphQLEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, apiErr(fmt.Errorf("decode %s response: %w", operation, err))
	}
	if len(envelope.Errors) > 0 {
		parts := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			parts = append(parts, item.Message)
		}
		return nil, apiErr(fmt.Errorf("%s: %s", operation, strings.Join(parts, "; ")))
	}
	if envelope.Data == nil {
		return nil, apiErr(fmt.Errorf("%s returned no data", operation))
	}
	return envelope.Data, nil
}

func cosmosGraphQLOperationKind(document string) (string, error) {
	trimmed := strings.TrimSpace(document)
	for strings.HasPrefix(trimmed, "#") {
		if newline := strings.IndexByte(trimmed, '\n'); newline >= 0 {
			trimmed = strings.TrimSpace(trimmed[newline+1:])
			continue
		}
		return "", fmt.Errorf("GraphQL document contains only comments")
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", fmt.Errorf("GraphQL document is empty")
	}
	switch fields[0] {
	case "query", "mutation":
		return fields[0], nil
	default:
		return "", fmt.Errorf("GraphQL document must explicitly begin with query or mutation")
	}
}

const cosmosElementFragment = `
fragment PPElement on ElementTile {
  __typename id createdAt shareUrl originalClusterId
  generatedCaption { text }
  source { url author { username fullName profileUrl } }
  owner { username }
  ... on MediaElementTile { media { mediaId url width height notSafeForWorkStatus aiGenerated } multipleMedia { mediaId url width height notSafeForWorkStatus aiGenerated } }
  ... on WebsiteElementTile { websiteTitle: title websiteDescription: description media { mediaId url width height notSafeForWorkStatus aiGenerated } }
  ... on ProductElementTile { productTitle: name productDescription: description productBrand: brand media { mediaId url width height notSafeForWorkStatus aiGenerated } }
  ... on TextElementTile { text }
}`

const cosmosGetMeQuery = `query GetMe { me { id username fullName email avatarUrl bio websiteUrl isPremium } }`

const cosmosGetClusterQuery = `query GetCluster($id: ClusterId!) {
  cluster(id: $id) { id name slug description isPrivate isPublicElementsCluster followersCount numberOfElements cover { url } owner { id username fullName avatarUrl isPremium } parentCluster { id name slug } }
}`

const cosmosFeaturedElementsQuery = `query GetFeaturedElements { featuredElements { items { ...PPElement } meta { count nextPageCursor } } } ` + cosmosElementFragment

const cosmosFeaturedClustersQuery = `query GetFeaturedClusters { featuredClusters { items { id name slug cover { url } parentCluster { id name slug } } } }`

const cosmosCreateClusterMutation = `mutation CreateCluster($input: CreateClusterInput!) { cluster { create(input: $input) { id name slug parentCluster { id name slug } } } }`

const cosmosEditConnectionsMutation = `mutation EditConnections($userId: UserId!, $elementIds: [ElementId!]!, $connect: [ClusterId!]!, $disconnect: [ClusterId!]!) {
  element { editElementsConnectionsToClusters(input: { userId: $userId, elementIds: $elementIds, clusterIdsToConnect: $connect, clusterIdsToDisconnect: $disconnect }) { success } }
}`

const cosmosCreateElementMutation = `mutation CreateElement($input: CreateElementInput!) { element { create(input: $input) { id shareUrl } } }`

func cosmosMe(flags *rootFlags, cmd *cobra.Command) (map[string]any, error) {
	data, err := cosmosGraphQL(flags, cmd, "GetMe", cosmosGetMeQuery, map[string]any{}, nil)
	if err != nil {
		return nil, err
	}
	me := mapAt(data, "me")
	if len(me) == 0 {
		return nil, notFoundErr(fmt.Errorf("Cosmos profile not found"))
	}
	return me, nil
}

func cosmosUserID(flags *rootFlags, cmd *cobra.Command) (any, error) {
	me, err := cosmosMe(flags, cmd)
	if err != nil {
		return nil, err
	}
	id, ok := me["id"]
	if !ok {
		return nil, apiErr(fmt.Errorf("Cosmos profile response omitted id"))
	}
	return id, nil
}

func cosmosCollectionElements(flags *rootFlags, cmd *cobra.Command, collectionID any, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 40
	}
	result := make([]map[string]any, 0, limit)
	var cursor any
	seenCursors := map[string]bool{}
	for len(result) < limit {
		pageSize := limit - len(result)
		data, err := cosmosCaptured(flags, cmd, newClustersElementsCmd, map[string]any{
			"clusterId": collectionID, "pageSize": pageSize, "pageCursor": cursor,
			"userId": nil, "isLoggedIn": false, "showCollaborator": false,
		})
		if err != nil {
			return nil, err
		}
		items := mapsAt(data, "clusterConnections", "items")
		for _, item := range items {
			if element := mapAt(item, "element"); len(element) > 0 {
				result = append(result, normalizeElement(element))
				if len(result) == limit {
					break
				}
			}
		}
		next := strings.TrimSpace(firstString(mapAt(data, "clusterConnections", "meta"), "nextPageCursor"))
		if next == "" {
			break
		}
		if seenCursors[next] {
			return nil, apiErr(fmt.Errorf("collection pagination repeated cursor %q", next))
		}
		seenCursors[next] = true
		cursor = next
	}
	return result, nil
}

func cosmosMyCollections(flags *rootFlags, cmd *cobra.Command, limit int) ([]map[string]any, error) {
	uid, err := cosmosUserID(flags, cmd)
	if err != nil {
		return nil, err
	}
	return cosmosMyCollectionsForUser(flags, cmd, uid, limit)
}

func cosmosMyCollectionsForUser(flags *rootFlags, cmd *cobra.Command, uid any, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	result := make([]map[string]any, 0, limit)
	seenCursors := map[string]bool{}
	var cursor any
	for len(result) < limit {
		pageSize := limit - len(result)
		if pageSize > 100 {
			pageSize = 100
		}
		data, err := cosmosCaptured(flags, cmd, newUsersClustersCmd, map[string]any{
			"ownerId": uid, "userId": uid, "ownerOrgId": nil, "pageCursor": cursor, "pageSize": pageSize,
			"order": nil, "filters": nil, "isLoggedIn": true,
		})
		if err != nil {
			return nil, err
		}
		items := mapsAt(data, "userClusters", "items")
		for _, item := range items {
			result = append(result, normalizeCollection(item))
			if len(result) == limit {
				break
			}
		}
		next := strings.TrimSpace(firstString(mapAt(data, "userClusters", "meta"), "nextPageCursor"))
		if next == "" {
			break
		}
		if seenCursors[next] {
			return nil, apiErr(fmt.Errorf("collection-list pagination repeated cursor %q", next))
		}
		seenCursors[next] = true
		cursor = next
	}
	return result, nil
}

func normalizeCollection(item map[string]any) map[string]any {
	return compactMap(map[string]any{
		"id": item["id"], "name": item["name"], "slug": item["slug"],
		"description": item["description"], "is_private": item["isPrivate"],
		"number_of_elements": item["numberOfElements"], "owner": mapAt(item, "owner"),
		"parent_collection": firstNonNil(item["parentCluster"], mapAt(item, "parentCluster")),
		"cover_url":         firstString(mapAt(item, "cover"), "url"),
	})
}

func normalizeElement(item map[string]any) map[string]any {
	source := mapAt(item, "source")
	author := mapAt(source, "author")
	media := mapAt(item, "media")
	caption := cleanCosmosText(firstString(mapAt(item, "generatedCaption"), "text"))
	return compactMap(map[string]any{
		"id": item["id"], "type": item["__typename"], "created_at": item["createdAt"],
		"share_url": item["shareUrl"], "title": cleanCosmosText(firstNonEmptyString(item["websiteTitle"], item["productTitle"])),
		"description": cleanCosmosText(firstNonEmptyString(item["websiteDescription"], item["productDescription"], item["text"])),
		"caption":     caption, "source_url": source["url"], "source_author": firstNonEmptyString(author["username"], author["fullName"]),
		"source_author_url": author["profileUrl"], "owner": firstString(mapAt(item, "owner"), "username"),
		"media_url": media["url"], "media_id": media["mediaId"], "ai_generated": media["aiGenerated"],
		"nsfw_status": media["notSafeForWorkStatus"], "original_collection_id": item["originalClusterId"],
		"connections_count": numberAt(mapAt(item, "userContext"), "connections", "meta", "count"),
	})
}

func cleanCosmosText(value string) string {
	value = html.UnescapeString(value)
	replacer := strings.NewReplacer("<n>", "", "</n>", "", "<N>", "", "</N>", "")
	return strings.TrimSpace(replacer.Replace(value))
}

func mapAt(v any, path ...string) map[string]any {
	current, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	for _, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			return map[string]any{}
		}
		current = next
	}
	return current
}

func mapsAt(v any, path ...string) []map[string]any {
	if len(path) == 0 {
		return nil
	}
	parent := mapAt(v, path[:len(path)-1]...)
	raw, ok := parent[path[len(path)-1]].([]any)
	if !ok {
		if direct, ok := parent[path[len(path)-1]].([]map[string]any); ok {
			return direct
		}
		return nil
	}
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func numberAt(v any, path ...string) any {
	if len(path) == 0 {
		return nil
	}
	parent := mapAt(v, path[:len(path)-1]...)
	return parent[path[len(path)-1]]
}

func firstString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func compactMap(m map[string]any) map[string]any {
	for key, value := range m {
		if value == nil || value == "" {
			delete(m, key)
		}
	}
	return m
}

func idString(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case json.Number:
		return n.String()
	default:
		return fmt.Sprint(v)
	}
}

func parseID(raw, label string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, usageErr(fmt.Errorf("%s must be a positive numeric ID", label))
	}
	return id, nil
}

func boundedLimit(value, max int) (int, error) {
	if value < 1 || value > max {
		return 0, usageErr(fmt.Errorf("--limit must be between 1 and %d", max))
	}
	return value, nil
}

func requireMutationApproval(flags *rootFlags) error {
	if flags.dryRun || flags.yes {
		return nil
	}
	return usageErr(fmt.Errorf("confirmation required: pass --yes or use --dry-run to preview"))
}

func printCosmos(cmd *cobra.Command, flags *rootFlags, value any) error {
	return printJSONFiltered(cmd.OutOrStdout(), value, flags)
}

func init() {
	whichIndex = append(whichIndex,
		whichEntry{Command: "auth identify", Description: "Resolve the Cosmos account ID needed to create a client profile."},
		whichEntry{Command: "discover elements", Description: "Search Cosmos for visual elements."},
		whichEntry{Command: "discover collections", Description: "Search Cosmos for collections."},
		whichEntry{Command: "discover all", Description: "Search Cosmos elements and collections together."},
		whichEntry{Command: "discover featured", Description: "Browse featured Cosmos elements and collections."},
		whichEntry{Command: "identity", Description: "Show the authenticated Cosmos account identity."},
		whichEntry{Command: "collection list", Description: "List the authenticated user's Cosmos collections."},
		whichEntry{Command: "collection show", Description: "Show Cosmos collection details."},
		whichEntry{Command: "collection elements", Description: "List the elements in a Cosmos collection."},
		whichEntry{Command: "collection search", Description: "Search the authenticated user's collection names."},
		whichEntry{Command: "collection create", Description: "Create a Cosmos collection."},
		whichEntry{Command: "collection create-sub", Description: "Create a Cosmos subcollection."},
		whichEntry{Command: "element save-url", Description: "Save an external URL as a Cosmos element."},
		whichEntry{Command: "collection connect", Description: "Connect an element to a Cosmos collection."},
		whichEntry{Command: "collection disconnect", Description: "Disconnect an element from a Cosmos collection."},
		whichEntry{Command: "element connections", Description: "Show collections and people connected to a Cosmos element."},
		whichEntry{Command: "element show", Description: "Show Cosmos element details."},
		whichEntry{Command: "element similar", Description: "Find visually similar Cosmos elements."},
		whichEntry{Command: "activity list", Description: "List recent Cosmos account activity."},
		whichEntry{Command: "feed", Description: "Show the personalized Cosmos feed."},
		whichEntry{Command: "import status", Description: "Show active Cosmos imports and progress."},
		whichEntry{Command: "export collection", Description: "Export normalized collection metadata as JSON."},
		whichEntry{Command: "export gallery", Description: "Export a Cosmos collection as a self-contained HTML gallery."},
	)
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		identity := newCosmosProfileMeCmd(flags)
		identity.Use = "identity"
		identity.Short = "Show the authenticated Cosmos account identity"
		addNovelCommandIfAbsent(root, identity)
		addNovelCommandIfAbsent(root, newCosmosDiscoverCmd(flags))
		addNovelCommandIfAbsent(root, newCosmosActivityCmd(flags))
		addNovelCommandIfAbsent(root, newCosmosFeedCmd(flags))
		addNovelCommandIfAbsent(root, newCosmosExportCmd(flags))
		addNovelCommandIfAbsent(root, newCosmosSyncCmd(flags))
		// Add the remaining approved human-facing command families before the
		// generated root's fallback registration. This lets the presentation
		// pass below see the complete public tree without editing root.go.
		addNovelCommandIfAbsent(root, newNovelCollectionCmd(flags))
		addNovelCommandIfAbsent(root, newNovelElementCmd(flags))
		addNovelCommandIfAbsent(root, newNovelProvenanceCmd(flags))
		addNovelCommandIfAbsent(root, newNovelReviewCmd(flags))
		addNovelCommandIfAbsent(root, newNovelSnapshotCmd(flags))
		if auth, _, err := root.Find([]string{"auth"}); err == nil {
			addNovelCommandIfAbsent(auth, newCosmosAuthIdentifyCmd(flags))
		}
		if importer, _, err := root.Find([]string{"import"}); err == nil {
			// The generated JSONL importer posts caller-supplied objects directly
			// to /graphql. Replace that generic transport with the one safe Cosmos
			// import capability whose query document is fixed in this package.
			root.RemoveCommand(importer)
			safeImport := &cobra.Command{Use: "import", Short: "Inspect Cosmos imports", RunE: parentNoSubcommandRunE(flags)}
			safeImport.AddCommand(newCosmosImportStatusCmd(flags))
			root.AddCommand(safeImport)
		}
		polishCosmosCommandTree(root)
	})
}

func newCosmosAuthIdentifyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "identify",
		Short:       "Resolve the Cosmos account identity needed for a client profile",
		Example:     "  cosmos-pp-cli auth identify --json",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := (cosmosEnvironmentResolver{}).Resolve(cmd.Context(), "env:COSMOS_TOKEN")
			if err != nil {
				return configErr(err)
			}
			credentials := platform.ResolvedCredentials{"credential": token}
			defer credentials.Zero()
			identity, err := (cosmosIdentityAdapter{}).ProbeIdentity(cmd.Context(), credentials, platform.SourceProfile{ExpectedBaseURL: "https://api.cosmos.so"})
			if err != nil {
				return apiErr(err)
			}
			return printCosmos(cmd, flags, map[string]any{"account_id": identity.AccountID, "base_url": identity.BaseURL})
		},
	}
}

func polishCosmosCommandTree(root *cobra.Command) {
	// The browser capture generates one command per raw GraphQL operation.
	// Keep those constructors available as exact query-document sources for the
	// human layer, but remove their transport-shaped command tree from the
	// public root. Hidden Cobra commands remain directly executable by name, so
	// remove them rather than relying on help/MCP annotations.
	for _, name := range []string{
		"alls", "clusters", "elements", "fors", "globals", "users",
		"actives", "activities", "connectables", "dids", "has", "loaders",
		"profiles", "quicks", "recentlies", "recents", "remembers", "similars", "subclusters",
	} {
		if raw, _, err := root.Find([]string{name}); err == nil && raw != root {
			root.RemoveCommand(raw)
		}
	}
	if api, _, err := root.Find([]string{"api"}); err == nil && api != root {
		root.RemoveCommand(api)
	}
	for path, example := range map[string]string{
		"activity list":         "  cosmos-pp-cli activity list --since 7d",
		"client list":           "  cosmos-pp-cli client list",
		"collection connect":    "  cosmos-pp-cli collection connect 101 202",
		"collection disconnect": "  cosmos-pp-cli collection disconnect 101 202",
		"collection elements":   "  cosmos-pp-cli collection elements 101 --limit 40",
		"collection list":       "  cosmos-pp-cli collection list --limit 50",
		"collection show":       "  cosmos-pp-cli collection show 101",
		"discover featured":     "  cosmos-pp-cli discover featured --limit 20",
		"feed":                  "  cosmos-pp-cli feed --limit 20",
		"identity":              "  cosmos-pp-cli identity",
		"import status":         "  cosmos-pp-cli import status",
		"sync":                  "  cosmos-pp-cli sync --full",
		"whoami":                "  cosmos-pp-cli whoami",
	} {
		if command, _, err := root.Find(strings.Fields(path)); err == nil {
			command.Example = example
		}
	}

	// Cobra omits the Examples heading when a leaf has no curated example.
	// Supply a help-only fallback without changing Command.Example, because the
	// live dogfood fixture resolver uses that field to choose runnable samples.
	const exampleBlock = "{{if .HasExample}}\n\nExamples:\n{{.Example}}{{end}}"
	const exampleFallback = "{{if .HasExample}}\n\nExamples:\n{{.Example}}{{else}}\n\nExamples:\n  {{.CommandPath}} --help{{end}}"
	root.SetUsageTemplate(strings.Replace(root.UsageTemplate(), exampleBlock, exampleFallback, 1))
}

func newCosmosProfileMeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "me", Short: "Show the authenticated Cosmos profile", Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			me, err := cosmosMe(flags, cmd)
			if err != nil {
				return err
			}
			return printCosmos(cmd, flags, me)
		},
	}
}

func newCosmosDiscoverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "discover", Short: "Search Cosmos elements and collections", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newCosmosDiscoverElementsCmd(flags), newCosmosDiscoverCollectionsCmd(flags), newCosmosDiscoverAllCmd(flags), newCosmosDiscoverFeaturedCmd(flags))
	return cmd
}

func newCosmosDiscoverElementsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "elements <query>", Short: "Search Cosmos visual elements by text query", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 40); err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newGlobalsElementsCmd, map[string]any{"userId": nil, "searchTerm": args[0], "contentType": nil, "origin": nil, "pageCursor": nil, "order": nil, "color": nil})
		if err != nil {
			return err
		}
		items := mapsAt(data, "searchElements", "items")
		out := make([]map[string]any, 0, min(limit, len(items)))
		for _, item := range items[:min(limit, len(items))] {
			out = append(out, normalizeElement(item))
		}
		return printCosmos(cmd, flags, map[string]any{"query": args[0], "elements": out, "count": len(out), "total_available": numberAt(data, "searchElements", "meta", "count")})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results (1-40)")
	return cmd
}

func newCosmosDiscoverCollectionsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "collections <query>", Short: "Search Cosmos collections by name or topic", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 50); err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newGlobalsClustersCmd, map[string]any{"searchTerm": args[0], "userId": nil, "pageSize": limit, "pageCursor": nil})
		if err != nil {
			return err
		}
		items := mapsAt(data, "searchClusters", "items")
		for i := range items {
			items[i] = normalizeCollection(items[i])
		}
		return printCosmos(cmd, flags, map[string]any{"query": args[0], "collections": items, "count": len(items), "total_available": numberAt(data, "searchClusters", "meta", "count")})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results (1-50)")
	return cmd
}

func newCosmosDiscoverAllCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "all <query>", Short: "Search elements and collections together", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 20); err != nil {
			return err
		}
		eData, err := cosmosCaptured(flags, cmd, newGlobalsElementsCmd, map[string]any{"userId": nil, "searchTerm": args[0], "contentType": nil, "origin": nil, "pageCursor": nil, "order": nil, "color": nil})
		if err != nil {
			return err
		}
		cData, err := cosmosCaptured(flags, cmd, newGlobalsClustersCmd, map[string]any{"searchTerm": args[0], "userId": nil, "pageSize": limit, "pageCursor": nil})
		if err != nil {
			return err
		}
		elements := mapsAt(eData, "searchElements", "items")
		collections := mapsAt(cData, "searchClusters", "items")
		if len(elements) > limit {
			elements = elements[:limit]
		}
		for i := range elements {
			elements[i] = normalizeElement(elements[i])
		}
		for i := range collections {
			collections[i] = normalizeCollection(collections[i])
		}
		return printCosmos(cmd, flags, map[string]any{"query": args[0], "elements": elements, "collections": collections})
	}}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results per category (1-20)")
	return cmd
}

func newCosmosDiscoverFeaturedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "featured", Short: "List featured elements and collections", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 50); err != nil {
			return err
		}
		eData, err := cosmosGraphQL(flags, cmd, "GetFeaturedElements", cosmosFeaturedElementsQuery, map[string]any{}, nil)
		if err != nil {
			return err
		}
		cData, err := cosmosGraphQL(flags, cmd, "GetFeaturedClusters", cosmosFeaturedClustersQuery, map[string]any{}, nil)
		if err != nil {
			return err
		}
		elements := mapsAt(eData, "featuredElements", "items")
		collections := mapsAt(cData, "featuredClusters", "items")
		if len(elements) > limit {
			elements = elements[:limit]
		}
		if len(collections) > limit {
			collections = collections[:limit]
		}
		for i := range elements {
			elements[i] = normalizeElement(elements[i])
		}
		for i := range collections {
			collections[i] = normalizeCollection(collections[i])
		}
		return printCosmos(cmd, flags, map[string]any{"elements": elements, "collections": collections})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results per category (1-50)")
	return cmd
}

func newCosmosCollectionListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List your Cosmos collections", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "list Cosmos collections")
		}
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		items, err := cosmosMyCollections(flags, cmd, limit)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"collections": items, "count": len(items)})
	}}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum collections (1-100)")
	return cmd
}

func newCosmosCollectionShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "show <collection-id>", Short: "Show Cosmos collection metadata by numeric ID", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "collection ID")
		if err != nil {
			return err
		}
		data, err := cosmosGraphQL(flags, cmd, "GetCluster", cosmosGetClusterQuery, map[string]any{"id": id}, nil)
		if err != nil {
			return err
		}
		item := mapAt(data, "cluster")
		if len(item) == 0 {
			return notFoundErr(fmt.Errorf("collection %d not found", id))
		}
		return printCosmos(cmd, flags, normalizeCollection(item))
	}}
}

func newCosmosCollectionElementsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "elements <collection-id>", Short: "List elements in a collection", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "collection ID")
		if err != nil {
			return err
		}
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		items, err := cosmosCollectionElements(flags, cmd, id, limit)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"collection_id": id, "elements": items, "count": len(items)})
	}}
	cmd.Flags().IntVar(&limit, "limit", 40, "Maximum elements (1-100)")
	return cmd
}

func newCosmosCollectionSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "search <query>", Short: "Search your collection names", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		items, err := cosmosMyCollections(flags, cmd, 100)
		if err != nil {
			return err
		}
		needle := strings.ToLower(args[0])
		filtered := make([]map[string]any, 0)
		for _, item := range items {
			if strings.Contains(strings.ToLower(fmt.Sprint(item["name"])), needle) || strings.Contains(strings.ToLower(fmt.Sprint(item["description"])), needle) {
				filtered = append(filtered, item)
				if len(filtered) == limit {
					break
				}
			}
		}
		return printCosmos(cmd, flags, map[string]any{"query": args[0], "collections": filtered, "count": len(filtered)})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum collections (1-100)")
	return cmd
}

func newCosmosCollectionCreateCmd(flags *rootFlags, sub bool) *cobra.Command {
	var name, description, parent string
	var private bool
	use := "create <name>"
	short := "Create a collection"
	if sub {
		use = "create-sub <parent-collection-id> <name>"
		short = "Create a subcollection"
	}
	cmd := &cobra.Command{Use: use, Short: short, Annotations: map[string]string{"mcp:may-write": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "create Cosmos collection")
		}
		if sub {
			if len(args) != 2 {
				return usageErr(fmt.Errorf("create-sub requires parent collection ID and name"))
			}
			parent = args[0]
			name = args[1]
		} else {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("create requires a name"))
			}
			name = args[0]
		}
		if err := requireMutationApproval(flags); err != nil {
			return err
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		input := map[string]any{"userId": uid, "name": name, "description": nil, "isPrivate": private}
		if description != "" {
			input["description"] = description
		}
		if sub {
			id, err := parseID(parent, "parent collection ID")
			if err != nil {
				return err
			}
			input["parentClusterId"] = id
		}
		data, err := cosmosGraphQL(flags, cmd, "CreateCluster", cosmosCreateClusterMutation, map[string]any{"input": input}, nil)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": true, "collection": mapAt(data, "cluster", "create")})
	}}
	cmd.Flags().StringVar(&description, "description", "", "Collection description")
	cmd.Flags().BoolVar(&private, "private", false, "Create a private collection")
	return cmd
}

func newCosmosCollectionConnectionCmd(flags *rootFlags, disconnect bool) *cobra.Command {
	verb := "connect"
	short := "Connect an element to a collection"
	if disconnect {
		verb = "disconnect"
		short = "Disconnect an element from a collection"
	}
	return &cobra.Command{Use: verb + " <collection-id> <element-id>", Short: short, Annotations: map[string]string{"mcp:may-write": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, verb+" Cosmos element")
		}
		if len(args) != 2 {
			return usageErr(fmt.Errorf("%s requires collection ID and element ID", verb))
		}
		clusterID, err := parseID(args[0], "collection ID")
		if err != nil {
			return err
		}
		elementID, err := parseID(args[1], "element ID")
		if err != nil {
			return err
		}
		if err := requireMutationApproval(flags); err != nil {
			return err
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		connect, remove := []any{clusterID}, []any{}
		if disconnect {
			connect, remove = []any{}, []any{clusterID}
		}
		data, err := cosmosGraphQL(flags, cmd, "EditConnections", cosmosEditConnectionsMutation, map[string]any{"userId": uid, "elementIds": []any{elementID}, "connect": connect, "disconnect": remove}, nil)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": mapAt(data, "element", "editElementsConnectionsToClusters")["success"], "collection_id": clusterID, "element_id": elementID, "action": verb})
	}}
}

func newCosmosElementShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "show <element-id>", Short: "Show Cosmos element metadata and source details by ID", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "element ID")
		if err != nil {
			return err
		}
		uid, _ := cosmosUserID(flags, cmd)
		data, err := cosmosCaptured(flags, cmd, newElementsGetCmd, map[string]any{"elementId": id, "userId": uid, "isLoggedIn": uid != nil})
		if err != nil {
			return err
		}
		view := mapAt(data, "elementView")
		item := mapAt(view, "element")
		if len(item) == 0 {
			return notFoundErr(fmt.Errorf("element %d not found", id))
		}
		if _, ok := item["media"]; !ok {
			if media := mapAt(view, "media"); len(media) > 0 {
				item["media"] = media
			}
		}
		return printCosmos(cmd, flags, normalizeElement(item))
	}}
}

func newCosmosElementSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "similar <element-id>", Short: "Find visually similar elements", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "element ID")
		if err != nil {
			return err
		}
		if _, err := boundedLimit(limit, 40); err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newSimilarsPromotedCmd, map[string]any{"userId": nil, "elementIds": []any{id}, "isLoggedIn": false, "isAdmin": false, "pageCursor": nil, "pageSize": limit})
		if err != nil {
			return err
		}
		items := mapsAt(data, "similarElementsV2", "items")
		for i := range items {
			items[i] = normalizeElement(items[i])
		}
		return printCosmos(cmd, flags, map[string]any{"similar_to": id, "elements": items, "count": len(items), "total_available": numberAt(data, "similarElementsV2", "meta", "count")})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results (1-40)")
	return cmd
}

func newCosmosElementSaveURLCmd(flags *rootFlags) *cobra.Command {
	var collection string
	cmd := &cobra.Command{Use: "save-url <url>", Short: "Save a URL as a new Cosmos element", Annotations: map[string]string{"mcp:may-write": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "save URL to Cosmos")
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("save-url requires one URL"))
		}
		parsed, err := url.ParseRequestURI(args[0])
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return usageErr(fmt.Errorf("URL must be an absolute http(s) URL"))
		}
		if err := requireMutationApproval(flags); err != nil {
			return err
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		input := map[string]any{"userId": uid, "clusterId": nil, "url": args[0], "image": nil, "text": nil, "videoS3ObjectKey": nil, "sourceUrl": nil}
		if collection != "" {
			id, err := parseID(collection, "collection ID")
			if err != nil {
				return err
			}
			input["clusterId"] = id
		}
		data, err := cosmosGraphQL(flags, cmd, "CreateElement", cosmosCreateElementMutation, map[string]any{"input": input}, nil)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": true, "element": mapAt(data, "element", "create"), "url": args[0], "collection_id": input["clusterId"]})
	}}
	cmd.Flags().StringVar(&collection, "collection", "", "Collection ID to add the new element to")
	return cmd
}

func newCosmosElementConnectionsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "connections <element-id>", Short: "Show collections and people connected to an element", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "element ID")
		if err != nil {
			return err
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newElementsSocialGraphCmd, map[string]any{"elementId": id, "userId": uid, "isLoggedIn": true})
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"element_id": id, "connections": mapsAt(data, "elementTopConnections", "items"), "users": mapsAt(data, "elementTopUsers", "items")})
	}}
}

func newCosmosActivityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "activity", Short: "Inspect Cosmos activity", RunE: parentNoSubcommandRunE(flags)}
	var since string
	list := &cobra.Command{Use: "list", Short: "List Cosmos account activity since a relative time or timestamp", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		start, err := timeFromRelative(since, time.Now())
		if err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newActivitiesPromotedCmd, map[string]any{"ownerId": uid, "start": start.Format(time.RFC3339), "end": nil, "onlyFollows": false, "pageCursor": nil})
		if err != nil {
			return err
		}
		items := mapsAt(data, "activityFeed", "items")
		return printCosmos(cmd, flags, map[string]any{"since": start.Format(time.RFC3339), "activity": items, "count": len(items)})
	}}
	list.Flags().StringVar(&since, "since", "7d", "Relative duration or RFC3339 timestamp")
	cmd.AddCommand(list)
	return cmd
}

func newCosmosFeedCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "feed", Short: "Show your personalized Cosmos feed", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "read Cosmos feed")
		}
		if _, err := boundedLimit(limit, 120); err != nil {
			return err
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		data, err := cosmosCaptured(flags, cmd, newForsYouElementsCmd, map[string]any{"pageCursor": nil, "userId": uid, "fetchRecommendationContext": true})
		if err != nil {
			return err
		}
		raw := mapsAt(data, "forYouElements", "items")
		out := make([]map[string]any, 0, min(limit, len(raw)))
		for _, item := range raw[:min(limit, len(raw))] {
			entry := normalizeElement(mapAt(item, "element"))
			if context := item["recommendationContext"]; context != nil {
				entry["recommendation_context"] = context
			}
			out = append(out, entry)
		}
		return printCosmos(cmd, flags, map[string]any{"elements": out, "count": len(out)})
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum feed items (1-120)")
	return cmd
}

func newCosmosImportStatusCmd(flags *rootFlags) *cobra.Command {
	var collection string
	cmd := &cobra.Command{Use: "status", Short: "Show active Cosmos imports", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		vars := map[string]any{"userId": uid, "clusterId": nil}
		if collection != "" {
			id, err := parseID(collection, "collection ID")
			if err != nil {
				return err
			}
			vars["clusterId"] = id
		}
		data, err := cosmosCaptured(flags, cmd, newActivesPromotedCmd, vars)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, data["activeImports"])
	}}
	cmd.Flags().StringVar(&collection, "collection", "", "Filter by collection ID")
	return cmd
}

func newCosmosExportCmd(flags *rootFlags) *cobra.Command {
	// Export accepts caller-selected host filesystem paths. Keep it available to
	// an interactive CLI user, but do not mirror it into the remotely callable
	// MCP surface.
	cmd := &cobra.Command{Use: "export", Short: "Export Cosmos collections", Annotations: map[string]string{"mcp:hidden": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newCosmosExportCollectionCmd(flags), newCosmosExportGalleryCmd(flags))
	return cmd
}

func newCosmosExportCollectionCmd(flags *rootFlags) *cobra.Command {
	var output string
	var limit int
	var overwrite bool
	cmd := &cobra.Command{Use: "collection <collection-id>", Short: "Export collection elements as JSON", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "collection ID")
		if err != nil {
			return err
		}
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		items, err := cosmosCollectionElements(flags, cmd, id, limit)
		if err != nil {
			return err
		}
		payload := map[string]any{"collection_id": id, "exported_at": time.Now().UTC().Format(time.RFC3339), "elements": items}
		if output == "" {
			return printCosmos(cmd, flags, payload)
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "write Cosmos collection export")
		}
		body, _ := json.MarshalIndent(payload, "", "  ")
		if err := writePrivateFile(output, append(body, '\n'), overwrite); err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": true, "path": output, "elements": len(items)})
	}}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output JSON path (stdout when omitted)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum elements (1-100)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing output file")
	return cmd
}

func newCosmosExportGalleryCmd(flags *rootFlags) *cobra.Command {
	var output string
	var limit int
	var overwrite bool
	cmd := &cobra.Command{Use: "gallery <collection-id>", Short: "Export a self-contained HTML gallery", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0], "collection ID")
		if err != nil {
			return err
		}
		if output == "" {
			return usageErr(fmt.Errorf("--output is required"))
		}
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		items, err := cosmosCollectionElements(flags, cmd, id, limit)
		if err != nil {
			return err
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "write Cosmos HTML gallery")
		}
		var b strings.Builder
		b.WriteString("<!doctype html><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\"><title>Cosmos collection</title><style>body{font:14px system-ui;margin:2rem;background:#111;color:#eee}.grid{columns:280px;column-gap:14px}.card{break-inside:avoid;margin:0 0 14px;background:#1d1d1d;border-radius:10px;overflow:hidden}.card img{width:100%;display:block}.meta{padding:10px;overflow-wrap:anywhere}a{color:#c9b7ff}</style><h1>Cosmos collection ")
		b.WriteString(strconv.FormatInt(id, 10))
		b.WriteString("</h1><div class=\"grid\">")
		for _, item := range items {
			media := html.EscapeString(mapString(item, "media_url"))
			share := html.EscapeString(mapString(item, "share_url"))
			source := html.EscapeString(mapString(item, "source_url"))
			b.WriteString("<article class=\"card\">")
			if media != "" {
				b.WriteString("<img loading=\"lazy\" src=\"")
				b.WriteString(media)
				b.WriteString("\" alt=\"\">")
			}
			b.WriteString("<div class=\"meta\"><a href=\"")
			b.WriteString(share)
			b.WriteString("\">Element ")
			b.WriteString(html.EscapeString(idString(item["id"])))
			b.WriteString("</a>")
			if source != "" {
				b.WriteString(" · <a href=\"")
				b.WriteString(source)
				b.WriteString("\">source</a>")
			}
			b.WriteString("</div></article>")
		}
		b.WriteString("</div>")
		if err := writePrivateFile(output, []byte(b.String()), overwrite); err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": true, "path": output, "elements": len(items)})
	}}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output HTML path")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum elements (1-100)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing output file")
	return cmd
}

func writePrivateFile(path string, body []byte, overwrite bool) error {
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".cosmos-write-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary output: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if overwrite {
		if err := os.Rename(tmpPath, clean); err != nil {
			return fmt.Errorf("replace output: %w", err)
		}
		return nil
	}
	if err := os.Link(tmpPath, clean); err != nil {
		if errors.Is(err, os.ErrExist) {
			return usageErr(fmt.Errorf("output file already exists; pass --overwrite to replace it"))
		}
		return fmt.Errorf("create output: %w", err)
	}
	return nil
}

type cosmosSnapshot struct {
	CapturedAt  time.Time                   `json:"captured_at"`
	AccountID   string                      `json:"account_id"`
	Collections []map[string]any            `json:"collections"`
	Elements    map[string][]map[string]any `json:"elements_by_collection"`
}

func newCosmosSyncCmd(flags *rootFlags) *cobra.Command {
	var resources string
	var full bool
	var limit int
	cmd := &cobra.Command{Use: "sync", Short: "Capture collections and membership for local analysis and snapshot diffs", Annotations: map[string]string{"mcp:local-write-open-world": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := boundedLimit(limit, 100); err != nil {
			return err
		}
		selected := map[string]bool{}
		for _, part := range strings.Split(resources, ",") {
			selected[strings.TrimSpace(part)] = true
		}
		if !selected["collections"] && !selected["elements"] {
			return usageErr(fmt.Errorf("--resources must include collections and/or elements"))
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "sync Cosmos collections and elements")
		}
		uid, err := cosmosUserID(flags, cmd)
		if err != nil {
			return err
		}
		collections, err := cosmosMyCollectionsForUser(flags, cmd, uid, 100)
		if err != nil {
			return err
		}
		snap := cosmosSnapshot{CapturedAt: time.Now().UTC(), AccountID: idString(uid), Collections: collections, Elements: map[string][]map[string]any{}}
		if selected["elements"] || full {
			for _, collection := range collections {
				id := collection["id"]
				items, err := cosmosCollectionElements(flags, cmd, id, limit)
				if err != nil {
					return fmt.Errorf("sync collection %s: %w", idString(id), err)
				}
				snap.Elements[idString(id)] = items
			}
		}
		path, err := saveCosmosSnapshot(flags, snap)
		if err != nil {
			return err
		}
		return printCosmos(cmd, flags, map[string]any{"success": true, "snapshot": path, "collections": len(collections), "elements": snapshotElementCount(snap)})
	}}
	cmd.Flags().StringVar(&resources, "resources", "collections,elements", "Comma-separated resources: collections,elements")
	cmd.Flags().BoolVar(&full, "full", false, "Fetch membership for every collection")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum elements per collection (1-100)")
	return cmd
}

func snapshotDir(flags *rootFlags) (string, error) {
	if flags == nil || flags.platformSession == nil || strings.TrimSpace(flags.platformSession.Paths.DataFile) == "" {
		return "", fmt.Errorf("verified client profile session is required for snapshot history")
	}
	return filepath.Join(filepath.Dir(flags.platformSession.Paths.DataFile), "cosmos-snapshots"), nil
}

func saveCosmosSnapshot(flags *rootFlags, snapshot cosmosSnapshot) (string, error) {
	dir, err := snapshotDir(flags)
	if err != nil {
		return "", err
	}
	expectedAccountID := snapshotSessionAccountID(flags)
	if expectedAccountID == "" || snapshot.AccountID != expectedAccountID {
		return "", fmt.Errorf("refusing to save snapshot outside the verified Cosmos account")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, snapshot.CapturedAt.Format("20060102T150405.000000000Z")+".json")
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", err
	}
	return path, writePrivateFile(path, append(body, '\n'), false)
}

func loadCosmosSnapshots(flags *rootFlags, targets ...time.Time) ([]cosmosSnapshot, error) {
	dir, err := snapshotDir(flags)
	if err != nil {
		return nil, err
	}
	expectedAccountID := snapshotSessionAccountID(flags)
	if expectedAccountID == "" {
		return nil, fmt.Errorf("verified Cosmos account identity is required for snapshot history")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	type snapshotFile struct {
		path       string
		capturedAt time.Time
	}
	candidates := make([]snapshotFile, 0, len(files))
	for _, file := range files {
		capturedAt, parseErr := time.Parse("20060102T150405.000000000Z", strings.TrimSuffix(filepath.Base(file), ".json"))
		if parseErr != nil {
			continue
		}
		candidates = append(candidates, snapshotFile{path: file, capturedAt: capturedAt})
	}
	selected := candidates
	if len(targets) > 0 {
		selected = nil
		seen := map[string]bool{}
		for _, target := range targets {
			if len(candidates) == 0 {
				break
			}
			nearest := candidates[0]
			nearestDistance := target.Sub(nearest.capturedAt).Abs()
			for _, candidate := range candidates[1:] {
				if distance := target.Sub(candidate.capturedAt).Abs(); distance < nearestDistance {
					nearest, nearestDistance = candidate, distance
				}
			}
			if !seen[nearest.path] {
				selected = append(selected, nearest)
				seen[nearest.path] = true
			}
		}
		// Keep one alternate candidate when both targets resolve to the same
		// snapshot so the caller can distinguish that case from missing history.
		if len(selected) == 1 && len(candidates) >= 2 {
			for _, candidate := range candidates {
				if !seen[candidate.path] {
					selected = append(selected, candidate)
					break
				}
			}
		}
	}
	snapshots := make([]cosmosSnapshot, 0, len(selected))
	for _, candidate := range selected {
		file, err := os.Open(candidate.path) // #nosec G304 -- paths come only from a private app-data directory plus a fixed *.json glob.
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(file, (16<<20)+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(body) > 16<<20 {
			return nil, fmt.Errorf("snapshot %s exceeds the 16 MiB safety limit", filepath.Base(candidate.path))
		}
		var snap cosmosSnapshot
		if err := json.Unmarshal(body, &snap); err != nil {
			return nil, fmt.Errorf("decode snapshot %s: %w", filepath.Base(candidate.path), err)
		}
		if snap.AccountID != expectedAccountID {
			return nil, fmt.Errorf("snapshot %s belongs to a different Cosmos account", filepath.Base(candidate.path))
		}
		snapshots = append(snapshots, snap)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].CapturedAt.Before(snapshots[j].CapturedAt) })
	return snapshots, nil
}

func snapshotSessionAccountID(flags *rootFlags) string {
	if flags == nil || flags.platformSession == nil {
		return ""
	}
	if accountID := strings.TrimSpace(flags.platformSession.ObservedIdentity["account_id"]); accountID != "" {
		return accountID
	}
	return strings.TrimSpace(flags.platformSession.ExpectedIdentity["account_id"])
}

func snapshotElementCount(s cosmosSnapshot) int {
	count := 0
	for _, items := range s.Elements {
		count += len(items)
	}
	return count
}

func timeFromRelative(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if lower == "" || lower == "now" {
		return now, nil
	}
	if strings.HasSuffix(lower, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(lower, "d"), 64)
		if err == nil && n >= 0 {
			return now.Add(-time.Duration(n * 24 * float64(time.Hour))), nil
		}
	}
	if d, err := time.ParseDuration(lower); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Time{}, usageErr(fmt.Errorf("invalid time %q: use 7d, 24h, now, or RFC3339", raw))
}
