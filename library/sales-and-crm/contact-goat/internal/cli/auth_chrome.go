// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/chromecookies"
	"github.com/spf13/cobra"
)

// appConfigDir is the directory under ~/.config where this CLI stores its
// cookie files. Keeping it as a var makes it overridable in tests.
var appConfigDir = "contact-goat-pp-cli"

// supportedCookieServices enumerates the --service values accepted by
// `auth login --chrome`. Designed to grow: add a case to extractCookies
// and an entry here.
var supportedCookieServices = map[string]struct{}{
	"happenstance": {},
}

// newAuthLoginChromeCmd builds the `auth login --chrome` subcommand. It
// reads the user's Chrome cookie jar, extracts cookies for the requested
// service, and writes them to ~/.config/<app>/cookies-<service>.json
// (mode 0600).
func newAuthLoginChromeCmd(flags *rootFlags) *cobra.Command {
	var service string
	var useChrome bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in by importing cookies from your local Chrome browser",
		Long: `Log in to a service (currently: happenstance) by reading its cookies from
your local Chrome cookie jar. macOS-only: the cookie jar is AES-encrypted
and the decryption key lives in the Keychain — we call 'security
find-generic-password' under the hood, which will show the standard
Keychain access prompt the first time.

Cookies are written to ~/.config/contact-goat-pp-cli/cookies-<service>.json
with mode 0600. The file contains the plaintext cookie values — treat it
like any other credential file.`,
		Example: `  contact-goat-pp-cli auth login --chrome --service happenstance
  contact-goat-pp-cli --no-input auth login --chrome --service happenstance`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !useChrome {
				return usageErr(errors.New("this subcommand currently requires --chrome (no other login methods are implemented yet)"))
			}
			if service == "" {
				service = "happenstance"
			}
			if _, ok := supportedCookieServices[service]; !ok {
				return usageErr(fmt.Errorf("unsupported --service %q (supported: happenstance)", service))
			}

			// Consent prompt — Chrome cookies are sensitive. Skip in
			// --no-input and --yes modes (agents can't answer anyway).
			if !flags.noInput && !flags.yes {
				fmt.Fprintf(cmd.OutOrStdout(),
					"This will read Chrome's encrypted cookie jar and extract cookies for %s.\n"+
						"macOS will prompt once for Keychain access to decrypt them.\n"+
						"Continue? [y/N]: ", service)
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				ans := strings.ToLower(strings.TrimSpace(line))
				if ans != "y" && ans != "yes" {
					return errors.New("aborted by user")
				}
			}

			cookies, err := extractCookies(service)
			if err != nil {
				if errors.Is(err, chromecookies.ErrUnsupportedPlatform) {
					return fmt.Errorf("reading Chrome cookies is only implemented on macOS today. Linux/Windows support is on the roadmap — see the README")
				}
				if errors.Is(err, chromecookies.ErrKeychainUnavailable) {
					return authErr(fmt.Errorf("could not read the Chrome Safe Storage password from Keychain. Make sure Chrome is installed and you allowed the Keychain prompt"))
				}
				return fmt.Errorf("extracting Chrome cookies: %w", err)
			}
			if len(cookies) == 0 {
				return fmt.Errorf("no cookies found for %s — are you signed in to https://happenstance.ai in Chrome?", service)
			}

			path, err := chromecookies.DefaultCookieFilePath(appConfigDir, service)
			if err != nil {
				return err
			}
			if err := chromecookies.WriteCookieFile(path, service, cookies); err != nil {
				return fmt.Errorf("saving cookies: %w", err)
			}

			// Report without leaking values. We do surface cookie names
			// because those are not secrets (the names are documented
			// in Clerk's docs) but NEVER values or JWT bodies.
			names := uniqueCookieNames(cookies)
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"service":      service,
					"path":         path,
					"cookie_count": len(cookies),
					"cookie_names": names,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %d cookies for %s to %s\n", len(cookies), service, path)
			fmt.Fprintf(cmd.OutOrStdout(), "Cookies: %s\n", strings.Join(names, ", "))
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'contact-goat-pp-cli doctor' to verify session expiry.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&useChrome, "chrome", false, "Import cookies from the local Chrome browser (required)")
	cmd.Flags().StringVar(&service, "service", "happenstance", "Service to log in to (supported: happenstance)")
	return cmd
}

// extractCookies dispatches to the per-service cookie extractor. Adding a
// new service means adding a case here.
func extractCookies(service string) ([]chromecookies.Cookie, error) {
	switch service {
	case "happenstance":
		return chromecookies.ReadHappenstanceCookies()
	default:
		return nil, fmt.Errorf("no extractor registered for service %q", service)
	}
}

func uniqueCookieNames(cookies []chromecookies.Cookie) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range cookies {
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		out = append(out, c.Name)
	}
	return out
}
