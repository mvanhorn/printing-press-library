package backtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunOptions configures a single backtest execution.
type RunOptions struct {
	TerminalPath string
	INIPath      string
	ReportDir    string
	Timeout      time.Duration
	Portable     bool // Pass /portable flag (uses terminal's own dir for data)
	Verbose      bool
}

// RunResult holds the outcome of a terminal run.
type RunResult struct {
	Success    bool
	INIPath    string
	ReportPath string // Path to generated HTML report
	Duration   time.Duration
	ExitCode   int
	Error      error
}

// Run launches terminal64.exe with the given ini config and waits for completion.
// MT5 exits automatically when ShutdownTerminal=1 is set in the ini [Tester] section.
func Run(opts RunOptions) *RunResult {
	start := time.Now()
	result := &RunResult{INIPath: opts.INIPath}

	args := []string{
		fmt.Sprintf(`/config:%s`, opts.INIPath),
	}
	if opts.Portable {
		args = append(args, "/portable")
	}

	if opts.Verbose {
		fmt.Printf("→ Launching: %s %s\n", opts.TerminalPath, strings.Join(args, " "))
	}

	cmd := exec.Command(opts.TerminalPath, args...)

	// Set timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 4 * time.Hour // generous default
	}

	// Start process
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("start terminal: %w", err)
		return result
	}

	// Wait with timeout
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		result.Duration = time.Since(start)
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
			// MT5 sometimes returns non-zero even on success; check for report file
		} else {
			result.ExitCode = 0
			result.Success = true
		}
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		result.Duration = time.Since(start)
		result.Error = fmt.Errorf("backtest timed out after %s", timeout)
		return result
	}

	// Find generated report
	reportPath := findReport(opts.ReportDir, opts.INIPath)
	if reportPath != "" {
		result.ReportPath = reportPath
		result.Success = true // if report exists, test ran
	} else if result.ExitCode != 0 {
		result.Success = false
		result.Error = fmt.Errorf("terminal exited with code %d and no report found", result.ExitCode)
	}

	return result
}

// findReport looks for the HTML/XML report MT5 generates.
// MT5 writes reports to the terminal data path; we search the report dir.
func findReport(reportDir, iniPath string) string {
	// Report name is based on the ini Report= key; try to find recent HTML files
	candidates := []string{reportDir}

	// Also check common MT5 report locations
	candidates = append(candidates,
		filepath.Join(filepath.Dir(iniPath)),
		filepath.Join(os.Getenv("APPDATA"), "MetaQuotes", "Terminal"),
	)

	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		// Walk looking for recently modified .htm files
		matches, _ := filepath.Glob(filepath.Join(dir, "*.htm"))
		for _, m := range matches {
			if isRecentFile(m, 10*time.Minute) {
				return m
			}
		}
		matches, _ = filepath.Glob(filepath.Join(dir, "*.html"))
		for _, m := range matches {
			if isRecentFile(m, 10*time.Minute) {
				return m
			}
		}
	}
	return ""
}

func isRecentFile(path string, within time.Duration) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(fi.ModTime()) < within
}

// WatchLogs tails MT5 terminal log files for progress updates.
// MT5 writes logs to <DataPath>/logs/YYYYMMDD.log
func WatchLogs(dataPath string, since time.Time, out chan<- string, stop <-chan struct{}) {
	logDir := filepath.Join(dataPath, "logs")
	logFile := filepath.Join(logDir, time.Now().Format("20060102")+".log")

	var lastSize int64
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			fi, err := os.Stat(logFile)
			if err != nil {
				continue
			}
			if fi.Size() == lastSize {
				continue
			}
			f, err := os.Open(logFile)
			if err != nil {
				continue
			}
			if lastSize > 0 {
				_, _ = f.Seek(lastSize, 0)
			}
			buf := make([]byte, fi.Size()-lastSize)
			n, _ := f.Read(buf)
			f.Close()
			lastSize = fi.Size()
			if n > 0 {
				lines := strings.Split(string(buf[:n]), "\n")
				for _, l := range lines {
					l = strings.TrimSpace(l)
					if l != "" {
						out <- l
					}
				}
			}
		}
	}
}
