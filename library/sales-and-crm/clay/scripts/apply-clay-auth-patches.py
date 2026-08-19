#!/usr/bin/env python3
"""Re-apply Clay's dual-credential auth fixes after `generate --force`.

Clay serves two APIs from one host with two different credentials:
  /v3        -> `claysession` browser cookie
  /public/v0 -> raw `clay-api-key` header (no Bearer scheme)

The generator's single-credential auth model cannot express this, and the two
edits below live in generated files with no extension hook, so regeneration
reverts them. Run this script after any `generate --force`.

Idempotent: re-running when the patches are already applied is a no-op.
"""
import sys

def patch(path, old, new, label):
    src = open(path).read()
    if new in src:
        print(f"  ok (already applied): {label}")
        return True
    if old not in src:
        print(f"  MISS: {label} -- anchor not found in {path}")
        return False
    open(path, "w").write(src.replace(old, new, 1))
    print(f"  applied: {label}")
    return True

ok = True

# 1. Cookie jar must carry the browser session, never CLAY_API_KEY.
# With a cookie-only spec the generator already emits `return c.AccessToken`;
# this patch only fires if a future spec change reintroduces the key preference.
if "if c.ClayApiKey != \"\" {" in open("internal/config/config.go").read():
    ok &= patch("internal/config/config.go",
"""func (c *Config) CookieCredential() string {
	if c.ClayApiKey != "" {
		return c.ClayApiKey
	}
	return c.AccessToken
}""",
"""func (c *Config) CookieCredential() string {
	// Clay: CLAY_API_KEY authenticates only /public/v0 via the clay-api-key
	// header. Seeding it as a cookie 401s every /v3 app-API command.
	if c.AccessToken != "" {
		return c.AccessToken
	}
	return ""
}""", "CookieCredential prefers browser session")
else:
    print("  ok (not needed): CookieCredential already returns the browser session")

# 2. An env API key must not discard a stored browser session.
ok &= patch("internal/config/config.go",
"""	if strings.HasPrefix(c.AuthSource, "env:") {
		return false
	}""",
"""	// Clay: CLAY_API_KEY is a Public-API-only credential; keep the persisted
	// jar so the /v3 browser session survives alongside it.
	if strings.TrimSpace(c.AccessToken) != "" && strings.TrimSpace(c.CredentialDomain) != "" {
		return true
	}
	if strings.HasPrefix(c.AuthSource, "env:") {
		return false
	}""", "UsePersistedCookieJar keeps stored session")

# 3. Send the raw public-API key on /public/ paths only.
ok &= patch("internal/client/client.go",
"""		if c.Config != nil {
			for k, v := range c.Config.Headers {
				req.Header.Set(k, v)
			}
		}""",
"""		if ClayIsPublicAPIPath(req.URL.Path) {
			if k := ClayPublicAPIKey(); k != "" {
				req.Header.Set(clayAPIKeyHeader, k)
			}
		}
		if c.Config != nil {
			for k, v := range c.Config.Headers {
				req.Header.Set(k, v)
			}
		}""", "public-API key scoping")

# 4. The generated `feedback` command ships without an Examples: section, which
# its own live-dogfood help check then fails. Template-shape gap; filed for retro.
import re as _re
_fb = "internal/cli/feedback.go"
try:
    _s = open(_fb).read()
    if "Example:" in _s.split("func newFeedbackCmd")[1][:900]:
        print("  ok (already applied): feedback Example")
    else:
        _m = _re.search(r'(func newFeedbackCmd[\s\S]{0,600}?Short:\s*"[^"]*",\n)', _s)
        if _m:
            _ins = _m.group(1) + '\t\tExample:     "  clay-pp-cli feedback \\"columns link needs a --pull flag\\"\\n  clay-pp-cli feedback list",\n'
            open(_fb, "w").write(_s[:_m.start(1)] + _ins + _s[_m.end(1):])
            print("  applied: feedback Example")
        else:
            print("  MISS: feedback Example -- anchor not found")
            ok = False
except FileNotFoundError:
    print("  skip: feedback.go not present")

sys.exit(0 if ok else 1)
