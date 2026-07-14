package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/mt5-backtester/internal/setfile"
)

// MT5Config holds all paths and connection info for the MT5 installation.
type MT5Config struct {
	TerminalPath   string // Path to terminal64.exe
	MetaEditorPath string // Path to metaeditor64.exe
	DataPath       string // MT5 data directory (AppData/Roaming/...)
	Login          string // Broker account number (optional)
	Server         string // Broker server (optional)
	Password       string // Account password (optional)
}

// BacktestParams defines a single backtest run.
type BacktestParams struct {
	Expert       string            // EA name e.g. "Experts\\MyEA.ex5"
	Symbol       string            // e.g. "EURUSD"
	Period       string            // e.g. "H1", "M15", "D1"
	FromDate     string            // e.g. "2023.01.01"
	ToDate       string            // e.g. "2024.01.01"
	Model        int               // 0=Every tick, 1=1min OHLC, 2=Open prices only, 4=Real ticks
	Deposit      float64           // Starting deposit
	Currency     string            // Deposit currency e.g. "USD"
	Leverage     int               // e.g. 100
	Optimization int               // 0=disabled, 1=slow complete, 2=genetic, 3=all symbols
	ForwardMode  int               // 0=no forward, 1=1/2, 2=1/3, 3=1/4, 4=custom
	ForwardDate  string            // Used when ForwardMode=4
	Report       string            // Report filename (no extension)
	ShutdownMode int               // 0=don't shutdown, 1=shutdown after test
	Inputs       map[string]string // EA input parameters (override SetFile values)
	SetFile      string            // Absolute path to a .set file to seed [TesterInputs]
}

// ModelName returns human-readable tick model name.
func (p *BacktestParams) ModelName() string {
	switch p.Model {
	case 0:
		return "Every Tick"
	case 1:
		return "1 Minute OHLC"
	case 2:
		return "Open Prices Only"
	case 4:
		return "Real Ticks"
	default:
		return fmt.Sprintf("Model(%d)", p.Model)
	}
}

// PeriodMinutes converts period string to minutes for display.
func PeriodMinutes(period string) string {
	periods := map[string]string{
		"M1": "1", "M2": "2", "M3": "3", "M4": "4", "M5": "5",
		"M6": "6", "M10": "10", "M12": "12", "M15": "15", "M20": "20",
		"M30": "30", "H1": "60", "H2": "120", "H3": "180", "H4": "240",
		"H6": "360", "H8": "480", "H12": "720", "D1": "1440",
		"W1": "10080", "MN1": "43200",
	}
	if v, ok := periods[strings.ToUpper(period)]; ok {
		return v
	}
	return period
}

var iniTemplate = `; MT5 Backtester CLI — generated {{ .Timestamp }}
[Tester]
Expert={{ .Expert }}
Symbol={{ .Symbol }}
Period={{ .Period }}
Optimization={{ .Optimization }}
Model={{ .Model }}
FromDate={{ .FromDate }}
ToDate={{ .ToDate }}
ForwardMode={{ .ForwardMode }}
{{ if .ForwardDate }}ForwardDate={{ .ForwardDate }}{{ end }}
Deposit={{ printf "%.2f" .Deposit }}
Currency={{ .Currency }}
Leverage={{ .Leverage }}
ShutdownTerminal={{ .ShutdownMode }}
Report={{ .Report }}
ReplaceReport=1
{{ if .TesterInputs }}
[TesterInputs]
{{ range .TesterInputs }}{{ .Name }}={{ .Value }}
{{ end }}{{ end }}`

// testerInput is one rendered row of the [TesterInputs] section.
type testerInput struct {
	Name  string
	Value string
}

type iniTemplateData struct {
	BacktestParams
	Timestamp    string
	TesterInputs []testerInput
}

