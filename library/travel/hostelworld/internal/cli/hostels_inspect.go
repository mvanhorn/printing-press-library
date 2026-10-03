// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
	"time"
)

func newNovelHostelsInspectCmd(flags *rootFlags) *cobra.Command {
	var save bool
	cmd := &cobra.Command{Use: "inspect [property-id]", Short: "Inspect property rules, facilities, check-in and tax evidence", Example: "  " + "hostelworld-pp-cli hostels inspect 67481 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "id=67481"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || !hw.ValidID(args[0]) {
			return usageErr(fmt.Errorf("inspect requires one numeric property ID"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		p, err := propertyObject(ctx, c, args[0])
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		v, err := hw.Property(p, time.Now())
		if err != nil {
			return err
		}
		return outputPlanning(cmd, flags, v, save)
	}}
	decoratePlanning(cmd, flags)
	cmd.Flags().BoolVar(&save, "save", false, "Save normalized property facts in the local bounded cache")
	return cmd
}
