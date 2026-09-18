// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Wiring for the hand-authored Garmin auth flow: it replaces the generated
// `auth setup` and `auth status` prose (which describes pasting an API token
// Garmin does not issue) and the generated `auth logout` (which leaves this
// CLI's identity sidecar behind), and installs the pre-call token refresh.
// Markerless on purpose, like garmin_auth.go.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(configureGarminAuth)
	registerClientHook(installGarminFreshToken)
}

// configureGarminAuth swaps the generated credential-paste helpers for the
// commands this CLI's auth actually has. `set-token` survives: it is a real
// escape hatch for an access token obtained elsewhere, just not the path
// anyone should reach for, so it stays wired and hidden.
func configureGarminAuth(root *cobra.Command, flags *rootFlags) {
	for _, cmd := range root.Commands() {
		if cmd.Name() != "auth" {
			continue
		}
		cmd.Short = "Sign in to Garmin Connect and inspect this home's stored account"
		for _, child := range cmd.Commands() {
			switch child.Name() {
			case "setup", "status", "logout":
				cmd.RemoveCommand(child)
			case "set-token":
				child.Hidden = true
				child.Short = "Store an access token obtained elsewhere (advanced; prefer `auth login`)"
				garminClearIdentityAfter(child, flags)
			}
		}
		cmd.AddCommand(newGarminAuthSetupCmd(flags))
		cmd.AddCommand(newGarminAuthStatusCmd(flags))
		cmd.AddCommand(newGarminAuthLogoutCmd(flags))
		return
	}
}

// garminClearIdentityAfter wraps a command that installs a credential this CLI
// cannot attribute to an account. Only `auth login` and `auth status --verify`
// assert which Garmin account a chain belongs to, so a credential stored by
// any other route must not inherit the account a previous login recorded for
// this home: that is how one household member's archive would silently
// accumulate another's rows.
func garminClearIdentityAfter(child *cobra.Command, flags *rootFlags) {
	inner := child.RunE
	if inner == nil {
		return
	}
	child.RunE = func(cmd *cobra.Command, args []string) error {
		if err := inner(cmd, args); err != nil {
			return err
		}
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return nil
		}
		if err := garminRemoveIdentity(cfg.Path); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"warning: could not clear this home's recorded account (%v); run `garmin-pp-cli auth status --verify` to re-record it\n", err)
		}
		return nil
	}
}

// garminCredentialIsAsserted reports whether the credential this run will
// actually present is the one the identity sidecar describes. `auth login` and
// `auth status --verify` are the only two paths that assert an account, and
// both write the sidecar beside the chain they asserted. A credential supplied
// through the environment is a different chain that nothing asserted, so the
// sidecar's email must not be reported as this home's account.
func garminCredentialIsAsserted(cfg *config.Config, verifiedNow bool) bool {
	if verifiedNow {
		return true
	}
	return cfg != nil && !strings.HasPrefix(cfg.AuthSource, "env:")
}

// garminUnassertedCredentialNote names the unasserted source and the one
// command that resolves it.
func garminUnassertedCredentialNote(cfg *config.Config) string {
	source := "an external credential"
	if cfg != nil && strings.TrimSpace(cfg.AuthSource) != "" {
		source = cfg.AuthSource
	}
	return "the credential in use comes from " + source +
		", which no login asserted; run `garmin-pp-cli auth status --verify` to confirm which account it belongs to"
}

// installGarminFreshToken refreshes an about-to-expire token before the
// command that built this client makes its first call. os.Stderr is the
// writer on purpose: a refresh that failed while the access token is still
// usable downgrades to a warning, and a warning nobody can read is the same
// as no warning at all. The hook also runs under internal/mcp, where stderr
// is the MCP server's log stream.
func installGarminFreshToken(c *client.Client) error {
	if c == nil || c.Config == nil {
		return nil
	}
	return garminEnsureFreshToken(c.Config, os.Stderr, time.Now())
}

func newGarminAuthSetupCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "setup",
		Short:   "Explain how to connect a Garmin account to this CLI",
		Example: "  garmin-pp-cli auth setup",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			fmt.Fprintln(w, "Garmin issues no personal API key, so there is nothing to copy and paste.")
			fmt.Fprintln(w, "Connect an account with one browser sign-in:")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  garmin-pp-cli auth login --email <your Garmin account email>")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "That opens Garmin's own sign-in page, catches the redirect on a loopback port, and")
			fmt.Fprintln(w, "stores the token pair it gets back. Your password never reaches this CLI.")
			fmt.Fprintln(w, "The stored token is discarded unless Garmin confirms the account you named.")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "One Garmin account per home. For a second household account, in this order:")
			fmt.Fprintln(w, "  1. sign out of Garmin in the browser")
			fmt.Fprintln(w, "  2. GARMIN_HOME=<that account's dir> garmin-pp-cli auth login --email <that account>")
			fmt.Fprintln(w, "  3. confirm the account email the login prints is the one you meant")
			fmt.Fprintln(w, "  4. sign out of Garmin in the browser again")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Clearing browser cookies is never required, and this CLI never attempts it.")
			fmt.Fprintln(w, "")
			fmt.Fprintf(w, "This home: %s\n", garminHomeRungLine())
			fmt.Fprintf(w, "Config:    %s\n", cfg.Path)
			return nil
		},
	}
}

