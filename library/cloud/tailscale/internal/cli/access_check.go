// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type accessPreviewView struct {
	To struct {
		Target string     `json:"target"`
		Device *deviceRef `json:"device,omitempty"`
		IP     string     `json:"ip"`
		Port   string     `json:"port"`
	} `json:"to"`
	From          string                 `json:"from,omitempty"`
	PolicySource  string                 `json:"policy_source"`
	Matches       []tsadmin.PreviewMatch `json:"matches"`
	Allowed       *bool                  `json:"allowed,omitempty"`
	Indeterminate bool                   `json:"indeterminate,omitempty"`
	Note          string                 `json:"note,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
}

// fromUserWarning checks --from against the tailnet's members and shared-in
// users. The preview endpoint evaluates any login string, so a typo matches a
// "*" rule and reads as allowed; the warning makes that visible. Selectors
// such as group:, tag:, and autogroup: are not logins and are not checked.
func fromUserWarning(ctx context.Context, c *client.Client, tailnet, from string) string {
	if !strings.Contains(from, "@") || strings.Contains(from, ":") {
		return ""
	}
	resp, err := getLive[struct {
		Users []struct {
			LoginName string `json:"loginName"`
		} `json:"users"`
	}](ctx, c, tailnetPath(tailnet, "/users"), map[string]string{"type": "all"}, "users")
	if err != nil {
		return fmt.Sprintf("could not confirm %s is a user of this tailnet (%v); the result shows what the policy allows that login", from, err)
	}
	for _, u := range resp.Users {
		if strings.EqualFold(u.LoginName, from) {
			return ""
		}
	}
	return fmt.Sprintf("%s is not a member or shared-in user of this tailnet; the result shows what the policy would allow that login, so check it for a typo", from)
}

func newNovelAccessCheckCmd(flags *rootFlags) *cobra.Command {
	var tailnet, to, from, policyFile string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Show which policy rules let a user reach a device and port, against the live policy or a candidate file.",
		Long: strings.TrimSpace(`
Use this command to check which policy rules let a user reach a device and
port, against the live policy or a candidate file. Do NOT use this command to
change the policy; use 'policy add-entry' instead.

