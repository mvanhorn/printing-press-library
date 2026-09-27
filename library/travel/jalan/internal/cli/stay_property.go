// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/jalan"
	"github.com/spf13/cobra"
)

func newNovelStayPropertyCmd(flags *rootFlags) *cobra.Command {
	var f stayFlags
	cmd := &cobra.Command{
		Use: "property <property-id>", Short: "Inspect Japanese property facts, access, amenities and source evidence",
		Long:        "Fetch one public property page. Property facilities do not establish exact room facilities. Japanese review-category labels and explicit unknowns are preserved. One source page, at most one transient retry; fresh by default.",
		Example:     "  jalan-pp-cli stay property 385995\n  jalan-pp-cli stay property 371898 --select id,name_ja,baths,address",
		Annotations: stayAnnotations("live", "<property-id>=385995;--max-age=5m"), SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return stayDryRun(cmd, f.query)
			}
			id, err := stayPropertyID(args)
			if err != nil {
				return stayReportError(cmd, err)
			}
			return stayCall(cmd, flags, &f, func(ctx context.Context, c stayService) (jalan.Response, error) { return c.Property(ctx, id) })
		},
	}
	addStayCacheFlags(cmd, &f)
	return cmd
}
