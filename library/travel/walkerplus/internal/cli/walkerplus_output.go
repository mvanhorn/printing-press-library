package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
	"github.com/spf13/cobra"
)

func walkerMeta(source string) map[string]any {
	return map[string]any{"source": source, "timezone": "Asia/Tokyo", "schema_version": 1}
}

func normalizeWalkerCoverage(c *walkerplus.Coverage) {
	if c.Reasons == nil {
		c.Reasons = []string{}
	}
	if c.Routes == nil {
		c.Routes = []string{}
	}
	if c.CatalogRoutes == nil {
		c.CatalogRoutes = []string{}
	}
	if c.NativeYearLabels == nil {
		c.NativeYearLabels = []string{}
	}
}

func writeWalkerDryRun(cmd *cobra.Command, flags *rootFlags, action string) error {
	copyFlags := *flags
	copyFlags.asJSON = true
	return writeDryRun(cmd.OutOrStdout(), &copyFlags, action)
}

func walkerJSONMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	err = json.Unmarshal(raw, &payload)
	return payload, err
}

// Project only event facts. The boundedness and provenance envelope is retained
// even when the result set is empty or the selected source value is null.
func printWalkerPayload(cmd *cobra.Command, flags *rootFlags, payload map[string]any, eventKey string) error {
	if flags.csv || flags.plain || flags.quiet {
		return usageErr(fmt.Errorf("Walkerplus commands emit JSON; omit --csv, --plain and --quiet"))
	}
	if flags.selectFields != "" {
		if eventKey == "" {
			return usageErr(fmt.Errorf("--select/--fields projects events; use it with search, shortlist or event"))
		}
		if err := validateWalkerSelection(flags.selectFields); err != nil {
			return usageErr(err)
		}
		normalized, err := walkerJSONMap(payload)
		if err != nil {
			return err
		}
		payload = normalized
		selection := walkerSelectionPaths(flags.selectFields)
		if eventKey == "events" {
			items, _ := payload[eventKey].([]any)
			projected := make([]any, 0, len(items))
			for _, item := range items {
				projected = append(projected, projectWalkerObject(item, selection))
			}
			payload[eventKey] = projected
		} else {
			payload[eventKey] = projectWalkerObject(payload[eventKey], selection)
		}
	}
	copyFlags := *flags
	copyFlags.asJSON = true
	copyFlags.compact = false
	copyFlags.selectFields = ""
	copyFlags.agent = false // Already a single provenance envelope.
	var formatted bytes.Buffer
	if err := printJSONFiltered(&formatted, payload, &copyFlags); err != nil {
		return err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, formatted.Bytes()); err != nil {
		return err
	}
	compact.WriteByte('\n')
	_, err := cmd.OutOrStdout().Write(compact.Bytes())
	return err
}

func walkerSelectionPaths(selection string) []string {
	paths := []string{}
	for _, field := range strings.Split(selection, ",") {
		field = strings.TrimSpace(field)
		field = strings.TrimPrefix(strings.TrimPrefix(field, "events."), "event.")
		paths = append(paths, field)
	}
	return paths
}

func validateWalkerSelection(selection string) error {
	if selection == "" {
		return nil
	}
	for _, field := range walkerSelectionPaths(selection) {
		if field == "" || !walkerJSONFieldExists(reflect.TypeOf(walkerplus.Event{}), strings.Split(field, ".")) {
			return fmt.Errorf("unknown event field %q in --select/--fields; run schema for available fields", field)
		}
	}
	return nil
}

func walkerJSONFieldExists(t reflect.Type, path []string) bool {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || len(path) == 0 {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.PkgPath != "" || name == "" || name == "-" || name != path[0] {
			continue
		}
		return len(path) == 1 || walkerJSONFieldExists(field.Type, path[1:])
	}
	return false
}

func projectWalkerObject(value any, paths []string) any {
	object, ok := value.(map[string]any)
	if !ok {
		return value
	}
	output := map[string]any{}
	children := map[string][]string{}
	for _, path := range paths {
		head, tail, nested := strings.Cut(path, ".")
		if !nested {
			output[head] = object[head]
		} else {
			children[head] = append(children[head], tail)
		}
	}
	for head, childPaths := range children {
		if _, already := output[head]; already {
			continue
		}
		child := object[head]
		if array, ok := child.([]any); ok {
			items := make([]any, 0, len(array))
			for _, item := range array {
				items = append(items, projectWalkerObject(item, childPaths))
			}
			output[head] = items
		} else {
			output[head] = projectWalkerObject(child, childPaths)
		}
	}
	return output
}

func newWalkerSchemaCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "schema",
		Short:       "Show event JSON fields accepted by --select and --fields",
		Example:     "  walkerplus-pp-cli schema\n  walkerplus-pp-cli schema --dry-run --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeWalkerDryRun(cmd, flags, "schema")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("schema accepts no positional arguments"))
			}
			paths := walkerFieldPaths(reflect.TypeOf(walkerplus.Event{}), "")
			sort.Strings(paths)
			return printWalkerPayload(cmd, flags, map[string]any{"event_fields": paths, "projection": "Projects event fields; query, coverage and meta remain present.", "meta": walkerMeta("computed")}, "")
		},
	}
}

func walkerFieldPaths(t reflect.Type, prefix string) []string {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	paths := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.PkgPath != "" || name == "" || name == "-" {
			continue
		}
		path := prefix + name
		paths = append(paths, path)
		paths = append(paths, walkerFieldPaths(field.Type, path+".")...)
	}
	return paths
}
