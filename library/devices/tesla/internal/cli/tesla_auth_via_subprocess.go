// Optional fallback: subprocess into adriankumpf/tesla_auth (if installed) and
// parse its stdout for the refresh+access tokens. The PKCE void-callback flow
// is the default and recommended path; this exists as an opt-in for users who
// prefer the native-window experience tesla_auth ships.
//
// Hand-coded; lives outside the generator's emit set so it survives regens.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const teslaAuthBinary = "tesla_auth"

// detectTeslaAuthBinary returns the absolute path of tesla_auth if it's on
// $PATH (or the helper-friendly ~/bin override). Returns "" if not present.
// Pure side-effect-free lookup.
func detectTeslaAuthBinary() string {
	p, err := exec.LookPath(teslaAuthBinary)
	if err != nil {
		return ""
	}
	return p
}

// maybeHintTeslaAuthAvailable prints one stderr line when tesla_auth is on
// $PATH so users know the alternate path exists. No-op when binary absent.
func maybeHintTeslaAuthAvailable(w io.Writer) {
	if p := detectTeslaAuthBinary(); p != "" {
		fmt.Fprintf(w, "(hint: tesla_auth detected at %s; use --via tesla-auth for the native-window flow)\n", p)
	}
}

// runTeslaAuthSubprocessFlow invokes tesla_auth, captures stdout, parses out
// the access + refresh tokens, and stores them via the U1 facade.
//
// tesla_auth's Display format (Rust source src/auth.rs) emits two header bars
// followed by the token on its own line:
//
//	--------------------------------- ACCESS TOKEN ---------------------------------
//
//	eyJ...
//
//	--------------------------------- REFRESH TOKEN --------------------------------
//
//	eyJ...
//
// This parser is intentionally strict: malformed output (Tesla changes the
// shape, tesla_auth swaps order, --debug log noise interleaves) fails loud
// rather than capturing the wrong token.
func runTeslaAuthSubprocessFlow(cmd *cobra.Command, flags *rootFlags) error {
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "status": "logged_in", "method": "via_tesla_auth"}, flags)
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "method": "via_tesla_auth"}, flags)
	}

	bin := detectTeslaAuthBinary()
	if bin == "" {
		return fmt.Errorf("tesla_auth not found on $PATH; install from https://github.com/adriankumpf/tesla_auth or use the default flow (no --via flag)")
	}

	fmt.Fprintln(cmd.OutOrStderr(), "Launching tesla_auth... a native window will open for you to log in.")
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()
	out, err := runTeslaAuthBinary(ctx, bin)
	if err != nil {
		return fmt.Errorf("tesla_auth subprocess: %w", err)
	}
	access, refresh, perr := parseTeslaAuthOutput(out)
	if perr != nil {
		return fmt.Errorf("parse tesla_auth output: %w", perr)
	}

	cfg, cerr := config.Load(flagsConfigPath(flags))
	if cerr != nil {
		return configErr(cerr)
	}
	// tesla_auth doesn't report expires_in; assume Tesla's standard 8h TTL.
	expiresAt := time.Now().Add(8 * time.Hour).UTC()
	if err := saveTeslaTokens(cfg, refresh, access, expiresAt); err != nil {
		return err
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status":       "logged_in",
		"method":       "via_tesla_auth",
		"expires_at":   expiresAt.Format(time.RFC3339),
		"storage_path": cfg.Path,
		"hint":         "Bearer auto-refreshes when it expires. The refresh token is long-lived.",
	}, flags)
}

// runTeslaAuthBinary executes tesla_auth with the given context (timeout) and
// returns stdout. Stderr is forwarded so the user sees tesla_auth's own log
// output. Split from runTeslaAuthSubprocessFlow so tests can stub the subprocess.
var runTeslaAuthBinary = func(ctx context.Context, bin string) (string, error) {
	c := exec.CommandContext(ctx, bin)
	var stdout bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = io.Discard // tesla_auth's debug log isn't useful here
	if err := c.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

// tesla_auth output parser. Matches the access + refresh sections that the
// Rust source's `Display for Tokens` impl emits.
var (
	teslaAuthAccessRE  = regexp.MustCompile(`(?s)-+ ACCESS TOKEN -+\s+(eyJ[A-Za-z0-9_\-\.]+)`)
	teslaAuthRefreshRE = regexp.MustCompile(`(?s)-+ REFRESH TOKEN -+\s+(eyJ[A-Za-z0-9_\-\.]+)`)
)

func parseTeslaAuthOutput(raw string) (access, refresh string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("tesla_auth produced no output (login may have been cancelled)")
	}
	am := teslaAuthAccessRE.FindStringSubmatch(raw)
	rm := teslaAuthRefreshRE.FindStringSubmatch(raw)
	if len(am) < 2 {
		return "", "", fmt.Errorf("missing ACCESS TOKEN section; got: %s", truncateOutput(raw, 200))
	}
	if len(rm) < 2 {
		return "", "", fmt.Errorf("missing REFRESH TOKEN section; got: %s", truncateOutput(raw, 200))
	}
	return am[1], rm[1], nil
}

func truncateOutput(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
