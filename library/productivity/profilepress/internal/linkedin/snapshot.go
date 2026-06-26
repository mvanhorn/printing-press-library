package linkedin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/profilepress/internal/profile"
)

type fixtureSnapshot struct {
	Source   string            `json:"source"`
	Sections []profile.Section `json:"sections"`
	Map      map[string]string `json:"section_map"`
}

func LoadSnapshotFixture(path string) ([]profile.Section, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var fx fixtureSnapshot
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, "", err
	}
	sections := make([]profile.Section, 0, len(fx.Sections)+len(fx.Map))
	seen := map[string]struct{}{}
	for _, section := range fx.Sections {
		key := canonicalSectionName(section.Name)
		if key == "" {
			return nil, "", fmt.Errorf("fixture contains section with empty name")
		}
		if _, ok := seen[key]; ok {
			return nil, "", fmt.Errorf("fixture contains duplicate section %q", key)
		}
		seen[key] = struct{}{}
		section.Name = key
		sections = append(sections, section)
	}
	for k, v := range fx.Map {
		key := canonicalSectionName(k)
		if key == "" {
			return nil, "", fmt.Errorf("fixture contains section_map entry with empty name")
		}
		if _, ok := seen[key]; ok {
			return nil, "", fmt.Errorf("fixture contains duplicate section %q in sections and section_map", key)
		}
		seen[key] = struct{}{}
		sections = append(sections, profile.Section{Name: key, RawText: v, Normalized: Normalize(v), Source: "fixture"})
	}
	if len(sections) == 0 {
		return nil, "", fmt.Errorf("fixture contains no sections")
	}
	for i := range sections {
		if sections[i].Normalized == "" {
			sections[i].Normalized = Normalize(sections[i].RawText)
		}
		if sections[i].Source == "" {
			sections[i].Source = "fixture"
		}
	}
	if fx.Source == "" {
		fx.Source = path
	}
	return sections, fx.Source, nil
}

func Normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func canonicalSectionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
