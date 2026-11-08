package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/chromecookies"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/exitcode"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

// sessionCookie is the one cookie the API authenticates. Proven live on 2026-11-07:
// AUTH_TOKEN alone returned 200 from /reservations/ and /users/me/, while appSession,
// sh-sso and csrftoken alone returned 401, and the same value as an Authorization
// header with no cookie also returned 401. So it must travel as a cookie.
const sessionCookie = "AUTH_TOKEN"

// consumerClient builds the hand-written client from the generated config, so env
// overrides, the credentials file and the base URL all have one source of truth.
func consumerClient(flags *rootFlags) (*consumer.Client, *config.Config, error) {
	// Building the generated client first applies its config and profile gates, so hand
	// commands refuse in the same situations as generated ones.
	if _, err := flags.newClient(); err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	var sess *session.Session
	if raw := strings.TrimSpace(cfg.CookieCredential()); raw != "" {
		if sess, err = session.ParseCookieInput(raw, cfg.AuthSource); err != nil {
			return nil, nil, authErr(err)
		}
	}
	c := consumer.New(sess)
	c.BaseURL = cfg.BaseURL
	c.CraigBaseURL = inventoryBaseURL()
	c.DryRun = flags.dryRun
	return c, cfg, nil
}

// mapConsumerErr keeps the documented exit codes when errors cross from the client
// into generated code.
func mapConsumerErr(err error) error {
	if err == nil {
		return nil
	}
	var ex *exitcode.Error
	if exitcode.As(err, &ex) {
		switch ex.Code {
		case exitcode.Usage:
			return usageErr(err)
		case exitcode.NotFound:
			return notFoundErr(err)
		case exitcode.Auth:
			return authErr(err)
		case exitcode.Transient:
			return rateLimitErr(err)
		default:
			return apiErr(err)
		}
	}
	return apiErr(err)
}

// verifyCookieHeader asks the API whether a session works. /users/me/ is used
// because /user/ returns 401 even for a good session.
func verifyCookieHeader(ctx context.Context, flags *rootFlags, header string) error {
	sess, err := session.ParseCookieInput(header, "verify")
	if err != nil {
		return exitcode.Authf("%v", err)
	}
	if sess.CookieValue(sessionCookie) == "" {
		return exitcode.Authf("no non-empty %s cookie in this session", sessionCookie)
	}
	c := consumer.New(sess)
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return err
	}
	c.BaseURL = cfg.BaseURL
	// Bounded so cancellation and a short --timeout also end this check.
	ctx, cancel := boundCtx(ctx, flags)
	defer cancel()
	c.Ctx = ctx
	me, err := c.GetMe()
	if err != nil {
		return err
	}
	// A 200 with no account is not an accepted session; a proxy or captive portal can
	// return one.
	if me.ID == 0 {
		return exitcode.Authf("the API returned no account for this session")
	}
	return nil
}

// verifySessionForLogin turns a failed check into a refusal with next steps.
func verifySessionForLogin(ctx context.Context, flags *rootFlags, header string) error {
	err := verifyCookieHeader(ctx, flags, header)
	if err == nil {
		return nil
	}
	var ex *exitcode.Error
	if exitcode.As(err, &ex) && ex.Code == exitcode.Transient {
		return rateLimitErr(fmt.Errorf("could not reach the API to verify the session, so nothing was saved: %v", err))
	}
	return authErr(fmt.Errorf("the API rejected this session, so nothing was saved (%v). Sign in at https://spothero.com in Chrome, wait a minute, and run auth login --chrome again", err))
}

