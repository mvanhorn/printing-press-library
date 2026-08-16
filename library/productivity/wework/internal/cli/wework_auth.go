// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless, self-registering) production auth onboarding for
// WeWork's composed credential set (bearer token + account uuid + member type).
// Adds `auth session-import` (persist all four) and `auth whoami` (status + expiry)
// under the generated `auth` parent command.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/config"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, c := range root.Commands() {
			if c.Name() == "auth" {
				addNovelCommandIfAbsent(c, newAuthLoginCmd(flags))
				addNovelCommandIfAbsent(c, newAuthImportCmd(flags))
				addNovelCommandIfAbsent(c, newAuthHandoffCmd(flags))
				addNovelCommandIfAbsent(c, newAuthPushCmd(flags))
				addNovelCommandIfAbsent(c, newAuthRefreshCmd(flags))
				addNovelCommandIfAbsent(c, newAuthWhoamiCmd(flags))
				break
			}
		}
	})
}

const weworkSessionCaptureScript = `const k=Object.keys(localStorage).find(k=>/@@auth0spajs@@/.test(k)&&/openid/.test(k));const c=JSON.parse(localStorage.getItem(k));copy(JSON.stringify({token:c.body.access_token,refreshToken:c.body.refresh_token,uuid:localStorage.getItem('CurrentAccountUUID'),memberType:localStorage.getItem('WWMemberType')}));`

const authImportLong = `Import and persist a complete WeWork session so it survives across shells.

WeWork authenticates with a short-lived auth0 Bearer token, a rotating refresh
token, and two headers (weworkuuid, weworkmembertype). Grab all four from a
logged-in browser session:

  1. Open members.wework.com (logged in) -> DevTools Console, paste:

     ` + weworkSessionCaptureScript + `

     (that copies a one-line JSON to your clipboard)

  2. Pipe or paste it into import:

     pbpaste | wework-pp-cli auth session-import --stdin

For a remote headless host, keep the bundle off disk and out of shell history:

     pbpaste | ssh booking-host 'wework-pp-cli auth session-import --stdin'

The refresh token rotates. After import, let this CLI own the refresh chain;
do not keep refreshing the same session in a personal browser.

By default import requires all four values so success means the host is actually
renewable. Use --allow-partial only to repair an existing installation.`

