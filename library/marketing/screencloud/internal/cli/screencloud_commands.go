// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/store"
	"github.com/spf13/cobra"
)

type studioListSpec struct {
	Use         string
	Short       string
	Field       string
	NodeFields  string
	Annotations map[string]string
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newRegionsCmd(flags))
		root.AddCommand(newOrgCmd(flags))
		root.AddCommand(newStudioListParent(flags, "apps", "List Studio applications", studioListSpec{Use: "list", Short: "List Studio applications", Field: "allApps", NodeFields: "id name slug isInstalled"}))
		root.AddCommand(newStudioListParent(flags, "spaces", "List organization spaces", studioListSpec{Use: "list", Short: "List organization spaces", Field: "allSpaces", NodeFields: "id name"}))
		root.AddCommand(newAppInstancesCmd(flags))
		root.AddCommand(newStudioListParent(flags, "app-installs", "Inspect installed applications", studioListSpec{Use: "list", Short: "List installed applications", Field: "allAppInstalls", NodeFields: "id"}))
		root.AddCommand(newStudioListParent(flags, "app-versions", "Inspect application versions", studioListSpec{Use: "list", Short: "List application versions", Field: "allAppVersions", NodeFields: "id"}))
		root.AddCommand(newTokensCmd(flags))
		root.AddCommand(newAppRuntimeCmd(flags))
		root.AddCommand(newScreenCloudSyncCmd(flags))
		root.AddCommand(newScreenCloudSearchCmd(flags))

		if auth, _, err := root.Find([]string{"auth"}); err == nil && auth != root {
			auth.AddCommand(newAuthInspectCmd(flags))
			auth.AddCommand(newNovelAuthCapabilitiesCmd(flags))
		}
		if graphql, _, err := root.Find([]string{"graphql"}); err == nil && graphql != root {
			graphql.AddCommand(newGraphQLRequestCmd(flags), newGraphQLParseCmd(flags), newGraphQLAtlasCmd(flags))
		}
		if playgrounds, _, err := root.Find([]string{"playgrounds"}); err == nil && playgrounds != root {
			playgrounds.AddCommand(newPlaygroundsTemplatesCmd(flags))
			playgrounds.AddCommand(newPlaygroundsFilesCmd(flags))
			playgrounds.AddCommand(newPlaygroundsDataCmd(flags))
			playgrounds.AddCommand(newPlaygroundsPreviewCmd(flags))
			playgrounds.AddCommand(newPlaygroundsViewerCmd(flags))
		}
	})
}

func newRegionsCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "regions", Short: "Resolve ScreenCloud Studio regional endpoints", RunE: parentNoSubcommandRunE(flags)}
	parent.AddCommand(&cobra.Command{
		Use: "endpoint", Short: "Show known ScreenCloud Studio regional endpoints",
		Example:     "  screencloud-pp-cli regions endpoint --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows := []map[string]any{
				{"region": "us", "graphql_url": "https://graphql.us.screencloud.com/graphql", "studio_url": "https://studio.us.screencloud.com"},
				{"region": "eu", "graphql_url": "https://graphql.eu.screencloud.com/graphql", "studio_url": "https://studio.eu.screencloud.com"},
			}
			return printValue(cmd, flags, rows)
		},
	})
	return parent
}

func newAuthInspectCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use: "inspect", Short: "Inspect credential shape and source without printing token material",
		Example:     "  screencloud-pp-cli auth inspect --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			header := cfg.AuthHeader()
			shape := "missing"
			if strings.HasPrefix(strings.ToLower(header), "bearer ") {
				shape = "bearer"
			} else if header != "" {
				shape = "custom"
			}
			bucket := "none"
			if n := len(strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))); n > 0 && n < 32 {
				bucket = "short"
			} else if n >= 32 && n < 128 {
				bucket = "medium"
			} else if n >= 128 {
				bucket = "long"
			}
			return printValue(cmd, flags, map[string]any{"configured": header != "", "shape": shape, "length_bucket": bucket, "source": cfg.AuthSource, "token_included": false})
		},
	}
}

func newOrgCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "org", Short: "Inspect the active Studio organization", RunE: parentNoSubcommandRunE(flags)}
	var expectedOrgID string
	current := &cobra.Command{
		Use: "current", Short: "Return the organization selected by the current credential",
		Example:     "  screencloud-pp-cli org current --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			expected := strings.TrimSpace(expectedOrgID)
			if expected == "" {
				expected = strings.TrimSpace(os.Getenv("SCREENCLOUD_ORGANIZATION_ID"))
			}
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "query currentOrgId", "expected_organization_configured": expected != "", "sent": false})
			}
			data, meta, err := runGraphQL(cmd.Context(), flags, `query CurrentOrganization { currentOrgId }`, nil)
			if err != nil {
				return err
			}
			var out map[string]any
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			if cost, ok := meta["graphqlQueryCost"]; ok {
				out["graphql_query_cost"] = cost
			}
			if expected != "" {
				actual := firstString(out, "currentOrgId")
				out["organization_match"] = actual == expected
				if actual != expected {
					if printErr := printValue(cmd, flags, out); printErr != nil {
						return printErr
					}
					return authErr(fmt.Errorf("the current credential belongs to a different organization than --expected-org-id/SCREENCLOUD_ORGANIZATION_ID"))
				}
			}
			return printValue(cmd, flags, out)
		},
	}
	current.Flags().StringVar(&expectedOrgID, "expected-org-id", "", "Expected organization UUID; defaults to SCREENCLOUD_ORGANIZATION_ID")
	parent.AddCommand(current)
	return parent
}

func newStudioListParent(flags *rootFlags, use, short string, spec studioListSpec) *cobra.Command {
	parent := &cobra.Command{Use: use, Short: short, RunE: parentNoSubcommandRunE(flags)}
	list := newStudioListCmd(flags, spec)
	list.Example = fmt.Sprintf("  screencloud-pp-cli %s %s --first 100 --json", use, spec.Use)
	parent.AddCommand(list)
	return parent
}

func newStudioListCmd(flags *rootFlags, spec studioListSpec) *cobra.Command {
	var first int
	cmd := &cobra.Command{
		Use: spec.Use, Short: spec.Short,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": spec.Field, "first": first, "sent": false})
			}
			if first < 1 || first > 500 {
				return usageErr(fmt.Errorf("--first must be between 1 and 500"))
			}
			query := fmt.Sprintf("query CLIList($first: Int!) { %s(first: $first) { totalCount nodes { %s } } }", spec.Field, spec.NodeFields)
			data, meta, err := runGraphQL(cmd.Context(), flags, query, map[string]any{"first": first})
			if err != nil {
				return err
			}
			var root map[string]struct {
				TotalCount int              `json:"totalCount"`
				Nodes      []map[string]any `json:"nodes"`
			}
			if err := json.Unmarshal(data, &root); err != nil {
				return err
			}
			conn := root[spec.Field]
			if flags.asJSON {
				out := map[string]any{"items": conn.Nodes, "total_count": conn.TotalCount}
				if cost, ok := meta["graphqlQueryCost"]; ok {
					out["graphql_query_cost"] = cost
				}
				return printValue(cmd, flags, out)
			}
			return printValue(cmd, flags, conn.Nodes)
		},
	}
	cmd.Flags().IntVar(&first, "first", 100, "Maximum records to return (1-500)")
	return cmd
}

func newAppInstancesCmd(flags *rootFlags) *cobra.Command {
	parent := newStudioListParent(flags, "app-instances", "Inspect and create Studio app instances", studioListSpec{Use: "list", Short: "List Studio app instances", Field: "allAppInstances", NodeFields: "id"})
	parent.AddCommand(newAppInstanceCreateCmd(flags))
	return parent
}

func newAppInstanceCreateCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a Studio app instance from a reviewed JSON input",
		Example:     "  screencloud-pp-cli app-instances create --input instance.json --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan := map[string]any{"operation": "createAppInstance", "input_file": filepath.Clean(inputPath), "target": "Studio app instance", "sent": false}
			if flags.dryRun {
				return printValue(cmd, flags, plan)
			}
			if strings.TrimSpace(inputPath) == "" {
				return usageErr(fmt.Errorf("--input is required"))
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("refusing mutation without --yes; review it first with --dry-run"))
			}
			raw, err := os.ReadFile(filepath.Clean(inputPath)) // #nosec G304 -- explicitly selected input file.
			if err != nil {
				return fmt.Errorf("reading --input: %w", err)
			}
			var input map[string]any
			if err := json.Unmarshal(raw, &input); err != nil {
				return usageErr(fmt.Errorf("--input must contain one JSON object: %w", err))
			}
			data, meta, err := runGraphQL(cmd.Context(), flags, `mutation CreateAppInstance($input: CreateAppInstanceInput!) { createAppInstance(input: $input) { appInstance { id } clientMutationId } }`, map[string]any{"input": input})
			if err != nil {
				return err
			}
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				return err
			}
			created, _ := result["createAppInstance"].(map[string]any)
			instance, _ := created["appInstance"].(map[string]any)
			receipt := map[string]any{
				"stage":                   "studio_instance_created",
				"app_instance_id":         firstString(instance, "id"),
				"app_uuid":                findIdentifier(input["config"], "appuuid", "app_uuid"),
				"space_id":                firstString(input, "spaceId", "space_id"),
				"studio_instance_created": true,
				"files_uploaded":          false,
				"data_uploaded":           false,
				"result":                  result,
			}
			if cost, ok := meta["graphqlQueryCost"]; ok {
				receipt["graphql_query_cost"] = cost
			}
			return printValue(cmd, flags, receipt)
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "Path to a CreateAppInstanceInput JSON object")
	return cmd
}

func newTokensCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "tokens", Short: "Mint short-lived Playgrounds-scoped JWTs", RunE: parentNoSubcommandRunE(flags)}
	for _, kind := range []string{"management", "viewer"} {
		kind := kind
		group := &cobra.Command{Use: kind, Short: "Manage " + kind + " tokens", RunE: parentNoSubcommandRunE(flags)}
		var spaceID, screenID string
		var showToken bool
		create := &cobra.Command{
			Use:         "create",
			Short:       "Mint a short-lived Playgrounds-scoped token without storing it",
			Example:     "  screencloud-pp-cli tokens " + kind + " create --space-id 6f14d9d8-7e6d-42a1-9bb4-0a3d75a8a123 --yes --json",
			Annotations: map[string]string{"mcp:read-only": "false"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				if flags.dryRun {
					return printValue(cmd, flags, map[string]any{"operation": "mint " + kind + " JWT", "space_id": spaceID, "screen_id": screenID, "stored": false, "sent": false})
				}
				if strings.TrimSpace(spaceID) == "" {
					return usageErr(fmt.Errorf("--space-id is required"))
				}
				token, public, err := mintScopedJWT(cmd.Context(), flags, kind, spaceID, screenID)
				if err != nil {
					return err
				}
				if showToken {
					public["token"] = token
				} else {
					public["token"] = "REDACTED"
				}
				public["stored"] = false
				return printValue(cmd, flags, public)
			},
		}
		create.Flags().StringVar(&spaceID, "space-id", "", "Space UUID that scopes the token")
		if kind == "viewer" {
			create.Flags().StringVar(&screenID, "screen-id", "", "Optional screen UUID that narrows viewer scope")
		}
		create.Flags().BoolVar(&showToken, "show-token", false, "Include the sensitive token in output for immediate piping")
		group.AddCommand(create)
		parent.AddCommand(group)
	}
	return parent
}

