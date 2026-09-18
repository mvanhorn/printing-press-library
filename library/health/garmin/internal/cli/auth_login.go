// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: the Garmin loopback SSO sign-in. Implemented in place from
// the printing-press novel-command scaffold; `generate --force` preserves an
// implemented body. Shared auth machinery lives in garmin_auth.go.
// pp:data-source live
// pp:client-call — the ticket exchange and the identity assertion call
// Garmin's own token and Connect endpoints through garminHTTPClient rather
// than through the generated internal/client, so the string heuristics cannot
// see the call. garminExchangeServiceTicket (garmin_auth.go) is the call.

package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/config"
	"github.com/spf13/cobra"
)

func newNovelAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		flagEmail    string
		flagNoLogout bool
		flagTimeout  time.Duration
		flagDomain   string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in through Garmin's own page in your browser and store this account's tokens",
		Long: "Sign in through Garmin's own page in your browser and store this account's tokens.\n\n" +
			"Garmin issues no personal API token, so this is the only way in. The command signs the\n" +
			"browser out of Garmin first, opens Garmin's sign-in page, catches the one-time ticket on a\n" +
			"loopback port, exchanges it for a token pair, and then asks Garmin which account it just\n" +
			"authenticated. If that account is not the address you passed to --email, nothing is stored.\n\n" +
			"Your password is never seen, stored, or transmitted by this CLI.\n\n" +
			"One Garmin account per home: give each additional household account its own GARMIN_HOME\n" +
			"(or --home) and run this command once inside it.",
		Example: "  garmin-pp-cli auth login --email you@example.com\n" +
			"  GARMIN_HOME=~/.local/share/garmin-homes/second garmin-pp-cli auth login --email other@example.com",
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "false", "pp:data-source": "live"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth login")
			}
			email := strings.TrimSpace(flagEmail)
			if email == "" {
				return authErr(errors.New("--email is required: it pre-fills Garmin's form and is the account this command asserts you signed in as"))
			}
			if flagTimeout <= 0 {
				return authErr(fmt.Errorf("--timeout must be positive, got %s", flagTimeout))
			}
			if err := garminCheckDomain(flagDomain); err != nil {
				return authErr(err)
			}
			return runGarminLogin(cmd, flags, garminLoginOptions{
				Email:    email,
				Domain:   flagDomain,
				NoLogout: flagNoLogout,
				Timeout:  flagTimeout,
			})
		},
	}
	cmd.Flags().StringVar(&flagEmail, "email", "", "The Garmin account email to sign in as; the stored token is discarded if Garmin reports a different account")
	cmd.Flags().BoolVar(&flagNoLogout, "no-logout", false, "Skip the browser sign-out step (only for a browser you know has no Garmin session)")
	cmd.Flags().DurationVar(&flagTimeout, "timeout", garminLoginTimeoutDefault, "How long to wait for the browser sign-in to come back")
	cmd.Flags().StringVar(&flagDomain, "domain", garminDomainGlobal, "Garmin deployment to sign in to (garmin.com or garmin.cn)")
	return cmd
}

// garminOpenURL is the browser-launch seam. It is a variable so the login
// choreography — which page opens before which prompt — can be asserted in a
// test without a browser. Production always holds openSetupURL.
var garminOpenURL = openSetupURL

// garminIsInteractive is the "is a person watching" seam. garminInteractive
// insists on a real char-device stdin, which a test cannot supply, so the
// prompt path would otherwise be untestable.
var garminIsInteractive = garminInteractive

type garminLoginOptions struct {
	Email    string
	Domain   string
	NoLogout bool
	Timeout  time.Duration
}

