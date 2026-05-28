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

func newNovelWwdcSymbolsCmd(flags *rootFlags) *cobra.Command {
	var flagFramework string
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "symbols <session-id>",
		Short: "For a WWDC session ID, list every doc page in a framework that cites the session",
		Long: strings.TrimSpace(`
For a WWDC session identifier (e.g. 'wwdc2024-10169' or 'wwdc24-10169'), walk
the framework index and check each leaf doc page's references{} for a video
reference whose URL or identifier matches the session ID. Return the symbols
that cite the session.

Without --framework the scan defaults to 'swiftui'. Use --framework to switch.
The scan is bounded by --max-scan-pages (default 80).

Use this command to enumerate every symbol whose doc page cites a given WWDC
session. Do NOT use it for keyword search across WWDC titles/abstracts; use
'wwdc search' instead.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli wwdc symbols wwdc2024-10169 --framework swiftui --agent
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
				fmt.Fprintln(cmd.OutOrStdout(), "would walk framework index and check references{} for video matches")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<session-id> is required (e.g. wwdc2024-10169)"))
			}
			session := args[0]
			fw := flagFramework
			if fw == "" {
				fw = "swiftui"
			}
			if cliutil.IsDogfoodEnv() && flagMaxScanPages > 6 {
				flagMaxScanPages = 6
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			idx, err := applejson.FetchIndex(cmd.Context(), c, fw)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var paths []string
			idx.WalkSwift(func(n *applejson.IndexNode) {
				if n.Path != "" && n.Type != "groupMarker" && !n.External {
					paths = append(paths, n.Path)
				}
			})
			scanCap := flagMaxScanPages
			scanned := paths
			scanCapHit := false
			if len(paths) > scanCap {
				scanned = paths[:scanCap]
				scanCapHit = true
			}

			type match struct {
				symbol  string
				path    string
				session string
			}
			matches := make([]match, 0)
			var mu sync.Mutex
			var failures []fetchFailure
			var wg sync.WaitGroup
			sem := make(chan struct{}, 4)
			for _, p := range scanned {
				wg.Add(1)
				go func(path string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					rel := strings.TrimPrefix(path, "/documentation/")
					page, err := applejson.FetchDoc(cmd.Context(), c, rel)
					if err != nil {
						mu.Lock()
						failures = append(failures, fetchFailure{ID: path, Error: err.Error()})
						mu.Unlock()
						return
					}
					for _, ref := range page.References {
						if ref.Kind != "video" {
							continue
						}
						if !sessionMatches(session, ref.URL, ref.Identifier) {
							continue
						}
						mu.Lock()
						matches = append(matches, match{
							symbol:  page.Title,
							path:    page.URL,
							session: ref.URL,
						})
						mu.Unlock()
						break
					}
				}(p)
			}
			wg.Wait()

			result := wwdcSymbolsResult{
				Session:       session,
				Framework:     fw,
				ScannedPages:  len(scanned),
				IndexCount:    len(paths),
				MaxScanPages:  scanCap,
				FetchFailures: failures,
			}
			for _, m := range matches {
				result.Symbols = append(result.Symbols, wwdcSymbolMatch{
					Symbol:     m.symbol,
					Path:       m.path,
					SessionURL: m.session,
				})
			}
			sort.SliceStable(result.Symbols, func(i, j int) bool { return result.Symbols[i].Path < result.Symbols[j].Path })
			result.MatchCount = len(result.Symbols)
			if len(matches) == 0 && scanCapHit {
				result.Note = fmt.Sprintf("scanned %d/%d pages in '%s' without a match; raise --max-scan-pages or try a different --framework", len(scanned), len(paths), fw)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d page fetches failed; matches computed over %d successful pages\n", len(failures), len(scanned), len(scanned)-len(failures))
			}
			return emitJSON(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagFramework, "framework", "swiftui", "Framework slug to scan")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 80, "Maximum framework pages to fetch while scanning")
	return cmd
}

type wwdcSymbolsResult struct {
	Session       string            `json:"session"`
	Framework     string            `json:"framework"`
	Symbols       []wwdcSymbolMatch `json:"symbols"`
	MatchCount    int               `json:"match_count"`
	ScannedPages  int               `json:"scanned_pages"`
	IndexCount    int               `json:"index_total_pages"`
	MaxScanPages  int               `json:"max_scan_pages"`
	Note          string            `json:"note,omitempty"`
	FetchFailures []fetchFailure    `json:"fetch_failures,omitempty"`
}

type wwdcSymbolMatch struct {
	Symbol     string `json:"symbol"`
	Path       string `json:"path"`
	SessionURL string `json:"session_url"`
}

// sessionMatches reports whether a WWDC reference URL/identifier matches
// the user-supplied session ID. Accepts wwdc24, wwdc2024, and bare
// numeric IDs. URLs look like https://developer.apple.com/videos/play/wwdc2024/10169/
// or /videos/play/wwdc24/10169.
func sessionMatches(session, refURL, refID string) bool {
	session = strings.ToLower(strings.TrimSpace(session))
	if session == "" {
		return false
	}
	hay := strings.ToLower(refURL + " " + refID)
	if strings.Contains(hay, session) {
		return true
	}
	// Normalize wwdc24 vs wwdc2024 variants.
	if strings.HasPrefix(session, "wwdc20") && len(session) >= 7 {
		shortened := "wwdc" + session[6:]
		if strings.Contains(hay, shortened) {
			return true
		}
	}
	if strings.HasPrefix(session, "wwdc") && len(session) >= 6 && !strings.HasPrefix(session, "wwdc20") {
		yearShort := session[4:6]
		yearLong := "20" + yearShort
		expanded := "wwdc" + yearLong + session[6:]
		if strings.Contains(hay, expanded) {
			return true
		}
	}
	return false
}