func newGraphQLRequestCmd(flags *rootFlags) *cobra.Command {
	var query, queryFile, variablesJSON string
	cmd := &cobra.Command{
		Use: "request", Short: "Execute a Studio GraphQL document with structured error and query-cost handling",
		Example:     "  screencloud-pp-cli graphql request --query 'query { currentOrgId }' --json",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--query=query { currentOrgId }"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "POST /graphql", "query_file": queryFile, "variables_supplied": variablesJSON != "", "sent": false})
			}
			if strings.TrimSpace(query) == "" && strings.TrimSpace(queryFile) == "" {
				return usageErr(fmt.Errorf("one of --query or --query-file is required"))
			}
			if query != "" && queryFile != "" {
				return usageErr(fmt.Errorf("--query and --query-file are mutually exclusive"))
			}
			if queryFile != "" {
				var raw []byte
				var err error
				if queryFile == "-" {
					raw, err = io.ReadAll(cmd.InOrStdin())
				} else {
					raw, err = os.ReadFile(filepath.Clean(queryFile))
				} // #nosec G304 -- explicitly selected query file.
				if err != nil {
					return fmt.Errorf("reading --query-file: %w", err)
				}
				query = string(raw)
			}
			if graphqlDocumentHasMutation(query) && !flags.yes {
				return usageErr(fmt.Errorf("refusing GraphQL mutation without --yes; review it first with --dry-run"))
			}
			variables := map[string]any{}
			if strings.TrimSpace(variablesJSON) != "" {
				if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
					return usageErr(fmt.Errorf("--variables must be a JSON object: %w", err))
				}
			}
			data, meta, err := runGraphQL(cmd.Context(), flags, query, variables)
			if err != nil {
				return err
			}
			var decoded any
			if err := json.Unmarshal(data, &decoded); err != nil {
				return err
			}
			return printValue(cmd, flags, map[string]any{"data": decoded, "meta": meta})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Inline GraphQL document")
	cmd.Flags().StringVar(&queryFile, "query-file", "", "GraphQL document file path, or - for stdin")
	cmd.Flags().StringVar(&variablesJSON, "variables", "", "GraphQL variables as one JSON object")
	return cmd
}

func newGraphQLParseCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	cmd := &cobra.Command{
		Use: "parse", Short: "Parse a saved GraphQL envelope and fail on GraphQL errors",
		Example:     "  screencloud-pp-cli graphql parse --input response.json --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--input=./fixtures/graphql-response.json"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flags.dryRun {
				return printValue(cmd, flags, map[string]any{"operation": "parse GraphQL envelope", "input": inputPath, "sent": false})
			}
			var reader io.Reader = cmd.InOrStdin()
			if inputPath != "-" {
				f, err := os.Open(filepath.Clean(inputPath))
				if err != nil {
					return err
				}
				defer f.Close()
				reader = f
			}
			var env graphQLEnvelope
			if err := json.NewDecoder(bufio.NewReader(reader)).Decode(&env); err != nil {
				return usageErr(fmt.Errorf("decoding GraphQL JSON: %w", err))
			}
			out := map[string]any{"ok": len(env.Errors) == 0, "data": json.RawMessage(env.Data), "error_count": len(env.Errors), "meta": env.Meta}
			if len(env.Errors) > 0 {
				out["errors"] = env.Errors
			}
			if err := printValue(cmd, flags, out); err != nil {
				return err
			}
			if len(env.Errors) > 0 {
				return apiErr(fmt.Errorf("GraphQL envelope contains %d error(s)", len(env.Errors)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "-", "GraphQL JSON file path or - for stdin")
	return cmd
}

func newGraphQLAtlasCmd(flags *rootFlags) *cobra.Command {
	var contains string
	cmd := &cobra.Command{
		Use: "atlas", Short: "Summarize the documented Studio GraphQL v2.103.0 surface",
		Example:     "  screencloud-pp-cli graphql atlas --contains Playgrounds --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			sections := []map[string]any{
				{"kind": "query", "count": 386}, {"kind": "mutation", "count": 319}, {"kind": "object", "count": 579},
				{"kind": "input", "count": 503}, {"kind": "enum", "count": 102}, {"kind": "interface", "count": 1}, {"kind": "scalar", "count": 13},
			}
			out := map[string]any{"version": "2.103.0", "published_pages": 1903, "reference": "https://screencloud.github.io/signage-next-graphql-docs/", "sections": sections}
			if contains != "" {
				out["filter"] = contains
				out["note"] = "Use graphql request for operations matching this atlas term."
			}
			return printValue(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&contains, "contains", "", "Annotate the summary with an operation or type search term")
	return cmd
}

func printValue(cmd *cobra.Command, flags *rootFlags, value any) error {
	raw, err := encodeRaw(value)
	if err != nil {
		return err
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if items, ok := value.([]map[string]any); ok {
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No results.")
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		}
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}

func sortedKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func loadStoreReadOnly() (*store.Store, error) {
	return store.OpenReadOnly(defaultDBPath("screencloud-pp-cli"))
}

func normalizedCommandPath(path string) string {
	return strings.TrimSpace(strings.TrimPrefix(path, "screencloud-pp-cli "))
}
