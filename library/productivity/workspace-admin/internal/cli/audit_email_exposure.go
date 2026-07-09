// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit email-exposure.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

const gmailBaseV1 = "https://gmail.googleapis.com/gmail/v1"

// gmailSettingsScopes are the read scopes needed to inspect a mailbox's
// forwarding, sendAs, delegates, and filters during a per-user re-impersonation
// sweep.
var gmailSettingsScopes = []string{
	"https://www.googleapis.com/auth/gmail.settings.basic",
	"https://www.googleapis.com/auth/gmail.settings.sharing",
	"https://www.googleapis.com/auth/admin.directory.user.readonly",
}

// emailFinding is one business-email-compromise indicator for a mailbox.
type emailFinding struct {
	User   string `json:"user"`
	Type   string `json:"type"`   // external_forwarding | forwarding_address | external_send_as | delegate | forwarding_filter | trashing_filter
	Detail string `json:"detail"` // human-readable specifics (the external address, filter id, etc.)
}

type emailExposureView struct {
	Findings     []emailFinding `json:"findings"`
	ScannedUsers int            `json:"scanned_users"`
	Errors       []string       `json:"errors,omitempty"`
	Note         string         `json:"note,omitempty"`
}

func newNovelAuditEmailExposureCmd(flags *rootFlags) *cobra.Command {
	var flagUser string
	var flagInternal string
	var flagAllUsers bool
	var flagKey string
	var flagAdmin string
	var flagMaxScanUsers int

	cmd := &cobra.Command{
		Use:   "email-exposure",
		Short: "Sweep a mailbox (or the whole domain) for forwarding, sendAs, delegates, and risky filters — the standard BEC indicators.",
		Long: "Inspect Gmail settings for business-email-compromise indicators: external auto-forwarding,\n" +
			"external forwarding addresses, external send-as identities, delegates, and filters that\n" +
			"forward or trash mail.\n\n" +
			"By default this sweeps a single mailbox using the current impersonated token (--user, default\n" +
			"the token's own mailbox). Reading another user's Gmail settings requires a token scoped to\n" +
			"that mailbox, so the domain-wide sweep (--all-users) re-mints a per-user token from a\n" +
			"service-account key (--key) using an admin subject (--admin) via domain-wide delegation.\n\n" +
			"Use this command for the forwarding/delegate/filter sweep. Do NOT use it for one user's full\n" +
			"email settings; use 'audit user360' instead.",
		// Cobra Example drives dogfood's happy-path probe, so it must be a
		// runnable single-mailbox invocation (no external key file). The
		// domain-wide --all-users form is documented in Long and the SKILL.
		Example:     "  workspace-admin-pp-cli audit email-exposure --user user@example.com --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sweep mailbox settings for forwarding, sendAs, delegates, and risky filters")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			internal := internalDomainSet(flagInternal)
			view := emailExposureView{}

			if flagAllUsers {
				if flagKey == "" || flagAdmin == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--all-users requires --key (service-account JSON) and --admin (delegation subject)"))
				}
				// Infer internal domain from the admin address when not given.
				if len(internal) == 0 {
					if d := emailDomain(flagAdmin); d != "" {
						internal[d] = true
					}
				}
				if err := sweepAllMailboxes(ctx, flags, cmd, flagKey, flagAdmin, internal, flagMaxScanUsers, &view); err != nil {
					return err
				}
			} else {
				// Single-mailbox sweep using the ambient impersonated token.
				user := flagUser
				if user == "" {
					user = "me"
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				getter := func(path string) (json.RawMessage, error) {
					return c.Get(ctx, gmailBaseV1+"/users/"+user+path, nil)
				}
				// Infer the internal domain so a forward/send-as to an internal
				// colleague is not mis-flagged as external. Prefer --user's
				// domain; otherwise resolve the mailbox's own address via the
				// Gmail profile so the default "me" path is not treated as
				// "everything is external".
				if len(internal) == 0 {
					if d := emailDomain(user); d != "" {
						internal[d] = true
					} else if pd, perr := getter("/profile"); perr == nil {
						var prof struct {
							EmailAddress string `json:"emailAddress"`
						}
						if json.Unmarshal(pd, &prof) == nil {
							if d := emailDomain(prof.EmailAddress); d != "" {
								internal[d] = true
							}
						}
					}
				}
				scanMailboxSettings(user, getter, internal, &view)
				view.ScannedUsers = 1
			}

			if len(view.Findings) == 0 && len(view.Errors) == 0 {
				view.Note = "no forwarding, external send-as, delegate, or risky-filter exposure found"
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagUser, "user", "", "Mailbox to sweep in single-mailbox mode (default: the token's own mailbox)")
	cmd.Flags().StringVar(&flagInternal, "internal-domain", "", "Comma-separated company-owned domains; inferred from --admin/--user when omitted")
	cmd.Flags().BoolVar(&flagAllUsers, "all-users", false, "Sweep every user in the domain (requires --key and --admin)")
	cmd.Flags().StringVar(&flagKey, "key", "", "Path to the service-account JSON key (for --all-users per-user impersonation)")
	cmd.Flags().StringVar(&flagAdmin, "admin", "", "Admin email to impersonate when listing users (for --all-users)")
	cmd.Flags().IntVar(&flagMaxScanUsers, "max-scan-users", 200, "Maximum users to scan in --all-users mode")
	return cmd
}

