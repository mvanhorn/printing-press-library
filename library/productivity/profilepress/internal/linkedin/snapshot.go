package linkedin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"profilepress-pp-cli/internal/profile"
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
	sections := fx.Sections
	for k, v := range fx.Map {
		sections = append(sections, profile.Section{Name: k, RawText: v, Normalized: Normalize(v), Source: "fixture"})
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
