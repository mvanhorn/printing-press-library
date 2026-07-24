// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/ti/internal/ticompliance"
)

// resolveCookiesPath applies the --cookies flag with the TI_COOKIES_FILE env
// var as fallback.
func resolveCookiesPath(flag string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv("TI_COOKIES_FILE")
}

type authCheckView struct {
	Alive     bool      `json:"alive"`
	Cookies   int       `json:"cookies_injected"`
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func newNovelAuthCheckCmd(flags *rootFlags) *cobra.Command {
	var flagCookies string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Probe whether the deposited myTI cookie snapshot still authenticates, without running an extraction.",
		Long: strings.TrimSpace(`
Injects the cookie snapshot into a fresh Browserbase session and opens the
Material Content home page. Alive means the gated commands (part ratings,
part fmd) will work; stale means trigger a fresh human-in-the-loop login and
re-deposit the snapshot. The CLI never logs in itself.`),
		Example: strings.Trim(`
  ti-pp-cli auth check --cookies ./ti-cookies.json --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would inject cookies and probe materialcontent auth")
				return nil
			}
			path := resolveCookiesPath(flagCookies)
			if path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cookies <file> (or TI_COOKIES_FILE) is required"))
			}
			cookies, err := ticompliance.LoadCookies(path)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			b, err := ticompliance.NewBrowser(false)
			if err != nil {
				return err
			}
			defer b.Close(ctx)
			view := authCheckView{Cookies: len(cookies), CheckedAt: time.Now().UTC()}
			if err := b.InjectCookies(ctx, cookies); err != nil {
				return err
			}
			if err := b.AuthCheck(ctx); err != nil {
				if errors.Is(err, ticompliance.ErrAuthRedirect) {
					view.Alive = false
					view.Detail = err.Error()
					if perr := printJSONFiltered(cmd.OutOrStdout(), view, flags); perr != nil {
						return perr
					}
					return fmt.Errorf("cookies are stale: trigger a fresh HIL login and re-deposit the snapshot")
				}
				return err
			}
			view.Alive = true
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCookies, "cookies", "", "path to the deposited myTI cookie snapshot JSON (fallback: TI_COOKIES_FILE)")
	return cmd
}