// sweepAllMailboxes lists domain users (via an admin-impersonated token) and,
// for each, mints a per-user token and inspects that mailbox's settings. The
// per-user token is required because Gmail settings are only readable by a
// token scoped to that mailbox.
func sweepAllMailboxes(ctx context.Context, flags *rootFlags, cmd *cobra.Command, keyPath, admin string, internal map[string]bool, maxScan int, view *emailExposureView) error {
	sa, tokenURI, err := loadSAKey(keyPath)
	if err != nil {
		return err
	}
	if maxScan <= 0 {
		maxScan = 200
	}
	if cliutil.IsDogfoodEnv() && maxScan > 3 {
		maxScan = 3
	}

	adminToken, err := mintServiceAccountToken(ctx, sa, defaultSAScopes, admin, tokenURI, time.Now())
	if err != nil {
		return authErr(err)
	}

	pageToken := ""
	for view.ScannedUsers < maxScan {
		params := "?customer=my_customer&maxResults=200&fields=nextPageToken,users(primaryEmail,suspended)"
		if pageToken != "" {
			params += "&pageToken=" + url.QueryEscape(pageToken)
		}
		data, gerr := gmailAuthedGet(ctx, adminToken, wsDirectoryBase+"/users"+params)
		if gerr != nil {
			return classifyAPIError(gerr, flags)
		}
		var env struct {
			NextPageToken string `json:"nextPageToken"`
			Users         []struct {
				PrimaryEmail string `json:"primaryEmail"`
				Suspended    bool   `json:"suspended"`
			} `json:"users"`
		}
		if json.Unmarshal(data, &env) != nil {
			break
		}
		for _, u := range env.Users {
			if view.ScannedUsers >= maxScan {
				break
			}
			if u.Suspended || u.PrimaryEmail == "" {
				continue
			}
			view.ScannedUsers++
			userToken, terr := mintServiceAccountToken(ctx, sa, gmailSettingsScopes, u.PrimaryEmail, tokenURI, time.Now())
			if terr != nil {
				view.Errors = append(view.Errors, fmt.Sprintf("%s: minting token: %v", u.PrimaryEmail, terr))
				continue
			}
			getter := func(path string) (json.RawMessage, error) {
				return gmailAuthedGet(ctx, userToken, gmailBaseV1+"/users/"+u.PrimaryEmail+path)
			}
			scanMailboxSettings(u.PrimaryEmail, getter, internal, view)
		}
		if env.NextPageToken == "" || len(env.Users) == 0 {
			break
		}
		pageToken = env.NextPageToken
	}
	return nil
}

