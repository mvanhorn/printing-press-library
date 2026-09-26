package tbprofile

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Prefs holds prefs.js values as strings (booleans and numbers as text).
type Prefs map[string]string

var userPrefRE = regexp.MustCompile(`^\s*user_pref\(\s*"((?:[^"\\]|\\.)*)"\s*,\s*(.*?)\s*\)\s*;\s*$`)

// ParsePrefs reads a prefs.js file.
func ParsePrefs(path string) (Prefs, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParsePrefsReader(f)
}

// ParsePrefsReader parses user_pref lines from r.
func ParsePrefsReader(r io.Reader) (Prefs, error) {
	out := Prefs{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		m := userPrefRE.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		out[unescapeJS(m[1])] = prefValue(m[2])
	}
	return out, sc.Err()
}

func prefValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return unescapeJS(raw[1 : len(raw)-1])
	}
	return raw
}

func unescapeJS(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'u':
			if i+4 < len(s) {
				if n, err := strconv.ParseUint(s[i+1:i+5], 16, 16); err == nil {
					b.WriteRune(rune(n)) // #nosec G115 -- bitSize 16 bounds n to 0..0xFFFF
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// Identity is a mail.identity.* sending identity.
type Identity struct {
	Key      string `json:"id"`
	Account  string `json:"account"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// Server is a mail.server.* incoming server.
type Server struct {
	Key       string `json:"key"`
	Type      string `json:"type"`
	Hostname  string `json:"hostname"`
	Name      string `json:"name"`
	UserName  string `json:"user_name"`
	Directory string `json:"directory"`
}

// Account is a mail.account.* entry with its server and identities.
type Account struct {
	Key        string     `json:"id"`
	Server     Server     `json:"server"`
	Identities []Identity `json:"identities"`
}

// Name returns the human account label.
func (a Account) Name() string {
	if a.Server.Name != "" {
		return a.Server.Name
	}
	if a.Server.UserName != "" && a.Server.Hostname != "" {
		return a.Server.UserName + "@" + a.Server.Hostname
	}
	return a.Key
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Accounts builds the account list in mail.accountmanager.accounts order.
func (p Prefs) Accounts(profileDir string) []Account {
	keys := splitList(p["mail.accountmanager.accounts"])
	if len(keys) == 0 {
		seen := map[string]bool{}
		for k := range p {
			if rest, ok := strings.CutPrefix(k, "mail.account."); ok {
				if acc, _, ok := strings.Cut(rest, "."); ok && !seen[acc] {
					seen[acc] = true
					keys = append(keys, acc)
				}
			}
		}
		sort.Strings(keys)
	}
	out := make([]Account, 0, len(keys))
	for _, key := range keys {
		srvKey := p["mail.account."+key+".server"]
		if srvKey == "" {
			continue
		}
		acc := Account{Key: key, Server: p.server(srvKey, profileDir), Identities: []Identity{}}
		for _, idKey := range splitList(p["mail.account."+key+".identities"]) {
			pre := "mail.identity." + idKey + "."
			acc.Identities = append(acc.Identities, Identity{
				Key:      idKey,
				Account:  key,
				Email:    p[pre+"useremail"],
				FullName: p[pre+"fullName"],
			})
		}
		out = append(out, acc)
	}
	return out
}

func (p Prefs) server(key, profileDir string) Server {
	pre := "mail.server." + key + "."
	s := Server{
		Key:      key,
		Type:     p[pre+"type"],
		Hostname: p[pre+"hostname"],
		Name:     p[pre+"name"],
		UserName: p[pre+"userName"],
	}
	s.Directory = ServerDirectory(profileDir, p[pre+"directory-rel"], p[pre+"directory"])
	return s
}

// ServerDirectory resolves a server's mail directory, preferring the
// [ProfD]-relative form so a moved profile still resolves.
func ServerDirectory(profileDir, rel, abs string) string {
	if r, ok := strings.CutPrefix(rel, "[ProfD]"); ok && r != "" {
		return filepath.Join(profileDir, filepath.FromSlash(r))
	}
	if abs != "" {
		return filepath.FromSlash(abs)
	}
	return ""
}

// LoadAccounts parses profileDir/prefs.js and returns its accounts.
func LoadAccounts(profileDir string) ([]Account, error) {
	p, err := ParsePrefs(filepath.Join(profileDir, "prefs.js"))
	if err != nil {
		return nil, err
	}
	return p.Accounts(profileDir), nil
}
