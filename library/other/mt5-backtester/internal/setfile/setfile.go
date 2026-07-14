// Package setfile parses and writes MetaTrader 5 strategy tester .set files.
//
// MT5 set file format (one parameter per line):
//
//	StopLoss=50||50||1||200||N
//	TakeProfit=100||100||1||500||Y
//	MagicNumber=99001
//	UseFilter=true
//
// Pipe-delimited fields are: value||opt_start||opt_step||opt_stop||opt_enabled.
// Y/N at the end controls whether the parameter is included in optimization
// sweeps. Lines without || are simple values with no optimization range.
// Lines beginning with ';' or blank lines are skipped.
package setfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Param is one row in a .set file.
type Param struct {
	Name     string
	Value    string
	HasRange bool   // true if line used || optimization syntax
	OptStart string // optimization start value (HasRange only)
	OptStep  string // optimization step (HasRange only)
	OptStop  string // optimization stop value (HasRange only)
	OptOn    bool   // optimization enabled (Y vs N)
}

// SetFile is an ordered collection of parameters.
type SetFile struct {
	Params []Param
}

// ── parsing ──────────────────────────────────────────────────────────────────

// Parse reads a .set file from disk.
func Parse(path string) (*SetFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open set file: %w", err)
	}
	defer f.Close()
	return ParseReader(f)
}

// ParseReader parses a .set file from a reader.
func ParseReader(r io.Reader) (*SetFile, error) {
	sf := &SetFile{}
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("line %d: expected name=value, got %q", lineNo, line)
		}
		name := strings.TrimSpace(line[:eq])
		rest := line[eq+1:]

		p := Param{Name: name}
		if strings.Contains(rest, "||") {
			parts := strings.Split(rest, "||")
			// Expected: value, start, step, stop, Y/N. Be lenient with shorter forms.
			p.HasRange = true
			if len(parts) > 0 {
				p.Value = parts[0]
			}
			if len(parts) > 1 {
				p.OptStart = parts[1]
			}
			if len(parts) > 2 {
				p.OptStep = parts[2]
			}
			if len(parts) > 3 {
				p.OptStop = parts[3]
			}
			if len(parts) > 4 {
				p.OptOn = strings.EqualFold(strings.TrimSpace(parts[4]), "Y")
			}
		} else {
			p.Value = rest
		}
		sf.Params = append(sf.Params, p)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read set file: %w", err)
	}
	return sf, nil
}

// ── writing ──────────────────────────────────────────────────────────────────

// Write saves a .set file to disk.
func (sf *SetFile) Write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create set file: %w", err)
	}
	defer f.Close()
	return sf.WriteWriter(f)
}

// WriteWriter renders a .set file to a writer.
func (sf *SetFile) WriteWriter(w io.Writer) error {
	bw := bufio.NewWriter(w)
	for _, p := range sf.Params {
		if _, err := bw.WriteString(p.Line() + "\n"); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// Line renders one parameter as a .set file line.
func (p Param) Line() string {
	if !p.HasRange {
		return p.Name + "=" + p.Value
	}
	yn := "N"
	if p.OptOn {
		yn = "Y"
	}
	return fmt.Sprintf("%s=%s||%s||%s||%s||%s",
		p.Name, p.Value, p.OptStart, p.OptStep, p.OptStop, yn)
}

// ── conversions ──────────────────────────────────────────────────────────────

// TesterInputs returns parameters in the format MT5 expects in the
// [TesterInputs] section of the backtester ini file. The format is the same
// pipe-delimited form as the source .set file. Parameters without an
// optimization range are emitted as a bare value (no pipes).
func (sf *SetFile) TesterInputs() map[string]string {
	out := make(map[string]string, len(sf.Params))
	for _, p := range sf.Params {
		out[p.Name] = p.TesterValue()
	}
	return out
}

// TesterValue renders the right-hand side of a [TesterInputs] entry.
func (p Param) TesterValue() string {
	if !p.HasRange {
		return p.Value
	}
	yn := "N"
	if p.OptOn {
		yn = "Y"
	}
	return fmt.Sprintf("%s||%s||%s||%s||%s",
		p.Value, p.OptStart, p.OptStep, p.OptStop, yn)
}

// OrderedNames returns parameter names in the order they were parsed/added.
// Useful for stable [TesterInputs] output when callers need it.
func (sf *SetFile) OrderedNames() []string {
	names := make([]string, len(sf.Params))
	for i, p := range sf.Params {
		names[i] = p.Name
	}
	return names
}

// Get returns the param with the given name and whether it exists.
func (sf *SetFile) Get(name string) (Param, bool) {
	for _, p := range sf.Params {
		if p.Name == name {
			return p, true
		}
	}
	return Param{}, false
}

// Set adds or updates a parameter, preserving any optimization range when the
// name already exists (only Value is replaced).
func (sf *SetFile) Set(name, value string) {
	for i := range sf.Params {
		if sf.Params[i].Name == name {
			sf.Params[i].Value = value
			return
		}
	}
	sf.Params = append(sf.Params, Param{Name: name, Value: value})
}

// ApplyOverrides updates Value on existing params and appends any new ones.
// Override values are simple key=value (no optimization ranges).
func (sf *SetFile) ApplyOverrides(overrides map[string]string) {
	if len(overrides) == 0 {
		return
	}
	// Sort override keys for deterministic append order.
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sf.Set(k, overrides[k])
	}
}

// FromInputs converts a simple key=value map into a SetFile with no
// optimization ranges. Keys are emitted in sorted order for determinism.
func FromInputs(inputs map[string]string) *SetFile {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sf := &SetFile{Params: make([]Param, 0, len(keys))}
	for _, k := range keys {
		sf.Params = append(sf.Params, Param{Name: k, Value: inputs[k]})
	}
	return sf
}

// IsBool reports whether a value looks like a boolean literal MT5 accepts.
func IsBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "false":
		return true
	}
	return false
}

