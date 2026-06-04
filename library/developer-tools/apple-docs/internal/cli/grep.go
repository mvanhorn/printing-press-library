// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"

	"github.com/spf13/cobra"
)

func newNovelGrepCmd(flags *rootFlags) *cobra.Command {
	var flagKind string
	var flagHasPlatform string
	var flagFramework string
	var flagDeprecated bool
	var flagLimit int
	var flagMaxScanFrameworks int

	cmd := &cobra.Command{
		Use:   "grep <pattern>",
		Short: "Regex search across Apple framework indexes, with kind/platform/deprecation filters",
		Long: strings.TrimSpace(`
Search for symbol titles or paths matching a regex pattern across one or more
Apple framework indexes, with optional filters on:
  --kind <kind>             keep only symbols whose index type matches (e.g.
                            'method', 'protocol', 'struct', 'init', 'class',
                            'instanceMethod', 'typeMethod', 'macro').
  --has-platform <name>     keep only symbols whose path is in a framework that
                            ships on the named platform. (Soft filter — Apple's
                            index doesn't carry per-symbol platforms; this
                            applies at the framework level.)
  --deprecated              keep only symbols marked deprecated.
  --framework <slug>        limit the scan to one framework slug (faster).

Use this command to find symbols matching a pattern across every synced
framework, with kind/platform/deprecation filters. Do NOT use it for plain
keyword search of doc titles and abstracts; use 'search' instead.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli grep '^on[A-Z]' --kind method --framework swiftui
  apple-docs-pp-cli grep '@Observable' --kind macro --framework swiftui --agent
  apple-docs-pp-cli grep '(?i)tableview' --framework uikit --deprecated
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan framework indexes")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<pattern> is required"))
			}
			re, err := regexp.Compile(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid regex: %w", err))
			}

			frameworks := []string{flagFramework}
			if flagFramework == "" {
				frameworks = defaultGrepFrameworks
			}
			if len(frameworks) > flagMaxScanFrameworks {
				frameworks = frameworks[:flagMaxScanFrameworks]
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := grepResult{
				Pattern:    args[0],
				Frameworks: frameworks,
				Filters:    map[string]string{},
			}
			if flagKind != "" {
				result.Filters["kind"] = flagKind
			}
			if flagHasPlatform != "" {
				result.Filters["has_platform"] = flagHasPlatform
			}
			if flagDeprecated {
				result.Filters["deprecated"] = "true"
			}

			// Dedupe by (framework, path) — WalkSwift can visit the same
			// node through more than one container (a method listed under
			// both a protocol's "Topics" and its owning type's "Topics"
			// renders the same node twice). Without dedupe, match_count
			// over-counts and the matches[] list shows visible repeats.
			seen := map[string]struct{}{}
			for _, fw := range frameworks {
				idx, err := applejson.FetchIndex(cmd.Context(), c, fw)
				if err != nil {
					result.FetchFailures = append(result.FetchFailures, fetchFailure{ID: fw, Error: err.Error()})
					continue
				}
				idx.WalkSwift(func(n *applejson.IndexNode) {
					result.ScannedSymbols++
					if n.Path == "" {
						return
					}
					if flagKind != "" && !strings.EqualFold(n.Type, flagKind) {
						return
					}
					if flagDeprecated && !n.Deprecated {
						return
					}
					// Match the pattern against three surfaces so anchored regexes
					// like ^on[A-Z] still work against symbol stems even though the
					// full path starts with "/documentation/...":
					//   1. title       — "func onChange<V>(of: V, perform: ...)"
					//   2. path        — "/documentation/swiftui/view/onchange(of:perform:)"
					//   3. path stem   — "onchange" (last segment, paren-stripped)
					stem := applejson.PathStem(n.Path)
					if !re.MatchString(n.Title) && !re.MatchString(n.Path) && !re.MatchString(stem) {
						return
					}
					// Dedupe by (framework, path). Apple's framework index
					// reaches the same leaf symbol through multiple parent
					// containers (e.g., a SwiftUI extension method appears
					// under both the extending type and the extension
					// group); WalkSwift visits each path. Without this
					// guard, grep returns the same row up to N times.
					// Other WalkSwift callers don't need this because they
					// dedupe implicitly via per-symbol predicates.
					key := fw + "\x00" + n.Path
					if _, dup := seen[key]; dup {
						return
					}
					seen[key] = struct{}{}
					if len(result.Matches) >= flagLimit {
						return
					}
					result.Matches = append(result.Matches, grepMatch{
						Framework:  fw,
						Title:      n.Title,
						Path:       n.Path,
						Type:       n.Type,
						Deprecated: n.Deprecated,
					})
				})
				if len(result.Matches) >= flagLimit {
					break
				}
			}
			sort.SliceStable(result.Matches, func(i, j int) bool {
				if result.Matches[i].Framework != result.Matches[j].Framework {
					return result.Matches[i].Framework < result.Matches[j].Framework
				}
				return result.Matches[i].Path < result.Matches[j].Path
			})
			result.MatchCount = len(result.Matches)
			return emitJSON(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagKind, "kind", "", "Filter by index type: method, protocol, struct, init, class, instanceMethod, typeMethod, macro, ...")
	cmd.Flags().StringVar(&flagHasPlatform, "has-platform", "", "Soft framework-level platform filter (visionOS, macOS, iOS, watchOS, tvOS)")
	cmd.Flags().StringVar(&flagFramework, "framework", "", "Limit scan to one framework slug (faster); default scans a curated set")
	cmd.Flags().BoolVar(&flagDeprecated, "deprecated", false, "Match only symbols marked deprecated in the index")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum matches to return")
	cmd.Flags().IntVar(&flagMaxScanFrameworks, "max-scan-frameworks", 4, "Maximum framework indexes to download when --framework is not set")
	return cmd
}

type grepResult struct {
	Pattern        string            `json:"pattern"`
	Frameworks     []string          `json:"frameworks"`
	Filters        map[string]string `json:"filters,omitempty"`
	Matches        []grepMatch       `json:"matches"`
	MatchCount     int               `json:"match_count"`
	ScannedSymbols int               `json:"scanned_symbols"`
	FetchFailures  []fetchFailure    `json:"fetch_failures,omitempty"`
}

type grepMatch struct {
	Framework  string `json:"framework"`
	Title      string `json:"title"`
	Path       string `json:"path"`
	Type       string `json:"type,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
}

type fetchFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// defaultGrepFrameworks is the priority order when --framework isn't set.
// These are the highest-traffic Apple frameworks; the user can override.
var defaultGrepFrameworks = []string{"swiftui", "foundation", "uikit", "swift"}
