// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

func newSharedInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info <sharedId>",
		Short:       "Full detail (title, description, counts, review count) for one shared deck",
		Example:     "  ankiweb-pp-cli shared info 241428882",
		Annotations: map[string]string{"pp:endpoint": "shared.info", "pp:method": "GET", "pp:path": "/svc/shared/item-info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s/svc/shared/item-info?sharedId=%s\n", "https://ankiweb.net", id)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), svc.SharedDeckInfo{ID: id}, flags)
			}

			c, _, err := flags.newSvcClient()
			if err != nil {
				return err
			}
			q := url.Values{}
			q.Set("sharedId", id)
			data, _, err := c.GetBytes(cmd.Context(), "/svc/shared/item-info", q)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			info, err := svc.DecodeItemInfo(id, data)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), info, flags)
		},
	}
	return cmd
}