func newGarminAuthStatusCmd(flags *rootFlags) *cobra.Command {
	var verify bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show which Garmin account this home is signed in as",
		Long: "Show which Garmin account this home is signed in as.\n\n" +
			"Reads local state only. Pass --verify to make one call to Garmin, confirm the stored\n" +
			"tokens still work, and re-record the account they belong to.",
		Example: "  garmin-pp-cli auth status\n  garmin-pp-cli auth status --verify",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			// Migrate before loading: a spike token file at the config-kind
			// path makes config.Load fail outright, not warn.
			if _, err := garminMigrateResolved(flags.configPath, cmd.ErrOrStderr()); err != nil {
				return configErr(err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			identity, _ := garminLoadIdentity(cfg.Path)
			stored := garminStoredTokens(cfg)
			authed := cfg.AuthHeader() != ""
			refusals := cfg.CredentialRefusalSummaries()

			verified := false
			var verifyErr error
			if verify && authed {
				verified, verifyErr = garminVerifyStored(cmd.Context(), cfg, identity, cmd.ErrOrStderr())
				// Verifying can rotate the chain: garminVerifyStored runs the
				// pre-call refresh, which writes a new access token into cfg
				// and a fresh refresh_expiry into the sidecar. Both have to be
				// re-read or the command reports the chain it just replaced.
				// Re-read even when the verify failed: the refresh can succeed
				// and the identity call that follows it fail.
				if reloaded, loadErr := garminLoadIdentity(cfg.Path); loadErr == nil && reloaded != nil {
					identity = reloaded
				}
				stored = garminStoredTokens(cfg)
			}

			needsRefresh := false
			if stored != nil {
				needsRefresh = garminNeedsRefresh(stored.AccessToken, stored.TokenExpiry, time.Now())
			}
			homeRung, homeDir, homeSource := garminHomeRung()

			if flags.asJSON {
				out := map[string]any{
					"authenticated": authed,
					"verified":      verified,
					"source":        cfg.AuthSource,
					"config":        cfg.Path,
					"home_rung":     homeRung,
					"home_dir":      homeDir,
					"home_source":   homeSource,
					"needs_refresh": needsRefresh,
				}
				if identity != nil {
					// The sidecar records the account the last `auth login`
					// (or `auth status --verify`) asserted for this home. An
					// env-supplied credential is a different chain that no
					// code path asserts, so it is never reported as this
					// home's account without saying so.
					if garminCredentialIsAsserted(cfg, verified) {
						out["email"] = identity.Email
					} else {
						out["email_last_asserted"] = identity.Email
						out["email_matches_credential_in_use"] = false
						out["credential_note"] = garminUnassertedCredentialNote(cfg)
					}
					out["profile_id"] = identity.ProfileID
					out["garmin_guid_present"] = identity.GarminGUID != ""
					out["refresh_expiry"] = garminFormatTime(identity.RefreshExpiry)
				}
				if stored != nil {
					out["token_expiry"] = garminFormatTime(garminEffectiveExpiry(stored))
				}
				if len(refusals) > 0 {
					out["credential_refused"] = true
					out["credential_refusals"] = refusals
				}
				if verifyErr != nil {
					out["verify_error"] = verifyErr.Error()
				}
				if printErr := printJSONFiltered(w, out, flags); printErr != nil {
					return printErr
				}
				if !authed && len(refusals) > 0 {
					return authErr(cfg.CredentialRefusalError())
				}
				if !authed {
					return authErr(errors.New("no credentials configured"))
				}
				if verifyErr != nil {
					return authErr(verifyErr)
				}
				return nil
			}

			if !authed && len(refusals) > 0 {
				fmt.Fprintln(w, red("Credentials present but refused"))
				for _, refusal := range refusals {
					fmt.Fprintf(w, "  %s\n", refusal)
				}
				return authErr(cfg.CredentialRefusalError())
			}
			if !authed {
				fmt.Fprintln(w, red("Not signed in"))
				fmt.Fprintf(w, "  Home:   %s\n", garminHomeRungLine())
				fmt.Fprintf(w, "  Config: %s\n", cfg.Path)
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Run: garmin-pp-cli auth login --email <your Garmin account email>")
				return authErr(errors.New("no credentials configured"))
			}

			if verified {
				fmt.Fprintln(w, green("Signed in (confirmed with Garmin just now)"))
			} else {
				fmt.Fprintln(w, green("Signed in (stored tokens; not re-checked with Garmin)"))
			}
			if identity != nil && identity.Email != "" && garminCredentialIsAsserted(cfg, verified) {
				fmt.Fprintf(w, "  Account:        %s\n", identity.Email)
			} else if identity != nil && identity.Email != "" {
				fmt.Fprintf(w, "  Last asserted:  %s (NOT the credential in use)\n", identity.Email)
				fmt.Fprintf(w, "  Account:        unconfirmed — %s\n", garminUnassertedCredentialNote(cfg))
			} else {
				fmt.Fprintln(w, "  Account:        not recorded — run `garmin-pp-cli auth status --verify`")
			}
			if identity != nil && identity.ProfileID != "" {
				fmt.Fprintf(w, "  Profile:        %s\n", identity.ProfileID)
			}
			fmt.Fprintf(w, "  Home:           %s\n", garminHomeRungLine())
			fmt.Fprintf(w, "  Config:         %s\n", cfg.Path)
			fmt.Fprintf(w, "  Source:         %s\n", cfg.AuthSource)
			if stored != nil {
				fmt.Fprintf(w, "  Token expires:  %s\n", garminFormatTime(garminEffectiveExpiry(stored)))
			}
			if identity != nil && !identity.RefreshExpiry.IsZero() {
				fmt.Fprintf(w, "  Refresh until:  %s\n", garminFormatTime(identity.RefreshExpiry))
			}
			fmt.Fprintf(w, "  Needs refresh:  %t\n", needsRefresh)
			if verifyErr != nil {
				return authErr(verifyErr)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&verify, "verify", false, "Make one call to Garmin to confirm the stored tokens and the account they belong to")
	return cmd
}

func newGarminAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Clear this home's Garmin tokens and the account it was bound to",
		Example: "  garmin-pp-cli auth logout",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := garminMigrateResolved(flags.configPath, cmd.ErrOrStderr()); err != nil {
				return configErr(err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			err = garminWithAuthLock(cfg.Path, func() error {
				if err := cfg.ClearTokens(); err != nil {
					return fmt.Errorf("clearing tokens: %w", err)
				}
				return garminRemoveIdentity(cfg.Path)
			})
			if err != nil {
				return configErr(err)
			}
			envStillSet := garminAuthEnvStillSet()
			if flags.asJSON {
				out := map[string]any{"cleared": true, "config": cfg.Path}
				if envStillSet != "" {
					out["note"] = envStillSet + " env var is still set"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Signed out. Tokens and the recorded account are cleared for this home.")
			if envStillSet != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Note: %s is still set in this shell and will still authenticate calls.\n", envStillSet)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Garmin was not asked to revoke the token; sign out in your browser too if you are done there.")
			return nil
		},
	}
}

// garminVerifyStored makes one authenticated call and records what it learns.
// It is the only place `auth status` touches the network.
func garminVerifyStored(ctx context.Context, cfg *config.Config, identity *garminIdentity, stderr io.Writer) (bool, error) {
	stored := garminStoredTokens(cfg)
	if stored == nil || stored.AccessToken == "" {
		return false, errors.New("no stored access token to verify")
	}
	if err := garminEnsureFreshToken(cfg, stderr, time.Now()); err != nil {
		return false, err
	}
	stored = garminStoredTokens(cfg)
	ep := garminEndpointsFor(cfg, garminDomainForConfig(cfg))
	hc := garminHTTPClient()
	callCtx, cancel := context.WithTimeout(ctx, garminHTTPTimeout)
	defer cancel()

	var pi garminPersonalInformation
	if err := garminGetJSON(callCtx, hc, ep, stored.AccessToken, garminPersonalInformationPath, &pi); err != nil {
		return false, err
	}
	email := strings.TrimSpace(pi.UserInfo.Email)
	if email == "" {
		return false, errors.New("Garmin's identity response carried no account email")
	}
	// The mismatch check reads the identity this function was handed, which is
	// correct here: a refresh only ever rewrites refresh_expiry in the sidecar
	// (garminPersistTokens), never the account it records.
	if identity != nil && identity.Email != "" && !strings.EqualFold(identity.Email, email) {
		return false, fmt.Errorf(
			"this home records %s but the stored token authenticates %s; run `garmin-pp-cli auth logout` then sign in again",
			identity.Email, email)
	}

	// The profile read is a second network call and stays outside the auth
	// lock: holding the lock across a Garmin round trip (plus the pacing
	// sleep) would stall a concurrent refresh for no reason.
	profileID, displayName := "", ""
	if identity == nil || identity.ProfileID == "" {
		time.Sleep(300 * time.Millisecond)
		var social garminSocialProfile
		if err := garminGetJSON(callCtx, hc, ep, stored.AccessToken, garminSocialProfilePath, &social); err == nil {
			profileID = social.ProfileID.String()
			displayName = social.DisplayName
		}
	}

	err := garminWithAuthLock(cfg.Path, func() error {
		// Re-read the sidecar under the lock rather than editing the copy
		// this function was handed. garminEnsureFreshToken above may have
		// refreshed the chain, and a refresh writes a fresh 30-day
		// refresh_expiry into that same file; the caller's copy predates the
		// write, so saving it would revert refresh_expiry to the login-time
		// value and "Refresh until" would never move.
		next, loadErr := garminLoadIdentity(cfg.Path)
		if loadErr != nil || next == nil {
			next = &garminIdentity{Domain: ep.Domain, ClientID: stored.ClientID}
			if identity != nil {
				clone := *identity
				next = &clone
			}
		}
		next.Email = email
		next.Domain = ep.Domain
		if guid := garminJWTString(stored.AccessToken, "garmin_guid"); guid != "" {
			next.GarminGUID = guid
		}
		if stored.ClientID != "" {
			next.ClientID = stored.ClientID
		}
		if next.ProfileID == "" && profileID != "" {
			next.ProfileID = profileID
			next.DisplayName = displayName
		}
		next.AssertedAt = time.Now().UTC()
		return garminSaveIdentity(cfg.Path, next)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// garminEffectiveExpiry prefers the token's own exp claim over the stored
// value: the claim is what Garmin will actually enforce.
func garminEffectiveExpiry(t *garminTokens) time.Time {
	if exp := garminJWTExpiry(t.AccessToken); !exp.IsZero() {
		return exp
	}
	return t.TokenExpiry
}

func garminAuthEnvStillSet() string {
	for _, name := range []string{"GARMIN_ACCESS_TOKEN", "GARMIN_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return name
		}
	}
	return ""
}

// garminHomeRung reports the rung the config directory resolved on, which is
// what tells a caller whether --home, GARMIN_HOME or the platform default is
// in effect.
func garminHomeRung() (rung, dir, source string) {
	resolution, err := cliutil.ResolveKindDir(cliutil.PathKindConfig)
	if err != nil {
		return "unknown", "", ""
	}
	return resolution.Rung, resolution.Dir, resolution.Source
}

func garminHomeRungLine() string {
	rung, dir, source := garminHomeRung()
	if dir == "" {
		return rung
	}
	if source != "" && source != rung {
		return fmt.Sprintf("%s (%s via %s)", dir, rung, source)
	}
	return fmt.Sprintf("%s (%s)", dir, rung)
}
