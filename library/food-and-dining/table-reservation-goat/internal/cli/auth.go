package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.
// PATCH: resy-source-port — Resy login flow added alongside the OT/Tock cookie import.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
)

// newAuthCmd manages session credentials for the three reservation networks:
// OpenTable + Tock import browser cookies from local Chrome; Resy uses a
// bearer-style auth token obtained by exchanging email+password for a JWT.
func newAuthCmd(flags *rootFlags) *cobra.Command {
	// No mcp:read-only on the parent — this is a command group whose
	// children include both reads (`status`) and writes (`login`,
	// `logout`). Per-subcommand annotations carry the right hint.
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage browser cookie sessions for OpenTable + Tock and the token for Resy",
		Long: "Import session cookies from your local Chrome profile for OpenTable + Tock, " +
			"or exchange Resy email+password for a long-lived API token. All credentials " +
			"are persisted to ~/.config/table-reservation-goat-pp-cli/session.json.",
	}
	cmd.AddCommand(newAuthLoginCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var fromChrome bool
	var withResy bool
	var resyEmail string
	// `auth login --chrome` writes session cookies to disk via
	// `session.Save()`. Do NOT mark mcp:read-only — an MCP host that
	// honors the hint would skip the call in side-effect-prohibited
	// contexts and the user-consented session import would silently
	// not happen.
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Import session cookies from Chrome (OT + Tock) or exchange Resy email+password for a token",
		Long: "Two flows in one subcommand:\n\n" +
			"  --chrome      Import session cookies from your local Chrome profile for opentable.com\n" +
			"                and exploretock.com. On macOS, Chrome encrypts cookies with a key in the\n" +
			"                system keychain — you may be prompted by macOS to authorize keychain access.\n\n" +
			"  --resy        Exchange Resy email + password for a long-lived auth token. The password is\n" +
			"                consumed and never persisted; only the returned token lives on disk.\n" +
			"                The password is read from the terminal with echo disabled when stdin is a\n" +
			"                TTY, or from stdin (one line, trimmed of trailing newline) when piped — there\n" +
			"                is no --password flag because that would leak the credential into shell\n" +
			"                history and `ps aux` listings.",
		Example: "  table-reservation-goat-pp-cli auth login --chrome\n" +
			"  table-reservation-goat-pp-cli auth login --resy --email you@example.com",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would import credentials (verify mode — skipping)")
				return nil
			}
			if !fromChrome && !withResy {
				return fmt.Errorf("specify --chrome (OpenTable + Tock) or --resy (Resy email + password) — at least one is required")
			}

			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading existing session: %w", err)
			}

			out := map[string]any{}

			if fromChrome {
				otCookies, tockCookies, result, err := auth.ImportFromChrome(cmd.Context())
				if err != nil {
					return fmt.Errorf("importing chrome cookies: %w", err)
				}
				session.OpenTableCookies = otCookies
				session.TockCookies = tockCookies
				out["opentable_imported"] = result.OpenTableImported
				out["tock_imported"] = result.TockImported
				out["opentable_skipped"] = result.OpenTableSkipped
				out["tock_skipped"] = result.TockSkipped
				if len(result.Notes) > 0 {
					out["chrome_notes"] = result.Notes
				}
			}

			if withResy {
				email := strings.TrimSpace(resyEmail)
				if email == "" {
					return fmt.Errorf("--resy requires --email")
				}
				// Honor --no-input / --agent: never prompt on TTY in
				// non-interactive mode. The non-TTY stdin path still
				// works for scripted/piped login, so fail fast ONLY
				// when stdin would actually trigger a prompt.
				if flags.noInput && term.IsTerminal(int(syscall.Stdin)) {
					return fmt.Errorf("--no-input / --agent mode: pipe the Resy password via stdin instead of relying on an interactive prompt")
				}
				password, perr := readPasswordInteractive("Resy password: ")
				if perr != nil {
					return fmt.Errorf("reading password: %w", perr)
				}
				if password == "" {
					return fmt.Errorf("password may not be empty")
				}
				resyAuth, lerr := resyLogin(cmd.Context(), email, password)
				if lerr != nil {
					return fmt.Errorf("resy login: %w", lerr)
				}
				session.Resy = resyAuth
				out["resy_logged_in"] = true
				out["resy_email"] = resyAuth.Email
				out["resy_user_id"] = resyAuth.UserID
			}

			if err := session.Save(); err != nil {
				return fmt.Errorf("saving session: %w", err)
			}
			if fromChrome {
				// `auth login --chrome` is the user's deliberate refresh path.
				// Clear any negative-cache entry so the next OT client re-walks
				// the keychain (which is now warm from this very kooky call) and
				// caches the fresh Akamai cookies.
				auth.ClearAkamaiCache("opentable.com")
				auth.ClearAkamaiCache("exploretock.com")
			}
			out["opentable_logged_in"] = session.LoggedIn(auth.NetworkOpenTable)
			out["tock_logged_in"] = session.LoggedIn(auth.NetworkTock)
			out["resy_logged_in_persisted"] = session.LoggedIn(auth.NetworkResy)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&fromChrome, "chrome", false, "Import OpenTable + Tock session cookies from local Chrome profile")
	cmd.Flags().BoolVar(&withResy, "resy", false, "Exchange Resy email + password for an auth token (password read from TTY with echo disabled, or stdin when piped)")
	cmd.Flags().StringVar(&resyEmail, "email", "", "Resy email (required with --resy)")
	return cmd
}

