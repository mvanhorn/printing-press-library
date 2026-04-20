package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

// Registry schema

type MCPBlock struct {
	Binary          string   `json:"binary"`
	Transport       string   `json:"transport"`
	ToolCount       int      `json:"tool_count"`
	PublicToolCount int      `json:"public_tool_count"`
	AuthType        string   `json:"auth_type"`
	EnvVars         []string `json:"env_vars"`
	MCPReady        string   `json:"mcp_ready"`
}

type RegistryEntry struct {
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	API         string    `json:"api"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	MCP         *MCPBlock `json:"mcp,omitempty"`
}

type Registry struct {
	SchemaVersion int             `json:"schema_version"`
	Entries       []RegistryEntry `json:"entries"`
}

// Manifest schema (subset we care about)

type Manifest struct {
	CLIName string `json:"cli_name"`
}

// Domain command parsed from --help

type DomainCommand struct {
	Name        string
	Description string
}

// Template context

type SkillContext struct {
	SkillName       string
	APIName         string
	Description     string
	EnrichedDesc    string
	CLIBinary       string
	InstallPath     string
	HasMCP          bool
	MCPBinary       string
	AuthType        string
	EnvVars         []string
	MCPReady        string
	ToolCount       int
	PublicToolCount int
	DomainCommands  []DomainCommand
	OpenClawMeta    string
}

// Framework commands to filter out of --help output
var frameworkCommands = map[string]bool{
	"api":        true,
	"auth":       true,
	"completion": true,
	"doctor":     true,
	"export":     true,
	"help":       true,
	"import":     true,
	"load":       true,
	"orphans":    true,
	"sql":        true,
	"stale":      true,
	"sync":       true,
	"version":    true,
	"workflow":   true,
}

func main() {
	// Read registry.json from current working directory (repo root)
	registryPath := "registry.json"
	registryData, err := os.ReadFile(registryPath)
	if err != nil {
		log.Fatalf("Error reading %s: %v\nRun this program from the repo root.", registryPath, err)
	}

	var registry Registry
	if err := json.Unmarshal(registryData, &registry); err != nil {
		log.Fatalf("Error parsing %s: %v", registryPath, err)
	}

	// Load templates
	templatePath := "tools/generate-skills/skill-template.md"
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("Error loading template %s: %v", templatePath, err)
	}
	commandTemplatePath := "tools/generate-skills/command-template.md"
	cmdTmpl, err := template.ParseFiles(commandTemplatePath)
	if err != nil {
		log.Fatalf("Error loading template %s: %v", commandTemplatePath, err)
	}

	// Snapshot existing pp-* skill dirs before generation
	beforeDirs := existingSkillDirs()

	var totalGenerated, enrichedCount, registryOnlyCount, skippedCount, upstreamCount int

	for _, entry := range registry.Entries {
		// Derive skill name: strip -pp-cli suffix, prepend pp-
		baseName := entry.Name
		baseName = strings.TrimSuffix(baseName, "-pp-cli")
		skillName := "pp-" + baseName

		// Resolve CLI binary name via layered precedence
		cliBinary := resolveCLIBinary(entry)

		// Resolve MCP/auth metadata from registry
		hasMCP := entry.MCP != nil
		var mcpBinary, authType, mcpReady string
		var envVars []string
		var toolCount, publicToolCount int

		if hasMCP {
			mcpBinary = entry.MCP.Binary
			authType = entry.MCP.AuthType
			envVars = entry.MCP.EnvVars
			mcpReady = entry.MCP.MCPReady
			toolCount = entry.MCP.ToolCount
			publicToolCount = entry.MCP.PublicToolCount
		}
		if authType == "" {
			authType = "none"
		}
		if envVars == nil {
			envVars = []string{}
		}

		// Try to get domain commands from --help
		domainCommands := parseDomainCommands(cliBinary)

		// Build enriched description
		enrichedDesc := buildEnrichedDescription(entry, domainCommands)

		isEnriched := domainCommands != nil

		ctx := SkillContext{
			SkillName:       skillName,
			APIName:         entry.API,
			Description:     entry.Description,
			EnrichedDesc:    enrichedDesc,
			CLIBinary:       cliBinary,
			InstallPath:     entry.Path,
			HasMCP:          hasMCP,
			MCPBinary:       mcpBinary,
			AuthType:        authType,
			EnvVars:         envVars,
			MCPReady:        mcpReady,
			ToolCount:       toolCount,
			PublicToolCount: publicToolCount,
			DomainCommands:  domainCommands,
		}
		ctx.OpenClawMeta = buildOpenClawMetadata(ctx)

		// Write skill file
		skillDir := filepath.Join("plugin", "skills", skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")

		// Upstream wins: if the printed CLI ships its own SKILL.md, copy it
		// verbatim. The generator has research context (novel features,
		// narrative, trigger phrases) that this tool can't reconstruct, so
		// upstream is strictly better than enriched or registry-only synthesis.
		copied, err := copyUpstreamSkill(entry.Path, skillDir, skillFile)
		if err != nil {
			log.Printf("Warning: could not copy upstream SKILL.md for %s: %v", entry.Name, err)
			continue
		}
		if copied {
			if err := writeCommandShim(cmdTmpl, ctx); err != nil {
				log.Printf("Warning: could not write command shim for %s: %v", entry.Name, err)
			}
			totalGenerated++
			upstreamCount++
			fmt.Printf("  %s -> %s (upstream)\n", entry.Name, skillFile)
			continue
		}

		// Downgrade protection: don't overwrite an enriched skill with a registry-only one.
		// This prevents CI (where CLIs aren't installed) from replacing locally-enriched skills.
		if !isEnriched {
			if existing, err := os.ReadFile(skillFile); err == nil {
				if strings.Contains(string(existing), "Key commands:") {
					fmt.Printf("  %s -> %s (skipped: existing skill is enriched, new would be registry-only)\n", entry.Name, skillFile)
					totalGenerated++
					skippedCount++
					continue
				}
			}
		}

		if err := os.MkdirAll(skillDir, 0755); err != nil {
			log.Printf("Warning: could not create directory %s: %v", skillDir, err)
			continue
		}

		f, err := os.Create(skillFile)
		if err != nil {
			log.Printf("Warning: could not create %s: %v", skillFile, err)
			continue
		}

		if err := tmpl.Execute(f, ctx); err != nil {
			f.Close()
			log.Printf("Warning: could not render template for %s: %v", skillName, err)
			continue
		}
		f.Close()

		if err := writeCommandShim(cmdTmpl, ctx); err != nil {
			log.Printf("Warning: could not write command shim for %s: %v", entry.Name, err)
		}

		totalGenerated++
		status := "registry-only"
		if isEnriched {
			status = "enriched"
			enrichedCount++
		} else {
			registryOnlyCount++
		}
		fmt.Printf("  %s -> %s (%s)\n", entry.Name, skillFile, status)
	}

	summary := fmt.Sprintf("\nGenerated %d skills (%d upstream, %d enriched, %d registry-only", totalGenerated, upstreamCount, enrichedCount, registryOnlyCount)
	if skippedCount > 0 {
		summary += fmt.Sprintf(", %d skipped to preserve enrichment", skippedCount)
	}
	summary += ")\n"
	fmt.Print(summary)

	// Bump plugin.json version if skill set changed
	afterDirs := existingSkillDirs()
	maybeUpdatePluginVersion(beforeDirs, afterDirs)
}

// writeCommandShim writes plugin/commands/<skillName>.md as a thin shim that
// invokes the corresponding skill. Claude Code discovers slash commands from
// this directory, so every skill we generate also gets a slash command. The
// shim reuses the skill's description + argument hint so /<skillName> ? shows
// the same help text as the skill.
func writeCommandShim(cmdTmpl *template.Template, ctx SkillContext) error {
	cmdDir := filepath.Join("plugin", "commands")
	if err := os.MkdirAll(cmdDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", cmdDir, err)
	}
	cmdFile := filepath.Join(cmdDir, ctx.SkillName+".md")
	f, err := os.Create(cmdFile)
	if err != nil {
		return fmt.Errorf("create %s: %w", cmdFile, err)
	}
	defer f.Close()
	if err := cmdTmpl.Execute(f, ctx); err != nil {
		return fmt.Errorf("render template for %s: %w", ctx.SkillName, err)
	}
	return nil
}

// copyUpstreamSkill copies <entryPath>/SKILL.md to skillFile if it exists and
// is non-empty. Returns (true, nil) on successful copy, (false, nil) when
// upstream is missing or empty (so the caller can fall through to synthesis),
// (false, err) on other filesystem errors.
func copyUpstreamSkill(entryPath, skillDir, skillFile string) (bool, error) {
	upstreamPath := filepath.Join(entryPath, "SKILL.md")
	data, err := os.ReadFile(upstreamPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", upstreamPath, err)
	}
	// Empty or whitespace-only upstream almost always signals a generator bug
	// (failed write mid-pipeline). Prefer thin synthesis over shipping a blank
	// SKILL.md. Log loudly so the upstream regression is visible.
	if len(strings.TrimSpace(string(data))) == 0 {
		log.Printf("Warning: upstream SKILL.md at %s is empty; falling through to synthesis", upstreamPath)
		return false, nil
	}
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", skillDir, err)
	}
	augmented := injectStaleBuildFallback(data)
	if err := os.WriteFile(skillFile, augmented, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", skillFile, err)
	}
	return true, nil
}

// injectStaleBuildFallback adds a short "if @latest is stale, install
// from @main" code block right after every `go install ...@latest` line
// that targets a printing-press-library cmd. Upstream (hand-authored)
// SKILL.md files bypass the template, so they need this augmentation at
// copy time to match what template-generated skills already ship.
//
// Idempotent: if the file already contains `@main` (either from a
// prior run of this generator or hand-added guidance), we make no
// changes and return the original bytes unchanged.
func injectStaleBuildFallback(data []byte) []byte {
	if bytes.Contains(data, []byte("@main")) {
		return data
	}
	// Pattern: a line ending in `cmd/<binary>@latest` that is NOT inside
	// the metadata JSON (metadata is always on line 6 of the frontmatter
	// and uses a different quoting shape — we filter by the leading whitespace
	// plus the `go install` prefix, which metadata does not have).
	goInstallRE := regexp.MustCompile(
		`(?m)^(\s*)(go install github\.com/mvanhorn/printing-press-library/library/[^\s@]+@latest)\s*$`)
	fallbackBlock := func(indent, installCmd string) string {
		mainCmd := strings.Replace(installCmd, "@latest", "@main", 1)
		return fmt.Sprintf(
			"%s%s\n%s\n%s# If `@latest` installs a stale build (Go module proxy cache lag), install from main:\n%sGOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \\\n%s  %s",
			indent, installCmd, indent, indent, indent, indent, mainCmd)
	}
	return goInstallRE.ReplaceAllFunc(data, func(match []byte) []byte {
		sub := goInstallRE.FindSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		indent := string(sub[1])
		installCmd := string(sub[2])
		return []byte(fallbackBlock(indent, installCmd))
	})
}

// resolveCLIBinary resolves the CLI binary name using layered precedence:
// 1. Read .printing-press.json manifest -> validate cmd/<cli_name>/ dir exists
// 2. If dir missing, try bare registry name
// 3. Registry heuristic (append -pp-cli) as last resort
func resolveCLIBinary(entry RegistryEntry) string {
	manifestPath := filepath.Join(entry.Path, ".printing-press.json")

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		// No manifest — use registry heuristic
		log.Printf("Warning: no manifest at %s, using heuristic for %s", manifestPath, entry.Name)
		return registryHeuristic(entry.Name)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		log.Printf("Warning: could not parse manifest at %s: %v", manifestPath, err)
		return registryHeuristic(entry.Name)
	}

	if manifest.CLIName == "" {
		log.Printf("Warning: manifest at %s has no cli_name, using heuristic", manifestPath)
		return registryHeuristic(entry.Name)
	}

	// Validate that cmd/<cli_name>/ directory exists in source tree
	cmdDir := filepath.Join(entry.Path, "cmd", manifest.CLIName)
	if info, err := os.Stat(cmdDir); err == nil && info.IsDir() {
		return manifest.CLIName
	}

	// cmd/<cli_name>/ doesn't exist — try bare registry name
	bareName := strings.TrimSuffix(entry.Name, "-pp-cli")
	bareCmdDir := filepath.Join(entry.Path, "cmd", bareName)
	if info, err := os.Stat(bareCmdDir); err == nil && info.IsDir() {
		log.Printf("Info: manifest cli_name %q has no cmd/ dir, using bare name %q for %s", manifest.CLIName, bareName, entry.Name)
		return bareName
	}

	// Fall back to manifest cli_name even though dir doesn't exist
	// (maybe the directory structure doesn't match expectations)
	log.Printf("Warning: no cmd/ dir found for %s (tried %q and %q), using manifest cli_name %q", entry.Name, manifest.CLIName, bareName, manifest.CLIName)
	return manifest.CLIName
}

// registryHeuristic derives the CLI binary name from the registry name.
// If the name already ends in -pp-cli, use as-is; otherwise append -pp-cli.
func registryHeuristic(name string) string {
	if strings.HasSuffix(name, "-pp-cli") {
		return name
	}
	return name + "-pp-cli"
}

// parseDomainCommands runs <binary> --help and parses the "Available Commands:" section.
// Returns nil if the binary is not found or --help fails.
func parseDomainCommands(binary string) []DomainCommand {
	cmd := exec.Command(binary, "--help")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Info: could not run %s --help (binary may not be installed): %v", binary, err)
		return nil
	}

	return parseHelpOutput(string(output))
}

// parseHelpOutput extracts domain commands from --help output.
func parseHelpOutput(output string) []DomainCommand {
	lines := strings.Split(output, "\n")
	inAvailableCommands := false
	var commands []DomainCommand

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect the "Available Commands:" section
		if trimmed == "Available Commands:" {
			inAvailableCommands = true
			continue
		}

		// End of section: blank line or new section header (non-indented, ends with ":")
		if inAvailableCommands {
			if trimmed == "" {
				// Blank line could mean end of section or separator within
				// Keep going — Cobra sometimes has blank lines in the section
				continue
			}
			// If line doesn't start with spaces, we've left the section
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				break
			}

			// Parse command line: "  <command>  <description>"
			parts := strings.Fields(trimmed)
			if len(parts) < 1 {
				continue
			}

			cmdName := parts[0]

			// Filter out framework commands
			if frameworkCommands[cmdName] {
				continue
			}

			// Reconstruct description from remaining fields
			desc := ""
			if len(parts) > 1 {
				desc = strings.Join(parts[1:], " ")
			}

			commands = append(commands, DomainCommand{
				Name:        cmdName,
				Description: desc,
			})
		}
	}

	if len(commands) == 0 {
		return nil
	}
	return commands
}

// buildEnrichedDescription creates a rich, composition-friendly description
// for the skill frontmatter.
func buildEnrichedDescription(entry RegistryEntry, domainCommands []DomainCommand) string {
	// Start with the Printing Press CLI identification and registry description
	desc := fmt.Sprintf("Printing Press CLI for %s. %s", entry.API, entry.Description)

	// Add domain command keywords if available
	if len(domainCommands) > 0 {
		var cmdNames []string
		for _, cmd := range domainCommands {
			cmdNames = append(cmdNames, cmd.Name)
		}
		desc += " Capabilities include: " + strings.Join(cmdNames, ", ") + "."
	}

	// Add trigger phrases for discoverability
	bareName := strings.TrimSuffix(entry.Name, "-pp-cli")
	triggerPhrases := fmt.Sprintf(
		" Trigger phrases: 'install %s', 'use %s', 'run %s', '%s commands', 'setup %s'.",
		bareName, bareName, bareName, entry.API, bareName,
	)
	desc += triggerPhrases

	// Escape any double quotes in the description (for YAML frontmatter)
	desc = strings.ReplaceAll(desc, `"`, `\"`)

	return desc
}

