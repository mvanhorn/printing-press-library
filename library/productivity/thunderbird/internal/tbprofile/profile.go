// Package tbprofile reads a Mozilla Thunderbird profile from disk without
// ever writing to it.
package tbprofile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// EnvProfile selects the profile (a directory path or a profiles.ini Name).
const EnvProfile = "THUNDERBIRD_PROFILE"

// EnvRoot overrides the Thunderbird root directory holding profiles.ini.
// It must differ from THUNDERBIRD_HOME, which relocates the CLI's own dirs.
const EnvRoot = "THUNDERBIRD_ROOT"

// ErrNoProfile reports that no Thunderbird installation was found.
var ErrNoProfile = errors.New("no Thunderbird profile found")

// ProfileEntry is one [ProfileN] section of profiles.ini.
type ProfileEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDefault bool   `json:"is_default"`
	Exists    bool   `json:"exists"`
}

// RootDir returns the directory holding profiles.ini for this OS.
func RootDir() string {
	if v := strings.TrimSpace(os.Getenv(EnvRoot)); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Thunderbird")
		}
		return filepath.Join(home, "AppData", "Roaming", "Thunderbird")
	case "darwin":
		return filepath.Join(home, "Library", "Thunderbird")
	default:
		return filepath.Join(home, ".thunderbird")
	}
}

// ParseINI reads a simple INI file into section -> key -> value.
func ParseINI(path string) (map[string]map[string]string, []string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	out := map[string]map[string]string{}
	var order []string
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			if _, ok := out[section]; !ok {
				out[section] = map[string]string{}
				order = append(order, section)
			}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, ok := out[section]; !ok {
			out[section] = map[string]string{}
			order = append(order, section)
		}
		out[section][strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, order, sc.Err()
}

func resolveINIPath(root string, sec map[string]string) string {
	p := sec["Path"]
	if p == "" {
		return ""
	}
	if sec["IsRelative"] == "1" {
		return filepath.Join(root, filepath.FromSlash(p))
	}
	return filepath.FromSlash(p)
}

// ListProfiles lists every profile declared in root/profiles.ini.
func ListProfiles(root string) ([]ProfileEntry, error) {
	secs, order, err := ParseINI(filepath.Join(root, "profiles.ini"))
	if err != nil {
		if os.IsNotExist(err) {
			return []ProfileEntry{}, nil
		}
		return nil, err
	}
	def := installDefault(root)
	out := make([]ProfileEntry, 0)
	for _, name := range order {
		if !strings.HasPrefix(name, "Profile") {
			continue
		}
		sec := secs[name]
		p := resolveINIPath(root, sec)
		if p == "" {
			continue
		}
		st, statErr := os.Stat(p)
		isDef := sec["Default"] == "1"
		if def != "" {
			isDef = samePath(def, p)
		}
		out = append(out, ProfileEntry{Name: sec["Name"], Path: p, IsDefault: isDef, Exists: statErr == nil && st.IsDir()})
	}
	return out, nil
}

func installDefault(root string) string {
	secs, order, err := ParseINI(filepath.Join(root, "installs.ini"))
	if err != nil {
		return ""
	}
	sort.Strings(order)
	for _, name := range order {
		d := secs[name]["Default"]
		if d == "" {
			continue
		}
		if filepath.IsAbs(d) {
			return filepath.FromSlash(d)
		}
		return filepath.Join(root, filepath.FromSlash(d))
	}
	return ""
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Resolve picks the profile directory. selector (flag or env value) may be
// a directory path or a profiles.ini Name; when empty the installs.ini
// default wins, then the profiles.ini Default=1 entry, then the first one.
func Resolve(selector, root string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		if st, err := os.Stat(selector); err == nil && st.IsDir() {
			return filepath.Abs(selector)
		}
		profiles, err := ListProfiles(root)
		if err != nil {
			return "", err
		}
		for _, p := range profiles {
			if p.Name == selector || filepath.Base(p.Path) == selector {
				if !p.Exists {
					return "", fmt.Errorf("thunderbird profile %q points to missing directory %s", selector, p.Path)
				}
				return p.Path, nil
			}
		}
		return "", fmt.Errorf("thunderbird profile %q not found (not a directory and not a profile name in %s)", selector, filepath.Join(root, "profiles.ini"))
	}
	if d := installDefault(root); d != "" {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d, nil
		}
	}
	profiles, err := ListProfiles(root)
	if err != nil {
		return "", err
	}
	for _, p := range profiles {
		if p.IsDefault && p.Exists {
			return p.Path, nil
		}
	}
	for _, p := range profiles {
		if p.Exists {
			return p.Path, nil
		}
	}
	return "", ErrNoProfile
}

// LockPresent reports whether Thunderbird holds the profile lock file.
func LockPresent(profileDir string) bool {
	for _, name := range []string{"parent.lock", "lock", ".parentlock"} {
		if _, err := os.Lstat(filepath.Join(profileDir, name)); err == nil {
			return true
		}
	}
	return false
}
