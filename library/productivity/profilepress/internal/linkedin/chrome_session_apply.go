package linkedin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ChromeSessionApplyAdapter applies supported LinkedIn profile sections by
// importing the user's existing local Chrome LinkedIn session and calling
// LinkedIn-specific SDUI server-request endpoints. It does not drive a browser
// and does not print cookies or CSRF values.
type ChromeSessionApplyAdapter struct {
	ProfileURL string
	Domain     string
	Timeout    time.Duration
}

func (a ChromeSessionApplyAdapter) Apply(section, value string) error {
	section = canonicalApplySection(section)
	if section != "headline" && section != "about" {
		return fmt.Errorf("live LinkedIn apply does not support section %q yet", section)
	}
	if a.Domain == "" {
		a.Domain = "linkedin.com"
	}
	if a.Timeout <= 0 {
		a.Timeout = 30 * time.Second
	}
	helper := `
import browser_cookie3, json, re, requests, sys, urllib.parse
profile_url, domain, section, value, timeout_raw = sys.argv[1:6]
timeout = float(timeout_raw)

def slug_from_url(raw):
    parts = urllib.parse.urlparse(raw).path.strip('/').split('/')
    if len(parts) >= 2 and parts[0] == 'in':
        return parts[1]
    raise RuntimeError('profile URL must be /in/<slug>')

def como_endpoint(html):
    m = re.search(r'<meta name="como-t" content="([^\"]+)"', html)
    if not m:
        return ''
    raw = m.group(1).replace('&quot;', '"')
    try:
        return json.loads(raw).get('ep') or ''
    except Exception:
        m2 = re.search(r'"ep"\s*:\s*"([^"]+)"', raw)
        return m2.group(1) if m2 else ''

def sdui_body(request_id, payload):
    return {
        'requestId': request_id,
        'requestedArguments': {
            '$type': 'proto.sdui.actions.requests.RequestedArguments',
            'payload': payload,
            'requestedStateKeys': [],
            'requestMetadata': {'$type': 'proto.sdui.common.RequestMetadata'},
        },
        'isStreaming': False,
        'isApfcEnabled': False,
        'rumPageKey': '',
        'maxRetries': 0,
        'backOffMultiplier': 0,
        'maxSeconds': 0,
    }

def post_sdui(session, ep, request_id, payload, referer, csrf):
    if not ep:
        raise RuntimeError('LinkedIn edit page did not expose an SDUI endpoint')
    url = urllib.parse.urljoin('https://www.linkedin.com', ep.rstrip('/') + '/rsc-action/actions/server-request?sduiid=' + urllib.parse.quote(request_id, safe=''))
    r = session.post(url, headers={
        'csrf-token': csrf,
        'Content-Type': 'application/json',
        'Accept': 'text/x-component',
        'x-li-rsc-stream': 'true',
        'x-requested-with': 'XMLHttpRequest',
        'Referer': referer,
    }, data=json.dumps(sdui_body(request_id, payload)), timeout=timeout)
    if r.status_code >= 400:
        raise RuntimeError('LinkedIn SDUI request failed status=%s body=%s' % (r.status_code, r.text[:500]))
    return r.text

slug = slug_from_url(profile_url)
cj = browser_cookie3.chrome(domain_name=domain)
s = requests.Session(); s.cookies.update(cj)
csrf = next((c.value.strip('"') for c in s.cookies if c.name == 'JSESSIONID'), '')
s.headers.update({
  'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0 Safari/537.36',
  'Accept-Language': 'en-US,en;q=0.9',
})
api_headers = dict(s.headers)
api_headers.update({'csrf-token': csrf, 'x-restli-protocol-version': '2.0.0', 'accept': 'application/vnd.linkedin.normalized+json+2.1'})
api = s.get('https://www.linkedin.com/voyager/api/identity/dash/profiles?q=memberIdentity&memberIdentity=' + urllib.parse.quote(slug), headers=api_headers, timeout=timeout)
api.raise_for_status()
profile = (api.json().get('included') or [{}])[0]
profile_id = (profile.get('entityUrn') or '').split(':')[-1]
if not profile_id:
    raise RuntimeError('could not determine LinkedIn profile id')
first = profile.get('firstName') or ''
last = profile.get('lastName') or ''
headline = profile.get('headline') or ''
summary = profile.get('summary') or ''

if section == 'headline':
    edit_url = 'https://www.linkedin.com/in/' + slug + '/edit/intro/'
    page = s.get(edit_url, timeout=timeout)
    page.raise_for_status()
    payload = {
        'profileId': profile_id,
        'vanityName': slug,
        'firstName': first,
        'lastName': last,
        'headline': value,
        'initialHeadline': headline,
        'hasChanges': True,
        'premiumUpsellEligible': False,
        'verificationNbaEligible': False,
        'isRefreshRequiredAfterSave': True,
        'showCurrentPosition': True,
        'showEducation': False,
        'showOpenProfile': False,
        'showPremiumBadge': False,
        'additionalName': '',
        'customPronouns': '',
        'pronouns': [],
        'additionalNameVisibilitySetting': 'AdditionalNamePronunciationVisibilityEnumValue_HIDDEN',
        'pronounsVisibilitySetting': 'PronounVisibilityEnumValue_MEMBERS',
        'namePronunciationVisibilitySetting': 'NamePronunciationVisibilityEnumValue_MEMBERS',
    }
    post_sdui(s, como_endpoint(page.text), 'com.linkedin.sdui.requests.profile.saveProfileIntroForm', payload, edit_url, csrf)
elif section == 'about':
    edit_url = 'https://www.linkedin.com/in/' + slug + '/edit/forms/summary/new/'
    page = s.get(edit_url, timeout=timeout)
    page.raise_for_status()
    payload = {
        'profileId': profile_id,
        'vanityName': slug,
        'about': value,
        'initialAbout': summary,
        'skills': [],
        'initialSkills': [],
        'hasChanges': True,
        'premiumUpsellEligible': False,
        'hasAtLeastOneTopSkillLackingAssociations': False,
    }
    post_sdui(s, como_endpoint(page.text), 'com.linkedin.sdui.requests.profile.saveProfileAboutForm', payload, edit_url, csrf)
print(json.dumps({'section': section, 'result': 'linkedin-section-applied'}))
`
	ctx, cancel := context.WithTimeout(context.Background(), a.Timeout+15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "uv", "run", "--with", "browser-cookie3", "--with", "requests", "python3", "-c", helper, a.ProfileURL, a.Domain, section, value, fmt.Sprintf("%.0f", a.Timeout.Seconds()))
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("apply LinkedIn %s through Chrome session: %w: %s", section, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return fmt.Errorf("apply LinkedIn %s through Chrome session: %w", section, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		return fmt.Errorf("decode LinkedIn apply response: %w", err)
	}
	return nil
}
