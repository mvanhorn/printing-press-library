package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RegistryEntry represents a single video<->script entry parsed from
// content-registry.md.
type RegistryEntry struct {
	VideoID    string
	Title      string
	ScriptPath string // resolved absolute path to the script, when discoverable
	ProjectDir string // "project files" reference if present
}

// videoIDRegex matches lines like `**Video ID:** dQw4w9WgXcQ` (the canonical shape).
var videoIDRegex = regexp.MustCompile(`(?i)^\s*-?\s*\*\*Video ID:\*\*\s+(\S+)\s*$`)

// projectFilesRegex matches lines like `**Project files:** \`projects/foo/\“.
var projectFilesRegex = regexp.MustCompile("(?i)^\\s*-?\\s*\\*\\*Project files:\\*\\*\\s+`([^`]+)`")

// headingRegex matches a `### <Title>` heading.
var headingRegex = regexp.MustCompile(`^###\s+(.+?)\s*$`)

// ParseRegistryFile parses ~/.openclaw/workspace/data/content-registry.md (or
// any path passed) and returns one RegistryEntry per video found.
//
// The format we look for:
//
//	### <Title>
//	- **Video ID:** <id>
//	- **Project files:** `projects/<dir>/`
func ParseRegistryFile(path string) ([]RegistryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		entries []RegistryEntry
		current RegistryEntry
		haveID  bool
	)

	flush := func() {
		if haveID {
			entries = append(entries, current)
		}
		current = RegistryEntry{}
		haveID = false
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 256*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := headingRegex.FindStringSubmatch(line); m != nil {
			flush()
			current.Title = strings.TrimSpace(m[1])
			continue
		}
		if m := videoIDRegex.FindStringSubmatch(line); m != nil {
			id := strings.Trim(m[1], "*_`()")
			if id != "" && !strings.HasPrefix(id, "(not") {
				current.VideoID = id
				haveID = true
			}
			continue
		}
		if m := projectFilesRegex.FindStringSubmatch(line); m != nil {
			current.ProjectDir = strings.Trim(m[1], "/")
			continue
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// FindRegistryFile probes the standard location and returns the path or "".
func FindRegistryFile(scriptDir string) string {
	if scriptDir == "" {
		home, _ := os.UserHomeDir()
		scriptDir = filepath.Join(home, ".openclaw", "workspace", "data")
	}
	candidate := filepath.Join(scriptDir, "content-registry.md")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// LookupRegistryByVideoID searches the registry for a video and returns the
// entry, or nil if not found.
func LookupRegistryByVideoID(registryPath, videoID string) (*RegistryEntry, error) {
	entries, err := ParseRegistryFile(registryPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.VideoID == videoID {
			return &e, nil
		}
	}
	return nil, nil
}

// ExtractFrameworkLines reads a script file and returns the first lines that
// match Signal / Belief-Shift / CTA markers. The matching is heuristic and
// looks for headings like `## Signal` or markdown task prefixes.
func ExtractFrameworkLines(scriptPath string) (signal, beliefShift, cta string, _ error) {
	f, err := os.Open(scriptPath)
	if err != nil {
		return "", "", "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)

	var section string
	for sc.Scan() {
		raw := sc.Text()
		trimmed := strings.TrimSpace(raw)
		lower := strings.ToLower(trimmed)
		// Section heading detection — covers ## Heading and **Heading** forms.
		switch {
		case strings.HasPrefix(lower, "## signal") || strings.HasPrefix(lower, "**signal"):
			section = "signal"
			continue
		case strings.HasPrefix(lower, "## belief") || strings.HasPrefix(lower, "**belief") || strings.HasPrefix(lower, "## belief-shift") || strings.HasPrefix(lower, "## belief shift"):
			section = "belief_shift"
			continue
		case strings.HasPrefix(lower, "## cta") || strings.HasPrefix(lower, "**cta") || strings.HasPrefix(lower, "## action") || strings.HasPrefix(lower, "## call to action"):
			section = "cta"
			continue
		case strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# "):
			// Any other top-level heading resets section
			section = ""
			continue
		}
		// First non-empty line in a section wins.
		if trimmed == "" {
			continue
		}
		switch section {
		case "signal":
			if signal == "" {
				signal = trimmed
			}
		case "belief_shift":
			if beliefShift == "" {
				beliefShift = trimmed
			}
		case "cta":
			if cta == "" {
				cta = trimmed
			}
		}
	}
	if err := sc.Err(); err != nil {
		return signal, beliefShift, cta, err
	}
	return signal, beliefShift, cta, nil
}

// ResolveScriptPath turns a RegistryEntry into an absolute script path by
// probing the project dir for common script filenames.
func ResolveScriptPath(scriptDir string, e *RegistryEntry) string {
	if e == nil {
		return ""
	}
	if e.ScriptPath != "" {
		return e.ScriptPath
	}
	if e.ProjectDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(scriptDir, e.ProjectDir, "script.md"),
		filepath.Join(scriptDir, e.ProjectDir, "vo-script.md"),
		filepath.Join(scriptDir, e.ProjectDir, "voiceover.md"),
		filepath.Join(scriptDir, e.ProjectDir, "README.md"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// fallback: any .md file in the project dir
	matches, _ := filepath.Glob(filepath.Join(scriptDir, e.ProjectDir, "*.md"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}
