// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"
)

var errPostTypeNotFound = errors.New("post type not found")

type wordpressPostType struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	RestBase      string `json:"rest_base"`
	RestNamespace string `json:"rest_namespace"`
}

type unroutableType struct {
	Type          string `json:"type"`
	RestBase      string `json:"rest_base"`
	RestNamespace string `json:"rest_namespace"`
}

type schemaOutput struct {
	Target                          string           `json:"target"`
	Type                            string           `json:"type"`
	RestBase                        string           `json:"rest_base"`
	RestNamespace                   string           `json:"rest_namespace"`
	TypesDeclaredButUnroutable      []unroutableType `json:"types_declared_but_unroutable"`
	DeclaredFieldsNeverCarryingData []string         `json:"declared_fields_that_never_carry_data"`
	MetaKeysNeverCarryingData       []string         `json:"meta_keys_that_never_carry_data"`
	SampleSource                    string           `json:"sample_source"`
	SampleRows                      int              `json:"sample_rows"`
	SampleNote                      string           `json:"sample_note,omitempty"`
	MetaNote                        string           `json:"meta_note,omitempty"`
	UnroutableNote                  string           `json:"unroutable_note"`
}

func newNovelSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema <type>",
		Short: "Expose unroutable post types and fields that never carry data",
		Example: "  wordpress-pp-cli schema post --json\n" +
			"  wordpress-pp-cli schema product",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare the WordPress type registry, live routes, endpoint schema, and sampled rows")
				return nil
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("schema requires exactly one post type"))
			}

			runtime, err := resolveWordPressRuntime(flags, "")
			if err != nil {
				return configErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			httpClient := &http.Client{Timeout: flags.timeout}
			out, err := runSchema(ctx, httpClient, flags, runtime, strings.TrimSpace(args[0]))
			if err != nil {
				if errors.Is(err, errPostTypeNotFound) {
					return notFoundErr(err)
				}
				return apiErr(err)
			}
			if flags.asJSON {
				raw, marshalErr := json.Marshal(out)
				if marshalErr != nil {
					return marshalErr
				}
				// The agent envelope's meta.source must agree with where the
				// sampled rows actually came from: "local" only when the
				// mirror supplied them, "live" when the sampler fell back to
				// a live fetch (or no rows existed but live probes ran).
				metaSource := "live"
				if out.SampleSource == "local" {
					metaSource = "local"
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": metaSource})
			}
			return printSchemaHuman(cmd, out)
		},
	}
	return cmd
}

// newSchemaCmd is the name the novel-command init hook in diagnose.go uses;
// the real constructor lives in newNovelSchemaCmd (the generated root's
// scaffold hook) so static command-tree walkers see the schema path there.
func newSchemaCmd(flags *rootFlags) *cobra.Command {
	return newNovelSchemaCmd(flags)
}