// readPasswordInteractive reads a password from /dev/tty with echo disabled
// so it never lands in shell history or scrollback. Falls back to plain
// stdin read when the process is not attached to a TTY (e.g., piped).
func readPasswordInteractive(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	defer fmt.Fprintln(os.Stderr)
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		raw, err := term.ReadPassword(fd)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
	// Non-TTY: fall back to a plain line read so the command still works in
	// scripted contexts (the caller is expected to pipe a single line).
	//
	// EOF handling: when the caller pipes a password without a trailing
	// newline — `printf %s "$RESY_PASSWORD" | trg auth login --resy ...` —
	// bufio.Reader.ReadString('\n') returns (line, io.EOF). Discarding the
	// line on any error would silently throw away the password. Accept
	// EOF as a successful terminator when at least one byte was read;
	// surface any other error as-is.
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	trimmed := strings.TrimRight(line, "\r\n")
	if err != nil {
		if err == io.EOF && trimmed != "" {
			return trimmed, nil
		}
		return "", err
	}
	return trimmed, nil
}

// resyLogin runs the email+password exchange and returns the persisted
// ResyAuth ready to assign to session.Resy. Extracted so tests can stub it.
func resyLogin(ctx context.Context, email, password string) (*auth.ResyAuth, error) {
	client := resy.New(resy.Credentials{}) // anonymous client; login fills in the token
	resp, err := client.LoginWithPassword(ctx, email, password)
	if err != nil {
		return nil, err
	}
	return &auth.ResyAuth{
		APIKey:    resy.PublicClientID,
		AuthToken: resp.Token,
		Email:     firstNonEmpty(resp.EmailAddress, email),
		FirstName: resp.FirstName,
		UserID:    resp.ID,
	}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether OpenTable, Tock, and Resy credentials are loaded and fresh",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			path, _ := auth.SessionPath()
			out := map[string]any{
				"session_path":        path,
				"updated_at":          session.UpdatedAt,
				"opentable_logged_in": session.LoggedIn(auth.NetworkOpenTable),
				"tock_logged_in":      session.LoggedIn(auth.NetworkTock),
				"resy_logged_in":      session.LoggedIn(auth.NetworkResy),
				"opentable_cookies":   len(session.OpenTableCookies),
				"tock_cookies":        len(session.TockCookies),
			}
			if session.Resy != nil {
				out["resy_email"] = session.Resy.Email
				out["resy_user_id"] = session.Resy.UserID
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	var network string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials (all networks, or one of opentable|tock|resy)",
		Long: "Without --network, deletes the entire session file. With --network <name>, clears " +
			"just that network's credentials so the other two stay logged in.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would clear session (verify mode — skipping)")
				return nil
			}
			if network == "" {
				if err := auth.Clear(); err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cleared": "all"}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			switch strings.ToLower(network) {
			case auth.NetworkOpenTable:
				session.OpenTableCookies = nil
			case auth.NetworkTock:
				session.TockCookies = nil
			case auth.NetworkResy:
				session.Resy = nil
			default:
				return fmt.Errorf("unknown --network %q (want opentable|tock|resy)", network)
			}
			if err := session.Save(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cleared": network}, flags)
		},
	}
	cmd.Flags().StringVar(&network, "network", "", "Clear just this network (opentable|tock|resy); default clears everything")
	return cmd
}
