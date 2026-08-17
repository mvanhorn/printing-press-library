// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

// Read-only authentication for the iClassPro Office Portal (staff surface).
// Credentials are accepted only through environment variables. The password is
// never persisted; only the server-issued session cookie is stored in the same
// 0600 session file used by customer catalog authentication.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	icpStaffUsernameEnv = "ICLASSPRO_STAFF_USERNAME"
	// #nosec G101 -- environment variable name, not a credential value.
	icpStaffPasswordEnv = "ICLASSPRO_STAFF_PASSWORD" //nolint:gosec
)

var icpStaffOfficeBase = "https://app.iclasspro.com"

type icpStaffSession struct {
	Cookie   string `json:"cookie"`
	Username string `json:"username"`
	SavedAt  string `json:"saved_at"`
	Endpoint string `json:"endpoint"`
}

func icpStaffAccount(account string) (string, error) {
	account = strings.ToLower(strings.TrimSpace(account))
	if account == "" {
		return "", fmt.Errorf("account is required (the Office Portal slug)")
	}
	for _, r := range account {
		if r != '-' && r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", fmt.Errorf("invalid account %q: use only letters, numbers, hyphens, and underscores", account)
		}
	}
	return account, nil
}

func icpStaffHeaders(account, cookie string) map[string]string {
	return map[string]string{
		"Accept":           "application/json",
		"Content-Type":     "application/json",
		"Cookie":           cookie,
		"Origin":           icpStaffOfficeBase,
		"Referer":          icpStaffOfficeBase + "/a/" + account + "/#!/dashboard",
		"X-Requested-With": "XMLHttpRequest",
	}
}

func icpStaffLogin(ctx context.Context, account, username, password string) (icpStaffSession, error) {
	account, err := icpStaffAccount(account)
	if err != nil {
		return icpStaffSession{}, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return icpStaffSession{}, fmt.Errorf("creating the staff session: %w", err)
	}
	client := &http.Client{Jar: jar, Timeout: 45 * time.Second}
	form := url.Values{
		"stafflogin": {"1"},
		"uname":      {username},
		"passwd":     {password},
	}
	loginURL := icpStaffOfficeBase + "/a/" + url.PathEscape(account) + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return icpStaffSession{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return icpStaffSession{}, fmt.Errorf("contacting the iClassPro staff login: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return icpStaffSession{}, authErr(fmt.Errorf("staff login returned HTTP %d", resp.StatusCode))
	}

	baseURL, err := url.Parse(icpStaffOfficeBase + "/")
	if err != nil {
		return icpStaffSession{}, err
	}
	cookies := jar.Cookies(baseURL)
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" && cookie.Value != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return icpStaffSession{}, authErr(fmt.Errorf("staff login did not establish a session"))
	}
	session := icpStaffSession{
		Cookie:   strings.Join(parts, "; "),
		Username: username,
		SavedAt:  icpNow().Format(time.RFC3339),
		Endpoint: icpStaffOfficeBase + "/api/v1",
	}
	if err := icpStaffVerify(ctx, account, session); err != nil {
		return icpStaffSession{}, err
	}
	return session, nil
}

func icpStaffVerify(ctx context.Context, account string, session icpStaffSession) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, icpStaffOfficeBase+"/api/v1/user/permissions", nil)
	if err != nil {
		return err
	}
	for k, v := range icpStaffHeaders(account, session.Cookie) {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("verifying the staff session: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 400 {
		return authErr(fmt.Errorf("staff session verification returned HTTP %d", resp.StatusCode))
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); ct != "" && !strings.Contains(ct, "json") {
		return authErr(fmt.Errorf("staff login was not accepted; the portal returned %s instead of JSON", ct))
	}
	return nil
}

func icpStaffSessionFor(account string) (icpStaffSession, error) {
	account, err := icpStaffAccount(account)
	if err != nil {
		return icpStaffSession{}, err
	}
	session, ok := icpLoadSessions().StaffSessions[account]
	if !ok || strings.TrimSpace(session.Cookie) == "" {
		return icpStaffSession{}, authErr(fmt.Errorf(
			"no staff session for %q; set %s and %s, then run 'iclasspro-pp-cli auth staff-login %s'",
			account, icpStaffUsernameEnv, icpStaffPasswordEnv, account))
	}
	return session, nil
}

func newIclassproStaffAuthLoginCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "staff-login <account>",
		Short:       "Create a read-only Office Portal session from environment credentials",
		Example:     "  ICLASSPRO_STAFF_USERNAME=staff-user ICLASSPRO_STAFF_PASSWORD=... iclasspro-pp-cli auth staff-login examplegym",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth staff-login")
			}
			account, err := icpStaffAccount(args[0])
			if err != nil {
				return usageErr(err)
			}
			username := strings.TrimSpace(os.Getenv(icpStaffUsernameEnv))
			password := os.Getenv(icpStaffPasswordEnv)
			if username == "" || strings.TrimSpace(password) == "" {
				return usageErr(fmt.Errorf("set %s and %s in the environment before staff-login", icpStaffUsernameEnv, icpStaffPasswordEnv))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			session, err := icpStaffLogin(ctx, account, username, password)
			if err != nil {
				return err
			}
			f := icpLoadSessions()
			f.StaffSessions[account] = session
			if err := icpSaveSessions(f); err != nil {
				return err
			}
			result := map[string]any{"account": account, "stored": true, "path": icpSessionPath(), "saved_at": session.SavedAt}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored a read-only staff session for %s at %s\n", account, icpSessionPath())
			return nil
		},
	}
	return cmd
}

func newIclassproStaffAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "staff-status",
		Short:       "List stored Office Portal sessions without exposing cookies",
		Example:     "  iclasspro-pp-cli auth staff-status",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth staff-status")
			}
			type row struct {
				Account  string `json:"account"`
				Username string `json:"username"`
				SavedAt  string `json:"saved_at"`
			}
			f := icpLoadSessions()
			accounts := make([]string, 0, len(f.StaffSessions))
			for account := range f.StaffSessions {
				accounts = append(accounts, account)
			}
			sort.Strings(accounts)
			rows := make([]row, 0, len(accounts))
			for _, account := range accounts {
				s := f.StaffSessions[account]
				rows = append(rows, row{Account: account, Username: s.Username, SavedAt: s.SavedAt})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stored staff sessions.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ACCOUNT\tUSERNAME\tSAVED")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Account, r.Username, r.SavedAt)
			}
			return tw.Flush()
		},
	}
}

func newIclassproStaffAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "staff-logout <account>",
		Short:       "Forget one stored Office Portal session",
		Example:     "  iclasspro-pp-cli auth staff-logout examplegym",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "auth staff-logout")
			}
			account, err := icpStaffAccount(args[0])
			if err != nil {
				return usageErr(err)
			}
			f := icpLoadSessions()
			delete(f.StaffSessions, account)
			if err := icpSaveSessions(f); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"account": account, "removed": true}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed the stored staff session for %s\n", account)
			return nil
		},
	}
}
