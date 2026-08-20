// Package zoomurl builds and parses zoommtg:// / zoomus:// URL-scheme links
// the Zoom desktop client registers as a handler for. Used by join, start,
// today, and the "saved add-from-url" parser.
package zoomurl

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Action discriminates start vs. join semantics in the URL scheme. Zoom uses
// "start" for the host launching a meeting, "join" for everyone else.
type Action string

const (
	ActionJoin  Action = "join"
	ActionStart Action = "start"
)

// Params captures everything we know how to round-trip through the URL scheme.
// Empty fields are omitted from the rendered URL. The "Encrypted" flag tells
// the caller whether Pwd is the URL-shaped encrypted_password (which Zoom's
// own web UI emits and the desktop client cannot consume directly) versus the
// raw numeric/string password the URL scheme expects.
type Params struct {
	Action    Action
	ConfNo    string
	Pwd       string // unencrypted; if Encrypted is true the URL scheme will not work
	Encrypted bool
	Uname     string
	UID       string // host user ID (only used with action=start)
	ZakToken  string // Zoom Access Key, needed to start as host without re-auth
	Stype     string // historical; "101" appears in many official invites
}

// Build returns a "zoommtg://zoom.us/<action>?…" URL ready for `open`,
// `xdg-open`, or `start`. Returns an error when ConfNo is empty (the desktop
// app cannot resolve a meeting without one) or when Encrypted is set (the URL
// scheme requires the unencrypted form).
func Build(p Params) (string, error) {
	if p.ConfNo == "" {
		return "", errors.New("zoomurl: ConfNo (meeting ID) is required")
	}
	if p.Encrypted {
		return "", errors.New("zoomurl: Pwd is marked Encrypted; the URL scheme requires the unencrypted password")
	}
	action := p.Action
	if action == "" {
		action = ActionJoin
	}
	q := url.Values{}
	q.Set("confno", strings.TrimSpace(p.ConfNo))
	if p.Pwd != "" {
		q.Set("pwd", p.Pwd)
	}
	if p.Uname != "" {
		q.Set("uname", p.Uname)
	}
	if p.UID != "" {
		q.Set("uid", p.UID)
	}
	if p.ZakToken != "" {
		q.Set("token", p.ZakToken)
	}
	if p.Stype != "" {
		q.Set("stype", p.Stype)
	}
	return fmt.Sprintf("zoommtg://zoom.us/%s?%s", action, q.Encode()), nil
}

// joinURLPattern matches the common shapes a Zoom meeting URL takes. The order
// of capture groups is (scheme-host, path-id, query). We deliberately accept
// any zoom.us subdomain (us02web, us04web, eu01web, …) and any path that
// starts with /j/, /s/, or /my/ (personal link).
var joinURLPattern = regexp.MustCompile(`https?://(?:[a-z0-9-]+\.)?zoom\.us/(?:j|s|my)/([^?#]+)(?:\?([^#]*))?`)

// schemePattern matches the zoommtg:// / zoomus:// URL forms.
var schemePattern = regexp.MustCompile(`^(?:zoommtg|zoomus)://[^/]*/(join|start)\?(.+)$`)

// Parse extracts Params from any Zoom URL shape we recognise:
//
//   - https://*.zoom.us/j/<id>[?pwd=…&uname=…]
//   - https://*.zoom.us/s/<id>[?…]
//   - https://*.zoom.us/my/<personal-link>
//   - zoommtg://zoom.us/join?confno=…&pwd=…
//   - zoomus://zoom.us/start?confno=…&token=…
//
// The pwd query parameter can be either the raw form (what the URL scheme
// consumes) or Zoom's URL-shaped "encrypted_password" — the encrypted form
// is materially longer (often 30+ chars with mixed case + symbols vs. a
// 6-digit numeric raw form) so we surface the distinction via Params.Encrypted
// rather than guessing on the user's behalf.
func Parse(raw string) (Params, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Params{}, errors.New("zoomurl: empty input")
	}

	if m := schemePattern.FindStringSubmatch(raw); m != nil {
		q, err := url.ParseQuery(m[2])
		if err != nil {
			return Params{}, fmt.Errorf("zoomurl: parsing query: %w", err)
		}
		p := fromQuery(Action(m[1]), q.Get("confno"), q)
		if raw := extractRawPwd(m[2]); raw != "" {
			p.Pwd = raw
			p.Encrypted = looksEncrypted(raw)
		}
		return p, nil
	}

	if m := joinURLPattern.FindStringSubmatch(raw); m != nil {
		confno := m[1]
		// Strip trailing slash if path was /my/<personal-link>/
		confno = strings.TrimRight(confno, "/")
		var q url.Values
		rawQuery := ""
		if len(m) > 2 && m[2] != "" {
			rawQuery = m[2]
			parsed, err := url.ParseQuery(m[2])
			if err != nil {
				return Params{}, fmt.Errorf("zoomurl: parsing query: %w", err)
			}
			q = parsed
		} else {
			q = url.Values{}
		}
		p := fromQuery(ActionJoin, confno, q)
		if rp := extractRawPwd(rawQuery); rp != "" {
			p.Pwd = rp
			p.Encrypted = looksEncrypted(rp)
		}
		return p, nil
	}

	// Bare numeric ID — accept as a join target with no other params.
	if matched, _ := regexp.MatchString(`^[0-9 -]{9,15}$`, raw); matched {
		return Params{
			Action: ActionJoin,
			ConfNo: stripIDFormatting(raw),
		}, nil
	}

	return Params{}, fmt.Errorf("zoomurl: unrecognised Zoom URL shape: %q", raw)
}

func fromQuery(action Action, confno string, q url.Values) Params {
	// Preserve the literal pwd as it appears in the raw query string. Zoom's
	// encrypted_password format frequently contains literal `+` (base64) which
	// url.ParseQuery would decode to space. We re-extract pwd manually.
	pwd := q.Get("pwd")
	return Params{
		Action:    action,
		ConfNo:    stripIDFormatting(confno),
		Pwd:       pwd,
		Encrypted: looksEncrypted(pwd),
		Uname:     q.Get("uname"),
		UID:       q.Get("uid"),
		ZakToken:  q.Get("token"),
		Stype:     q.Get("stype"),
	}
}

// extractRawPwd pulls the pwd value out of a raw query string without
// URL-decoding the `+` character. The result is what the user literally typed
// into the URL bar. Returns "" when no pwd= field is present.
func extractRawPwd(rawQuery string) string {
	for _, kv := range strings.Split(rawQuery, "&") {
		if !strings.HasPrefix(kv, "pwd=") {
			continue
		}
		return strings.TrimPrefix(kv, "pwd=")
	}
	return ""
}

func stripIDFormatting(s string) string {
	r := strings.NewReplacer(" ", "", "-", "")
	return r.Replace(s)
}

// looksEncrypted heuristically distinguishes Zoom's URL-shaped encrypted_password
// (long, mixed case, contains symbols) from the raw form (typically 6 digits, or a
// short alphanumeric word). False positives are tolerable — the only consequence is
// the URL scheme will refuse to launch, which is what we want to surface to the user.
func looksEncrypted(pwd string) bool {
	if pwd == "" {
		return false
	}
	if len(pwd) > 12 {
		return true
	}
	hasUpper := strings.ContainsAny(pwd, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasSymbol := strings.ContainsAny(pwd, "+/=._-")
	return hasUpper && hasSymbol
}
