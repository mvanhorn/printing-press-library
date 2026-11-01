package voice

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Session struct {
	AccountID  string            `json:"account_id"`
	Cookies    map[string]string `json:"cookies"`
	Headers    map[string]string `json:"headers"`
	PublicKey  string            `json:"public_key"`
	ImportedAt string            `json:"imported_at"`
}

var cookieNames = map[string]bool{"SID": true, "HSID": true, "SSID": true, "OSID": true, "APISID": true, "SAPISID": true, "__Secure-1PSID": true, "__Secure-3PSID": true, "__Secure-1PAPISID": true, "__Secure-3PAPISID": true, "__Secure-1PSIDTS": true, "__Secure-3PSIDTS": true, "SIDCC": true, "__Secure-1PSIDCC": true, "__Secure-3PSIDCC": true}
var headerNames = map[string]bool{"user-agent": true, "x-client-version": true, "x-clientdetails": true, "x-javascript-user-agent": true, "x-goog-authuser": true, "x-origin": true, "x-referer": true, "x-requested-with": true, "origin": true, "referer": true}

func Profile() (string, error) {
	p := os.Getenv("GOOGLE_VOICE_PROFILE")
	if p == "" {
		p = "default"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(p) {
		return "", fmt.Errorf("GOOGLE_VOICE_PROFILE must contain only letters, digits, underscores or hyphens")
	}
	return p, nil
}
func privatePath(path string) error {
	p, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for probe := p; ; probe = filepath.Dir(probe) {
		if resolved, e := filepath.EvalSymlinks(probe); e == nil {
			p = resolved + strings.TrimPrefix(p, probe)
			break
		}
		if filepath.Dir(probe) == probe {
			break
		}
	}
	for _, c := range strings.Split(p, string(filepath.Separator)) {
		if strings.EqualFold(c, "Dropbox") {
			return fmt.Errorf("private Voice data must stay outside Dropbox")
		}
	}
	return nil
}
func Paths() (string, string, error) {
	profile, err := Profile()
	if err != nil {
		return "", "", err
	}
	cfg, err := cliutil.ConfigDir()
	if err != nil {
		return "", "", err
	}
	data, err := cliutil.DataDir()
	if err != nil {
		return "", "", err
	}
	s := filepath.Join(cfg, "voice-profiles", profile, "session.json")
	d := filepath.Join(data, "voice-profiles", profile, "messages.sqlite3")
	for _, p := range []string{s, d} {
		if e := privatePath(p); e != nil {
			return "", "", e
		}
	}
	return s, d, nil
}
func ReadPrivate(path string) ([]byte, error) {
	if e := privatePath(path); e != nil {
		return nil, e
	}
	fi, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("private Voice file must not be a symlink")
	}
	o, e := cliutil.ReadOpenedConfig(path)
	if e != nil {
		return nil, e
	}
	if o.PermErr != nil {
		return nil, fmt.Errorf("private Voice file permissions: %w", o.PermErr)
	}
	return o.Data, nil
}
func LoadSession() (Session, error) {
	path, _, err := Paths()
	if err != nil {
		return Session{}, err
	}
	b, err := ReadPrivate(path)
	if err != nil {
		return Session{}, fmt.Errorf("no usable private Voice session; run google-voice-pp-cli auth import-har --file /private/path/capture.har: %w", err)
	}
	var s Session
	if json.Unmarshal(b, &s) != nil || s.Cookies["SAPISID"] == "" || s.AccountID == "" {
		return Session{}, fmt.Errorf("invalid private Voice session; import a fresh HAR")
	}
	return s, nil
}
func SaveSession(s Session) error {
	path, _, err := Paths()
	if err != nil {
		return err
	}
	if _, e := os.Lstat(path); e == nil {
		if _, e = ReadPrivate(path); e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err = os.Chmod(filepath.Dir(path), 0700); err != nil { // #nosec G302 -- This is a directory: owner traversal is required, and group/other permissions are absent.
		return err
	}
	if err = verifyPrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".voice-session-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err != nil {
		return errors.Join(err, tmp.Close())
	}
	// Verify the empty temporary file before any credential bytes reach disk.
	if err = cliutil.VerifyCredsPerms(name); err != nil {
		return errors.Join(fmt.Errorf("unsafe Voice session publication: %w", err), tmp.Close())
	}
	if _, err = tmp.Write(b); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err = tmp.Sync(); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ImportHAR(raw []byte) (Session, error) {
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					URL     string                         `json:"url"`
					Headers []struct{ Name, Value string } `json:"headers"`
				} `json:"request"`
				Response struct {
					Status int `json:"status"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	if json.Unmarshal(raw, &har) != nil {
		return Session{}, fmt.Errorf("--file must contain a valid local HAR")
	}
	var out Session
	for _, e := range har.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || u.Scheme != "https" || u.Host != "clients6.google.com" || !readPaths[u.Path] || e.Response.Status != 200 {
			continue
		}
		s := Session{Cookies: map[string]string{}, Headers: map[string]string{}, PublicKey: u.Query().Get("key"), ImportedAt: time.Now().UTC().Format(time.RFC3339)}
		for _, h := range e.Request.Headers {
			name := strings.ToLower(h.Name)
			if name == "cookie" {
				for _, v := range strings.Split(h.Value, ";") {
					kv := strings.SplitN(strings.TrimSpace(v), "=", 2)
					if len(kv) == 2 && cookieNames[kv[0]] {
						s.Cookies[kv[0]] = kv[1]
					}
				}
			} else if headerNames[name] {
				s.Headers[name] = h.Value
			}
		}
		if s.Cookies["SID"] != "" && s.Cookies["SAPISID"] != "" && s.PublicKey != "" {
			out = s
		}
	}
	if len(out.Cookies) == 0 {
		return out, fmt.Errorf("HAR has no successful authenticated Voice read with cookies; export with sensitive data to a private local file")
	}
	return out, nil
}
