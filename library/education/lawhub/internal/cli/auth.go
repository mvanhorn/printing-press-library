package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/spf13/cobra"
)

type storageState struct {
	Cookies []storageCookie `json:"cookies"`
	Origins []storageOrigin `json:"origins"`
}
type storageCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}
type storageOrigin struct {
	Origin       string        `json:"origin"`
	LocalStorage []storageItem `json:"localStorage"`
}
type storageItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func captureCurrentLocalStorage(p *rod.Page) []storageOrigin {
	out := []storageOrigin{}
	func() {
		defer func() { _ = recover() }()
		raw := p.MustEval(`() => JSON.stringify({origin: location.origin, items: Object.keys(localStorage).map(k => ({name:k, value:localStorage.getItem(k)}))})`).String()
		var got struct {
			Origin string        `json:"origin"`
			Items  []storageItem `json:"items"`
		}
		_ = json.Unmarshal([]byte(raw), &got)
		if got.Origin != "" && got.Origin != "null" && len(got.Items) > 0 {
			out = append(out, storageOrigin{Origin: got.Origin, LocalStorage: got.Items})
		}
	}()
	return out
}

func restoreLocalStorage(p *rod.Page, origins []storageOrigin) {
	for _, o := range origins {
		if o.Origin == "" || len(o.LocalStorage) == 0 {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			p.MustNavigate(o.Origin).MustWaitLoad()
			raw, _ := json.Marshal(o.LocalStorage)
			p.MustEval(`itemsJson => { const items = JSON.parse(itemsJson); for (const it of items) localStorage.setItem(it.name, it.value); }`, string(raw))
		}()
	}
}

type accountState struct {
	UserID       string `json:"user_id,omitempty"`
	DiscoveredAt string `json:"discovered_at,omitempty"`
	Source       string `json:"source,omitempty"`
}

func sessionPath() string { return filepath.Join(cfg.SecureDir, "storage-state.json") }
func accountPath() string { return filepath.Join(cfg.SecureDir, "account.json") }
func profilePath() string { return filepath.Join(cfg.SecureDir, "browser-profile") }

func resolveBrowserPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chrome", "brave-browser", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "/usr/bin/google-chrome"
}

func isLawHubCookieDomain(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return d == "lawhub.org" || strings.HasSuffix(d, ".lawhub.org")
}

func readAccount() accountState {
	var st accountState
	raw, err := os.ReadFile(accountPath())
	if err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	return st
}

func writeAccount(st accountState) error {
	if err := os.MkdirAll(cfg.SecureDir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(accountPath(), raw, 0o600)
}

func resolveUserID(p *rod.Page) (string, string, error) {
	if cfg.UserID != "" {
		return cfg.UserID, "--user-id", nil
	}
	if env := strings.TrimSpace(os.Getenv("LAWHUB_USER_ID")); env != "" {
		return env, "LAWHUB_USER_ID", nil
	}
	if st := readAccount(); st.UserID != "" {
		return st.UserID, "account.json", nil
	}
	if p != nil {
		if id := discoverUserIDFromPage(p); id != "" {
			_ = writeAccount(accountState{UserID: id, DiscoveredAt: nowISO(), Source: "browser-page"})
			return id, "browser-page", nil
		}
	}
	return "", "", fmt.Errorf("LawHub user id not found; run `lawhub-pp-cli auth login --cdp http://127.0.0.1:9222` after signing into a debuggable browser, or pass --user-id / set LAWHUB_USER_ID")
}

func discoverUserIDFromPage(p *rod.Page) string {
	js := `() => {
		const vals = [];
		try { vals.push(location.href, document.body ? document.body.innerText : '', document.documentElement ? document.documentElement.innerHTML : ''); } catch(e) {}
		try { for (let i=0; i<localStorage.length; i++) vals.push(localStorage.key(i)+'='+localStorage.getItem(localStorage.key(i))); } catch(e) {}
		try { for (let i=0; i<sessionStorage.length; i++) vals.push(sessionStorage.key(i)+'='+sessionStorage.getItem(sessionStorage.key(i))); } catch(e) {}
		return vals.join('\n').slice(0, 200000);
	}`
	var hay string
	func() {
		defer func() { _ = recover() }()
		hay = p.MustEval(js).String()
	}()
	if hay == "" {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/api/user/([A-Za-z0-9_.@-]+)/history/`),
		regexp.MustCompile(`/user/([A-Za-z0-9_.@-]+)/history/`),
		regexp.MustCompile(`(?i)"(?:username|userName|user_id|userId|accountName)"\s*:\s*"([A-Za-z0-9_.@-]{3,})"`),
		regexp.MustCompile(`(?i)(?:username|userName|user_id|userId|accountName)[=:]([A-Za-z0-9_.@-]{3,})`),
	}
	for _, re := range patterns {
		if m := re.FindStringSubmatch(hay); len(m) > 1 {
			id := strings.Trim(m[1], `"' ,;`)
			if id != "" && !strings.Contains(strings.ToLower(id), "undefined") {
				return id
			}
		}
	}
	return ""
}

