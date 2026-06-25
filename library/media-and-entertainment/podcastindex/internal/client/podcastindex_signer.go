package client

import (
	"crypto/sha1" // #nosec G505 -- SHA-1 is mandated by the PodcastIndex auth protocol (Authorization: sha1hex(key+secret+date)); not used for integrity or secrecy
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

// signPodcastIndex applies the four PodcastIndex authentication headers to req,
// recomputing the per-request signature each call:
//
//	X-Auth-Key:    <api key>
//	X-Auth-Date:   <unix seconds, now>
//	Authorization: sha1hex(key + secret + date)
//	User-Agent:    <set if absent>
//
// The generator models these as static apiKey schemes, which cannot work for an
// API that requires a fresh timestamp + SHA1 on every request. This signer is
// the single seam that makes every generated endpoint command (and every
// hand-authored command) authenticate correctly. It overrides any static values
// set by the templated composed-auth block in doInternal.
//
// Lives in its own file so it survives generator regen.
func (c *Client) signPodcastIndex(req *http.Request) {
	if c == nil || c.Config == nil {
		return
	}
	key := c.Config.PodcastindexAuthKey()
	secret := c.Config.PodcastindexSecret()
	if key == "" || secret == "" {
		// Missing material: leave the templated auth path to surface the
		// standard "no credentials" error rather than sending a half-signed
		// request.
		return
	}
	date := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha1.Sum([]byte(key + secret + date)) // #nosec G401 -- PodcastIndex requires the sha1hex signature; protocol-mandated, not a security hash choice
	req.Header.Set("X-Auth-Key", key)
	req.Header.Set("X-Auth-Date", date)
	req.Header.Set("Authorization", hex.EncodeToString(sum[:]))
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "podcastindex-pp-cli/1.12.1")
	}
}