func runGarminLogin(cmd *cobra.Command, flags *rootFlags, opts garminLoginOptions) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	if _, err := garminMigrateResolved(flags.configPath, out); err != nil {
		return configErr(fmt.Errorf("migrating the existing token file: %w", err))
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	ep := garminEndpointsFor(cfg, opts.Domain)

	// A home is bound to one Garmin account. Signing a different account into
	// the same home would mix two people's data in one archive.
	priorIdentity, _ := garminLoadIdentity(cfg.Path)
	if priorIdentity != nil && priorIdentity.Email != "" && !strings.EqualFold(priorIdentity.Email, opts.Email) {
		return authErr(fmt.Errorf(
			"this home is already signed in as %s, not %s.\n"+
				"  One Garmin account per home. Either run `garmin-pp-cli auth logout` here first,\n"+
				"  or give the other account its own home with GARMIN_HOME=<dir> or --home <dir>",
			priorIdentity.Email, opts.Email))
	}

	// 1. Sign the browser out of Garmin. The session cookies live in the
	//    browser, so this has to be a browser navigation, not an HTTP call
	//    from here: fetching the URL from Go would clear nothing.
	if !opts.NoLogout {
		if err := garminBrowserSignOut(cmd, ep); err != nil {
			return authErr(err)
		}
	} else {
		fmt.Fprintln(out, "Skipping the browser sign-out step (--no-logout).")
	}

	// 2. Bind the loopback listener and open Garmin's sign-in page.
	state, err := garminNewState()
	if err != nil {
		return authErr(err)
	}
	cb, err := garminStartCallback(state)
	if err != nil {
		return authErr(err)
	}
	defer func() { _ = cb.Close() }()

	ssoURL := garminBuildSSOURL(ep, cb.URL, opts.Email)
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(out, "would launch: %s\n", ssoURL)
		return nil
	}
	fmt.Fprintf(out, "Opening Garmin's sign-in page for %s in your browser.\n", opts.Email)
	if err := garminOpenURL(ssoURL); err != nil {
		fmt.Fprintf(errOut, "could not open a browser automatically: %v\n", err)
		fmt.Fprintf(out, "Open this URL yourself to continue:\n  %s\n", ssoURL)
	}

	// 2b. Only now can anyone see what the sign-out actually achieved. The
	//     confirmation has to come after the sign-in page is on screen: asked
	//     between the two navigations it can only confirm the logout page,
	//     which says "Logged out!" whether or not Garmin then re-fills the
	//     form from a surviving SSO session. That autofilled form is the exact
	//     failure this prompt exists to catch (N131 §2 H1).
	//
	//     The deadline starts here rather than after the confirmation. The
	//     prompt is answered by a person, so it can be left outstanding
	//     indefinitely; bounding it with the same --timeout means someone who
	//     neither answers nor signs in gets the timeout error instead of a
	//     terminal that waits forever.
	//
	//     This deadline covers the two steps a person is inside — the
	//     confirmation prompt and the wait for the sign-in to come back — and
	//     nothing after them. Step 4 opens its own budget, so answering `y`
	//     one second before --timeout expires still leaves the ticket
	//     exchange a full HTTP timeout to spend.
	ctx, cancel := context.WithTimeout(cmd.Context(), opts.Timeout)
	defer cancel()
	ticket, err := garminConfirmEmptySignInForm(ctx, cmd, flags, opts, ep, cb)
	if err != nil {
		return authErr(err)
	}

	// 3. Wait for exactly one ticket, unless the sign-in already came back
	//    while the confirmation was outstanding. The listener stays open for
	//    the whole deadline: a callback that arrives at a closed port shows
	//    the browser an error page even though Garmin has signed the user in.
	if ticket == "" {
		fmt.Fprintf(out, "Waiting up to %s for the sign-in to come back.\n", opts.Timeout)
		select {
		case ticket = <-cb.Ticket():
		case <-ctx.Done():
			return authErr(garminLoginTimeoutError(opts.Timeout, cb.Rejected()))
		}
	}
	_ = cb.Close()

	// The person is done; everything below is machine-to-machine and gets its
	// own budget. Sharing the --timeout deadline would hand the single-use
	// ticket exchange whatever a slow answer left of it, which for an answer
	// near the default 10 minutes is close to nothing — and the failure would
	// surface as a raw context deadline rather than anything actionable.
	exchangeCtx, exchangeCancel := context.WithTimeout(cmd.Context(), garminHTTPTimeout)
	defer exchangeCancel()

	// 4. Exchange the single-use ticket. One attempt only: a failed exchange
	//    burns the ticket, so retrying would fail for a second reason.
	hc := garminHTTPClient()
	tokens, err := garminExchangeServiceTicket(exchangeCtx, hc, ep, ticket, cb.URL)
	if err != nil {
		return authErr(err)
	}

	// 5. Assert the identity before anything is written.
	assertedEmail, err := garminAssertEmail(exchangeCtx, hc, ep, tokens.AccessToken, opts.Email)
	if err != nil {
		if errors.Is(err, errGarminIdentityMismatch) {
			return authErr(fmt.Errorf(
				"signed in as %s, but you asked for %s. Nothing was stored.\n"+
					"  Sign out at %s in your browser, then run this command again",
				assertedEmail, opts.Email, ep.SSOLogout))
		}
		return authErr(fmt.Errorf("could not confirm which Garmin account was signed in, so nothing was stored: %w", err))
	}

	guid := garminJWTString(tokens.AccessToken, "garmin_guid")
	if priorIdentity != nil && priorIdentity.GarminGUID != "" && guid != "" && priorIdentity.GarminGUID != guid {
		return authErr(fmt.Errorf(
			"this home is bound to a different Garmin account id than the one that just signed in. Nothing was stored.\n" +
				"  Run `garmin-pp-cli auth logout` here first, or use a separate home for the other account"))
	}

	// profileId is a second read, so it is best-effort: a login that proved
	// the account email is complete without it.
	profileID, displayName := "", ""
	time.Sleep(300 * time.Millisecond)
	var social garminSocialProfile
	if err := garminGetJSON(exchangeCtx, hc, ep, tokens.AccessToken, garminSocialProfilePath, &social); err == nil {
		profileID = social.ProfileID.String()
		displayName = social.DisplayName
	}

	// 6. Persist under the lock so a concurrent refresh cannot interleave.
	//    The non-secret identity sidecar is written first and the token chain
	//    second, so the credential write is the commit point: see
	//    garminPersistLoginResult.
	err = garminPersistLoginResult(cfg.Path, flags.configPath, tokens, &garminIdentity{
		Email:         assertedEmail,
		GarminGUID:    guid,
		ProfileID:     profileID,
		DisplayName:   displayName,
		Domain:        ep.Domain,
		ClientID:      tokens.ClientID,
		RefreshExpiry: tokens.RefreshExpiry,
		AssertedAt:    time.Now().UTC(),
	})
	if err != nil {
		return configErr(fmt.Errorf("storing the Garmin tokens: %w", err))
	}

	if flags.asJSON {
		return printJSONFiltered(out, map[string]any{
			"authenticated": true,
			"verified":      true,
			"email":         assertedEmail,
			"profile_id":    profileID,
			"config":        cfg.Path,
			"token_expiry":  garminFormatTime(tokens.TokenExpiry),
		}, flags)
	}
	fmt.Fprintln(out, green("Signed in to Garmin Connect"))
	fmt.Fprintf(out, "  Account: %s\n", assertedEmail)
	if profileID != "" {
		fmt.Fprintf(out, "  Profile: %s\n", profileID)
	}
	fmt.Fprintf(out, "  Home:    %s\n", garminHomeRungLine())
	fmt.Fprintf(out, "  Token expires: %s\n", garminFormatTime(tokens.TokenExpiry))
	return nil
}

