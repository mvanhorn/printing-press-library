package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/tabwriter"
)

// Profile holds configuration for one MT5 terminal instance.
type Profile struct {
	Name         string `json:"name"`
	TerminalPath string `json:"terminal"`  // path to terminal64.exe
	EditorPath   string `json:"editor"`    // path to metaeditor64.exe (auto-derived if empty)
	DataPath     string `json:"data_path"` // MT5 data dir (auto-derived if empty)
	Portable     bool   `json:"portable"`  // pass /portable flag
	Login        string `json:"login"`     // broker account number
	Server       string `json:"server"`    // broker server
	Description  string `json:"description"`
}

// Store is the on-disk profiles file.
type Store struct {
	Profiles map[string]*Profile `json:"profiles"`
	Default  string              `json:"default"` // name of default profile
}

// DefaultStorePath returns ~/.mt5-backtester-pp-cli/profiles.json
func DefaultStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mt5-backtester-pp-cli", "profiles.json")
}

// Load reads the profiles store from disk. Returns empty store if file missing.
func Load() (*Store, error) {
	path := DefaultStorePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Store{Profiles: make(map[string]*Profile)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]*Profile)
	}
	return &s, nil
}

// Save writes the store to disk.
func (s *Store) Save() error {
	path := DefaultStorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Get returns a profile by name, or the default if name is empty.
func (s *Store) Get(name string) (*Profile, error) {
	if name == "" {
		name = s.Default
	}
	if name == "" {
		return nil, fmt.Errorf("no profile specified and no default set — run: mt5-backtester-pp-cli profile add")
	}
	p, ok := s.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile %q not found — run: mt5-backtester-pp-cli profile list", name)
	}
	return p, nil
}

// Add adds or updates a profile.
func (s *Store) Add(p *Profile) {
	s.Profiles[p.Name] = p
	if len(s.Profiles) == 1 {
		s.Default = p.Name // first profile becomes default
	}
}

// Remove deletes a profile.
func (s *Store) Remove(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(s.Profiles, name)
	if s.Default == name {
		s.Default = ""
	}
	return nil
}

// SetDefault sets the default profile.
func (s *Store) SetDefault(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	s.Default = name
	return nil
}

// Print displays all profiles in a table.
func (s *Store) Print() {
	if len(s.Profiles) == 0 {
		fmt.Println("No profiles configured. Run: mt5-backtester-pp-cli profile add --name broker1 --terminal \"C:\\MT5\\terminal64.exe\"")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDEFAULT\tTERMINAL\tSERVER\tDESCRIPTION")
	fmt.Fprintln(w, "────\t───────\t────────\t──────\t───────────")
	for name, p := range s.Profiles {
		def := ""
		if name == s.Default {
			def = "✓"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, def, p.TerminalPath, p.Server, p.Description)
	}
	w.Flush()
}

// DeriveEditorPath returns the metaeditor64.exe path alongside terminal64.exe.
func DeriveEditorPath(terminalPath string) string {
	dir := filepath.Dir(terminalPath)
	candidate := filepath.Join(dir, "metaeditor64.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// DeriveDataPath returns the MT5 data directory for a portable or standard install.
func DeriveDataPath(terminalPath string, portable bool) string {
	if portable {
		// Portable: data lives next to terminal64.exe
		return filepath.Dir(terminalPath)
	}
	// Standard: data lives in %APPDATA%\MetaQuotes\Terminal\<hash>
	// We can't easily derive the hash without reading origin.txt, so return APPDATA base
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "MetaQuotes", "Terminal")
		}
	}
	return ""
}

// WriteServiceScript generates a PowerShell script to install all profiles as Windows services.
func WriteServiceScript(s *Store, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprint(f, `# mt5-backtester-pp-cli — Install MT5 instances as Windows Services via NSSM
# Requires NSSM: https://nssm.cc or 'choco install nssm'
# Run as Administrator

$nssmPath = (Get-Command nssm -ErrorAction SilentlyContinue)?.Source
if (-not $nssmPath) {
    Write-Host "NSSM not found. Install it:" -ForegroundColor Red
    Write-Host "  choco install nssm" -ForegroundColor Yellow
    Write-Host "  or download from https://nssm.cc/download" -ForegroundColor Yellow
    exit 1
}

Write-Host "Installing MT5 services..." -ForegroundColor Cyan
`)

	for name, p := range s.Profiles {
		svcName := "MT5-" + name
		args := `/config:"` + filepath.Join(filepath.Dir(p.TerminalPath), "config", "common.ini") + `"`
		if p.Portable {
			args = "/portable " + args
		}
		fmt.Fprintf(f, `
# ── %s ──────────────────────────────────────────
$svc = %q
$exe = %q
$args = %q

Write-Host "Installing service: $svc" -ForegroundColor Yellow
nssm install $svc $exe
nssm set $svc AppParameters $args
nssm set $svc Start SERVICE_AUTO_START
nssm set $svc AppStdout "$env:TEMP\$svc-stdout.log"
nssm set $svc AppStderr "$env:TEMP\$svc-stderr.log"
nssm set $svc Description "MetaTrader 5 - %s (mt5-backtester-pp-cli managed)"
nssm start $svc
Write-Host "Started: $svc" -ForegroundColor Green
`, name, svcName, p.TerminalPath, args, p.Description)
	}

	fmt.Fprint(f, `
Write-Host ""
Write-Host "Done! Services installed:" -ForegroundColor Green
Get-Service | Where-Object { $_.Name -like "MT5-*" } | Format-Table Name, Status, StartType
`)

	return nil
}
