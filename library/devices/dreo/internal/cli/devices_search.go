// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDevicesSearchCmd(rflags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search over cached device name, room, model, and serial",
		Example:     "  dreo-pp-cli devices search bedroom\n  dreo-pp-cli devices search HTF008S",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			q := strings.Join(args, " ")
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			devs, err := st.SearchDevices(ctx, q)
			if err != nil {
				return err
			}
			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), devs, rflags)
			}
			if len(devs) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No devices match %q. Run `dreo-pp-cli devices list` to refresh the cache.\n", q)
				return nil
			}
			headers := []string{"NAME", "ROOM", "MODEL", "SN", "ONLINE"}
			rows := make([][]string, 0, len(devs))
			for _, d := range devs {
				online := "no"
				if d.Online {
					online = "yes"
				}
				rows = append(rows, []string{d.Name, d.Room, d.Model, d.Sn, online})
			}
			return rflags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}