func loginCmd() *cobra.Command {
	return loginCobra("login")
}

func loginCobra(use string) *cobra.Command {
	var targetURL string
	var cdpURL string
	c := &cobra.Command{Use: use, Short: "Import LawHub auth from an existing Chrome session", RunE: func(cmd *cobra.Command, args []string) error {
		if targetURL == "" {
			targetURL = lawhubURL
		}
		if cdpURL == "" {
			cdpURL = "http://127.0.0.1:9222"
		}
		return authLoginExisting(cdpURL, targetURL)
	}}
	c.Flags().StringVar(&targetURL, "url", lawhubURL, "LawHub URL to open in existing browser")
	c.Flags().StringVar(&cdpURL, "cdp", "http://127.0.0.1:9222", "Chrome DevTools Protocol HTTP/WebSocket URL")
	return c
}

func authLoginExisting(cdpURL, targetURL string) error {
	ws, err := resolveCDPWebSocket(cdpURL)
	if err != nil {
		return err
	}
	b := rod.New().ControlURL(ws).MustConnect()
	defer b.MustClose()
	p := b.MustPage(targetURL)
	p.MustWaitLoad()
	fmt.Printf("Attached to existing browser at %s. Ensure LawHub is logged in, then press Enter.\n", cdpURL)
	_, _ = fmt.Fscanln(os.Stdin)
	out := storageState{Origins: []storageOrigin{}}
	if cookies, err := b.GetCookies(); err == nil {
		for _, c := range cookies {
			if !isLawHubCookieDomain(c.Domain) {
				continue
			}
			out.Cookies = append(out.Cookies, storageCookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: float64(c.Expires), HTTPOnly: c.HTTPOnly, Secure: c.Secure, SameSite: string(c.SameSite)})
		}
	}
	out.Origins = captureCurrentLocalStorage(p)
	if len(out.Cookies) == 0 && len(out.Origins) == 0 {
		return fmt.Errorf("could not capture cookies or localStorage from existing browser")
	}
	if err := os.MkdirAll(cfg.SecureDir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(sessionPath(), raw, 0o600); err != nil {
		return err
	}
	if userID := discoverUserIDFromPage(p); userID != "" {
		_ = writeAccount(accountState{UserID: userID, DiscoveredAt: nowISO(), Source: "existing-browser"})
	}
	return emit(map[string]any{"session_file": sessionPath(), "mode": "storage-state", "cookies": len(out.Cookies), "origins": len(out.Origins), "saved": true})
}

func resolveCDPWebSocket(cdpURL string) (string, error) {
	if strings.HasPrefix(cdpURL, "ws://") || strings.HasPrefix(cdpURL, "wss://") {
		return cdpURL, nil
	}
	base := strings.TrimRight(cdpURL, "/")
	cdpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := cdpClient.Get(base + "/json/version")
	if err != nil {
		return "", fmt.Errorf("connect to Chrome CDP at %s failed: %w. Start Chrome with --remote-debugging-port=9222", cdpURL, err)
	}
	defer resp.Body.Close()
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("Chrome CDP did not expose webSocketDebuggerUrl at %s", cdpURL)
	}
	return v.WebSocketDebuggerURL, nil
}

func authCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage LawHub session"}
	cmd.AddCommand(loginCobra("login"), authImportFileCmd(), authStatusCmd(), authPathCmd(), authLogoutCmd())
	return cmd
}