// nativeChromeSession reads each Chrome profile that holds the session cookie and
// returns the first one the API accepts. Testing against the API rather than
// prompting means multi-profile users get the right account with no question, and
// agents with no input still work.
func nativeChromeSession(ctx context.Context, w io.Writer, flags *rootFlags, domain, profileFlag string) (string, error) {
	var candidates []chromeProfile
	if profileFlag != "" {
		p, err := resolveProfileByName(profileFlag)
		if err != nil {
			return "", err
		}
		candidates = []chromeProfile{p}
	} else {
		profiles, err := discoverChromeProfilesNative(ctx, domain)
		if err != nil {
			return "", err
		}
		candidates = profiles
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no Chrome profile found")
	}
	key, err := chromecookies.SafeStorageKey(ctx)
	if err != nil {
		return "", err
	}
	var lastErr error = chromecookies.ErrNoCookies
	for _, p := range candidates {
		db, ok := chromecookies.LocateDB(filepath.Join(p.DataDir, p.Dir))
		if !ok {
			continue
		}
		cookies, err := chromecookies.Read(ctx, db, domain, key)
		if err != nil {
			lastErr = err
			continue
		}
		if cookies[sessionCookie] == "" {
			lastErr = fmt.Errorf("profile %s has no %s cookie; sign in at https://spothero.com", p.profileLocation(), sessionCookie)
			continue
		}
		parts := []string{sessionCookie + "=" + cookies[sessionCookie]}
		if v := cookies["csrftoken"]; v != "" {
			parts = append(parts, "csrftoken="+v)
		}
		header := strings.Join(parts, "; ")
		if err := verifyCookieHeader(ctx, flags, header); err != nil {
			lastErr = fmt.Errorf("profile %s: %v", p.profileLocation(), err)
			continue
		}
		fmt.Fprintf(w, "Read the session from Chrome profile %q.\n", p.profileLocation())
		return header, nil
	}
	return "", lastErr
}

// discoverChromeProfilesNative lists profiles without the sqlite3 binary the
// generated discovery needs, with the conventional Default profile first.
func discoverChromeProfilesNative(_ context.Context, _ string) ([]chromeProfile, error) {
	dirs, err := chromeChannelDirs()
	if err != nil {
		return nil, err
	}
	var out []chromeProfile
	for _, ch := range dirs {
		for _, name := range chromeProfileDirNames(ch.DataDir) {
			out = append(out, chromeProfile{Channel: ch.Channel, DataDir: ch.DataDir, Dir: name})
		}
	}
	return out, nil
}

// jwtExpiry reads exp from the session JWT for display. It is not a security check,
// so the signature is not verified, and the API remains the authority.
func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0).UTC(), true
}

// runVerifiedAuthStatus reports authenticated only when the API accepts the session.
func runVerifiedAuthStatus(cmd *cobra.Command, flags *rootFlags) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	report := map[string]any{"authenticated": false, "domain": "spothero.com", "config": cfg.Path}
	emit := func(human string) error {
		if flags.asJSON {
			return flags.printJSON(cmd, report)
		}
		fmt.Fprintln(cmd.OutOrStdout(), human)
		for _, k := range []string{"source", "expires_at", "reason", "hint"} {
			if v, ok := report[k]; ok && v != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", k, v)
			}
		}
		return nil
	}
	if refusals := cfg.CredentialRefusalSummaries(); len(refusals) > 0 {
		report["reason"] = strings.Join(refusals, "; ")
		_ = emit(red("Credentials present but refused"))
		return authErr(cfg.CredentialRefusalError())
	}
	header := strings.TrimSpace(cfg.CookieCredential())
	if header == "" {
		report["reason"] = "no session saved"
		report["hint"] = "spothero-pp-cli auth login --chrome"
		_ = emit(red("Not authenticated"))
		return authErr(errors.New("no credentials configured"))
	}
	report["source"] = cfg.AuthSource
	if sess, perr := session.ParseCookieInput(header, ""); perr == nil {
		if exp, ok := jwtExpiry(sess.CookieValue(sessionCookie)); ok {
			report["expires_at"] = exp.Format(time.RFC3339)
		}
	}
	if flags.dryRun {
		report["reason"] = "dry run: session not checked with the API"
		return emit("Not checked (dry run)")
	}
	if err := verifyCookieHeader(cmd.Context(), flags, header); err != nil {
		var ex *exitcode.Error
		if exitcode.As(err, &ex) && ex.Code == exitcode.Transient {
			report["reason"] = "could not reach the API to check the session"
			_ = emit(red("Not verified"))
			return rateLimitErr(err)
		}
		report["reason"] = err.Error()
		report["hint"] = "spothero-pp-cli auth login --chrome"
		_ = emit(red("Not authenticated: the API rejected the saved session"))
		return authErr(err)
	}
	report["authenticated"] = true
	return emit(green("Authenticated") + " (accepted by the API)")
}
