// get: fetch a single listing by id (local store first, then live scan).
// open: print (or launch) the canonical Zameen URL for a listing.
package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/zameen"
)

// newGetCmd looks a listing up by external id.
// pp:data-source local
func newGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var idFlag string
	cmd := &cobra.Command{
		Use:         "get <external-id>",
		Short:       "Get a single stored listing by its Zameen id",
		Example:     "  zameen-pp-cli get 54194131",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up a listing by id")
				return nil
			}
			id := strings.TrimSpace(idFlag)
			if id == "" && len(args) >= 1 {
				id = strings.TrimSpace(args[0])
			}
			if id == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide an external id as an argument or via --id"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			listings, err := loadStoredListings(cmd.Context(), dbPath)
			if err != nil {
				if errors.Is(err, errNoMirror) {
					return emitEmptyMirrorHint(cmd, flags, dbPath)
				}
				return err
			}
			for _, l := range listings {
				if l.ExternalId == id {
					return emitObject(cmd, flags, l)
				}
			}
			return notFoundErr(fmt.Errorf("listing %q not in local store; run: zameen-pp-cli pull ... first, or open its URL with 'zameen-pp-cli open %s'", id, id))
		},
	}
	cmd.Flags().StringVar(&idFlag, "id", "", "Zameen listing external id (alternative to the positional argument)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}

// newOpenCmd prints (default) or launches the canonical Zameen listing URL.
// pp:data-source local
func newOpenCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var launch bool
	cmd := &cobra.Command{
		Use:     "open <external-id>",
		Short:   "Print (or --launch) the Zameen web URL for a stored listing",
		Example: "  zameen-pp-cli open 54194131 --launch",
		// No error path: any id yields a constructable Zameen URL, so a bogus
		// id cannot be distinguished from a valid one without inventing
		// semantics. Skip dogfood's error_path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would print the listing URL")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("external-id is required"))
			}
			id := strings.TrimSpace(args[0])
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			var url string
			listings, err := loadStoredListings(cmd.Context(), dbPath)
			if err == nil {
				for _, l := range listings {
					if l.ExternalId == id {
						url = l.Url
						break
					}
				}
			}
			if url == "" {
				// Fall back to a Zameen search deep-link for the id.
				url = fmt.Sprintf("%s/Property/-%s.html", zameen.BaseURL, id)
			}
			if !launch || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would launch:", url)
				return nil
			}
			if lErr := launchURL(url); lErr != nil {
				fmt.Fprintln(cmd.OutOrStdout(), url)
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "launched:", url)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the URL in your default browser")
	return cmd
}

func launchURL(url string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin, args = "open", []string{url}
	case "windows":
		bin, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		bin, args = "xdg-open", []string{url}
	}
	// #nosec G204 -- bin is constrained to one of three hardcoded literals by the
	// runtime.GOOS switch above (open/rundll32/xdg-open); the URL is passed as a
	// separate argv element, not through a shell, so no command injection is possible.
	return exec.Command(bin, args...).Start()
}
