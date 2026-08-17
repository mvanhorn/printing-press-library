// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Fetches each model's documentation assets and writes them to disk.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"

	"github.com/spf13/cobra"
)

type submittalModel struct {
	Model        string   `json:"model"`
	Assets       int      `json:"assets"`
	Downloaded   int      `json:"downloaded"`
	FailedAssets int      `json:"failed_assets,omitempty"`
	Kinds        []string `json:"asset_kinds,omitempty"`
	Missing      []string `json:"missing_kinds,omitempty"`
	Dir          string   `json:"dir,omitempty"`
	Source       string   `json:"source,omitempty"`
	NeedsSync    bool     `json:"needs_sync,omitempty"`
	Unknown      bool     `json:"unknown_model,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type submittalView struct {
	Models       []submittalModel `json:"models"`
	OutDir       string           `json:"out_dir,omitempty"`
	TotalAssets  int              `json:"total_assets"`
	Downloaded   int              `json:"downloaded"`
	FailedAssets int              `json:"failed_assets,omitempty"`
	DryRun       bool             `json:"listed_only"`
	Failures     []submittalModel `json:"fetch_failures,omitempty"`
	Note         string           `json:"note,omitempty"`
}

// expectedKinds are the asset classes a complete submittal package usually
// needs. Reporting which are absent is the point: a missing Guide Spec is
// normally discovered only after the package has been assembled.
var expectedKinds = []string{"spec-sheet", "manual", "guide-spec", "cad", "revit", "certificate"}

func newNovelSubmittalCmd(flags *rootFlags) *cobra.Command {
	var flagOut string
	var flagDB string
	var flagKinds string

	cmd := &cobra.Command{
		Use:   "submittal <model>...",
		Short: "Download every documentation asset for a list of models into per-model folders with a coverage report.",
		Long: strings.Trim(`
Assemble a submittal package: every documentation asset for each model, in its
own folder, with a report of which asset classes are missing.

Crestron attaches roughly twenty assets to a product — spec sheet, CSI Guide
Spec, product manual, Security Reference Guide, certificates, CAD drawings and
Revit families — each behind its own click. Crestron's own Spec Sheet Collection
tool covers spec sheets only.

Without --out the assets are listed rather than downloaded, so the coverage
report can be reviewed first.

Use this command to assemble a multi-model, multi-asset-class submittal folder.
Do NOT use it to fetch one known file; use 'asset get'.
Do NOT use it to merely list one product's assets; use 'product resources'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli submittal DM-NVX-360 --agent
  crestron-pp-cli submittal DM-NVX-360 DM-NVX-363 --out ./submittal --agent
  crestron-pp-cli submittal DM-NVX-360 --out ./submittal --kinds spec-sheet,guide-spec
`, "\n"),
		// No mcp:read-only: with --out this downloads assets and writes
		// user-visible files, which the positional write-sink guard (positionals
		// only) does not cover, so MCP hosts should prompt before calling it.
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "submittal")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one model is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			models := args
			// Live dogfood and scorecard sampling run under a flat per-command
			// timeout, so keep the fan-out to a single model there.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				if len(models) > 1 {
					models = models[:1]
				}
			}

			wantKinds := map[string]bool{}
			for _, k := range strings.Split(flagKinds, ",") {
				if k = strings.TrimSpace(strings.ToLower(k)); k != "" {
					wantKinds[k] = true
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}

			view := submittalView{
				Models: make([]submittalModel, 0, len(models)),
				OutDir: flagOut, DryRun: flagOut == "",
			}
			view.Failures = make([]submittalModel, 0)

			for _, model := range models {
				entry := submittalModel{Model: model}
				r, err := resolveModel(ctx, c, st, model)
				if err != nil && needsSync(err) {
					// The model exists; only its catalog path is unknown. The
					// asset fallback below can still find its documents.
					entry.NeedsSync = true
					err = nil
				}
				if err != nil {
					entry.Error = err.Error()
					entry.Unknown = true
					view.Failures = append(view.Failures, entry)
					view.Models = append(view.Models, entry)
					continue
				}
				docID := r.DocumentID
				if docID == "" && r.URL != "" {
					if p, _, perr := fetchProductPage(ctx, c, r.URL); perr == nil {
						docID = p.DocumentID
					}
				}
				assets, source, err := assetsForModel(ctx, c, model, docID)
				entry.Source = source
				if err != nil {
					entry.Error = err.Error()
					view.Failures = append(view.Failures, entry)
					view.Models = append(view.Models, entry)
					continue
				}

				kinds := map[string]bool{}
				selected := make([]crestronparse.Asset, 0, len(assets))
				for _, a := range assets {
					kinds[a.Kind] = true
					if len(wantKinds) > 0 && !wantKinds[a.Kind] {
						continue
					}
					selected = append(selected, a)
				}
				entry.Assets = len(selected)
				view.TotalAssets += len(selected)
				for k := range kinds {
					entry.Kinds = append(entry.Kinds, k)
				}
				sort.Strings(entry.Kinds)
				for _, k := range expectedKinds {
					if !kinds[k] {
						entry.Missing = append(entry.Missing, k)
					}
				}

				if flagOut != "" {
					dir := filepath.Join(flagOut, sanitizeFilename(model))
					if err := os.MkdirAll(dir, 0o750); err != nil {
						entry.Error = err.Error()
						view.Failures = append(view.Failures, entry)
						view.Models = append(view.Models, entry)
						continue
					}
					entry.Dir = dir
					for _, a := range selected {
						body, err := c.Get(ctx, a.URL, nil)
						if err != nil {
							entry.FailedAssets++
							continue
						}
						name := sanitizeFilename(a.Title)
						if ext := filepath.Ext(a.URL); ext != "" && len(ext) <= 5 {
							name += ext
						} else {
							name += ".pdf"
						}
						if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
							entry.FailedAssets++
							continue
						}
						entry.Downloaded++
						view.Downloaded++
					}
					view.FailedAssets += entry.FailedAssets
				}
				view.Models = append(view.Models, entry)
			}

			// Exit non-zero only when every requested model was genuinely
			// unknown to Crestron. An unsynced catalog is a setup state, not
			// bad input, and still yields assets via the search fallback.
			allUnknown := len(view.Models) > 0
			for _, m := range view.Models {
				if !m.Unknown {
					allUnknown = false
					break
				}
			}
			if allUnknown {
				return fmt.Errorf("no documentation assets found for any requested model: %s", view.Failures[0].Error)
			}
			if len(view.Failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d models could not be resolved; totals cover the remaining %d\n",
					len(view.Failures), len(view.Models), len(view.Models)-len(view.Failures))
			}
			// A package missing assets still exits 0 — the models resolved and
			// the rest of the package is usable — but saying nothing would let
			// an incomplete submittal read as a complete one.
			if view.FailedAssets > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d asset(s) could not be downloaded; the package under %s is incomplete\n",
					view.FailedAssets, flagOut)
			}
			if view.DryRun {
				view.Note = "no --out given, so assets were listed but not downloaded"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL\tASSETS\tSAVED\tMISSING CLASSES")
			for _, m := range view.Models {
				missing := strings.Join(m.Missing, ",")
				if m.Error != "" {
					missing = "error: " + truncateText(m.Error, 40)
				}
				fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", m.Model, m.Assets, m.Downloaded, dashIfEmpty(missing))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d assets across %d models; %d saved\n",
				view.TotalAssets, len(view.Models), view.Downloaded)
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOut, "out", "", "Directory to write per-model asset folders into; omit to list only")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().StringVar(&flagKinds, "kinds", "", "Comma-separated asset kinds to include, e.g. spec-sheet,guide-spec,manual")
	return cmd
}

// sanitizeFilename keeps asset filenames safe across platforms.
func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return "asset"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
