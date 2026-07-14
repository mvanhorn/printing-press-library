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
	KeepOpen     bool // ini has ShutdownTerminal=0: poll for the report, never wait for exit
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

	if opts.KeepOpen {
		// ShutdownTerminal=0 keeps the terminal open after the test, so
		// cmd.Wait() would sit at the timeout and then kill the terminal we
		// were asked to preserve. Poll for the report instead and leave the
		// process running (the goroutine just reaps it if the user closes it).
		go func() { _ = cmd.Wait() }()
		for {
			if p := findReport(opts.ReportDir, opts.INIPath, start); p != "" {
				result.ReportPath = p
				result.Success = true
				result.Duration = time.Since(start)
				return result
			}
			if time.Since(start) > timeout {
				result.Duration = time.Since(start)
				result.Error = fmt.Errorf("no report after %s — terminal left running (--no-shutdown)", timeout)
				return result
			}
			time.Sleep(2 * time.Second)
		}
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
	reportPath := findReport(opts.ReportDir, opts.INIPath, start)
	if reportPath != "" {
		result.ReportPath = reportPath
		result.Success = true // if report exists, test ran
	} else if result.ExitCode != 0 {
		result.Success = false
		result.Error = fmt.Errorf("terminal exited with code %d and no report found", result.ExitCode)
	}

	return result
}

// findReport looks for the HTML report MT5 generated after `since` (the run
// start), returning the newest match. A stock install writes it under the
// instance data dir %APPDATA%\MetaQuotes\Terminal\<hash>\ — either the root
// or its tester\ subdirectory — so those are globbed per instance; portable
// installs land in ReportDir or next to the ini.
func findReport(reportDir, iniPath string, since time.Time) string {
	var patterns []string
	addDir := func(dir string) {
		if dir == "" {
			return
		}
		for _, ext := range []string{"*.htm", "*.html"} {
			patterns = append(patterns,
				filepath.Join(dir, ext),
				filepath.Join(dir, "tester", ext),
			)
		}
	}
	addDir(reportDir)
	addDir(filepath.Dir(iniPath))
	if app := os.Getenv("APPDATA"); app != "" {
		instances := filepath.Join(app, "MetaQuotes", "Terminal")
		for _, ext := range []string{"*.htm", "*.html"} {
			patterns = append(patterns,
				filepath.Join(instances, "*", ext),
				filepath.Join(instances, "*", "tester", ext),
			)
		}
	}

	best, bestTime := "", time.Time{}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil || fi.ModTime().Before(since) {
				continue
			}
			if fi.ModTime().After(bestTime) {
				best, bestTime = m, fi.ModTime()
			}
		}
	}
	return best
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
