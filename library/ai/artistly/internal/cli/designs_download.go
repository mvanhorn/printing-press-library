// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `designs download` saves a single design's images to disk by
// id or uuid, using the CDN URLs from the design record. No quota cost.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newDesignsDownloadCmd(flags *rootFlags) *cobra.Command {
	var to string
	var nameTemplate string
	cmd := &cobra.Command{
		Use:   "download <design-id>",
		Short: "Download a design's image(s) to disk by id or uuid",
		Long: trimLong(`
Download the rendered image(s) of one design, identified by its numeric id or
uuid, into a directory (default: current directory). Uses the design's CDN URLs;
costs no quota. To bulk-download many designs by query, use 'export'.`),
		Example:     "  artistly-pp-cli designs download <design-id> --to ./out",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download design image(s)")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a design id or uuid is required"))
			}
			ident := strings.TrimSpace(args[0])

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			designs, err := fetchPersonalDesigns(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var match *Design
			idNum, numErr := strconv.Atoi(ident)
			for i := range designs {
				if (numErr == nil && designs[i].ID == idNum) || designs[i].UUID == ident {
					match = &designs[i]
					break
				}
			}
			if match == nil {
				return notFoundErr(fmt.Errorf("design %q not found in your recent designs", ident))
			}
			dir := to
			if dir == "" {
				dir = "."
			}
			files, err := downloadDesign(ctx, *match, dir, nameTemplate)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return apiErr(fmt.Errorf("design %q has no downloadable images (status: %s)", ident, match.Status))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": match.ID, "uuid": match.UUID, "files": files}, flags)
			}
			for _, f := range files {
				fmt.Fprintln(cmd.OutOrStdout(), f)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Destination directory (default: current directory)")
	cmd.Flags().StringVar(&nameTemplate, "name-template", "{id}", "Filename template: {id} {uuid} {prompt} {n}")
	return cmd
}