// existingSkillDirs returns the sorted set of pp-* directory names under skills/.
func existingSkillDirs() []string {
	entries, err := os.ReadDir(filepath.Join("plugin", "skills"))
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "pp-") {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// bumpPatchVersion increments the patch component of a semver string.
// "1.1.0" -> "1.1.1", "1.2.3" -> "1.2.4"
func bumpPatchVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver: %s", version)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version: %s", parts[2])
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, "."), nil
}

// maybeUpdatePluginVersion bumps the plugin.json patch version if the set of
// pp-* skill directories changed. Uses string replacement to preserve field order.
func maybeUpdatePluginVersion(beforeDirs, afterDirs []string) {
	if slicesEqual(beforeDirs, afterDirs) {
		return
	}

	pluginPath := filepath.Join("plugin", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		log.Printf("Warning: could not read %s for version bump: %v", pluginPath, err)
		return
	}

	// Extract current version from JSON
	var parsed struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		log.Printf("Warning: could not parse %s: %v", pluginPath, err)
		return
	}

	newVersion, err := bumpPatchVersion(parsed.Version)
	if err != nil {
		log.Printf("Warning: could not bump version %q: %v", parsed.Version, err)
		return
	}

	// Replace version in-place to preserve field order and formatting
	content := string(data)
	old := fmt.Sprintf(`"version": "%s"`, parsed.Version)
	updated := fmt.Sprintf(`"version": "%s"`, newVersion)
	content = strings.Replace(content, old, updated, 1)

	if err := os.WriteFile(pluginPath, []byte(content), 0644); err != nil {
		log.Printf("Warning: could not write %s: %v", pluginPath, err)
		return
	}

	fmt.Printf("Bumped plugin version: %s -> %s\n", parsed.Version, newVersion)
}

