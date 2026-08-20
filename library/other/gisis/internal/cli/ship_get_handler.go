// Hand-authored — NOT generated. Implements `gisis-pp-cli ship get <imo>`
// by fetching the GISIS Ship Particulars HTML page, parsing it into a typed
// struct, caching it in the local SQLite store, and emitting JSON via the
// press's standard output helpers. The fetch/cache/error helpers here are
// shared by the ship history, batch, and refresh novel commands.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/store"

	"github.com/spf13/cobra"
)

const shipDetailsPath = "/SHIPS/ShipDetails.aspx"

func shipSourceURL(imo string) string {
	return fmt.Sprintf("https://gisis.imo.org/Public%s?IMONumber=%s", shipDetailsPath, imo)
}

// fetchShipParticulars performs a single GISIS Ship Particulars GET + parse.
// Reusing one *client.Client across calls (as batch/refresh do) keeps every
// fetch under the shared adaptive rate limiter. The returned error may be
// errLoginWall, errNotFound, a parse error, or a transport error — callers
// map it with mapShipFetchError.
func fetchShipParticulars(ctx context.Context, c *client.Client, imo string) (shipParticulars, error) {
	body, err := c.Get(ctx, shipDetailsPath, map[string]string{"IMONumber": imo})
	if err != nil {
		return shipParticulars{}, err
	}
	return parseShipParticularsHTML(body, imo, shipSourceURL(imo))
}

// mapShipFetchError converts a fetchShipParticulars error into the CLI's typed
// exit-code error with an actionable message.
func mapShipFetchError(imo string, err error, flags *rootFlags) error {
	switch {
	case errors.Is(err, errLoginWall):
		return &cliError{code: 4, err: fmt.Errorf("auth failure: %w\nRun 'gisis-pp-cli doctor' and re-authenticate via your browser, then try again", err)}
	case errors.Is(err, errNotFound):
		return &cliError{code: 3, err: fmt.Errorf("ship not found: IMO %s — verify the IMO number is correct and the vessel is listed in GISIS Ship and Company Particulars", imo)}
	default:
		return classifyAPIError(fmt.Errorf("fetching ship details for IMO %s: %w", imo, err), flags)
	}
}

// cacheShipParticulars best-effort upserts a fetched ship into the local store,
// keyed by IMO. Skipped when --no-cache is set. A write failure is returned so
// the caller can warn; it never aborts a successful fetch.
func cacheShipParticulars(ctx context.Context, flags *rootFlags, ship shipParticulars) error {
	if flags != nil && flags.noCache {
		return nil
	}
	if strings.TrimSpace(ship.IMONumber) == "" {
		return nil
	}
	data, err := json.Marshal(ship)
	if err != nil {
		return err
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("gisis-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()
	return db.UpsertShipByIMO(ship.IMONumber, data)
}

func runShipGet(flags *rootFlags) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			imo := "<imo>"
			if len(args) > 0 {
				imo = args[0]
			}
			fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n(dry run - no request sent)\n", shipSourceURL(imo))
			return nil
		}
		if len(args) < 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("IMO number is required\nUsage: %s <imo>", cmd.CommandPath()))
		}
		imo := strings.TrimSpace(args[0])
		if imo == "" {
			return usageErr(fmt.Errorf("IMO number is required"))
		}
		// PATCH(pr-953 greptile): reject malformed IMOs before the GISIS fetch.
		if !isValidIMOFormat(imo) {
			return usageErr(fmt.Errorf("invalid IMO %q: expected a 7-digit number", imo))
		}

		c, err := flags.newClient()
		if err != nil {
			return err
		}

		ship, err := fetchShipParticulars(cmd.Context(), c, imo)
		if err != nil {
			return mapShipFetchError(imo, err, flags)
		}

		if cerr := cacheShipParticulars(cmd.Context(), flags, ship); cerr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetched %s but failed to cache it locally: %v\n", imo, cerr)
		}

		// Default to JSON for piping; the press's printJSONFiltered honors
		// --json, --select, --compact, --csv, --quiet.
		return printJSONFiltered(cmd.OutOrStdout(), ship, flags)
	}
}
