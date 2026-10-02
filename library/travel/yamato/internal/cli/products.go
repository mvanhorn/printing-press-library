// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelProductsCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "products"}
	var service string
	cmd = luggageCmd(cmd, "Read current product policy evidence, sizes and conditional charges.", "--service airport --agent", "live", f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "products")
		}
		if e := validateDataSourceStrategy(f, cmd.Annotations["pp:data-source"]); e != nil {
			return usageErr(e)
		}
		if cmd.Annotations["pp:data-source"] == "computed" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("parcel uses verified local rules; choose --data-source auto or local"))
		}
		if e := noLuggageArgs(args); e != nil {
			return e
		}
		if service != "takkyubin" && service != "airport" && service != "roundtrip" && service != "same-day" {
			return usageErr(fmt.Errorf("--service must be takkyubin, airport, roundtrip or same-day"))
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c := luggageClient(f)
		p, e := c.Product(ctx, service)
		if e != nil {
			return luggageError(e)
		}
		return luggageOutput(cmd, f, c, p)
	}
	cmd.Flags().StringVar(&service, "service", "takkyubin", "Distinct product policy: takkyubin, airport, roundtrip or same-day")
	return cmd
}