// WriteINI generates a tester .ini file and returns its path.
func WriteINI(params *BacktestParams, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Sanitize report name
	reportBase := params.Report
	if reportBase == "" {
		safe := strings.NewReplacer(" ", "_", "\\", "_", "/", "_", ".", "_")
		reportBase = fmt.Sprintf("%s_%s_%s_%s",
			safe.Replace(params.Expert),
			params.Symbol,
			params.Period,
			time.Now().Format("20060102_150405"),
		)
	}

	iniPath := filepath.Join(outDir, reportBase+".ini")

	tmpl, err := template.New("ini").Parse(iniTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	f, err := os.Create(iniPath)
	if err != nil {
		return "", fmt.Errorf("create ini: %w", err)
	}
	defer f.Close()

	rendered, err := buildTesterInputs(params)
	if err != nil {
		return "", err
	}

	data := iniTemplateData{
		BacktestParams: *params,
		Timestamp:      time.Now().Format("2006-01-02 15:04:05"),
		TesterInputs:   rendered,
	}
	data.Report = reportBase

	if err := tmpl.Execute(f, data); err != nil {
		return "", fmt.Errorf("write ini: %w", err)
	}

	return iniPath, nil
}

// buildTesterInputs renders the [TesterInputs] rows. When a SetFile is set it
// is parsed and used as the base; Inputs override individual values while
// preserving any optimization range from the set file. When no SetFile is set
// the simple key=value Inputs are emitted in sorted order.
func buildTesterInputs(params *BacktestParams) ([]testerInput, error) {
	if params.SetFile == "" {
		if len(params.Inputs) == 0 {
			return nil, nil
		}
		keys := make([]string, 0, len(params.Inputs))
		for k := range params.Inputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]testerInput, 0, len(keys))
		for _, k := range keys {
			out = append(out, testerInput{Name: k, Value: params.Inputs[k]})
		}
		return out, nil
	}

	sf, err := setfile.Parse(params.SetFile)
	if err != nil {
		return nil, fmt.Errorf("read set file %s: %w", params.SetFile, err)
	}
	sf.ApplyOverrides(params.Inputs)
	out := make([]testerInput, 0, len(sf.Params))
	for _, p := range sf.Params {
		out = append(out, testerInput{Name: p.Name, Value: p.TesterValue()})
	}
	return out, nil
}

// DefaultMT5Paths returns common MT5 install locations on Windows.
func DefaultMT5Paths() []string {
	return []string{
		`C:\Program Files\MetaTrader 5\terminal64.exe`,
		`C:\Program Files (x86)\MetaTrader 5\terminal64.exe`,
		`C:\MT5\terminal64.exe`,
	}
}

// FindTerminal searches common paths and $MT5_PATH env var.
func FindTerminal() string {
	if env := os.Getenv("MT5_PATH"); env != "" {
		candidate := filepath.Join(env, "terminal64.exe")
		if fileExists(candidate) {
			return candidate
		}
		if fileExists(env) {
			return env
		}
	}
	for _, p := range DefaultMT5Paths() {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// FindMetaEditor derives metaeditor64.exe path from terminal path.
func FindMetaEditor(terminalPath string) string {
	dir := filepath.Dir(terminalPath)
	candidate := filepath.Join(dir, "metaeditor64.exe")
	if fileExists(candidate) {
		return candidate
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ResolveExpertPath takes a user-supplied EA name and ensures it has the
// Experts\ prefix and .ex5 extension as MT5 expects in the ini.
func ResolveExpertPath(ea string) string {
	// Strip leading path separators
	ea = strings.TrimLeft(ea, `/\`)

	// Add Experts\ prefix if not present
	if !strings.HasPrefix(strings.ToLower(ea), "experts") {
		ea = `Experts\` + ea
	}

	// Ensure .ex5 extension (strip .mq5 if given)
	ea = strings.TrimSuffix(ea, ".mq5")
	if !strings.HasSuffix(strings.ToLower(ea), ".ex5") {
		ea += ".ex5"
	}

	return ea
}