// garminSaveIdentityForPersist is the sidecar write garminPersistLoginResult
// performs. It is a variable so a test can fail that WRITE while the
// prior-bytes read succeeds: blocking the sidecar path on disk fails the READ
// instead, and the read guard returns before SaveTokens in either write order,
// so it pins nothing about the order this function exists to establish.
var garminSaveIdentityForPersist = garminSaveIdentity

// garminPersistLoginResult writes what a completed login produced: the
// non-secret identity sidecar and the token chain. It is a function rather
// than an inline closure so the order those two writes happen in can be driven
// by a test without a browser.
//
// ORDER IS THE POINT FOR A LOGIN. The sidecar goes first and SaveTokens is
// the commit point. The other order — credentials first, sidecar second —
// reports a failed login over credentials that have already changed: the
// caller is told nothing was stored, while the next command uses the new token
// with no record of which account issued it. Written this way, a sidecar
// failure returns before any credential is touched, and a credential failure
// puts the sidecar back the way it was. The sidecar holds no secret, so
// restoring it costs nothing; the credential chain is what an owner-gated
// browser login would be needed to recreate.
//
// garminPersistTokens and garminMigrateSpikeTokenFile keep the opposite
// order on purpose: there the chain is already rotated or already being
// migrated, so the credential write is the thing that must not be undone and
// the sidecar update is a non-secret follow-up.
//
// The sidecar also shares the config dir with config.toml, which is the second
// of SaveTokens' own two writes (credentials.toml in the DATA dir first,
// config.toml second). So writing the sidecar first exercises the config dir
// before SaveTokens has to, though it says nothing about the data dir.
//
// configPath is the already-resolved config file, which locates the lock;
// configFlag is the raw --config value, so config.Load inside the lock walks
// the same home ladder every other command does.
func garminPersistLoginResult(configPath, configFlag string, tokens *garminTokens, id *garminIdentity) error {
	return garminWithAuthLock(configPath, func() error {
		saveCfg, err := config.Load(configFlag)
		if err != nil {
			return err
		}
		// Raw bytes rather than garminLoadIdentity: what goes back on a
		// rollback is exactly what was there, not a re-encoding of it. Absent
		// is a valid prior state; anything else that stops the read is not,
		// and returning here leaves both files as they were.
		sidecarPath := garminIdentityPath(saveCfg.Path)
		prior, err := os.ReadFile(filepath.Clean(sidecarPath))
		hadPrior := true
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("reading the existing identity file %s: %w", sidecarPath, err)
			}
			hadPrior = false
			prior = nil
		}
		// SaveTokens is two writes, not one: credentials.toml in the DATA dir
		// first, then config.toml. A failure at the second leaves the NEW
		// chain on disk, and putting the old sidecar back there would make
		// this home name an account it no longer holds tokens for. Fingerprint
		// the credential file so the rollback can tell the two apart; a digest
		// rather than the bytes so no secret is held past the call.
		credsPath, credsPathErr := cliutil.CredentialsFilePath()
		credsBefore, credsKnown := garminCredentialsFingerprint(credsPath, credsPathErr)
		if err := garminSaveIdentityForPersist(saveCfg.Path, id); err != nil {
			return err
		}
		if err := saveCfg.SaveTokens(tokens.ClientID, "", tokens.AccessToken, tokens.RefreshToken, tokens.TokenExpiry); err != nil {
			after, afterKnown := garminCredentialsFingerprint(credsPath, credsPathErr)
			if !credsKnown || !afterKnown || after == credsBefore {
				// Not observed to have changed — including the case where the
				// credential path could not be read at all, which is also why
				// SaveCredentials could not write it. Roll the sidecar back.
				return garminRollbackIdentity(saveCfg.Path, prior, hadPrior, err)
			}
			return fmt.Errorf("%w; the token chain at %s had already been replaced when that failed, so this home now holds the account that just signed in and %s names it — run `garmin-pp-cli auth status --verify` to confirm, or `garmin-pp-cli auth logout` to clear both",
				err, credsPath, garminIdentityPath(saveCfg.Path))
		}
		return nil
	})
}

