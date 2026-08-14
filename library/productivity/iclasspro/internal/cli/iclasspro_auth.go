// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

// Read-only customer sessions for sign-in-gated tenants.
//
// Some iClassPro accounts hide their class and camp catalog behind a customer
// login. This file exchanges a customer email and password for the portal's
// access token and replays it on catalog reads only.
//
// Scope is deliberate and narrow: there is no cart, enrollment, promo-code, or
// checkout command anywhere in this CLI. The token is used to read a catalog
// that other accounts publish anonymously, nothing more.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const (
	icpEmailEnv = "ICLASSPRO_EMAIL"
	// #nosec G101 -- this is the name of an environment variable the user sets, not a credential value.
	icpPasswordEnv  = "ICLASSPRO_PASSWORD" //nolint:gosec // env var name, not a credential
	icpSessionPerms = 0o600
)

var icpJWTBase = "https://app.iclasspro.com/api/jwt/v1"

// icpSessionFile stores one token per account.
type icpSessionFile struct {
	Sessions      map[string]icpSession      `json:"sessions"`
	StaffSessions map[string]icpStaffSession `json:"staff_sessions,omitempty"`
}

type icpSession struct {
	Token    string `json:"token"`
	Email    string `json:"email"`
	SavedAt  string `json:"saved_at"`
	Endpoint string `json:"endpoint"`
}

var (
	icpSessionOnce  sync.Once
	icpSessionCache icpSessionFile
)

func icpSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "iclasspro-pp-cli", "session.json")
}

func icpLoadSessions() icpSessionFile {
	icpSessionOnce.Do(func() {
		icpSessionCache = icpSessionFile{
			Sessions:      map[string]icpSession{},
			StaffSessions: map[string]icpStaffSession{},
		}
		p := icpSessionPath()
		if p == "" {
			return
		}
		// #nosec G304 -- p is icpSessionPath(), a fixed path under the user's own
		// home directory; it is never derived from command-line or API input.
		raw, err := os.ReadFile(p)
		if err != nil {
			return
		}
		var f icpSessionFile
		if err := json.Unmarshal(raw, &f); err == nil {
			if f.Sessions == nil {
				f.Sessions = map[string]icpSession{}
			}
			if f.StaffSessions == nil {
				f.StaffSessions = map[string]icpStaffSession{}
			}
			icpSessionCache = f
		}
	})
	return icpSessionCache
}

func icpSaveSessions(f icpSessionFile) error {
	if f.Sessions == nil {
		f.Sessions = map[string]icpSession{}
	}
	if f.StaffSessions == nil {
		f.StaffSessions = map[string]icpStaffSession{}
	}
	p := icpSessionPath()
	if p == "" {
		return fmt.Errorf("cannot determine home directory for the session file")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, raw, icpSessionPerms); err != nil {
		return err
	}
	if err := os.Chmod(p, icpSessionPerms); err != nil {
		return err
	}
	icpSessionCache = f
	return nil
}

// icpTokenFor returns the stored read-only token for an account, if any.
func icpTokenFor(account string) string {
	return icpLoadSessions().Sessions[strings.ToLower(account)].Token
}