func runSchema(ctx context.Context, httpClient *http.Client, flags *rootFlags, runtime wordpressRuntime, typeName string) (schemaOutput, error) {
	out := schemaOutput{
		Target: runtime.Origin, Type: typeName,
		TypesDeclaredButUnroutable:      make([]unroutableType, 0),
		DeclaredFieldsNeverCarryingData: make([]string, 0),
		MetaKeysNeverCarryingData:       make([]string, 0),
		UnroutableNote:                  "Types listed here have no matching live route; they were registered without show_in_rest and are invisible to every API client.",
	}

	typesProbe := executeProbe(ctx, httpClient, runtime, "types", runtimeProbeForm(runtime), http.MethodGet, runtimeRESTURL(runtime, "/wp/v2/types", nil), runtime.HasAuth)
	if !typesProbe.OK {
		return out, probeError("fetch post type registry", typesProbe)
	}
	types := make(map[string]wordpressPostType)
	if err := json.Unmarshal(typesProbe.body, &types); err != nil {
		return out, fmt.Errorf("decode post type registry: %w", err)
	}
	postType, ok := types[typeName]
	if !ok {
		for key, candidate := range types {
			if candidate.Slug == typeName {
				postType = candidate
				typeName = key
				ok = true
				break
			}
		}
	}
	if !ok {
		return out, fmt.Errorf("%w: %q is absent from /wp/v2/types", errPostTypeNotFound, typeName)
	}
	postType = normalizePostType(typeName, postType)
	out.Type = typeName
	out.RestBase = postType.RestBase
	out.RestNamespace = postType.RestNamespace

	indexProbe := executeProbe(ctx, httpClient, runtime, "rest-root", runtimeProbeForm(runtime), http.MethodGet, runtimeRESTURL(runtime, "/", nil), runtime.HasAuth)
	if !indexProbe.OK {
		return out, probeError("fetch REST route index", indexProbe)
	}
	var index wordpressRESTIndex
	if err := json.Unmarshal(indexProbe.body, &index); err != nil {
		return out, fmt.Errorf("decode REST route index: %w", err)
	}
	if index.Routes == nil {
		index.Routes = make(map[string]json.RawMessage)
	}
	out.TypesDeclaredButUnroutable = findUnroutableTypes(types, index.Routes)

	route := "/" + strings.Trim(postType.RestNamespace, "/") + "/" + strings.Trim(postType.RestBase, "/")
	properties := make(map[string]any)
	_, routeExists := index.Routes[route]
	if routeExists {
		optionsProbe := executeProbe(ctx, httpClient, runtime, "schema-options", runtimeProbeForm(runtime), http.MethodOptions, runtimeRESTURL(runtime, route, nil), runtime.HasAuth)
		if !optionsProbe.OK {
			return out, probeError("fetch endpoint schema", optionsProbe)
		}
		var options struct {
			Schema struct {
				Properties map[string]any `json:"properties"`
			} `json:"schema"`
		}
		if err := json.Unmarshal(optionsProbe.body, &options); err != nil {
			return out, fmt.Errorf("decode endpoint schema: %w", err)
		}
		if options.Schema.Properties != nil {
			properties = options.Schema.Properties
		}
	}

	samples, source, note, err := loadSchemaSamples(ctx, httpClient, flags, runtime, typeName, postType, routeExists)
	if err != nil {
		return out, err
	}
	out.SampleSource = source
	out.SampleRows = len(samples)
	out.SampleNote = note
	if len(samples) == 0 {
		out.SampleNote = strings.TrimSpace(note + " No rows were returned, so field-data differences are inconclusive.")
	}
	out.DeclaredFieldsNeverCarryingData = fieldsNeverCarryingData(properties, samples)
	out.MetaKeysNeverCarryingData = metaKeysNeverCarryingData(properties, samples)
	if len(metaSchemaProperties(properties)) == 0 {
		out.MetaNote = "The endpoint's meta object is empty. register_meta defaults show_in_rest to false, so most plugin and ACF fields never appear in REST."
	}
	return out, nil
}

func loadSchemaSamples(ctx context.Context, httpClient *http.Client, flags *rootFlags, runtime wordpressRuntime, typeName string, postType wordpressPostType, routeExists bool) ([]map[string]any, string, string, error) {
	samples := make([]map[string]any, 0)
	dbPath := wordpressDBPath(flags)
	if _, statErr := os.Stat(dbPath); statErr == nil {
		localStore, err := store.OpenWithContext(ctx, dbPath)
		if err != nil {
			return samples, "", "", fmt.Errorf("open local WordPress mirror: %w", err)
		}
		defer localStore.DB().Close()
		resourceTypes := []string{typeName}
		if postType.RestBase != typeName {
			resourceTypes = append(resourceTypes, postType.RestBase)
		}
		for _, resourceType := range resourceTypes {
			rows, listErr := localStore.List(resourceType, 50)
			if listErr != nil {
				continue
			}
			for _, row := range rows {
				var sample map[string]any
				if json.Unmarshal(row, &sample) == nil {
					samples = append(samples, sample)
				}
			}
			if len(samples) > 0 {
				return samples, "local", fmt.Sprintf("Sampled up to 50 mirrored %s rows.", resourceType), nil
			}
		}
	} else if !os.IsNotExist(statErr) {
		return samples, "", "", fmt.Errorf("inspect local WordPress mirror: %w", statErr)
	}

	if !routeExists {
		return samples, "none", "No local mirror rows exist and the post type has no live route to sample.", nil
	}
	params := url.Values{"per_page": []string{"50"}}
	route := "/" + strings.Trim(postType.RestNamespace, "/") + "/" + strings.Trim(postType.RestBase, "/")
	liveProbe := executeProbe(ctx, httpClient, runtime, "sample", runtimeProbeForm(runtime), http.MethodGet, runtimeRESTURL(runtime, route, params), runtime.HasAuth)
	if !liveProbe.OK {
		return samples, "", "", probeError("fetch live schema sample", liveProbe)
	}
	if err := json.Unmarshal(liveProbe.body, &samples); err != nil {
		return samples, "", "", fmt.Errorf("decode live schema sample: %w", err)
	}
	if samples == nil {
		samples = make([]map[string]any, 0)
	}
	return samples, "live", "No local mirror was available; this is one live page and therefore a shallow sample.", nil
}

