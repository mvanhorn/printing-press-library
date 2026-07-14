package compile

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result holds the output of a compilation attempt.
type Result struct {
	Success    bool
	SourceFile string
	OutputFile string
	Errors     []string
	Warnings   []string
	Duration   time.Duration
	LogPath    string
}

// Options for compilation.
type Options struct {
	MetaEditorPath string
	SourceFile     string   // .mq5 file path
	IncludePaths   []string // Additional include dirs
	Verbose        bool
}

// Run compiles an MQL5 source file using metaeditor64.exe.
// metaeditor64 /compile:"path\to\EA.mq5" /log /inc:"extra\includes"
func Run(opts Options) (*Result, error) {
	start := time.Now()
	result := &Result{
		SourceFile: opts.SourceFile,
	}

	// Determine expected output file
	result.OutputFile = strings.TrimSuffix(opts.SourceFile, ".mq5") + ".ex5"

	// Build args
	args := []string{
		fmt.Sprintf(`/compile:%s`, opts.SourceFile),
		"/log",
	}
	if len(opts.IncludePaths) > 0 {
		args = append(args, fmt.Sprintf(`/inc:%s`, strings.Join(opts.IncludePaths, ";")))
	}

	cmd := exec.Command(opts.MetaEditorPath, args...)
	cmd.Dir = filepath.Dir(opts.SourceFile)

	out, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)

	// metaeditor exits 0 on success, 1 on error — but sometimes lies.
	// Parse the log file it writes alongside the source.
	logPath := strings.TrimSuffix(opts.SourceFile, ".mq5") + ".log"
	result.LogPath = logPath

	// Parse log if it exists
	if logContent, lerr := os.ReadFile(logPath); lerr == nil {
		parseLog(string(logContent), result)
	} else if len(out) > 0 {
		// Fall back to stdout/stderr
		parseLog(string(out), result)
	}

	// Double-check: if .ex5 was created, it's a success regardless of exit code
	if _, serr := os.Stat(result.OutputFile); serr == nil {
		result.Success = true
	} else if err != nil {
		result.Success = false
		if len(result.Errors) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("compiler exited with error: %v", err))
		}
	}

	return result, nil
}

func parseLog(content string, result *Result) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "error"):
			result.Errors = append(result.Errors, line)
		case strings.Contains(lower, "warning"):
			result.Warnings = append(result.Warnings, line)
		case strings.Contains(lower, "0 error"):
			result.Success = true
		}
	}
}

// PrintResult displays compilation result to stdout.
func PrintResult(result *Result, verbose bool) {
	fmt.Printf("\nCompile: %s\n", filepath.Base(result.SourceFile))
	fmt.Printf("Output:  %s\n", filepath.Base(result.OutputFile))
	fmt.Printf("Time:    %s\n\n", result.Duration.Round(time.Millisecond))

	if result.Success {
		fmt.Println("✓ Compiled successfully")
	} else {
		fmt.Println("✗ Compilation failed")
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  ! %s\n", e)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("\nWarnings (%d):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("  ~ %s\n", w)
		}
	}

	if verbose && result.LogPath != "" {
		fmt.Printf("\nFull log: %s\n", result.LogPath)
	}
}