// icpLogin exchanges credentials for a portal access token.
func icpLogin(ctx context.Context, account, email, password string) (string, error) {
	// Payload shape lifted from the portal bundle (main.*.js, AuthService.login):
	// {email, password, type:"customer", account, multipleLoginSupport:true}.
	// "portal" as the account key returns "Account is required"; omitting type
	// returns "Server Error".
	body, err := json.Marshal(map[string]any{
		"email":                email,
		"password":             password,
		"type":                 "customer",
		"account":              account,
		"multipleLoginSupport": true,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, icpJWTBase+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://portal.iclasspro.com")
	req.Header.Set("Referer", "https://portal.iclasspro.com/"+account+"/")

	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("contacting the iClassPro login endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var decoded struct {
		AccessToken string          `json:"access_token"`
		Token       string          `json:"token"`
		Message     string          `json:"message"`
		Data        json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decoding the login response (HTTP %d): %w", resp.StatusCode, err)
	}
	token := decoded.AccessToken
	if token == "" {
		token = decoded.Token
	}
	if token == "" && len(decoded.Data) > 0 {
		var inner struct {
			AccessToken string `json:"access_token"`
			Token       string `json:"token"`
		}
		if json.Unmarshal(decoded.Data, &inner) == nil {
			if inner.AccessToken != "" {
				token = inner.AccessToken
			} else {
				token = inner.Token
			}
		}
	}
	if token == "" {
		msg := strings.TrimSpace(decoded.Message)
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d with no access_token in the response", resp.StatusCode)
		}
		return "", authErr(fmt.Errorf("login failed: %s", msg))
	}
	return token, nil
}

func newIclassproAuthCmd(flags *rootFlags) *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage read-only customer and staff sessions",
		Long: strings.Trim(`
Attach a read-only customer session to accounts that hide their catalog.

Most iClassPro accounts publish their classes and camps anonymously and need no
credentials at all. Some set a portal flag that requires a customer login first;
those accounts answer with HTTP 200, an empty list, and "Please sign in to see
classes." Run 'tenant <account>' to find out which kind you have.

The stored token is replayed on catalog reads only. This CLI has no cart,
enrollment, promo-code, or checkout command, by design.`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	auth.AddCommand(
		newIclassproAuthLoginCmd(flags),
		newIclassproAuthStatusCmd(flags),
		newIclassproAuthLogoutCmd(flags),
		newIclassproStaffAuthLoginCmd(flags),
		newIclassproStaffAuthStatusCmd(flags),
		newIclassproStaffAuthLogoutCmd(flags),
	)
	return auth
}

func newIclassproAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var email string

	cmd := &cobra.Command{
		Use:   "login [account]",
		Short: "Exchange customer credentials for a read-only session token",
		Long: strings.Trim(`
Read credentials from the environment and store the resulting token.

Set ICLASSPRO_EMAIL and ICLASSPRO_PASSWORD before running. Credentials are never
accepted as command-line flags, because flags land in shell history and process
listings. The password is used once, in the login request, and is not written to
disk; only the returned token is stored, at ~/.config/iclasspro-pp-cli/session.json
with 0600 permissions.`, "\n"),
		Example:     "  ICLASSPRO_EMAIL=me@example.com ICLASSPRO_PASSWORD=... iclasspro-pp-cli auth login examplegym",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth login")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if strings.TrimSpace(email) == "" {
				email = os.Getenv(icpEmailEnv)
			}
			password := os.Getenv(icpPasswordEnv)
			if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
				return usageErr(fmt.Errorf(
					"set %s and %s in the environment before running auth login", icpEmailEnv, icpPasswordEnv))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			token, err := icpLogin(ctx, account, email, password)
			if err != nil {
				return err
			}
			f := icpLoadSessions()
			if f.Sessions == nil {
				f.Sessions = map[string]icpSession{}
			}
			f.Sessions[strings.ToLower(account)] = icpSession{
				Token: token, Email: email,
				SavedAt: icpNow().Format(time.RFC3339), Endpoint: icpJWTBase,
			}
			if err := icpSaveSessions(f); err != nil {
				return err
			}

			result := map[string]any{"account": account, "stored": true, "path": icpSessionPath()}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored a read-only session for %s at %s\n", account, icpSessionPath())
			fmt.Fprintf(cmd.OutOrStdout(), "Verify with: iclasspro-pp-cli tenant %s\n", account)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Customer email (defaults to $"+icpEmailEnv+")")
	return cmd
}

func newIclassproAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "List accounts with a stored read-only session",
		Example:     "  iclasspro-pp-cli auth status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth status")
			}
			f := icpLoadSessions()
			type row struct {
				Account string `json:"account"`
				Email   string `json:"email"`
				SavedAt string `json:"saved_at"`
			}
			rows := make([]row, 0, len(f.Sessions))
			for acct, s := range f.Sessions {
				rows = append(rows, row{Account: acct, Email: s.Email, SavedAt: s.SavedAt})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stored sessions. Most accounts do not need one.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ACCOUNT\tEMAIL\tSAVED")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Account, r.Email, r.SavedAt)
			}
			return tw.Flush()
		},
	}
}

func newIclassproAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "logout [account]",
		Short:       "Forget the stored session for an account",
		Example:     "  iclasspro-pp-cli auth logout examplegym",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth logout")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			f := icpLoadSessions()
			delete(f.Sessions, strings.ToLower(account))
			if err := icpSaveSessions(f); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"account": account, "removed": true}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed the stored session for %s\n", account)
			return nil
		},
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newIclassproAuthCmd(flags))
	})
}