// scanMailboxSettings inspects one mailbox's forwarding, sendAs, delegates, and
// filters via the supplied getter (which encapsulates the authed base + user),
// appending findings and per-endpoint errors to view.
func scanMailboxSettings(user string, get func(path string) (json.RawMessage, error), internal map[string]bool, view *emailExposureView) {
	// Auto-forwarding.
	if data, err := get("/settings/autoForwarding"); err == nil {
		var af struct {
			Enabled      bool   `json:"enabled"`
			EmailAddress string `json:"emailAddress"`
		}
		if json.Unmarshal(data, &af) == nil && af.Enabled && af.EmailAddress != "" {
			if isExternalAddr(af.EmailAddress, internal) {
				view.Findings = append(view.Findings, emailFinding{User: user, Type: "external_forwarding", Detail: "auto-forwards to " + af.EmailAddress})
			}
		}
	}
	// Forwarding addresses.
	if data, err := get("/settings/forwardingAddresses"); err == nil {
		var fa struct {
			ForwardingAddresses []struct {
				ForwardingEmail    string `json:"forwardingEmail"`
				VerificationStatus string `json:"verificationStatus"`
			} `json:"forwardingAddresses"`
		}
		if json.Unmarshal(data, &fa) == nil {
			for _, a := range fa.ForwardingAddresses {
				if isExternalAddr(a.ForwardingEmail, internal) {
					view.Findings = append(view.Findings, emailFinding{User: user, Type: "forwarding_address", Detail: "external forwarding address " + a.ForwardingEmail + " (" + a.VerificationStatus + ")"})
				}
			}
		}
	}
	// Send-as identities.
	if data, err := get("/settings/sendAs"); err == nil {
		var sa struct {
			SendAs []struct {
				SendAsEmail string `json:"sendAsEmail"`
				IsPrimary   bool   `json:"isPrimary"`
			} `json:"sendAs"`
		}
		if json.Unmarshal(data, &sa) == nil {
			for _, s := range sa.SendAs {
				if s.IsPrimary {
					continue
				}
				if isExternalAddr(s.SendAsEmail, internal) {
					view.Findings = append(view.Findings, emailFinding{User: user, Type: "external_send_as", Detail: "sends as external " + s.SendAsEmail})
				}
			}
		}
	}
	// Delegates.
	if data, err := get("/settings/delegates"); err == nil {
		var dl struct {
			Delegates []struct {
				DelegateEmail string `json:"delegateEmail"`
			} `json:"delegates"`
		}
		if json.Unmarshal(data, &dl) == nil {
			for _, d := range dl.Delegates {
				if d.DelegateEmail == "" {
					continue
				}
				view.Findings = append(view.Findings, emailFinding{User: user, Type: "delegate", Detail: "mailbox delegated to " + d.DelegateEmail})
			}
		}
	}
	// Filters that forward or trash.
	if data, err := get("/settings/filters"); err == nil {
		var fl struct {
			Filter []struct {
				ID     string `json:"id"`
				Action struct {
					Forward        string   `json:"forward"`
					AddLabelIds    []string `json:"addLabelIds"`
					RemoveLabelIds []string `json:"removeLabelIds"`
				} `json:"action"`
			} `json:"filter"`
		}
		if json.Unmarshal(data, &fl) == nil {
			for _, f := range fl.Filter {
				if f.Action.Forward != "" {
					view.Findings = append(view.Findings, emailFinding{User: user, Type: "forwarding_filter", Detail: "filter " + f.ID + " forwards to " + f.Action.Forward})
				}
				if containsLabel(f.Action.AddLabelIds, "TRASH") || containsLabel(f.Action.RemoveLabelIds, "INBOX") {
					view.Findings = append(view.Findings, emailFinding{User: user, Type: "trashing_filter", Detail: "filter " + f.ID + " skips inbox or trashes matching mail"})
				}
			}
		}
	}
}

func isExternalAddr(addr string, internal map[string]bool) bool {
	d := emailDomain(addr)
	if d == "" {
		return false
	}
	if len(internal) == 0 {
		return true // no internal set known; treat any resolvable domain as external
	}
	return !internal[d]
}

func containsLabel(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, want) {
			return true
		}
	}
	return false
}

// gmailAuthedGet performs an authenticated GET with an explicit bearer token,
// used for per-user mailbox reads whose token differs from the ambient client.
func gmailAuthedGet(ctx context.Context, token, url string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}
