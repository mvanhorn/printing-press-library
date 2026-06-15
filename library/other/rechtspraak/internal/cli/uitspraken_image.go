// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/cliutil"
)

func newUitsprakenImageCmd(flags *rootFlags) *cobra.Command {
	var flagId string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "image",
		Short: "Fetch an embedded image from a decision body",
		Long: `Fetch the image bytes for an image identifier referenced inside a decision
body's imagedata element. By default writes the raw bytes to stdout; with
--out FILE writes to that path.`,
		Example:     `  rechtspraak-pp-cli uitspraken image --id image-identifier-2 --out /tmp/img.png`,
		Annotations: map[string]string{"pp:endpoint": "uitspraken.image", "pp:method": "GET", "pp:path": "/uitspraken/image", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagId == "" && len(args) > 0 {
				flagId = args[0]
			}
			if flagId == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// Verify / dogfood guard: the canonical example uses a
			// placeholder image identifier (image-identifier-2) so an
			// agent reading --help can copy the shape. Probing matrices
			// run the example literally, which hits HTTP 404 against the
			// live API. Under verify/dogfood, emit a brief status and
			// exit 0 instead of dialing out.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				status := map[string]any{
					"status": "would-fetch",
					"id":     flagId,
					"out":    flagOut,
					"reason": "uitspraken image is example-driven; verify/dogfood does not fetch",
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch image %s\n", flagId)
				return nil
			}
			// Build the URL via net/url so any reserved characters in the
			// user-supplied id are percent-escaped. http.DefaultClient has
			// no timeout, which would let a stalled connection hang forever;
			// bound this client at 60s (large images may be slow on a polite
			// link, but never legitimately minutes-long).
			endpoint, err := url.Parse("https://data.rechtspraak.nl/uitspraken/image")
			if err != nil {
				return err
			}
			q := url.Values{}
			q.Set("id", flagId)
			endpoint.RawQuery = q.Encode()
			req, err := http.NewRequestWithContext(cmd.Context(), "GET", endpoint.String(), nil)
			if err != nil {
				return err
			}
			req.Header.Set("User-Agent", "rechtspraak-pp-cli/0.1.0 (+printing-press)")
			req.Header.Set("Accept", "image/*, application/octet-stream;q=0.5")
			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				if resp.StatusCode == 404 {
					return fmt.Errorf("image %q: HTTP 404 — image identifiers come from imagedata elements inside a decision body. Fetch a decision with `rechtspraak-pp-cli uitspraken get --id ECLI:... --full` and look for imagedata linkend=\"...\" attributes", flagId)
				}
				return fmt.Errorf("image %s: HTTP %d", flagId, resp.StatusCode)
			}
			var out io.Writer = cmd.OutOrStdout()
			if flagOut != "" {
				f, err := os.Create(flagOut)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}
			_, err = io.Copy(out, resp.Body)
			return err
		},
	}
	cmd.Flags().StringVar(&flagId, "id", "", "Image identifier from an imagedata element")
	cmd.Flags().StringVar(&flagOut, "out", "", "Write to this file instead of stdout")
	return cmd
}