// ── path resolution ──────────────────────────────────────────────────────────

// TesterSubdir is the conventional subpath where MT5 stores .set files inside
// a terminal data folder.
const TesterSubdir = `MQL5\Profiles\Tester`

// ResolvePath finds an absolute path for a .set file referenced by name.
//
// Resolution order:
//  1. If name is absolute or exists as given, use it.
//  2. If portable, look under <terminalDir>/MQL5/Profiles/Tester/.
//  3. Otherwise, scan %APPDATA%/MetaQuotes/Terminal/*/MQL5/Profiles/Tester/
//     and return the first match.
//
// terminalPath may be empty if the caller cannot resolve it; in that case
// steps (2) and (3) for portable installs are skipped.
func ResolvePath(name, terminalPath string, portable bool) (string, error) {
	if name == "" {
		return "", fmt.Errorf("set file name is empty")
	}
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", fmt.Errorf("set file not found: %s", name)
	}
	if _, err := os.Stat(name); err == nil {
		abs, _ := filepath.Abs(name)
		return abs, nil
	}

	if portable && terminalPath != "" {
		cand := filepath.Join(filepath.Dir(terminalPath), TesterSubdir, name)
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}

	for _, dir := range TesterDirs(terminalPath, portable) {
		cand := filepath.Join(dir, name)
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}

	return "", fmt.Errorf("set file %q not found in CWD or any %s directory", name, TesterSubdir)
}

// TesterDirs returns every candidate MQL5\Profiles\Tester directory for the
// current host. Portable terminals expose exactly one location; standard
// installs may have many (one per terminal hash under %APPDATA%).
func TesterDirs(terminalPath string, portable bool) []string {
	var dirs []string
	if portable && terminalPath != "" {
		dirs = append(dirs, filepath.Join(filepath.Dir(terminalPath), TesterSubdir))
		return dirs
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			base := filepath.Join(appData, "MetaQuotes", "Terminal")
			entries, err := os.ReadDir(base)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					// Skip the "Common" pseudo-dir; only hash dirs hold MQL5.
					if strings.EqualFold(e.Name(), "Common") {
						continue
					}
					cand := filepath.Join(base, e.Name(), TesterSubdir)
					if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
						dirs = append(dirs, cand)
					}
				}
			}
		}
	}
	if terminalPath != "" {
		// Always also try the portable layout as a last-resort fallback.
		dirs = append(dirs, filepath.Join(filepath.Dir(terminalPath), TesterSubdir))
	}
	return dirs
}

// ListSetFiles returns paths of all .set files found across TesterDirs.
func ListSetFiles(terminalPath string, portable bool) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, dir := range TesterDirs(terminalPath, portable) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.EqualFold(filepath.Ext(e.Name()), ".set") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			if _, ok := seen[full]; ok {
				continue
			}
			seen[full] = struct{}{}
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}