func normalizePostType(name string, postType wordpressPostType) wordpressPostType {
	if postType.RestBase == "" {
		postType.RestBase = name
	}
	if postType.RestNamespace == "" {
		postType.RestNamespace = "wp/v2"
	}
	return postType
}

func findUnroutableTypes(types map[string]wordpressPostType, routes map[string]json.RawMessage) []unroutableType {
	results := make([]unroutableType, 0)
	for _, typeName := range sortedKeys(types) {
		postType := normalizePostType(typeName, types[typeName])
		route := "/" + strings.Trim(postType.RestNamespace, "/") + "/" + strings.Trim(postType.RestBase, "/")
		if !hasMatchingTypeRoute(routes, route) {
			results = append(results, unroutableType{Type: typeName, RestBase: postType.RestBase, RestNamespace: postType.RestNamespace})
		}
	}
	return results
}

func hasMatchingTypeRoute(routes map[string]json.RawMessage, collectionRoute string) bool {
	if _, ok := routes[collectionRoute]; ok {
		return true
	}
	prefix := strings.TrimRight(collectionRoute, "/") + "/"
	for route := range routes {
		if strings.HasPrefix(route, prefix) {
			return true
		}
	}
	return false
}

func fieldsNeverCarryingData(properties map[string]any, samples []map[string]any) []string {
	results := make([]string, 0)
	if len(samples) == 0 {
		return results
	}
	for _, field := range sortedKeys(properties) {
		seenData := false
		for _, sample := range samples {
			if value, ok := sample[field]; ok && !emptySchemaValue(value) {
				seenData = true
				break
			}
		}
		if !seenData {
			results = append(results, field)
		}
	}
	return results
}

func metaKeysNeverCarryingData(properties map[string]any, samples []map[string]any) []string {
	metaProperties := metaSchemaProperties(properties)
	metaSamples := make([]map[string]any, 0, len(samples))
	for _, sample := range samples {
		if meta, ok := sample["meta"].(map[string]any); ok {
			metaSamples = append(metaSamples, meta)
		} else {
			metaSamples = append(metaSamples, map[string]any{})
		}
	}
	return fieldsNeverCarryingData(metaProperties, metaSamples)
}

func metaSchemaProperties(properties map[string]any) map[string]any {
	empty := make(map[string]any)
	meta, ok := properties["meta"].(map[string]any)
	if !ok {
		return empty
	}
	metaProperties, ok := meta["properties"].(map[string]any)
	if !ok {
		return empty
	}
	return metaProperties
}

func emptySchemaValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		if len(typed) == 0 {
			return true
		}
		for _, item := range typed {
			if !emptySchemaValue(item) {
				return false
			}
		}
		return true
	case map[string]any:
		if len(typed) == 0 {
			return true
		}
		for _, item := range typed {
			if !emptySchemaValue(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func printSchemaHuman(cmd *cobra.Command, out schemaOutput) error {
	fmt.Fprintf(cmd.OutOrStdout(), "schema: %s uses %s/%s; sampled %d row(s) from %s\n", out.Type, out.RestNamespace, out.RestBase, out.SampleRows, out.SampleSource)

	unroutableRows := make([]map[string]any, 0, len(out.TypesDeclaredButUnroutable))
	for _, item := range out.TypesDeclaredButUnroutable {
		unroutableRows = append(unroutableRows, map[string]any{"type": item.Type, "rest_namespace": item.RestNamespace, "rest_base": item.RestBase})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "types declared but unroutable:")
	if err := printAutoTable(cmd.OutOrStdout(), unroutableRows); err != nil {
		return err
	}

	fieldRows := make([]map[string]any, 0, len(out.DeclaredFieldsNeverCarryingData))
	for _, field := range out.DeclaredFieldsNeverCarryingData {
		fieldRows = append(fieldRows, map[string]any{"field": field})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "declared fields that never carry data:")
	if err := printAutoTable(cmd.OutOrStdout(), fieldRows); err != nil {
		return err
	}

	metaRows := make([]map[string]any, 0, len(out.MetaKeysNeverCarryingData))
	for _, key := range out.MetaKeysNeverCarryingData {
		metaRows = append(metaRows, map[string]any{"meta_key": key})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "meta keys that never carry data:")
	if err := printAutoTable(cmd.OutOrStdout(), metaRows); err != nil {
		return err
	}
	if out.SampleNote != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "sample note:", out.SampleNote)
	}
	if out.MetaNote != "" {
		fmt.Fprintln(cmd.OutOrStdout(), "meta note:", out.MetaNote)
	}
	return nil
}