// garminCredentialsFingerprint digests the credential file so a failed
// SaveTokens can be told apart from a half-done one. Absent is a fingerprint
// of its own. The second return is false when the path could not be resolved or
// the credential file could not be read for any reason other than absence.
func garminCredentialsFingerprint(path string, pathErr error) ([32]byte, bool) {
	var zero [32]byte
	if pathErr != nil || path == "" {
		return zero, false
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return sha256.Sum256(nil), true
		}
		return zero, false
	}
	return sha256.Sum256(data), true
}

// garminRollbackIdentity puts the sidecar back after a failed credential
// write and returns the error the caller should report. When the put-back
// fails too, the returned error says what is actually on disk now rather than
// claiming a clean failure: the sidecar and the token chain disagree, and only
// the person at the terminal can decide what to do about it.
func garminRollbackIdentity(configPath string, prior []byte, hadPrior bool, cause error) error {
	path := garminIdentityPath(configPath)
	if !hadPrior {
		if err := garminRemoveIdentity(configPath); err != nil {
			return fmt.Errorf("%w; the identity file this login wrote at %s could not be removed either (%v), so it names an account this home holds no tokens for — delete it, or run `garmin-pp-cli auth login` again once the credential path is writable", cause, path, err)
		}
		return cause
	}
	if err := cliutil.AtomicWritePrivateFile(path, prior, 0o600, 0o700); err != nil {
		return fmt.Errorf("%w; the previous identity file at %s could not be put back either (%v), so it now names the account that just signed in while the stored token chain was not observed to change — run `garmin-pp-cli auth status --verify` to see which account that chain belongs to, and `garmin-pp-cli auth logout` only if it is not the account you just signed in as", cause, path, err)
	}
	return cause
}

// garminBrowserSignOut navigates the browser to Garmin's CAS logout route and,
// when someone is watching, waits for them to confirm the sign-in form came up
// empty. The route was probed live on 2026-09-07: HTTP 200, body
// "<p>Logged out!</p>". Neither reference client implements a logout at all,
// so this is verified by probe rather than borrowed.
func garminBrowserSignOut(cmd *cobra.Command, ep garminEndpoints) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Signing your browser out of Garmin first: %s\n", ep.SSOLogout)
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(out, "would launch: %s\n", ep.SSOLogout)
		return nil
	}
	if err := garminOpenURL(ep.SSOLogout); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "could not open a browser automatically: %v\n", err)
		fmt.Fprintf(out, "Open this URL yourself to sign out:\n  %s\n", ep.SSOLogout)
	}
	return nil
}

