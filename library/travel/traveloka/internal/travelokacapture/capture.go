package travelokacapture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Backend, OutputDir, Market, Locale, Currency string
	Depart, ReturnDate, CheckIn, CheckOut        string
}
type Result struct {
	CookiesFile    string `json:"cookies_file"`
	RequestsFile   string `json:"requests_file"`
	OperationCount int    `json:"operation_count"`
	BrowserClosed  bool   `json:"browser_closed"`
}
type backendReply struct {
	Success bool `json:"success"`
	Data    struct {
		Raw    string `json:"_raw_text"`
		Result any    `json:"result"`
	} `json:"data"`
}

func BackendPath(requested string) (string, error) {
	if requested != "" {
		p, err := exec.LookPath(requested)
		if err != nil {
			return "", fmt.Errorf("browser-use backend unavailable; provide --browser-use with an installed executable")
		}
		return p, nil
	}
	if p, e := exec.LookPath("browser-use"); e == nil {
		return p, nil
	}
	if h, e := os.UserHomeDir(); e == nil {
		p := filepath.Join(h, ".local", "bin", "browser-use")
		if s, e := os.Stat(p); e == nil && !s.IsDir() && s.Mode()&0111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("browser-use is required for normal browser capture; use an existing installation or auth import-session with scoped private JSON files")
}
func SourceURLs(o Options) ([]string, error) {
	prefix := strings.ToLower(strings.ReplaceAll(o.Locale, "_", "-"))
	if prefix == "" {
		prefix = "en-sg"
	}
	if !safeContext(o.Market, o.Locale, o.Currency) {
		return nil, fmt.Errorf("--market, --locale and --currency must be valid uppercase country/currency and language-region codes")
	}
	dates := []string{o.Depart, o.ReturnDate, o.CheckIn, o.CheckOut}
	ds := make([]string, len(dates))
	for i, d := range dates {
		v, e := time.Parse("2006-01-02", d)
		if e != nil || v.Format("2006-01-02") != d {
			return nil, fmt.Errorf("capture dates must use valid YYYY-MM-DD values")
		}
		ds[i] = v.Format("2-1-2006")
	}
	if dates[1] < dates[0] || dates[3] <= dates[2] {
		return nil, fmt.Errorf("capture return/check-out dates must follow their departure/check-in dates")
	}
	nights := int(mustDate(o.CheckOut).Sub(mustDate(o.CheckIn)).Hours() / 24)
	base := "https://www.traveloka.com/" + prefix
	return []string{base, fmt.Sprintf("%s/flight/fulltwosearch?ap=SIN.CGK&dt=%s.%s&ps=1.0.0&sc=ECONOMY", base, ds[0], ds[1]), fmt.Sprintf("%s/hotel/search?spec=%s.%s.%d.1.HOTEL_GEO.10000045.Bangkok.2", base, ds[2], ds[3], nights), fmt.Sprintf("%s/hotel/detail?spec=%s.%s.%d.1.HOTEL.9000000001714.The%%20Berkeley%%20Hotel%%20Pratunam.2", base, ds[2], ds[3], nights)}, nil
}
func mustDate(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func safeContext(m, l, c string) bool {
	if len(m) != 2 || len(c) != 3 {
		return false
	}
	for _, x := range m + c {
		if x < 'A' || x > 'Z' {
			return false
		}
	}
	parts := strings.Split(strings.ReplaceAll(l, "_", "-"), "-")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return false
	}
	for _, x := range parts[0] {
		if x < 'a' || x > 'z' {
			return false
		}
	}
	return strings.EqualFold(parts[1], m)
}
func Capture(ctx context.Context, o Options) (*Result, error) {
	urls, err := SourceURLs(o)
	if err != nil {
		return nil, err
	}
	bin, err := BackendPath(o.Backend)
	if err != nil {
		return nil, err
	}
	if o.OutputDir == "" {
		return nil, fmt.Errorf("capture output directory is required")
	}
	if err = os.MkdirAll(o.OutputDir, 0700); err != nil {
		return nil, fmt.Errorf("create private capture directory: %w", err)
	}
	if err = os.Chmod(o.OutputDir, 0700); err != nil { // #nosec G302 -- This is a private directory; owner execute permission is required to traverse it.
		return nil, fmt.Errorf("protect capture directory: %w", err)
	}
	home, err := os.MkdirTemp("", "tvb-")
	if err != nil {
		return nil, err
	}
	// Short socket names fit macOS Unix-domain socket limits inside its long TMPDIR.
	session := "capture"
	closed := false
	env := append(os.Environ(), "BROWSER_USE_HOME="+home, "BROWSER_USE_ANONYMIZED_TELEMETRY=false", "TRAVELOKA_CAPTURE_DIR="+o.OutputDir)
	run := func(name string, args ...string) (backendReply, error) {
		a := append([]string{"--session", session, "--json"}, args...)
		cmd := exec.CommandContext(ctx, bin, a...) // #nosec G204 -- BackendPath validates the operator-selected local executable; fixed argv is passed without a shell.
		cmd.Env = env
		b, e := cmd.Output()
		if e != nil {
			return backendReply{}, fmt.Errorf("normal browser capture step %s failed; retry with a usable guest browser session or use scoped manual import", name)
		}
		var r backendReply
		if json.Unmarshal(b, &r) != nil || !r.Success {
			return r, fmt.Errorf("normal browser capture step %s failed; session values were suppressed", name)
		}
		return r, nil
	}
	closeBrowser := func() {
		if closed {
			return
		}
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, bin, "--session", session, "--json", "close")
		cmd.Env = env
		if cmd.Run() == nil {
			closed = true
			_ = os.RemoveAll(home)
		}
	}
	defer closeBrowser()
	// The isolated task browser never connects to a daily profile or another site.
	if _, err = run("open guest site", "--headed", "open", urls[0]); err != nil {
		return nil, err
	}
	install := filepath.Join(home, "capture-install.py")
	save := filepath.Join(home, "capture-save.py")
	status := filepath.Join(home, "capture-status.py")
	for p, s := range map[string]string{install: installScript, save: saveScript, status: statusScript} {
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			return nil, e
		}
	}
	if _, err = run("install scoped network observer", "python", "--file", install); err != nil {
		return nil, err
	}
	guest := `(() => {const e=[...document.querySelectorAll('div,button,a')].find(e=>e.children.length===0&&e.textContent.trim()==='Browse as a guest');if(e)e.click();return {guest_clicked:!!e};})()`
	airportFocus := `(() => {const e=document.querySelector('input[placeholder=Origin]');if(!e)return {found:false};e.focus();e.select();return {found:true};})()`
	choose := `(() => {const e=[...document.querySelectorAll('div,button')].find(e=>e.children.length===0&&e.textContent.trim()==='Choose');if(e)e.click();return {found:!!e};})()`
	hotelFocus := `(() => {const e=document.querySelector('input[placeholder="City, hotel, place to go"]');if(!e)return {found:false};e.focus();e.select();return {found:true};})()`
	pause := func(d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	// Wait for source controls and read-only traffic, rather than navigating away
	// while an asynchronous return-flight prefetch is still being prepared.
	waitPath := func(path string) error {
		for attempt := 0; attempt < 30; attempt++ {
			r, e := run("wait for source operation", "python", "--file", status)
			if e != nil {
				return e
			}
			var v struct {
				Paths []string `json:"paths"`
			}
			if json.Unmarshal([]byte(strings.TrimSpace(r.Data.Raw)), &v) == nil {
				for _, got := range v.Paths {
					if got == path {
						return nil
					}
				}
			}
			if e = pause(500 * time.Millisecond); e != nil {
				return e
			}
		}
		return fmt.Errorf("normal source did not expose required read-only operation %s; access may need user intervention", path)
	}
	chooseFlight := func() error {
		for attempt := 0; attempt < 24; attempt++ {
			r, e := run("read first flight offer", "eval", choose)
			if e != nil {
				return e
			}
			v, _ := r.Data.Result.(map[string]any)
			if got, _ := v["found"].(bool); got {
				return nil
			}
			if e = pause(500 * time.Millisecond); e != nil {
				return e
			}
		}
		return fmt.Errorf("normal flight page has no selectable outbound offer; inspect the guest source before retrying capture")
	}
	for i, u := range urls[1:] {
		if _, err = run("open dated source search", "open", u); err != nil {
			return nil, err
		}
		if err = pause(2 * time.Second); err != nil {
			return nil, err
		}
		if _, err = run("normal guest entry", "eval", guest); err != nil {
			return nil, err
		}
		if i == 0 {
			if _, err = run("focus airport search", "eval", airportFocus); err != nil {
				return nil, err
			}
			if _, err = run("type airport query", "type", "Singapore"); err != nil {
				return nil, err
			}
			if err = pause(time.Second); err != nil {
				return nil, err
			}
			// Reload the dated search after autocomplete; Choose reads source prefetch/return inventory.
			if _, err = run("restore dated return search", "open", u); err != nil {
				return nil, err
			}
			if err = pause(2 * time.Second); err != nil {
				return nil, err
			}
			if err = chooseFlight(); err != nil {
				return nil, err
			}
			if err = waitPath("/api/v2/flight/search/redirection"); err != nil {
				return nil, err
			}
		}
		if i == 1 {
			if _, err = run("focus destination search", "eval", hotelFocus); err != nil {
				return nil, err
			}
			if _, err = run("type hotel destination", "type", "Bangkok"); err != nil {
				return nil, err
			}
			if err = pause(time.Second); err != nil {
				return nil, err
			}
		}
	}
	if err = pause(time.Second); err != nil {
		return nil, err
	}
	if _, err = run("save Traveloka-only request profiles", "python", "--file", save); err != nil {
		return nil, err
	}
	cookies := filepath.Join(o.OutputDir, "cookies.json")
	requests := filepath.Join(o.OutputDir, "requests.json")
	if _, err = run("export Traveloka-only cookies", "cookies", "export", cookies, "--url", "https://www.traveloka.com/"); err != nil {
		return nil, err
	}
	if err = os.Chmod(cookies, 0600); err != nil {
		return nil, err
	}
	if err = os.Chmod(requests, 0600); err != nil {
		return nil, err
	}
	reply, err := run("check scoped operation coverage", "python", "--file", status)
	if err != nil {
		return nil, err
	}
	var coverage struct {
		Paths []string `json:"paths"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(reply.Data.Raw)), &coverage) != nil {
		return nil, fmt.Errorf("capture coverage could not be verified")
	}
	needed := []string{"/api/v2/airport/search-nexus", "/api/v1/hotel/autocomplete", "/api/v2/flight/search/initial", "/api/v2/flight/search/poll", "/api/v2/flight/search/redirection", "/api/v2/hotel/searchList", "/api/v2/hotel/search/rooms"}
	have := map[string]bool{}
	for _, p := range coverage.Paths {
		have[p] = true
	}
	missing := []string{}
	for _, p := range needed {
		if !have[p] {
			missing = append(missing, p)
		}
	}
	closeBrowser()
	if len(missing) > 0 {
		return nil, fmt.Errorf("normal source capture is incomplete (%s); private scoped files were retained for manual import. Use airport/hotel autocomplete, select an outbound return-flight offer, and open the hotel's room list in a normal guest session", strings.Join(missing, ", "))
	}
	if !closed {
		return nil, fmt.Errorf("capture completed but task browser close was not confirmed; close the task-owned session before HTTP replay")
	}
	return &Result{cookies, requests, len(coverage.Paths), true}, nil
}

const installScript = `import json, urllib.parse, os
traveloka_requests = {}
traveloka_allowed = {'/api/v2/airport/search-nexus','/api/v1/hotel/autocomplete','/api/v2/hotel/autocomplete/features','/api/v2/flight/search/initial','/api/v2/flight/search/poll','/api/v2/flight/search/redirection','/api/v2/hotel/searchList','/api/v2/hotel/search/rooms'}
def traveloka_request(params, session_id):
 r=params.get('request',{});u=urllib.parse.urlsplit(r.get('url',''))
 if u.scheme=='https' and u.hostname=='www.traveloka.com' and u.path in traveloka_allowed and r.get('method')=='POST' and r.get('postData'):
  try: data=json.loads(r['postData']).get('data',{})
  except (ValueError,TypeError): return
  if u.path=='/api/v2/flight/search/redirection' and data.get('isPrefetch') is not True: return
  traveloka_requests[params['requestId']]={'method':'POST','url':r['url'],'headers':r.get('headers',{}),'body':r['postData']}
async def traveloka_install():
 s=await browser._session.get_or_create_cdp_session()
 await s.cdp_client.send.Network.enable(session_id=s.session_id)
 browser._session.cdp_client.register.Network.requestWillBeSent(traveloka_request)
browser._run(traveloka_install())
print(json.dumps({'observer':'Traveloka-only read-only POST profiles'}))
`
const saveScript = `import json, pathlib, os
p=pathlib.Path(os.environ['TRAVELOKA_CAPTURE_DIR'])/'requests.json'
p.write_text(json.dumps(list(traveloka_requests.values())))
os.chmod(p,0o600)
print(json.dumps({'saved_profiles':len(traveloka_requests)}))
`
const statusScript = `import json, urllib.parse
print(json.dumps({'paths':sorted(set(urllib.parse.urlsplit(x['url']).path for x in traveloka_requests.values()))}))
`
