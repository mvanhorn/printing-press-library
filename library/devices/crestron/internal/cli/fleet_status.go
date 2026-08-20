// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Prefers the local mirror, where sync has already built the model-to-release
// join, and falls back to live firmware search when no mirror exists.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronfw"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"

	"github.com/spf13/cobra"
)

// fleetSearcher adapts the generated HTTP client to crestronfw.Searcher.
type fleetSearcher struct {
	flags *rootFlags
}

func (f fleetSearcher) SearchFirmware(ctx context.Context, query string, limit int) ([]crestronparse.SearchResult, error) {
	c, err := f.flags.newClient()
	if err != nil {
		return nil, err
	}
	perPage := "25"
	if limit > 25 {
		perPage = "50"
	}
	data, err := c.Get(ctx, "/Support/Search-Results", map[string]string{
		"q": query, "c": "4", "type": "Firmware",
		"o": "Created:desc", "p": "1", "m": perPage,
	})
	if err != nil {
		return nil, err
	}
	page, err := crestronparse.ParseSearchResults(data)
	if err != nil {
		return nil, err
	}
	return page.Results, nil
}

type fleetView struct {
	Models      []crestronfw.Status `json:"models"`
	Scanned     int                 `json:"scanned_models"`
	Outdated    int                 `json:"outdated"`
	Current     int                 `json:"current"`
	Unknown     int                 `json:"unknown"`
	NoRelease   int                 `json:"no_release"`
	FetchErrors []fleetFetchError   `json:"fetch_failures,omitempty"`
	Note        string              `json:"note,omitempty"`
}

type fleetFetchError struct {
	Model string `json:"model"`
	Error string `json:"error"`
}

func newNovelFleetStatusCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	var flagModels []string
	var flagConcurrency int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check every model in your installed fleet against current firmware in one command.",
		Long: strings.Trim(`
Check a list of installed Crestron models against the firmware releases that
currently cover them.

Crestron scopes one firmware release to a whole model family, so a release
titled "TSW-570/TSW-770/TSW-1070/..." is the current firmware for seven
distinct models. Searching the site for one of those models can miss the
release that governs it. This command resolves that mapping, so the
"covered_by" field often names a release you would not have searched for.

Use this command to check many models at once against a saved fleet list.
Do NOT use it to search release-note text; use 'search --type firmware_release'.
Do NOT use it to determine whether a model is discontinued; use 'lifecycle'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli fleet status --file fleet.txt --agent
  crestron-pp-cli fleet status --model DM-NVX-384 --model CP4N
  crestron-pp-cli fleet status --file fleet.txt --agent --select models.model,models.state,models.latest
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fleet status")
			}
			if flagFile == "" && len(flagModels) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file or at least one --model is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var preResolved []crestronfw.Status
			entries := make([]crestronfw.Status, 0)
			if flagFile != "" {
				// #nosec G304 -- flagFile is the user's own --file argument; reading
				// the caller-named fleet list is this command's entire purpose.
				content, err := os.ReadFile(filepath.Clean(flagFile))
				if err != nil {
					return fmt.Errorf("reading fleet file: %w", err)
				}
				entries = append(entries, crestronfw.ParseFleetFile(string(content))...)
			}
			for _, m := range flagModels {
				for _, part := range strings.Split(m, ",") {
					if p := strings.TrimSpace(part); p != "" {
						entries = append(entries, crestronfw.Status{Model: p})
					}
				}
			}
			if len(entries) == 0 {
				return usageErr(fmt.Errorf("no models found in the fleet list"))
			}

			// Live dogfood runs against the real site under a flat per-command
			// timeout, so keep the fan-out small there.
			workers := flagConcurrency
			if workers < 1 {
				workers = 4
			}
			if cliutil.IsDogfoodEnv() {
				if len(entries) > 2 {
					entries = entries[:2]
				}
				workers = 1
			}

			// The mirror already contains the expanded model-to-release join, so
			// use it when present: it is faster and needs no network at all.
			if st, ok := openMirror(ctx, flagDB); ok {
				defer func() { _ = st.Close() }()
				resolved := make([]crestronfw.Status, 0, len(entries))
				remaining := make([]crestronfw.Status, 0)
				for _, e := range entries {
					rels, err := st.ReleasesForModel(ctx, e.Model)
					if err != nil || len(rels) == 0 {
						remaining = append(remaining, e)
						continue
					}
					rows := make([]crestronparse.SearchResult, 0, len(rels))
					for _, r := range rels {
						rows = append(rows, crestronparse.SearchResult{
							Title: r.Title, Date: r.Date, URL: r.URL,
						})
					}
					resolved = append(resolved, crestronfw.Resolve(ctx, staticSearcher{rows: rows}, e, 0))
				}
				if len(remaining) == 0 {
					return renderFleet(cmd, flags, resolved)
				}
				// Some models were not in the mirror; look those up live and
				// merge, so a partially-synced mirror still answers fully.
				entries = remaining
				defer func() { _ = 0 }()
				preResolved = resolved
			}

			searcher := fleetSearcher{flags: flags}
			results := make([]crestronfw.Status, len(entries))
			sem := make(chan struct{}, workers)
			var wg sync.WaitGroup
			for i := range entries {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					results[idx] = crestronfw.Resolve(ctx, searcher, entries[idx], 25)
				}(i)
			}
			wg.Wait()

			results = append(preResolved, results...)
			return renderFleet(cmd, flags, results)
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a fleet list: one model per line, optionally followed by the installed version")
	cmd.Flags().StringArrayVar(&flagModels, "model", nil, "Model to check; repeatable, or comma-separated")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "Parallel firmware lookups")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path; when a synced mirror exists it is used instead of live lookups")
	return cmd
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// staticSearcher serves releases already read from the local mirror, so the
// same currency logic runs against local and live data.
type staticSearcher struct {
	rows []crestronparse.SearchResult
}

func (s staticSearcher) SearchFirmware(context.Context, string, int) ([]crestronparse.SearchResult, error) {
	return s.rows, nil
}

// renderFleet tallies verdicts and writes them, keeping failed lookups out of
// the clean-verdict counts.
func renderFleet(cmd *cobra.Command, flags *rootFlags, results []crestronfw.Status) error {
	view := fleetView{Models: results, Scanned: len(results)}
	view.FetchErrors = make([]fleetFetchError, 0)
	for _, r := range results {
		switch r.State {
		case crestronfw.StateOutdated:
			view.Outdated++
		case crestronfw.StateCurrent:
			view.Current++
		case crestronfw.StateUnknown:
			view.Unknown++
		case crestronfw.StateNoRelease:
			view.NoRelease++
		case crestronfw.StateError:
			view.FetchErrors = append(view.FetchErrors, fleetFetchError{Model: r.Model, Error: r.Note})
		}
	}
	if len(view.FetchErrors) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: %d of %d model lookups failed; counts below cover the remaining %d\n",
			len(view.FetchErrors), len(results), len(results)-len(view.FetchErrors))
	}
	if view.Unknown > 0 {
		view.Note = "models marked unknown have no installed version in the fleet list; add it after the model to get a currency verdict"
	}

	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODEL\tSTATE\tINSTALLED\tLATEST\tDATE\tCOVERED BY")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Model, r.State, dashIfEmpty(r.Installed), dashIfEmpty(r.Latest),
			dashIfEmpty(r.Date), dashIfEmpty(r.CoveredBy))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d scanned: %d outdated, %d current, %d unknown, %d without a release\n",
		view.Scanned, view.Outdated, view.Current, view.Unknown, view.NoRelease)
	if len(view.FetchErrors) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%d lookup(s) failed and are excluded from those counts\n", len(view.FetchErrors))
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
	}
	return nil
}