// garminConfirmEmptySignInForm asks the person at the terminal whether the
// sign-in page Garmin actually rendered is empty, and refuses to wait for a
// callback when it is not.
//
// Ordering is the whole point. The sign-out navigation and the sign-in
// navigation are two separate browser trips, and only the second one shows
// whether the SSO session really went away: a surviving session re-fills the
// form with the previous account, and submitting it signs in as that account
// instead. The identity assertion still refuses to store that token, so the
// cost of getting this wrong is a wasted round trip rather than a wrong
// account — but the round trip includes a real Garmin login, which is exactly
// what a household with two accounts should not have to repeat.
//
// The prompt is skipped in the three cases where nobody can answer it or
// nothing was signed out: --no-logout, --no-input (which --agent sets) or
// --yes, and a session with no terminal on both ends.
//
// The question is never allowed to outlive its own premise. The browser can
// complete the sign-in before anyone types, and by then the answer is moot:
// the form that mattered is gone from the screen and the ticket Garmin just
// issued is single-use and short-lived, so blocking on stdin only ages it out
// (observed 2026-09-12: a login sat at this prompt for ten minutes and the
// ticket expired). A ticket arriving on cb therefore wins the race, and ctx —
// the login's own --timeout — bounds the wait when neither happens.
//
// The returned string is the ticket that arrived early, or "" when the caller
// still has to wait for one. Empty is an unambiguous sentinel: the callback
// handler answers a ticket-less request with 204 and never pushes an empty
// string onto the channel (garminCallback.handle in garmin_auth.go).
func garminConfirmEmptySignInForm(ctx context.Context, cmd *cobra.Command, flags *rootFlags, opts garminLoginOptions, ep garminEndpoints, cb *garminCallback) (string, error) {
	out := cmd.OutOrStdout()
	if opts.NoLogout || flags.noInput || flags.yes || !garminIsInteractive(cmd) {
		fmt.Fprintf(out, "If the form is already filled in with another account, sign out at %s and run this again.\n", ep.SSOLogout)
		return "", nil
	}
	fmt.Fprintf(out, "Does the Garmin page show an EMPTY sign-in form? [y/N]: ")

	type garminFormAnswer struct {
		line string
		err  error
	}
	// The read moves off the main path so the ticket can win. When nobody
	// answers, this goroutine stays blocked on stdin for the life of the
	// process — deliberate, and free: the CLI exits as soon as the login
	// returns, and there is no portable way to interrupt a pending terminal
	// read. The channel is buffered so the send never blocks either.
	answers := make(chan garminFormAnswer, 1)
	go func() {
		line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		answers <- garminFormAnswer{line: line, err: err}
	}()

	continuing := func(ticket string) (string, error) {
		fmt.Fprintln(out, "Sign-in came back before the form check; continuing.")
		return ticket, nil
	}
	select {
	case ticket := <-cb.Ticket():
		return continuing(ticket)
	case got := <-answers:
		// select picks pseudorandomly when both are ready, so a ticket that
		// landed while the answer was in flight is re-checked here: signing
		// in is the act, typing is the commentary, and the act wins.
		select {
		case ticket := <-cb.Ticket():
			return continuing(ticket)
		default:
		}
		if got.err != nil && !errors.Is(got.err, io.EOF) {
			return "", fmt.Errorf("waiting for the sign-in form confirmation: %w", got.err)
		}
		switch strings.ToLower(strings.TrimSpace(got.line)) {
		case "y", "yes":
			return "", nil
		}
		return "", fmt.Errorf(
			"the sign-in form is not empty, so Garmin still has a session for another account. Nothing was stored.\n"+
				"  Sign out at %s in your browser, close the sign-in tab, then run this command again",
			ep.SSOLogout)
	case <-ctx.Done():
		return "", garminLoginTimeoutError(opts.Timeout, cb.Rejected())
	}
}

// garminInteractive reports whether a person is at the terminal to answer the
// sign-out prompt. Both ends matter: stdin must be a terminal to read from and
// stdout must be one for the prompt to be seen.
func garminInteractive(cmd *cobra.Command) bool {
	if !isTerminal(cmd.OutOrStdout()) {
		return false
	}
	if f, ok := cmd.InOrStdin().(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// garminLoginTimeoutError is the message for a sign-in that never came back.
// It must say that Garmin may have signed the user in regardless: a callback
// that lands after the deadline shows an error page in the browser while the
// Garmin session is live, and the next attempt then signs in the wrong
// account silently unless the user signs out first.
func garminLoginTimeoutError(timeout time.Duration, rejected int) error {
	return fmt.Errorf(
		"the browser sign-in did not come back within %s (%d callback(s) rejected for a bad nonce).\n"+
			"  Garmin may have signed you in anyway — sign out in the browser before retrying",
		timeout, rejected)
}

func garminFormatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.UTC().Format(time.RFC3339)
}
