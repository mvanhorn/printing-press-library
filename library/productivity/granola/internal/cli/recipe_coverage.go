// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newRecipeCoverageCmd(flags *rootFlags) *cobra.Command {
	var since, until, last string
	var limit int
	cmd := &cobra.Command{
		Use:   "coverage [recipe-slug]",
		Short: "List meetings in the window that do NOT have a named panel applied",
		Long: `For each meeting in the window, calls /v1/get-document-panels and
emits ndjson for meetings where the slug is missing.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			from, to, err := parseTimeWindow(last, since, until)
			if err != nil {
				return usageErr(err)
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			ids := selectDocsInWindow(c, from, to, limit)
			ic, ierr := granola.NewInternalClient()
			w := cmd.OutOrStdout()
			for _, id := range ids {
				d := c.DocumentByID(id)
				var panels map[string]string
				if ierr == nil {
					panels, _ = ic.GetDocumentPanels(id)
				}
				if _, has := panels[slug]; has {
					continue
				}
				_ = emitNDJSONLine(w, map[string]any{
					"id":             id,
					"title":          d.Title,
					"started_at":     d.CreatedAt,
					"missing_recipe": slug,
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVar(&last, "last", "", "Time window")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap meetings checked")
	return cmd
}

// Ensure time/fmt referenced.
var (
	_ = time.Now
	_ = fmt.Sprintf
)