func authPing() map[string]any {
	out := map[string]any{"checked": false, "ok": false}
	if _, err := os.Stat(sessionPath()); err != nil {
		out["error"] = "missing session"
		return out
	}
	b, p, err := browserPage()
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	defer b.MustClose()
	out["checked"] = true
	if id, src, err := resolveUserID(p); err == nil {
		out["user_id"] = id
		out["user_source"] = src
	}
	status := 0
	bodySample := ""
	probe := "library-page"
	func() {
		defer func() { _ = recover() }()
		raw := p.MustEval(`async () => {
			const body = (document.body && document.body.innerText || '').slice(0, 2000);
			return JSON.stringify({status: 200, text: body, url: location.href});
		}`).String()
		var x struct {
			Status int    `json:"status"`
			Text   string `json:"text"`
			URL    string `json:"url"`
		}
		_ = json.Unmarshal([]byte(raw), &x)
		status = x.Status
		bodySample = x.Text
	}()
	out["status"] = status
	out["probe"] = probe
	low := strings.ToLower(bodySample)
	if strings.Contains(low, "sign in") || strings.Contains(low, "forgot your password") || strings.Contains(low, "we can't sign you in") {
		out["ok"] = false
		out["body_sample"] = bodySample[:min(len(bodySample), 300)]
	} else if strings.Contains(low, "preptest") || strings.Contains(low, "lawhub") || strings.Contains(low, "library") || strings.Contains(low, "cookies") {
		out["ok"] = true
	} else {
		out["ok"] = false
		out["body_sample"] = bodySample[:min(len(bodySample), 300)]
	}
	return out
}

func authImportFileCmd() *cobra.Command {
	return &cobra.Command{Use: "import-file <storage-state.json>", Short: "Import Playwright/browser-use storage state", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		in := args[0]
		f, err := os.Open(in)
		if err != nil {
			return err
		}
		defer f.Close()
		var st storageState
		if err := json.NewDecoder(f).Decode(&st); err != nil {
			return fmt.Errorf("invalid storage state JSON: %w", err)
		}
		if len(st.Cookies) == 0 && len(st.Origins) == 0 {
			return fmt.Errorf("storage state has no cookies or origins")
		}
		if err := os.MkdirAll(cfg.SecureDir, 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(sessionPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
		if _, err := io.Copy(out, f); err != nil {
			return err
		}
		return emit(map[string]any{"imported": true, "session_file": sessionPath(), "cookies": len(st.Cookies), "origins": len(st.Origins)})
	}}
}

func authStatusCmd() *cobra.Command {
	var live bool
	c := &cobra.Command{Use: "status", Short: "Show saved session status", RunE: func(cmd *cobra.Command, args []string) error {
		st := readAccount()
		_, sessErr := os.Stat(sessionPath())
		_, acctErr := os.Stat(accountPath())
		out := map[string]any{"session_file": sessionPath(), "session_exists": sessErr == nil, "account_file": accountPath(), "account_exists": acctErr == nil, "user_id": nullIfEmpty(st.UserID), "source": nullIfEmpty(st.Source), "discovered_at": nullIfEmpty(st.DiscoveredAt)}
		if live {
			out["live"] = authPing()
		}
		return emit(out)
	}}
	c.Flags().BoolVar(&live, "live", false, "perform live authenticated ping")
	return c
}

func authPathCmd() *cobra.Command {
	return &cobra.Command{Use: "path", Short: "Print auth/session paths", RunE: func(cmd *cobra.Command, args []string) error {
		return emit(map[string]any{"secure_dir": cfg.SecureDir, "session_file": sessionPath(), "account_file": accountPath()})
	}}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "Delete saved LawHub session", RunE: func(cmd *cobra.Command, args []string) error {
		removed := []string{}
		for _, p := range []string{sessionPath(), accountPath()} {
			if err := os.Remove(p); err == nil {
				removed = append(removed, p)
			}
		}
		if err := os.RemoveAll(profilePath()); err == nil {
			removed = append(removed, profilePath())
		}
		return emit(map[string]any{"removed": removed, "logged_out": true})
	}}
}
