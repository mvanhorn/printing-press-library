package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const skillOutputDir = "cli-skills"

type PrintManifest struct {
	APIName string `json:"api_name"`
}

type LibrarySkill struct {
	Name string
	Path string
}

func main() {
	librarySkills, err := discoverLibrarySkills("library")
	if err != nil {
		log.Fatal(err)
	}

	// Track every skill name the library asks for so we can prune
	// pp-<oldslug>/ directories left behind by renames or removals. Filled
	// at the top of the loop (before any error paths) so a transient write
	// failure for an entry doesn't make us delete its existing skill.
	expectedSkills := make(map[string]struct{}, len(librarySkills))

	var (
		copiedCount int
		missing     []string
		writeErrors []string
	)

	for _, entry := range librarySkills {
		skillName := "pp-" + entry.Name
		expectedSkills[skillName] = struct{}{}

		skillDir := filepath.Join(skillOutputDir, skillName)
		skillFile := filepath.Join(skillDir, "SKILL.md")

		copied, err := copyUpstreamSkill(entry.Path, skillDir, skillFile)
		if err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}
		if !copied {
			missing = append(missing, fmt.Sprintf("%s (expected %s/SKILL.md)", entry.Name, entry.Path))
			continue
		}
		copiedCount++
		fmt.Printf("  %s -> %s\n", entry.Name, skillFile)
	}

	prunedCount := pruneOrphanSkills(skillOutputDir, expectedSkills)

	fmt.Printf("\nMirrored %d skill(s) from library/ to %s/\n", copiedCount, skillOutputDir)
	if prunedCount > 0 {
		fmt.Printf("Pruned %d orphan skill dir(s) with no library manifest.\n", prunedCount)
	}

	if len(writeErrors) > 0 {
		log.Printf("Write errors:\n  %s", strings.Join(writeErrors, "\n  "))
	}

	if len(missing) > 0 {
		log.Fatalf(
			"Missing or empty library SKILL.md for %d entr(y/ies):\n  %s\n"+
				"Every CLI must ship a library SKILL.md (see AGENTS.md \"SKILL.md coverage\"). "+
				"Generator behavior belongs in cli-printing-press; this catalog only mirrors it.",
			len(missing),
			strings.Join(missing, "\n  "),
		)
	}

	if len(writeErrors) > 0 {
		os.Exit(1)
	}
}

func discoverLibrarySkills(libraryRoot string) ([]LibrarySkill, error) {
	manifestPaths, err := filepath.Glob(filepath.Join(libraryRoot, "*", "*", ".printing-press.json"))
	if err != nil {
		return nil, fmt.Errorf("discover library manifests: %w", err)
	}
	if len(manifestPaths) == 0 {
		return nil, fmt.Errorf("no .printing-press.json files found under %s/*/*; run this program from the repo root", libraryRoot)
	}

	skills := make([]LibrarySkill, 0, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", manifestPath, err)
		}
		var manifest PrintManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestPath, err)
		}
		apiName := strings.TrimSpace(manifest.APIName)
		if apiName == "" {
			return nil, fmt.Errorf("%s is missing api_name", manifestPath)
		}
		skills = append(skills, LibrarySkill{
			Name: apiName,
			Path: filepath.ToSlash(filepath.Dir(manifestPath)),
		})
	}

	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Name == skills[j].Name {
			return skills[i].Path < skills[j].Path
		}
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

// copyUpstreamSkill copies <entryPath>/SKILL.md to skillFile if it exists and
// is non-empty. Returns (true, nil) on successful copy, (false, nil) when
// upstream is missing or empty/whitespace-only (caller reports it as missing),
// (false, err) on other filesystem errors.
//
// An empty/whitespace-only upstream almost always signals a generator bug
// (failed write mid-pipeline); treat it as missing so the caller fails loudly
// rather than mirroring a blank SKILL.md.
func copyUpstreamSkill(entryPath, skillDir, skillFile string) (bool, error) {
	upstreamPath := filepath.Join(entryPath, "SKILL.md")
	data, err := os.ReadFile(upstreamPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", upstreamPath, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return false, nil
	}
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", skillDir, err)
	}
	if err := os.WriteFile(skillFile, data, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", skillFile, err)
	}
	return true, nil
}

// pruneOrphanSkills removes cli-skills/pp-<slug>/ directories whose pp-<slug>
// is not in the expected set (i.e., the library has no corresponding manifest).
// Without this, renaming a CLI's slug leaves the old mirror behind: the
// library drops the old manifest, the main loop above only writes the
// new entry, and `git add cli-skills/` in CI sees no working-tree change for
// the orphan dir. See issue #250 for the flightgoat -> flight-goat case.
//
// Scoped to pp-* directories only so unrelated content under dir is preserved
// if anyone adds it later. dir is parameterized for testability.
func pruneOrphanSkills(dir string, expected map[string]struct{}) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		log.Printf("Warning: could not read %s for orphan prune: %v", dir, err)
		return 0
	}
	var removed int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "pp-") {
			continue
		}
		if _, ok := expected[name]; ok {
			continue
		}
		target := filepath.Join(dir, name)
		if err := os.RemoveAll(target); err != nil {
			log.Printf("Warning: could not remove orphan %s: %v", target, err)
			continue
		}
		fmt.Printf("  removed orphan %s (no library manifest)\n", target)
		removed++
	}
	return removed
}
