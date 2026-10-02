// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacapture"
	"github.com/spf13/cobra"
	"path/filepath"
	"time"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		// Revalidate the scoped airport endpoint in the caller's chosen home.
		// This makes doctor proof portable across isolated verifier/MCP homes.
		doctor, _, findErr := root.Find([]string{"doctor"})
		if findErr == nil && doctor != root {
			previous := doctor.RunE
			doctor.RunE = func(cmd *cobra.Command, args []string) error {
				if !dryRunOK(f) {
					if _, e := traveloka.NewClient(travelokaSessionFile(cmd)); e == nil {
						ctx, cancel := boundCtx(cmd.Context(), f)
						defer cancel()
						if e = travelokaValidateSession(ctx, cmd, f, travelokaSessionFile(cmd)); e != nil {
							return travelokaFail(cmd, f, e)
						}
					}
				}
				return previous(cmd, args)
			}
		}

		auth, _, e := root.Find([]string{"auth"})
		if e != nil {
			return
		}
		auth.AddCommand(newTravelokaImportCmd(f), newTravelokaCaptureCmd(f))
		// Cookie-only generic login cannot supply operation profiles; keep old
		// spellings discoverable as actionable unsupported operations.
		for _, name := range []string{"login", "refresh", "set-token"} {
			old, _, err := auth.Find([]string{name})
			if err != nil || old == auth {
				continue
			}
			old.Hidden = true
			if old.Annotations == nil {
				old.Annotations = map[string]string{}
			}
			old.Annotations["mcp:hidden"] = "true"
			old.RunE = func(cmd *cobra.Command, args []string) error {
				if dryRunOK(f) {
					return writeDryRun(cmd.OutOrStdout(), f, "use scoped Traveloka session setup")
				}
				return travelokaFail(cmd, f, &traveloka.APIError{Code: "UNSUPPORTED_OPERATION", Message: "Cookie-only login is insufficient for Traveloka consumer requests; use auth capture --launch --timeout 2m or auth import-session with scoped cookie and request-profile JSON files"})
			}
		}

	})
}
func newTravelokaImportCmd(f *rootFlags) *cobra.Command {
	var cookies, requests, out string
	c := &cobra.Command{Use: "import-session", Short: "Import scoped Traveloka cookie/request JSON and verify HTTP access", Example: "  traveloka-pp-cli auth import-session --cookies-file /private/tmp/traveloka-cookies.json --requests-file /private/tmp/traveloka-requests.json --output-file /private/tmp/traveloka-session.json --agent", Annotations: travelokaAnnotations("live", "--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "import private Traveloka session")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named session setup flags; positional arguments are unsupported"})
		}
		if e := travelokaMode(f, "live"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if cookies == "" || requests == "" {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--cookies-file and --requests-file are required Traveloka-only JSON paths"})
		}
		if out == "" {
			out = travelokaDefaultSessionFile()
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		info, e := traveloka.ImportSession(cookies, requests, out)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = travelokaValidateSession(ctx, cmd, f, out); e != nil {
			return travelokaFail(cmd, f, e)
		}
		return f.printJSON(cmd, map[string]any{"session": info, "http_verified": true, "validation_method": "POST", "validation_path": "/api/v2/airport/search-nexus", "next_action": "Use --session-file " + out + " or set TRAVELOKA_SESSION_FILE to this private path"})
	}}
	c.Flags().StringVar(&cookies, "cookies-file", "", "Traveloka-only browser-use cookie array JSON file")
	c.Flags().StringVar(&requests, "requests-file", "", "Captured allowlisted Traveloka POST request-profile array JSON file")
	c.Flags().StringVar(&out, "output-file", "", "Private normalized session destination; defaults to CLI state directory")
	return c
}
func newTravelokaCaptureCmd(f *rootFlags) *cobra.Command {
	var launch bool
	var backend, outdir, out string
	c := &cobra.Command{Use: "capture", Short: "Capture an isolated Traveloka guest session for direct HTTP replay", Example: "  traveloka-pp-cli auth capture --launch --timeout 2m --agent", Annotations: travelokaAnnotations("live", "--dry-run"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "plan scoped normal-browser capture")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named session setup flags; positional arguments are unsupported"})
		}
		if e := travelokaMode(f, "live"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		shop, e := travelokaShopper(cmd)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		today := time.Now().UTC()
		if out == "" {
			out = travelokaDefaultSessionFile()
		}
		if outdir == "" {
			outdir = filepath.Join(filepath.Dir(out), "capture")
		}
		o := travelokacapture.Options{Backend: backend, OutputDir: outdir, Market: shop.Market, Locale: shop.Locale, Currency: shop.Currency, Depart: today.AddDate(0, 0, 30).Format("2006-01-02"), ReturnDate: today.AddDate(0, 0, 37).Format("2006-01-02"), CheckIn: today.AddDate(0, 0, 60).Format("2006-01-02"), CheckOut: today.AddDate(0, 0, 62).Format("2006-01-02")}
		urls, e := travelokacapture.SourceURLs(o)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if !launch {
			return f.printJSON(cmd, map[string]any{"action": "capture_plan", "urls": urls, "scope": "isolated anonymous Traveloka-only cookies and read-only flight/hotel profiles", "session_file": out, "next_action": "Run auth capture --launch --timeout 2m; an installed browser-use backend is required"})
		}
		if cliutil.IsAnyHarness() {
			return writeHarnessRefusal(cmd.OutOrStdout(), f, "launch isolated guest browser for session capture")
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		captured, e := travelokacapture.Capture(ctx, o)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		info, e := traveloka.ImportSession(captured.CookiesFile, captured.RequestsFile, out)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = travelokaValidateSession(ctx, cmd, f, out); e != nil {
			return travelokaFail(cmd, f, e)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "Traveloka guest browser closed; scoped session verified over HTTP.")
		return f.printJSON(cmd, map[string]any{"session": info, "capture": captured, "http_verified": true, "next_action": "Use --session-file " + out + " or set TRAVELOKA_SESSION_FILE to this private path"})
	}}
	c.Flags().BoolVar(&launch, "launch", false, "Opt in to opening an isolated normal guest browser for capture")
	c.Flags().StringVar(&backend, "browser-use", "", "Path to an already installed browser-use executable")
	c.Flags().StringVar(&outdir, "output-dir", "", "Private directory for scoped cookie and request JSON export")
	c.Flags().StringVar(&out, "output-file", "", "Private normalized session destination; defaults to CLI state directory")
	return c
}
