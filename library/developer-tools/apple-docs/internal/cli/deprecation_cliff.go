// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelDeprecationCliffCmd(flags *rootFlags) *cobra.Command {
	var flagOs string
	var flagVersion string
	var flagFramework string
	var flagMaxFetches int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "deprecation-cliff",
		Short: "List every Apple API deprecated in a given platform version, grouped by framework + kind",
		Long: strings.TrimSpace(`
List every Apple API marked deprecated in a framework, optionally narrowed to a
specific platform version.

Without --version:  one HTTP call per --framework. Walks the framework index
                    and lists every symbol with deprecated=true. Fast.
With --version:     additionally fetches each deprecated symbol's full doc page
                    to read metadata.platforms[].deprecatedAt, and keeps only
                    those whose deprecation version matches. Bounded by
                    --max-fetches (default 100).

Use this command for 'every symbol Apple deprecated in version N of platform
X'. Do NOT use it for diffing two arbitrary snapshots; use 'snapshot diff'
instead. Do NOT use it to find a per-symbol replacement; use 'port-to' instead.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli deprecation-cliff --framework swiftui --agent
  apple-docs-pp-cli deprecation-cliff --framework uikit --os iOS --version 18 --max-fetches 50
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would walk framework index")
				return nil
			}
			if flagFramework == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--framework is required"))
			}
			if (flagOs == "") != (flagVersion == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--os and --version must be used together"))
			}
			if cliutil.IsDogfoodEnv() {
				if flagMaxFetches > 8 {
					flagMaxFetches = 8
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			idx, err := applejson.FetchIndex(cmd.Context(), c, flagFramework)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			result := deprecationCliffResult{
				Framework: flagFramework,
				OS:        flagOs,
				Version:   flagVersion,
				ByKind:    map[string]int{},
			}
			var candidates []deprecatedSymbol
			idx.WalkSwift(func(n *applejson.IndexNode) {
				if n.Path == "" || !n.Deprecated {
					return
				}
				candidates = append(candidates, deprecatedSymbol{
					Title: n.Title,
					Path:  n.Path,
					Type:  n.Type,
				})
			})

			result.IndexCount = len(candidates)
			if flagOs == "" {
				result.Symbols = candidates
				if flagLimit > 0 && len(result.Symbols) > flagLimit {
					result.Symbols = result.Symbols[:flagLimit]
				}
				for _, s := range result.Symbols {
					result.ByKind[s.Type]++
				}
				result.MatchCount = len(result.Symbols)
				return emitJSON(cmd, flags, result)
			}

			// Version-specific filtering: fetch each candidate's page (bounded).
			if len(candidates) > flagMaxFetches {
				result.Note = fmt.Sprintf("scanning %d of %d deprecated symbols; raise --max-fetches to widen", flagMaxFetches, len(candidates))
				candidates = candidates[:flagMaxFetches]
			}
			type fetchOutcome struct {
				sym  deprecatedSymbol
				keep bool
				err  error
			}
			outs := make(chan fetchOutcome, len(candidates))
			var wg sync.WaitGroup
			sem := make(chan struct{}, novelHTTPConcurrency) // small concurrency cap
			for _, cand := range candidates {
				wg.Add(1)
				go func(s deprecatedSymbol) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					path := strings.TrimPrefix(s.Path, "/documentation/")
					page, err := applejson.FetchDoc(cmd.Context(), c, path)
					if err != nil {
						outs <- fetchOutcome{sym: s, err: err}
						return
					}
					for _, p := range page.Platforms {
						if !strings.EqualFold(p.Name, flagOs) {
							continue
						}
						// Apple emits "18.0" but users naturally pass "18".
						// Treat the input as a prefix match against the emitted
						// deprecatedAt so both forms work.
						input := strings.TrimSpace(flagVersion)
						target := p.DeprecatedAt
						if target != "" && (target == input ||
							strings.HasPrefix(target, input+".") ||
							strings.HasPrefix(input, target+".")) {
							s.DeprecatedAt = target
							outs <- fetchOutcome{sym: s, keep: true}
							return
						}
					}
					outs <- fetchOutcome{sym: s, keep: false}
				}(cand)
			}
			go func() {
				wg.Wait()
				close(outs)
			}()
			for o := range outs {
				if o.err != nil {
					result.FetchFailures = append(result.FetchFailures, fetchFailure{ID: o.sym.Path, Error: o.err.Error()})
					continue
				}
				if !o.keep {
					continue
				}
				result.Symbols = append(result.Symbols, o.sym)
				result.ByKind[o.sym.Type]++
			}
			sort.SliceStable(result.Symbols, func(i, j int) bool {
				return result.Symbols[i].Path < result.Symbols[j].Path
			})
			if flagLimit > 0 && len(result.Symbols) > flagLimit {
				result.Symbols = result.Symbols[:flagLimit]
			}
			result.MatchCount = len(result.Symbols)
			if len(result.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d page fetches failed; matches counted from the %d successful pages only\n", len(result.FetchFailures), len(candidates), len(candidates)-len(result.FetchFailures))
			}
			return emitJSON(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagOs, "os", "", "Platform name: iOS, macOS, watchOS, tvOS, visionOS. Required with --version.")
	cmd.Flags().StringVar(&flagVersion, "version", "", "Deprecation version (e.g. '18.0'). Required with --os; fetches per-symbol pages to verify.")
	cmd.Flags().StringVar(&flagFramework, "framework", "", "Framework slug to scan (required)")
	cmd.Flags().IntVar(&flagMaxFetches, "max-fetches", 100, "Max per-symbol page fetches when --version is set (bounded for cost)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum results to return (0 = unlimited)")
	return cmd
}

type deprecationCliffResult struct {
	Framework     string             `json:"framework"`
	OS            string             `json:"os,omitempty"`
	Version       string             `json:"version,omitempty"`
	Symbols       []deprecatedSymbol `json:"symbols"`
	MatchCount    int                `json:"match_count"`
	IndexCount    int                `json:"index_deprecated_count"`
	ByKind        map[string]int     `json:"by_kind,omitempty"`
	Note          string             `json:"note,omitempty"`
	FetchFailures []fetchFailure     `json:"fetch_failures,omitempty"`
}

type deprecatedSymbol struct {
	Title        string `json:"title"`
	Path         string `json:"path"`
	Type         string `json:"type,omitempty"`
	DeprecatedAt string `json:"deprecated_at,omitempty"`
}
