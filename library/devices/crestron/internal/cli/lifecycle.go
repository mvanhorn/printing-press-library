// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Prefers the local mirror for sellable status and falls back to a live lookup.

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"

	"github.com/spf13/cobra"
)

type lifecycleEntry struct {
	Model       string                `json:"model"`
	Status      string                `json:"status"`
	Description string                `json:"description,omitempty"`
	URL         string                `json:"url,omitempty"`
	ReplacedBy  []string              `json:"replaced_by,omitempty"`
	EndOfSale   []crestronparse.Asset `json:"end_of_sale_notices,omitempty"`
	Source      string                `json:"source,omitempty"`
	Note        string                `json:"note,omitempty"`
}

type lifecycleView struct {
	Models  []lifecycleEntry `json:"models"`
	Scanned int              `json:"scanned_models"`
	Active  int              `json:"active"`
	Retired int              `json:"discontinued"`
	Unknown int              `json:"unknown"`
	Note    string           `json:"note,omitempty"`
}

func newNovelLifecycleCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "lifecycle <model>...",
		Short: "Report whether a model is still sellable and trace its replacement chain.",
		Long: strings.Trim(`
Report whether each model is still sellable and, when it is not, what replaced
it.

Crestron keeps discontinued products under an Inactive/Discontinued catalog
path and attaches an End-of-Sale notice plus a replacement-product list. None of
that is queryable in bulk on the website, so triaging an as-built list means
visiting every product page by hand.

Use this command to determine sellable status and find a successor part.
Do NOT use it to list accessories for a current product; use 'product get --related'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli lifecycle UC-FCM-Z --agent
  crestron-pp-cli lifecycle DM-NVX-360 UC-FCM-Z --agent --select models.model,models.status
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lifecycle")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one model is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, haveMirror := openMirror(ctx, flagDB)
			if haveMirror {
				defer func() { _ = st.Close() }()
			}

			view := lifecycleView{Models: make([]lifecycleEntry, 0, len(args))}
			for _, model := range args {
				e := lifecycleEntry{Model: model, Status: "unknown"}
				if haveMirror {
					if p, ok, err := st.FindProduct(ctx, model); err == nil && ok {
						e.Model, e.Description, e.URL = p.Model, p.Description, p.URL
						e.Source = "local"
						if p.Discontinued {
							e.Status = "discontinued"
						} else {
							e.Status = "active"
						}
					}
				}
				// A model's own asset list is the authoritative End-of-Sale
				// signal: Crestron attaches the notice to the product. Falling
				// back to resource search means this still works without a
				// synced mirror.
				docID := ""
				if haveMirror {
					if p, ok, err := st.FindProduct(ctx, model); err == nil && ok {
						docID = p.DocumentID
					}
				}
				if assets, source, err := assetsForModel(ctx, c, model, docID); err == nil {
					if e.Source == "" {
						e.Source = source
					}
					for _, a := range assets {
						if a.Kind == "end-of-sale" {
							e.EndOfSale = append(e.EndOfSale, a)
						}
					}
				}
				if len(e.EndOfSale) > 0 && e.Status != "discontinued" {
					e.Status = "discontinued"
					e.Note = "an End-of-Sale notice is attached to this model"
				}
				if e.Status == "unknown" {
					if !haveMirror {
						e.Note = "no local mirror; run 'crestron-pp-cli sync' so sellable status can be read from the catalog"
					} else {
						e.Note = "model not found in the local mirror; it may be in a category that has not been synced"
					}
				}
				switch e.Status {
				case "active":
					view.Active++
				case "discontinued":
					view.Retired++
				default:
					view.Unknown++
				}
				view.Models = append(view.Models, e)
			}
			view.Scanned = len(view.Models)
			// Every model unresolved means the input was wrong; exit non-zero so
			// a script can distinguish "these parts are fine" from "bad input".
			if view.Unknown == len(view.Models) && len(view.Models) > 0 {
				// Distinguish "Crestron does not know this part" from "the
				// catalog has not been synced yet".
				anyKnown := false
				for _, m := range args {
					if _, err := resolveModel(ctx, c, st, m); err == nil || needsSync(err) {
						anyKnown = true
						break
					}
				}
				if !anyKnown {
					return fmt.Errorf("none of the requested models were found on crestron.com: %s", strings.Join(args, ", "))
				}
				view.Note = "run 'crestron-pp-cli sync' so sellable status can be read from the catalog"
			}
			if view.Unknown > 0 && !haveMirror {
				view.Note = "run 'crestron-pp-cli sync' to build the local catalog so sellable status is available"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL\tSTATUS\tEOS NOTICES\tDESCRIPTION")
			for _, e := range view.Models {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", e.Model, e.Status, len(e.EndOfSale), truncateText(e.Description, 45))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d scanned: %d active, %d discontinued, %d unknown\n",
				view.Scanned, view.Active, view.Retired, view.Unknown)
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}
