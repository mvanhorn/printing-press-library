package mcp

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/cobratree"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestEtsySinkConformance(t *testing.T) {
	// Exact semantic exceptions: these values select rules or identifiers, not files.
	exceptions := map[string]bool{
		// etsy-ext: inline content never opens a filesystem path.
		"etsy-pp-cli market gaps --rules-json":       true,
		"etsy-pp-cli market gaps --control-ids":      true,
		"etsy-pp-cli market price --rules-json":      true,
		"etsy-pp-cli listings audit --rules":         true, // built-in rule set enum
		"etsy-pp-cli --client-profile":               true, // profile name, not a filename
		"etsy-pp-cli --profile":                      true, // saved profile name
		"etsy-pp-cli auth login --redirect-uri":      true, // OAuth URL
		"etsy-pp-cli login --redirect-uri":           true, // OAuth URL alias
		"etsy-pp-cli doctor --fail-on":               true, // diagnostic check names
		"etsy-pp-cli profile save --description":     true, // inline description mentions profile
		"etsy-pp-cli sync --global-param":            true, // inline API key=value, help mentions path parameters
		"etsy-pp-cli sync --param":                   true, // inline API key=value
		"etsy-pp-cli sync --path-context":            true, // URL template key=value, not filesystem
		"etsy-pp-cli teach --playbook-json":          true, // inline JSON, help mentions file alternative
		"etsy-pp-cli teach-playbook --playbook-json": true, // inline JSON
		"etsy-pp-cli teach-playbook --notes":         true, // inline text, help mentions notes-file

	}
	usage := regexp.MustCompile(`(?i)\bfile\b|\bpath\b|\.json|\.csv|\.yaml|FILE`)
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Hidden || cmd.Deprecated != "" {
			return
		}
		if cmd.Annotations[cobratree.EndpointAnnotation] == "" {
			cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if f.Value.Type() != "string" && f.Value.Type() != "stringArray" && f.Value.Type() != "stringSlice" {
					return
				}
				pathish := usage.MatchString(f.Usage)
				for _, word := range []string{"file", "path", "plan", "rules", "terms", "control", "out", "dir"} {
					pathish = pathish || strings.Contains(f.Name, word)
				}
				key := cmd.CommandPath() + " --" + f.Name
				if pathish && !exceptions[key] && !cobratree.DestinationFlagBlocked(cmd, f.Name) && !cobratree.InputFlagBlocked(cmd, f.Name) {
					t.Errorf("uncovered filesystem candidate: %s", key)
				}
			})

		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(cli.RootCmd())
}
func TestEtsySinkSchemas(t *testing.T) {
	t.Setenv(cliutil.MCPReadOnlyEnvVar, "")
	s := server.NewMCPServer("etsy", "test")
	RegisterTools(s)
	for _, tc := range []struct {
		name    string
		sinks   []string
		sibling string
	}{
		{"listings_audit", []string{"plan-out", "plan_out"}, "rules"},
		{"market_snapshot", []string{"terms"}, "term"},
		{"market_gaps", []string{"rules", "control"}, "snapshot"},
		{"market_price", []string{"rules"}, "currency"},
		{"listings_bulk_edit", []string{"plan"}, "field"},
	} {
		tool := s.ListTools()[tc.name]
		if tool == nil {
			t.Errorf("missing %s", tc.name)
			continue
		}
		props := tool.Tool.InputSchema.Properties
		// etsy-ext: inline alternatives remain available while file flags stay hidden.
		if tc.name == "market_gaps" || tc.name == "market_price" {
			if _, ok := props["rules-json"]; !ok {
				t.Errorf("%s missing rules-json", tc.name)
			}
		}
		if tc.name == "market_gaps" {
			if _, ok := props["control-ids"]; !ok {
				t.Error("missing control-ids")
			}
		}
		for _, sink := range tc.sinks {
			if _, ok := props[sink]; ok {
				t.Errorf("%s exposes %s", tc.name, sink)
			}
		}
		if _, ok := props[tc.sibling]; !ok {
			t.Errorf("%s missing sibling %s; properties=%v", tc.name, tc.sibling, props)
		}
	}
}
func TestEtsySnapshotReadOnlyRegistration(t *testing.T) {
	for _, readOnly := range []string{"1", ""} {
		t.Setenv(cliutil.MCPReadOnlyEnvVar, readOnly)
		s := server.NewMCPServer("etsy", "test")
		RegisterTools(s)
		_, exists := s.ListTools()["market_snapshot"]
		if exists != (readOnly == "") {
			t.Errorf("snapshot registration with read-only=%q: %v", readOnly, exists)
		}
		if s.ListTools()["market_gaps"] == nil {
			t.Fatal("offline gaps missing")
		}
	}
}
