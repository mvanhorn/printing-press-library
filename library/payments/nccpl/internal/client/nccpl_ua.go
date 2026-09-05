package client

import (
	"net/http"
	"os"
	"strings"
)

// nccplClearanceUA is the User-Agent the Cloudflare clearance cookie is bound to.
//
// Cloudflare ties cf_clearance to the exact User-Agent that solved the challenge and
// rejects the cookie when a later request presents a different one. Surf v1.0.199
// impersonates Chrome 145, while the browser that mints the cookie here is Chrome 149,
// so the replay is refused with a fresh "Just a moment..." challenge even though the
// cookie is valid and on the wire.
//
// Override it with NCCPL_USER_AGENT when the local Chrome moves to a new major, so a
// browser upgrade does not require rebuilding the CLI.
const nccplClearanceUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

// applyNCCPLClearanceUA pins the request User-Agent to the one that earned the
// clearance cookie. Applied after Surf's impersonation so it wins.
func applyNCCPLClearanceUA(req *http.Request) {
	if req == nil {
		return
	}
	ua := strings.TrimSpace(os.Getenv("NCCPL_USER_AGENT"))
	if ua == "" {
		ua = nccplClearanceUA
	}
	req.Header.Set("User-Agent", ua)
}
