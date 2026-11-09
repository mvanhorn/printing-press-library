package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestPlatformAnalyticsSpecRegistryMatchesCommandTree(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	actual := map[string]bool{}
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, prefix []string) {
		for _, child := range cmd.Commands() {
			path := append(append([]string(nil), prefix...), strings.ToLower(child.Name()))
			actual[strings.Join(path, " ")] = true
			walk(child, path)
		}
	}
	walk(root, nil)
	for commandPath, spec := range platformAnalyticsSpecs {
		if !actual[commandPath] {
			t.Errorf("reviewed computed command %q is absent from the Cobra command tree", commandPath)
		}
		if len(spec.RequiredInputs) == 0 || len(spec.MinimumInputRows) == 0 || len(spec.BusinessMetrics) == 0 || spec.JoinKeys == nil {
			t.Errorf("computed command %q has an incomplete analytics declaration: %#v", commandPath, spec)
		}
		for _, input := range spec.RequiredInputs {
			if spec.MinimumInputRows[input] < 1 {
				t.Errorf("computed command %q input %q has no positive minimum", commandPath, input)
			}
		}
	}
}

func TestPlatformAnalyticsSpecDeclaresAggregateSafeDiagnostics(t *testing.T) {
	for commandPath, spec := range platformAnalyticsSpecs {
		if spec.JoinRequired && (len(spec.RequiredInputs) < 2 || len(spec.JoinKeys) == 0) {
			t.Fatalf("joined command %q omitted join keys", commandPath)
		}
		minimumRows := int64(1)
		for _, minimum := range spec.MinimumInputRows {
			if minimum > minimumRows {
				minimumRows = minimum
			}
		}
		rows := make([]any, 0, minimumRows)
		for index := int64(0); index < minimumRows; index++ {
			row := map[string]any{"date": "2026-07-19"}
			for _, key := range spec.JoinKeys {
				row[key] = "fixture-key"
			}
			rows = append(rows, row)
		}
		payload := map[string]any{"rows": rows, "matched_rows": minimumRows}
		for _, input := range spec.RequiredInputs {
			if input == "result" {
				t.Fatalf("computed command %q uses the forbidden generic result input", commandPath)
			}
			payload[input+"_rows"] = spec.MinimumInputRows[input]
		}
		for _, metric := range spec.BusinessMetrics {
			if metric == "total" || metric == "count" || metric == "value" || metric == "delta" {
				t.Fatalf("computed command %q uses generic metric %q", commandPath, metric)
			}
			payload[metric] = 5
		}
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		cmd := &cobra.Command{Use: commandPath}
		flags := &rootFlags{platformSession: platformCommandTestSession(t)}
		autoDeclarePlatformAnalytics(cmd, data, flags)
		if flags.platformAnalytics == nil {
			t.Fatalf("computed command %q did not declare analytics", commandPath)
		}
		declaration := flags.platformAnalytics
		if declaration.JoinKeys == nil || declaration.ValidJoinKeys == nil || declaration.MissingJoinKeys == nil || declaration.NewestRowAt == nil || declaration.FreshnessStatus == nil || declaration.MinimumInputRows == nil {
			t.Fatalf("computed command %q omitted aggregate-safe diagnostics: %#v", commandPath, declaration)
		}
		if len(declaration.BusinessMetrics) == 0 || len(spec.BusinessMetrics) == 0 || strings.TrimSpace(declaration.Remediation) == "" {
			t.Fatalf("computed command %q omitted named metrics or remediation", commandPath)
		}
		if err := validatePlatformAnalytics(flags); err != nil {
			t.Fatalf("computed command %q declaration did not validate: %v", commandPath, err)
		}
	}
}