func newAuthImportCmd(flags *rootFlags) *cobra.Command {
	var token, refreshToken, uuid, member string
	var fromStdin, allowPartial bool
	cmd := &cobra.Command{
		Use:     "session-import",
		Aliases: []string{"import"},
		Short:   "Import and persist a renewable WeWork session",
		Long:    authImportLong,
		Example: strings.Trim(`
  pbpaste | wework-pp-cli auth session-import --stdin
  wework-pp-cli auth session-import --token "$WEWORK_TOKEN" --refresh-token "$WEWORK_REFRESH_TOKEN" --uuid "$WEWORK_UUID" --member-type "$WEWORK_MEMBER_TYPE"`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// If asked to read stdin, or nothing was passed via flags, try a
			// JSON blob on stdin (the DevTools snippet output).
			if fromStdin || (token == "" && refreshToken == "" && uuid == "" && member == "") {
				data, _ := io.ReadAll(cmd.InOrStdin())
				if len(bytes.TrimSpace(data)) > 0 {
					t, r, u, m, err := parseComposedAuthJSON(data)
					if err != nil {
						return usageErr(fmt.Errorf("reading credentials JSON from stdin: %w", err))
					}
					if token == "" {
						token = t
					}
					if refreshToken == "" {
						refreshToken = r
					}
					if uuid == "" {
						uuid = u
					}
					if member == "" {
						member = m
					}
				}
			}
			if token == "" && refreshToken == "" && uuid == "" && member == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("provide at least one credential flag, or a JSON bundle via --stdin"))
			}
			if !allowPartial {
				missing := make([]string, 0, 4)
				for name, value := range map[string]string{
					"token": token, "refreshToken": refreshToken, "uuid": uuid, "memberType": member,
				} {
					if strings.TrimSpace(value) == "" {
						missing = append(missing, name)
					}
				}
				if len(missing) > 0 {
					sort.Strings(missing)
					return usageErr(fmt.Errorf("complete renewable session requires token, refreshToken, uuid, and memberType (missing: %s); use --allow-partial only to repair existing credentials", strings.Join(missing, ", ")))
				}
			}
			if token != "" && refreshToken != "" {
				if err := config.ValidateWeworkRenewableAccessToken(token); err != nil {
					return authErr(fmt.Errorf("refusing renewable session import: %w", err))
				}
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveWeworkAuth(token, refreshToken, uuid, member); err != nil {
				return configErr(fmt.Errorf("saving credentials: %w", err))
			}
			hasT, hasU, hasM, exp := cfg.ComposedAuthStatus()
			hasR, renewable := cfg.WeworkRefreshStatus()
			if flags.asJSON {
				out := map[string]any{
					"saved": true, "config_path": cfg.Path,
					"token": hasT, "refresh_token": hasR, "renewable": renewable,
					"uuid": hasU, "member_type": hasM,
				}
				if !exp.IsZero() {
					out["token_expires"] = exp.UTC().Format(time.RFC3339)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Credentials saved to %s\n", cfg.Path)
			fmt.Fprintf(w, "  token:       %s\n", presence(hasT))
			fmt.Fprintf(w, "  refresh:     %s\n", presence(hasR))
			fmt.Fprintf(w, "  renewable:   %t\n", renewable)
			fmt.Fprintf(w, "  uuid:        %s\n", presence(hasU))
			fmt.Fprintf(w, "  member-type: %s\n", presence(hasM))
			if !hasU || !hasM {
				fmt.Fprintln(w, "\nwarning: uuid and member-type are required for API calls; import them too.")
			}
			if !exp.IsZero() {
				fmt.Fprintf(w, "  token expires: %s (%s)\n", exp.Local().Format("2006-01-02 15:04 MST"), humanUntil(exp))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "WeWork auth0 bearer token")
	cmd.Flags().StringVar(&refreshToken, "refresh-token", "", "Rotating Auth0 refresh token (prefer --stdin to avoid shell history)")
	cmd.Flags().StringVar(&uuid, "uuid", "", "Account UUID (weworkuuid header)")
	cmd.Flags().StringVar(&member, "member-type", "", "Member type code (weworkmembertype header)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read a {token,refreshToken,uuid,memberType} JSON bundle from stdin")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Explicitly repair selected fields without requiring a complete renewable bundle")
	return cmd
}

var safeSSHTarget = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]*$`)

// pp:data-source computed
func newAuthHandoffCmd(flags *rootFlags) *cobra.Command {
	var sshTarget string
	cmd := &cobra.Command{
		Use:   "handoff",
		Short: "Print a remote-device login and secure stdin handoff",
		Long: "Prints a windowless booking-host workflow: log in on another computer, " +
			"capture the renewable browser session, and pipe it directly to this CLI over SSH. " +
			"WeWork has not enabled an OAuth device grant or CLI callback, so this workflow " +
			"requires the capture script after login rather than claiming a one-click callback.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if sshTarget != "" && !safeSSHTarget.MatchString(sshTarget) {
				return usageErr(errors.New("--ssh-target may contain only a host or SSH alias with an optional user@ prefix"))
			}
			importCommand := "pbpaste | wework-pp-cli auth session-import --stdin"
			if sshTarget != "" {
				importCommand = "pbpaste | ssh " + sshTarget + " 'wework-pp-cli auth session-import --stdin'"
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"login_url":             weworkOrigin,
					"capture_script":        weworkSessionCaptureScript,
					"import_command":        importCommand,
					"automatic_callback":    false,
					"device_flow_available": false,
					"reason":                "WeWork's Auth0 application does not enable a device grant or CLI redirect URI",
				}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "1. On the remote computer, open and log in: %s\n\n", weworkOrigin)
			fmt.Fprintln(w, "2. In that tab's DevTools Console, run this one-line capture script:")
			fmt.Fprintf(w, "\n%s\n\n", weworkSessionCaptureScript)
			fmt.Fprintln(w, "3. Send the clipboard bundle directly to the booking host:")
			fmt.Fprintf(w, "\n%s\n\n", importCommand)
			fmt.Fprintln(w, "The booking host never opens a browser. A fully automatic link callback is unavailable")
			fmt.Fprintln(w, "because WeWork has not enabled OAuth device flow or a CLI callback for its Auth0 client.")
			return nil
		},
	}
	cmd.Flags().StringVar(&sshTarget, "ssh-target", "", "Remote SSH host or user@host receiving the session bundle")
	return cmd
}

const remoteAuthPushCommand = "wework-pp-cli --no-learn auth session-import --stdin --json >/dev/null && " +
	"wework-pp-cli --no-learn auth refresh --force --json >/dev/null && " +
	"wework-pp-cli --no-learn auth whoami --json"

// pp:data-source live
func newAuthPushCmd(flags *rootFlags) *cobra.Command {
	var sshTarget string
	cmd := &cobra.Command{
		Use:   "push --ssh-target user@booking-host",
		Short: "Securely push the stored renewable session to a headless host",
		Long: "Send the complete locally stored WeWork session to another machine over SSH stdin. " +
			"The remote CLI imports the bundle, forces a refresh so it becomes the sole owner of the " +
			"rotating token family, and reports whether the remote host is headless-ready. Credentials " +
			"are never placed in process arguments, environment variables, output, or a transfer file.",
		Example:     "  wework-pp-cli auth push --ssh-target user@booking-host\n  wework-pp-cli auth push --ssh-target booking-host --json",
		Annotations: map[string]string{"mcp:hidden": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(sshTarget) == "" {
				return usageErr(errors.New("--ssh-target is required"))
			}
			if !safeSSHTarget.MatchString(sshTarget) {
				return usageErr(errors.New("--ssh-target may contain only a host or SSH alias with an optional user@ prefix"))
			}
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.ApplyWeworkAuthBootstrap()
			token := cfg.WeworkToken
			if token == "" {
				token = cfg.AccessToken
			}
			if token == "" || cfg.RefreshToken == "" || cfg.WeworkUuid == "" || cfg.WeworkMemberType == "" {
				return authErr(errors.New("complete renewable local session required; run 'auth login --chrome' or 'auth session-import --stdin' first"))
			}
			if err := config.ValidateWeworkRenewableAccessToken(token); err != nil {
				return authErr(fmt.Errorf("local session is not safely renewable: %w", err))
			}

			bundle, err := json.Marshal(map[string]string{
				"token": token, "refreshToken": cfg.RefreshToken,
				"uuid": cfg.WeworkUuid, "memberType": cfg.WeworkMemberType,
			})
			if err != nil {
				return configErr(fmt.Errorf("encoding local session: %w", err))
			}

			ssh := exec.CommandContext(cmd.Context(), "ssh", sshTarget, remoteAuthPushCommand)
			ssh.Stdin = bytes.NewReader(bundle)
			var remoteStdout, remoteStderr bytes.Buffer
			ssh.Stdout = &remoteStdout
			ssh.Stderr = &remoteStderr
			if err := ssh.Run(); err != nil {
				// Do not reflect remote output: an untrusted or misconfigured endpoint
				// could echo the credential bundle it received on stdin.
				return authErr(fmt.Errorf("remote auth push failed via SSH: %w", err))
			}

			var remote map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(remoteStdout.Bytes()), &remote); err != nil {
				return authErr(errors.New("remote auth push completed but returned an invalid verification response"))
			}
			remoteStatus := map[string]bool{
				"token":          remote["token"] == true,
				"refresh_token":  remote["refresh_token"] == true,
				"renewable":      remote["renewable"] == true,
				"uuid":           remote["uuid"] == true,
				"member_type":    remote["member_type"] == true,
				"request_ready":  remote["request_ready"] == true,
				"headless_ready": remote["headless_ready"] == true,
			}
			verified := remoteStatus["refresh_token"] && remoteStatus["renewable"] && remoteStatus["headless_ready"]
			if !verified {
				return authErr(errors.New("remote session did not verify as renewable and headless-ready"))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"pushed": true, "remote_verified": true, "ssh_target": sshTarget,
					"remote": remoteStatus,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Renewable session pushed to %s and verified headless-ready.\n", sshTarget)
			fmt.Fprintln(cmd.OutOrStdout(), "This remote host now owns the rotating refresh-token family; stop using the source session.")
			return nil
		},
	}
	cmd.Flags().StringVar(&sshTarget, "ssh-target", "", "Remote SSH host or user@host receiving and owning the session")
	return cmd
}

// pp:data-source live
func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:         "refresh",
		Short:       "Mint and persist a new access token from the stored refresh token",
		Example:     "  wework-pp-cli auth refresh\n  wework-pp-cli auth refresh --force --json",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.ApplyWeworkAuthBootstrap()
			hasRefresh, renewable := cfg.WeworkRefreshStatus()
			if !hasRefresh {
				return authErr(errors.New("no refresh token configured; import a complete session with 'auth session-import --stdin'"))
			}
			if !renewable {
				return authErr(errors.New("refresh token is present, but the access token lacks a trusted WeWork issuer/client id; import the complete session again"))
			}
			var refreshed bool
			if force {
				refreshed, err = cfg.RefreshWeworkTokenNow(nil)
			} else {
				refreshed, err = cfg.RefreshWeworkTokenIfNeeded(nil)
			}
			if err != nil {
				return authErr(fmt.Errorf("refreshing WeWork session: %w", err))
			}
			_, _, _, exp := cfg.ComposedAuthStatus()
			if flags.asJSON {
				out := map[string]any{"refreshed": refreshed, "refresh_token": true, "renewable": true}
				if !exp.IsZero() {
					out["token_expires"] = exp.UTC().Format(time.RFC3339)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if refreshed {
				fmt.Fprintln(cmd.OutOrStdout(), "Access token refreshed; rotated refresh token saved.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Access token is still valid; use --force to rotate it now.")
			}
			if !exp.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "Token expires: %s (%s)\n", exp.Local().Format("2006-01-02 15:04 MST"), humanUntil(exp))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Refresh even when the access token is still valid")
	return cmd
}

func newAuthWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "whoami",
		Short:       "Show which composed-auth values are set and when the token expires",
		Example:     "  wework-pp-cli auth whoami\n  wework-pp-cli auth whoami --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.ApplyWeworkAuthBootstrap()
			hasT, hasU, hasM, exp := cfg.ComposedAuthStatus()
			expired := !exp.IsZero() && time.Now().After(exp)
			hasR, renewable := cfg.WeworkRefreshStatus()
			configured := hasT && hasU && hasM
			requestReady := configured && !expired
			refreshRequired := configured && expired && renewable
			headlessReady := configured && (!expired || renewable)
			if flags.asJSON {
				out := map[string]any{
					"token": hasT, "refresh_token": hasR, "renewable": renewable,
					"uuid": hasU, "member_type": hasM,
					"ready": requestReady, "request_ready": requestReady,
					"headless_ready": headlessReady, "refresh_required": refreshRequired,
				}
				if !exp.IsZero() {
					out["token_expires"] = exp.UTC().Format(time.RFC3339)
					out["token_expired"] = expired
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "token:       %s\n", presence(hasT))
			fmt.Fprintf(w, "refresh:     %s\n", presence(hasR))
			fmt.Fprintf(w, "renewable:   %t\n", renewable)
			fmt.Fprintf(w, "headless:    %t\n", headlessReady)
			fmt.Fprintf(w, "uuid:        %s\n", presence(hasU))
			fmt.Fprintf(w, "member-type: %s\n", presence(hasM))
			if !exp.IsZero() {
				state := humanUntil(exp)
				if expired && renewable {
					state = "EXPIRED — run 'auth refresh --force'"
				} else if expired {
					state = "EXPIRED — re-run 'auth session-import'"
				}
				fmt.Fprintf(w, "token expiry: %s (%s)\n", exp.Local().Format("2006-01-02 15:04 MST"), state)
			}
			if requestReady {
				fmt.Fprintln(w, "\nReady for API calls.")
			} else if refreshRequired {
				fmt.Fprintln(w, "\nHeadless-ready — the next live command will refresh before its request.")
			} else {
				fmt.Fprintln(w, "\nNot ready — run 'wework-pp-cli auth session-import' (see 'auth session-import --help').")
			}
			return nil
		},
	}
	return cmd
}

// parseComposedAuthJSON accepts the DevTools snippet output (or similar) and
// pulls token/refresh-token/uuid/member-type from common key spellings.
func parseComposedAuthJSON(data []byte) (token, refresh, uuid, member string, err error) {
	var raw map[string]any
	if err = json.Unmarshal(bytes.TrimSpace(data), &raw); err != nil {
		return "", "", "", "", err
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := raw[k]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return ""
	}
	token = pick("token", "WEWORK_TOKEN", "access_token", "accessToken", "bearer")
	refresh = pick("refreshToken", "refresh_token", "WEWORK_REFRESH_TOKEN")
	uuid = pick("uuid", "WEWORK_UUID", "CurrentAccountUUID", "accountUuid", "account_uuid")
	member = pick("memberType", "member_type", "WEWORK_MEMBER_TYPE", "WWMemberType")
	return token, refresh, uuid, member, nil
}

func presence(ok bool) string {
	if ok {
		return "set"
	}
	return "missing"
}

func humanUntil(t time.Time) string {
	d := time.Until(t)
	if d <= 0 {
		return "expired"
	}
	if d < time.Hour {
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("in %dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("in %dd", int(d.Hours())/24)
}
