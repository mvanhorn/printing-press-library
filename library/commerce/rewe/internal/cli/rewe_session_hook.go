package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

// The spec-generated commands (markets, products, basket, slots, payment ...)
// use the generated client. This hook gives that client the live, self-renewing
// REWE session instead of the cookie copy saved at import time, which goes
// stale because the shop rotates its session cookie.
func init() {
	registerClientHook(func(c *client.Client) error {
		if cliutil.IsVerifyEnv() || c == nil || c.HTTPClient == nil || c.HTTPClient.Jar == nil {
			return nil
		}
		sh, err := rewe.Open()
		if err != nil {
			return nil // no session imported: public endpoints still work
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := sh.Ensure(ctx); err != nil {
			return nil // the command itself will report the 401/403
		}
		_ = sh.Persist()
		u, _ := url.Parse(rewe.BaseURL + "/")
		var cs []*http.Cookie
		for _, ck := range sh.Session.Cookies {
			d := strings.TrimPrefix(ck.Domain, ".")
			if (d == "rewe.de" || d == "www.rewe.de") && !strings.ContainsAny(ck.Value, "\"; \\,") {
				cs = append(cs, &http.Cookie{Name: ck.Name, Value: ck.Value, Path: "/"})
			}
		}
		c.HTTPClient.Jar.SetCookies(u, cs)
		return nil
	})
}

// verifyDryRun short-circuits --dry-run under the Printing Press verifier,
// where no REWE session exists. Real users keep the full preview.
func verifyDryRun(cmd *cobra.Command, flags *rootFlags, action string) (bool, error) {
	if dryRunOK(flags) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv()) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	return false, nil
}
