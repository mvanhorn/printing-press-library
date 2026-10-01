// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package tsadmin

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Invite is a device share invite as returned by the API.
type Invite struct {
	ID              string `json:"id"`
	Created         string `json:"created"`
	DeviceID        int64  `json:"deviceId"`
	SharerID        int64  `json:"sharerId"`
	MultiUse        bool   `json:"multiUse"`
	AllowExitNode   bool   `json:"allowExitNode"`
	Email           string `json:"email"`
	LastEmailSentAt string `json:"lastEmailSentAt"`
	InviteURL       string `json:"inviteUrl"`
	Accepted        bool   `json:"accepted"`
	AcceptedBy      *struct {
		ID        int64  `json:"id"`
		LoginName string `json:"loginName"`
	} `json:"acceptedBy"`
}

// AcceptedByLogin returns the acceptor's login name, or "".
func (i Invite) AcceptedByLogin() string {
	if i.AcceptedBy == nil {
		return ""
	}
	return i.AcceptedBy.LoginName
}

// Redeemable reports whether someone holding the invite URL could still
// accept it: unaccepted invites, and multi-use invites even after one use.
func (i Invite) Redeemable() bool { return !i.Accepted || i.MultiUse }

// ShareFilter selects invites. Zero value matches everything.
type ShareFilter struct {
	Pending    bool
	Accepted   bool
	AcceptedBy string
}

// Match reports whether inv passes the filter.
func (f ShareFilter) Match(inv Invite) bool {
	if f.Pending && inv.Accepted {
		return false
	}
	if f.Accepted && !inv.Accepted {
		return false
	}
	if f.AcceptedBy != "" && !strings.EqualFold(inv.AcceptedByLogin(), f.AcceptedBy) {
		return false
	}
	return true
}

// RedactInviteURL keeps the host and path prefix but hides the code, since
// anyone holding the full URL can accept the invite.
func RedactInviteURL(u string) string {
	if u == "" {
		return ""
	}
	idx := strings.LastIndex(u, "/")
	if idx < 0 || idx == len(u)-1 {
		return "<redacted>"
	}
	return u[:idx+1] + "<redacted>"
}

// PreviewMatch is one rule match from POST /acl/preview.
type PreviewMatch struct {
	Users      []string `json:"users"`
	Ports      []string `json:"ports"`
	LineNumber int      `json:"lineNumber"`
}

// IntersectByLine keeps matches from a whose rule line also appears in b.
func IntersectByLine(a, b []PreviewMatch) []PreviewMatch {
	lines := map[int]bool{}
	for _, m := range b {
		lines[m.LineNumber] = true
	}
	out := make([]PreviewMatch, 0)
	for _, m := range a {
		if lines[m.LineNumber] {
			out = append(out, m)
		}
	}
	return out
}

// ParseTarget splits "<host>:<port>" where host may be a device selector, an
// IPv4 address, or a bracketed IPv6 address. The port must be 1-65535.
func ParseTarget(s string) (host, port string, err error) {
	s = strings.TrimSpace(s)
	h, p, err := net.SplitHostPort(s)
	if err != nil {
		return "", "", fmt.Errorf("target %q must look like <device-or-ip>:<port> (e.g. nas:445)", s)
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("target %q has an invalid port %q", s, p)
	}
	if h == "" {
		return "", "", fmt.Errorf("target %q is missing a device or IP", s)
	}
	return h, p, nil
}

// AmbiguousLines returns the matched lines that hold more than one rule.
func AmbiguousLines(matches []PreviewMatch, ruleLines map[int]int) []int {
	out := make([]int, 0)
	for _, m := range matches {
		if ruleLines[m.LineNumber] > 1 {
			out = append(out, m.LineNumber)
		}
	}
	return out
}
