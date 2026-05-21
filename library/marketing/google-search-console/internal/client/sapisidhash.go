// PATCH(crawl-stats: SAPISIDHASH compute for the Crawl Stats batchexecute
// endpoint). The public Search Console v1 API uses OAuth bearer tokens, but
// the private SearchConsoleAggReportUi.batchexecute endpoint that powers the
// Crawl Stats UI requires a Google-web SAPISIDHASH header. The hash is
// deterministic from (unix_ts, SAPISID cookie value, origin). See
// .printing-press-patches.json patch id: crawl-stats and the discovery report
// at manuscripts/google-search-console/amend-2026-05-21T1402/.

package client

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"
)

// SAPISIDHash returns the value for the Authorization header expected by
// Google's logged-in web endpoints (search.google.com, mail.google.com, etc.).
//
// Format: "SAPISIDHASH <ts>_<sha1hex(ts + " " + sapisid + " " + origin)>"
//
// `ts` is the current Unix timestamp in seconds. `sapisid` is the value of
// the user's SAPISID cookie. `origin` is the scheme+host of the page making
// the request — for Crawl Stats this is "https://search.google.com".
//
// The Authorization header MUST be paired with the seven Google session
// cookies (SAPISID, __Secure-1PSID, __Secure-3PSID, SID, HSID, SSID, APISID)
// for the request to authenticate.
func SAPISIDHash(sapisid, origin string, now time.Time) string {
	ts := fmt.Sprintf("%d", now.Unix())
	payload := ts + " " + sapisid + " " + origin
	sum := sha1.Sum([]byte(payload))
	return "SAPISIDHASH " + ts + "_" + hex.EncodeToString(sum[:])
}
