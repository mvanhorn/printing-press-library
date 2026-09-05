package cli

import "net/url"

// nccplDecodedCookieMap URL-decodes cookie values before they are composed into a
// request header.
//
// Laravel sets XSRF-TOKEN as a percent-encoded cookie value: the encrypted token is
// base64 and its "=" padding arrives as "%3D". Laravel's VerifyCsrfToken middleware
// decrypts the X-XSRF-TOKEN *header*, so it must receive the decoded value; a still
// encoded one fails decryption and every POST returns 419. Browser clients do this
// with decodeURIComponent, and the reference implementation for this API
// (hmehmood56-debug/PSX-Trader) does the same.
//
// The generated composeAuthFromCookies performs a plain string substitution with no
// decoding step, so the decode has to happen on the map handed to it. The Cookie
// header is composed separately from the raw map and correctly keeps the encoded
// form, which is what the server expects there.
//
// Values without percent-escapes are returned unchanged, so this is safe to apply
// unconditionally.
func nccplDecodedCookieMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if decoded, err := url.QueryUnescape(v); err == nil {
			out[k] = decoded
			continue
		}
		out[k] = v
	}
	return out
}