// buildOpenClawMetadata constructs the single-line JSON for the metadata frontmatter field.
// This gives OpenClaw dependency gating, auto-install, and API key prompting.
func buildOpenClawMetadata(ctx SkillContext) string {
	type installBlock struct {
		ID      string   `json:"id"`
		Kind    string   `json:"kind"`
		Command string   `json:"command"`
		Bins    []string `json:"bins"`
		Label   string   `json:"label"`
	}

	type requires struct {
		Bins []string `json:"bins"`
		Env  []string `json:"env,omitempty"`
	}

	type openclawBlock struct {
		Requires   requires       `json:"requires"`
		PrimaryEnv string         `json:"primaryEnv,omitempty"`
		Install    []installBlock `json:"install"`
	}

	type metadataWrapper struct {
		OpenClaw openclawBlock `json:"openclaw"`
	}

	goInstallCmd := fmt.Sprintf("go install github.com/mvanhorn/printing-press-library/%s/cmd/%s@latest", ctx.InstallPath, ctx.CLIBinary)

	req := requires{
		Bins: []string{ctx.CLIBinary},
	}
	if len(ctx.EnvVars) > 0 && ctx.AuthType == "api_key" {
		req.Env = ctx.EnvVars
	}

	oc := openclawBlock{
		Requires: req,
		Install: []installBlock{
			{
				ID:      "go",
				Kind:    "shell",
				Command: goInstallCmd,
				Bins:    []string{ctx.CLIBinary},
				Label:   "Install via go install",
			},
		},
	}

	if len(ctx.EnvVars) > 0 && ctx.AuthType == "api_key" {
		oc.PrimaryEnv = ctx.EnvVars[0]
	}

	wrapper := metadataWrapper{OpenClaw: oc}
	data, err := json.Marshal(wrapper)
	if err != nil {
		log.Printf("Warning: could not marshal OpenClaw metadata for %s: %v", ctx.SkillName, err)
		return ""
	}
	return string(data)
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
