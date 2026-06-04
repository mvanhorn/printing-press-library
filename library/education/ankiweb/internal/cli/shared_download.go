// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSharedDownloadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download <sharedId>",
		Short: "Show the download URL for a shared deck (.apkg requires an in-browser token)",
		Long: `AnkiWeb's /svc/shared/download-deck/{id} endpoint requires a short-lived
signed JWT (op=sdd) that is minted by the site's client-side JavaScript and
cannot be reproduced from outside the browser. This command therefore does not
fetch the file — it prints the intended URL and the info page to download from
manually.`,
		Example:     "  ankiweb-pp-cli shared download 241428882\n  ankiweb-pp-cli shared download 241428882 --dry-run",
		Annotations: map[string]string{"pp:endpoint": "shared.download", "pp:method": "GET", "pp:path": "/svc/shared/download-deck/{sharedId}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := args[0]
			base := "https://ankiweb.net"
			downloadURL := fmt.Sprintf("%s/svc/shared/download-deck/%s", base, id)
			infoURL := fmt.Sprintf("%s/shared/info/%s", base, id)

			// --dry-run: print the intended URL and exit 0 cleanly.
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s?t=<signed-token>\n", downloadURL)
				return nil
			}

			msg := "AnkiWeb requires a signed download token minted in-browser; automated download is not supported yet — download from " + infoURL + " directly."

			if flags.asJSON {
				_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"id":           id,
					"download_url": downloadURL,
					"info_url":     infoURL,
					"supported":    false,
					"error":        msg,
				})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "download url: %s?t=<signed-token>\n", downloadURL)
				fmt.Fprintln(cmd.ErrOrStderr(), msg)
			}
			// Honest non-zero exit: the operation did not complete. No panic.
			return apiErr(fmt.Errorf("automated download not supported: signed token must be minted in-browser"))
		},
	}
	return cmd
}
