package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CaptureWithChromeSession reads the user's existing local Chrome/Chromium
// LinkedIn session cookies through browser-cookie3 and fetches profile data
// without driving or relaunching the user's browser. Cookie values are kept
// inside the helper process and are never printed by ProfilePress.
func CaptureWithChromeSession(ctx context.Context, profileURL, domain string, timeout time.Duration) (PageText, error) {
	if strings.TrimSpace(profileURL) == "" {
		return PageText{}, fmt.Errorf("--profile-url is required with --chrome-session")
	}
	if domain == "" {
		domain = "linkedin.com"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	helper := `
import browser_cookie3, html.parser, json, re, requests, sys, urllib.parse

url = sys.argv[1]
domain = sys.argv[2]
timeout = float(sys.argv[3])

class TextExtractor(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.skip = 0
        self.parts = []
    def handle_starttag(self, tag, attrs):
        if tag in ('script','style','noscript','svg'):
            self.skip += 1
        if tag in ('br','p','div','section','li','h1','h2','h3','span'):
            self.parts.append('\n')
    def handle_endtag(self, tag):
        if tag in ('script','style','noscript','svg') and self.skip:
            self.skip -= 1
        if tag in ('p','div','section','li','h1','h2','h3','span'):
            self.parts.append('\n')
    def handle_data(self, data):
        if not self.skip:
            s = ' '.join(data.split())
            if s:
                self.parts.append(s)
    def text(self):
        raw = ' '.join(self.parts)
        raw = re.sub(r'\s*\n\s*', '\n', raw)
        raw = re.sub(r'[ \t]+', ' ', raw)
        return '\n'.join([line.strip() for line in raw.splitlines() if line.strip()])

def profile_slug(raw_url):
    path = urllib.parse.urlparse(raw_url).path.strip('/')
    parts = path.split('/')
    if len(parts) >= 2 and parts[0] == 'in':
        return parts[1]
    return ''

def locale_value(v):
    if isinstance(v, str):
        return v
    if isinstance(v, dict):
        for key in ('en_US', 'en', 'default'):
            if isinstance(v.get(key), str):
                return v[key]
        for val in v.values():
            if isinstance(val, str):
                return val
    return ''

def visible_text(html):
    parser = TextExtractor()
    parser.feed(html)
    return parser.text()

def slice_section(text, marker):
    idx = text.find(marker)
    if idx == -1:
        return ''
    tail = text[idx:]
    for stop in ['Profile language', 'Ad Options', 'About\nAccessibility']:
        s = tail.find(stop)
        if s > 0:
            tail = tail[:s]
    return tail.strip()

cj = browser_cookie3.chrome(domain_name=domain)
s = requests.Session()
s.cookies.update(cj)
csrf = next((c.value.strip('"') for c in s.cookies if c.name == 'JSESSIONID'), '')
s.headers.update({
  'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0 Safari/537.36',
  'Accept-Language': 'en-US,en;q=0.9',
})
slug = profile_slug(url)
if not slug:
    raise RuntimeError('profile URL must be /in/<slug>')

profile_resp = s.get(url, timeout=timeout, allow_redirects=True)
profile_resp.raise_for_status()
html = profile_resp.text
m = re.search(r'<title[^>]*>(.*?)</title>', html, re.I|re.S)
title = re.sub(r'\s+', ' ', (m.group(1) if m else '')).strip()

api_headers = dict(s.headers)
api_headers.update({'csrf-token': csrf, 'x-restli-protocol-version': '2.0.0', 'accept': 'application/vnd.linkedin.normalized+json+2.1'})
api_url = 'https://www.linkedin.com/voyager/api/identity/dash/profiles?q=memberIdentity&memberIdentity=' + urllib.parse.quote(slug)
api = s.get(api_url, headers=api_headers, timeout=timeout)
api.raise_for_status()
profile = (api.json().get('included') or [{}])[0]
name = ' '.join([profile.get('firstName',''), profile.get('lastName','')]).strip()
headline = profile.get('headline') or locale_value(profile.get('multiLocaleHeadline'))
summary = profile.get('summary') or locale_value(profile.get('multiLocaleSummary'))

parts = []
if title:
    parts.append(title)
if name:
    parts.append(name)
if headline:
    parts.append(headline)
if summary:
    parts.extend(['About', summary])

exp_url = urllib.parse.urljoin(url, '/in/' + slug + '/details/experience/')
exp_resp = s.get(exp_url, timeout=timeout, allow_redirects=True)
if exp_resp.ok:
    exp_text = slice_section(visible_text(exp_resp.text), 'Experience')
    if exp_text:
        parts.append(exp_text)

print(json.dumps({'url': profile_resp.url, 'title': title, 'text': '\n'.join(parts)}))
`
	cmdCtx, cancel := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "uv", "run", "--with", "browser-cookie3", "--with", "requests", "python3", "-c", helper, profileURL, domain, fmt.Sprintf("%.0f", timeout.Seconds()))
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return PageText{}, fmt.Errorf("import Chrome session and fetch %s: %w: %s", profileURL, err, strings.TrimSpace(string(exit.Stderr)))
		}
		return PageText{}, fmt.Errorf("import Chrome session and fetch %s: %w", profileURL, err)
	}
	var page PageText
	if err := json.Unmarshal(out, &page); err != nil {
		return PageText{}, fmt.Errorf("decode Chrome session capture payload: %w", err)
	}
	return page, nil
}
