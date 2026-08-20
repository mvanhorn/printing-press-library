package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// doctorTokenProbe is one row in the per-spec-family reachability matrix.
type doctorTokenProbe struct {
	Family   string `json:"family"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	OK       bool   `json:"ok"`
	Reason   string `json:"reason,omitempty"`
	TierHint string `json:"tier_hint,omitempty"`
}

func newDoctorTokenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Probe one GET per spec family to map what this credential can reach",
		Long: `Probes one representative GET per Roam spec family (HQ, On-Air, Chat, SCIM, Webhooks)
and prints a 5-row matrix of which families this token can reach. Personal Access Tokens
typically only see the v0 surface; full org access reaches v1 and SCIM.

Exit codes:
  0  every family reachable
  2  one or more families unreachable (PAT vs full-access asymmetry)`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
			"mcp:read-only":       "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			probes := []doctorTokenProbe{
				{Family: "HQ", Path: "/token.info"},
				{Family: "On-Air", Path: "/onair.event.list"},
				{Family: "Chat", Path: "/chat.list"},
				{Family: "SCIM", Path: "/Users"},
				{Family: "Webhooks", Path: "/webhook.subscribe"},
			}
			anyFail := false
			for i := range probes {
				p := &probes[i]
				code, err := c.ProbeGet(p.Path)
				p.Status = code
				switch {
				case err != nil && code == 0:
					p.OK = false
					p.Reason = err.Error()
				case code >= 200 && code < 300:
					p.OK = true
				case code == 401:
					p.OK = false
					p.Reason = "unauthorized"
				case code == 403:
					p.OK = false
					p.Reason = "forbidden (likely PAT vs full-access tier)"
					p.TierHint = "requires full org access"
				case code == 404:
					p.OK = false
					p.Reason = "not found"
				case code == 405:
					// Probe used GET on a POST-only endpoint; reachability proven.
					p.OK = true
					p.Reason = "endpoint reachable (POST-only)"
				default:
					p.OK = false
					p.Reason = fmt.Sprintf("HTTP %d", code)
				}
				if !p.OK {
					anyFail = true
				}
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				out, _ := json.Marshal(map[string]any{"probes": probes, "ok": !anyFail})
				fmt.Fprintln(w, string(out))
			} else {
				fmt.Fprintf(w, "%-10s  %-25s  %-6s  %s\n", "FAMILY", "PROBE", "STATUS", "REASON")
				for _, p := range probes {
					mark := "ok"
					if !p.OK {
						mark = "FAIL"
					}
					reason := p.Reason
					if reason == "" {
						reason = "ok"
					}
					fmt.Fprintf(w, "%-10s  %-25s  %-6d  %s %s\n", p.Family, p.Path, p.Status, mark, reason)
				}
				if anyFail {
					fmt.Fprintln(w)
					fmt.Fprintln(w, "Hint: Personal Access Tokens cannot reach v1 endpoints (groups, recordings, users, audit log).")
					fmt.Fprintln(w, "      Use a full-org-access bot token to reach every family.")
				}
			}
			if anyFail {
				return &cliError{code: 2, err: fmt.Errorf("one or more spec families unreachable")}
			}
			return nil
		},
	}
	_ = strings.TrimSpace
	return cmd
}