--to takes <device-or-ip>:<port>; a device is resolved to its Tailscale IPv4
address. The rules come from the API's preview endpoint, evaluated
server-side. With --from <user>, only rules that match both the user and the
target are kept (intersected by policy line), and allowed reports whether any
rule matched. A --from login that is not a member or shared-in user of the
tailnet gets a warning, since a typo still matches a "*" rule. If a matched
line holds more than one rule, the result is marked indeterminate instead. --policy-file previews a candidate policy instead of the live
one, so you can check access before writing a change.`),
		Example: strings.Trim(`
  tailscale-pp-cli access check --to nas:445
  tailscale-pp-cli access check --to self:22 --agent
  tailscale-pp-cli access check --to nas:445 --policy-file candidate.hujson`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--to=self:22"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "access check")
			}
			if strings.TrimSpace(to) == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("--to <device-or-ip>:<port> is required"))
			}
			host, port, err := tsadmin.ParseTarget(to)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := tsLiveClient(ctx, flags)
			if err != nil {
				return err
			}
			var view accessPreviewView
			view.To.Target, view.To.Port, view.From = to, port, from
			if addr, err := netip.ParseAddr(host); err == nil {
				view.To.IP = addr.String()
			} else {
				devs, err := fetchDevicesLive(ctx, c, tailnet, false)
				if err != nil {
					return err
				}
				d, err := resolveDevice(ctx, devs, host)
				if err != nil {
					return err
				}
				ref := refOf(d)
				view.To.Device = &ref
				view.To.IP = d.FirstIPv4()
				if view.To.IP == "" {
					return usageErr(fmt.Errorf("%s has no Tailscale address", d.Label()))
				}
			}

			// Send the HuJSON text itself so preview line numbers refer to
			// text this command holds, which lets it detect lines that hold
			// more than one rule.
			var policy []byte
			if policyFile != "" {
				raw, err := os.ReadFile(filepath.Clean(policyFile))
				if err != nil {
					return usageErr(fmt.Errorf("reading --policy-file: %w", err))
				}
				if _, err := tsadmin.StandardizeJSON(raw); err != nil {
					return usageErr(fmt.Errorf("--policy-file is not valid HuJSON: %w", err))
				}
				policy = raw
				view.PolicySource = "file:" + policyFile
			} else {
				snap, err := fetchPolicyHuJSON(ctx, c, tailnet)
				if err != nil {
					return err
				}
				policy = snap.Text
				view.PolicySource = "live"
			}
			preview := func(kind, value string) ([]tsadmin.PreviewMatch, error) {
				q := url.Values{}
				q.Set("type", kind)
				q.Set("previewFor", value)
				r, err := tsRawRequest(ctx, c, http.MethodPost, tailnetPath(tailnet, "/acl/preview")+"?"+q.Encode(), policy, map[string]string{"Content-Type": "application/hujson", "Accept": "application/json"})
				if err != nil {
					return nil, err
				}
				if r.Status != http.StatusOK {
					return nil, tsHTTPError("previewing rule matches", r)
				}
				var resp struct {
					Matches []tsadmin.PreviewMatch `json:"matches"`
				}
				if err := json.Unmarshal(r.Body, &resp); err != nil {
					return nil, apiErr(fmt.Errorf("decoding preview: %w", err))
				}
				if resp.Matches == nil {
					resp.Matches = []tsadmin.PreviewMatch{}
				}
				return resp.Matches, nil
			}
			ipMatches, err := preview("ipport", net.JoinHostPort(view.To.IP, port))
			if err != nil {
				return err
			}
			view.Matches = ipMatches
			if from != "" {
				userMatches, err := preview("user", from)
				if err != nil {
					return err
				}
				view.Matches = tsadmin.IntersectByLine(ipMatches, userMatches)
				if warn := fromUserWarning(ctx, c, tailnet, from); warn != "" {
					view.Warnings = append(view.Warnings, warn)
				}
				ruleLines, err := tsadmin.RuleStartLines(policy)
				if err != nil {
					return usageErr(fmt.Errorf("parsing policy: %w", err))
				}
				if amb := tsadmin.AmbiguousLines(view.Matches, ruleLines); len(amb) > 0 {
					view.Indeterminate = true
					view.Note = fmt.Sprintf("indeterminate: policy line(s) %v hold more than one rule, so a user match and a target match on the same line may be different rules; put one rule per line to get a definite answer", amb)
				} else {
					allowed := len(view.Matches) > 0
					view.Allowed = &allowed
				}
			}
			if len(view.Matches) == 0 && view.Note == "" {
				view.Note = "no rule in the policy matches this target, so the connection is not allowed by this policy"
			}
			if ok, err := emitMachine(cmd, flags, view); ok {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "access to %s (%s:%s), policy %s\n", view.To.Target, view.To.IP, view.To.Port, view.PolicySource)
			if view.Allowed != nil {
				fmt.Fprintf(w, "  from %s: allowed=%t\n", view.From, *view.Allowed)
			}
			for _, warn := range view.Warnings {
				fmt.Fprintf(w, "  warning: %s\n", warn)
			}
			if view.Indeterminate {
				fmt.Fprintf(w, "  %s\n", view.Note)
			}
			if len(view.Matches) == 0 {
				fmt.Fprintf(w, "  %s\n", view.Note)
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "  LINE\tSOURCES\tDESTINATIONS")
			for _, m := range view.Matches {
				fmt.Fprintf(tw, "  %d\t%s\t%s\n", m.LineNumber, joinOrDash(m.Users), joinOrDash(m.Ports))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "Target as <device-or-ip>:<port>, e.g. nas:445 or 100.64.0.5:22")
	cmd.Flags().StringVar(&from, "from", "", "Only rules that also match this user (login name)")
	cmd.Flags().StringVar(&policyFile, "policy-file", "", "Preview a candidate HuJSON policy file instead of the live policy")
	addTailnetFlag(cmd, &tailnet)
	return cmd
}
