// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written in-binary transport allowlist (grill R3-C6). Every outgoing
// request is checked at the single transport choke point (doInternal);
// anything not on the hard-coded list of permitted Gmail operations is
// refused before URL building, auth minting, or dialing. This is a
// structural guarantee, not a policy file: no config, env var, or flag can
// widen it — widening requires editing this source and recompiling.
//
// The list is exactly the 19 operations the cleanup engine and its read
// surface need. Notably absent by construction: message sending, draft
// management, settings (filters/forwarding/vacation), permanent message
// deletion (single or batch), and label deletion.

package client

import (
	"fmt"
	"regexp"
	"strings"
)

// allowedSeg matches one path segment: a resolved id, "me", an email
// address, or an unresolved "{userId}" template marker (template
// substitution happens later, in buildURL).
const allowedSeg = `[^/?#]+`

type allowedOp struct {
	name   string
	method string
	re     *regexp.Regexp
}

func mustOp(name, method, pattern string) allowedOp {
	return allowedOp{name: name, method: method, re: regexp.MustCompile(pattern)}
}

// transportAllowlist is the closed set of permitted (method, path) shapes.
// Path patterns are matched against the request path with any query string
// stripped, anchored at both ends.
var transportAllowlist = []allowedOp{
	mustOp("profile.get", "GET", `^/gmail/v1/users/`+allowedSeg+`/profile$`),

	mustOp("messages.list", "GET", `^/gmail/v1/users/`+allowedSeg+`/messages$`),
	mustOp("messages.get", "GET", `^/gmail/v1/users/`+allowedSeg+`/messages/`+allowedSeg+`$`),
	mustOp("attachments.get", "GET", `^/gmail/v1/users/`+allowedSeg+`/messages/`+allowedSeg+`/attachments/`+allowedSeg+`$`),
	mustOp("messages.modify", "POST", `^/gmail/v1/users/`+allowedSeg+`/messages/`+allowedSeg+`/modify$`),
	mustOp("messages.trash", "POST", `^/gmail/v1/users/`+allowedSeg+`/messages/`+allowedSeg+`/trash$`),
	mustOp("messages.untrash", "POST", `^/gmail/v1/users/`+allowedSeg+`/messages/`+allowedSeg+`/untrash$`),
	mustOp("messages.batchModify", "POST", `^/gmail/v1/users/`+allowedSeg+`/messages/batchModify$`),

	mustOp("threads.list", "GET", `^/gmail/v1/users/`+allowedSeg+`/threads$`),
	mustOp("threads.get", "GET", `^/gmail/v1/users/`+allowedSeg+`/threads/`+allowedSeg+`$`),
	mustOp("threads.modify", "POST", `^/gmail/v1/users/`+allowedSeg+`/threads/`+allowedSeg+`/modify$`),
	mustOp("threads.trash", "POST", `^/gmail/v1/users/`+allowedSeg+`/threads/`+allowedSeg+`/trash$`),
	mustOp("threads.untrash", "POST", `^/gmail/v1/users/`+allowedSeg+`/threads/`+allowedSeg+`/untrash$`),

	mustOp("labels.list", "GET", `^/gmail/v1/users/`+allowedSeg+`/labels$`),
	mustOp("labels.get", "GET", `^/gmail/v1/users/`+allowedSeg+`/labels/`+allowedSeg+`$`),
	mustOp("labels.create", "POST", `^/gmail/v1/users/`+allowedSeg+`/labels$`),
	mustOp("labels.update", "PUT", `^/gmail/v1/users/`+allowedSeg+`/labels/`+allowedSeg+`$`),
	mustOp("labels.patch", "PATCH", `^/gmail/v1/users/`+allowedSeg+`/labels/`+allowedSeg+`$`),

	mustOp("history.list", "GET", `^/gmail/v1/users/`+allowedSeg+`/history$`),
}

// TransportRefusedError reports a request shape outside the allowlist.
type TransportRefusedError struct {
	Method string
	Path   string
}

func (e *TransportRefusedError) Error() string {
	return fmt.Sprintf(
		"transport allowlist refused %s %s: not one of the %d Gmail operations this binary is permitted to perform (the allowlist is compiled in; see internal/client/allowlist.go)",
		e.Method, e.Path, len(transportAllowlist))
}

// checkTransportAllowlist returns nil when (method, path) matches a
// permitted operation and a *TransportRefusedError otherwise. The query
// string (and any fragment) is ignored for matching; only the path shape
// and method are load-bearing.
func checkTransportAllowlist(method, path string) error {
	trimmed := path
	if i := strings.IndexAny(trimmed, "?#"); i >= 0 {
		trimmed = trimmed[:i]
	}
	m := strings.ToUpper(method)
	for _, op := range transportAllowlist {
		if op.method == m && op.re.MatchString(trimmed) {
			return nil
		}
	}
	return &TransportRefusedError{Method: m, Path: trimmed}
}
