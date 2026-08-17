// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// The invisible hCaptcha solve: render an offscreen invisible widget bound to
// Suno's sitekey + first-party hosts, then await hcaptcha.execute(). Mirrors
// paperfoot/suno-cli's proven render_and_execute payload.

package captcha

import (
	"fmt"
	"strings"
)

// solveJS returns the page script that renders an invisible hCaptcha widget and
// resolves to the token string (or "ERR:<reason>"). This is the headless,
// no-interaction attempt: it awaits execute() inline and is used before any
// window is shown.
func solveJS() string {
	return fmt.Sprintf(`/*pp:invisible*/
(async () => {
  try {
    const div = document.createElement('div');
    div.style.cssText = 'position:fixed;top:-9999px;left:-9999px;';
    document.body.appendChild(div);
    const id = hcaptcha.render(div, {
      sitekey: '%s',
      size:'invisible',
      sentry: false,
      endpoint: '%s',
      assethost: '%s',
      imghost: '%s',
      reportapi: '%s',
    });
    const r = await hcaptcha.execute(id, { async: true });
    return (r && r.response) ? r.response : '';
  } catch (e) {
    return 'ERR:' + String(e);
  }
})()`, SunoHCaptchaSitekey, hcaptchaEndpoint, hcaptchaAssetHost, hcaptchaImgHost, hcaptchaReportAPI)
}

// interactiveKickJS renders a fresh invisible widget and fires execute()
// WITHOUT awaiting it inline, stashing the eventual token on window.__ppTok (or
// the error on window.__ppErr). Once the solver window is on-screen, execute()
// surfaces the visible hCaptcha challenge overlay; the user solves it and the
// promise resolves. Because it doesn't block, the Go side can poll both for the
// token (interactiveTokenJS) and for the challenge actually rendering
// (challengeVisibleJS) instead of re-rendering a new widget every tick.
func interactiveKickJS() string {
	return fmt.Sprintf(`/*pp:kick*/
(() => {
  try {
    window.__ppTok = ''; window.__ppErr = '';
    const div = document.createElement('div');
    div.style.cssText = 'position:fixed;top:-9999px;left:-9999px;';
    document.body.appendChild(div);
    const id = hcaptcha.render(div, {
      sitekey: '%s', size:'invisible', sentry: false,
      endpoint: '%s', assethost: '%s', imghost: '%s', reportapi: '%s',
    });
    hcaptcha.execute(id, { async: true })
      .then(r => { window.__ppTok = (r && r.response) ? r.response : ''; })
      .catch(e => { window.__ppErr = String(e); });
    return 'ok';
  } catch (e) { return 'ERR:' + String(e); }
})()`, SunoHCaptchaSitekey, hcaptchaEndpoint, hcaptchaAssetHost, hcaptchaImgHost, hcaptchaReportAPI)
}

// interactiveTokenJS reports the result of the in-flight interactive execute():
// the token once solved, "ERR:<reason>" if it rejected, or "" while still
// waiting on the user.
func interactiveTokenJS() string {
	return `/*pp:token*/
(() => {
  if (window.__ppTok) return window.__ppTok;
  if (window.__ppErr) return 'ERR:' + window.__ppErr;
  return '';
})()`
}

// challengeVisibleJS returns "visible" once an hCaptcha challenge iframe is
// actually rendered with a non-trivial on-screen box, else "hidden". This is how
// the solver verifies the captcha was genuinely presented to the user, so it can
// fail fast with a clear message instead of waiting out the whole budget on a
// window showing nothing solvable.
func challengeVisibleJS() string {
	return `/*pp:visible*/
(() => {
  const fs = Array.from(document.querySelectorAll('iframe'));
  for (const f of fs) {
    const t = (f.title || '').toLowerCase();
    const s = (f.src || '').toLowerCase();
    if (t.indexOf('hcaptcha') < 0 && s.indexOf('hcaptcha') < 0) continue;
    const r = f.getBoundingClientRect();
    if (r.width > 40 && r.height > 40) return 'visible';
  }
  return 'hidden';
})()`
}

// classifyToken interprets the raw JS result:
//   - non-empty, non-ERR        -> (token, false, nil)
//   - empty                     -> ("", true, nil)   interactive needed
//   - ERR:...challenge-expired  -> ("", true, nil)   interactive needed
//   - any other ERR:...         -> ("", false, error) hard infra/JS failure
func classifyToken(raw string) (token string, interactiveNeeded bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true, nil
	}
	if strings.HasPrefix(raw, "ERR:") {
		reason := strings.TrimPrefix(raw, "ERR:")
		if strings.Contains(strings.ToLower(reason), "challenge-expired") ||
			strings.Contains(strings.ToLower(reason), "challenge expired") {
			return "", true, nil
		}
		return "", false, fmt.Errorf("hcaptcha solver: %s", reason)
	}
	return raw, false, nil
}
