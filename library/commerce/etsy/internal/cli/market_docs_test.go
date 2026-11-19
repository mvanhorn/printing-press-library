package cli

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestMarketDocsWhichDrift(t *testing.T) {
	data, err := os.ReadFile("../../.manuscripts/20261116-133635-274efd3c/research.json")
	if err != nil {
		t.Fatalf("WHICH_DRIFT: %v", err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("WHICH_DRIFT: %v", err)
	}
	indexed := map[string]string{}
	for _, entry := range whichIndex {
		indexed[entry.Command] = entry.WhyItMatters
	}
	research := map[string][]string{}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			command, hasCommand := node["command"].(string)
			why, hasWhy := node["why_it_matters"].(string)
			if hasCommand && hasWhy {
				research[command] = append(research[command], why)
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(document)
	// The generator adds non-market entries (e.g. production-partners) that
	// research.json never carried, so which -> research is enforced for market commands only.
	for command := range indexed {
		if _, exists := research[command]; !exists && strings.HasPrefix(command, "market ") {
			t.Errorf("WHICH_COMMAND_MISSING_FROM_RESEARCH: %s", command)
		}
	}
	for command := range research {
		if _, exists := indexed[command]; !exists {
			t.Errorf("RESEARCH_COMMAND_MISSING_FROM_WHICH: %s", command)
		}
	}
	for _, filename := range []string{"README.md", "SKILL.md"} {
		doc, err := os.ReadFile("../../" + filename)
		if err != nil {
			t.Fatalf("DOCS_READ_FAILED %s: %v", filename, err)
		}
		if !strings.Contains(string(doc), indexed["market snapshot"]) {
			t.Errorf("DOCS_WHY_HEAD_MISSING %s", filename)
		}
		for _, needle := range []string{"Market commands emit", "even under --agent", "Don't mix anchored", "not_fetched", "shop_name is nullable"} {
			if !strings.Contains(string(doc), needle) {
				t.Errorf("DOCS_GUIDANCE_MISSING %s: %s", filename, needle)
			}
		}
	}
	commands := make([]string, 0, len(research))
	for command := range research {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	sanitize := regexp.MustCompile(`[^A-Za-z0-9_]`)
	for _, command := range commands {
		t.Run(sanitize.ReplaceAllString(command, "_"), func(t *testing.T) {
			if _, exists := indexed[command]; !exists {
				return
			}
			for _, why := range research[command] {
				if indexed[command] != why {
					t.Errorf("WHICH_DRIFT %s: index %q research %q", command, indexed[command], why)
				}
			}
			if command == "market snapshot" && strings.Contains(indexed[command], "Market commands emit") {
				t.Error("envelope guidance belongs in docs")
			}
		})
	}
	parent := newNovelMarketCmd(&rootFlags{})
	for _, needle := range []string{"Market commands emit", "even under --agent", "--select", "{results, notes}"} {
		if !strings.Contains(parent.Long, needle) {
			t.Errorf("MCP_PARENT_GUIDANCE_MISSING: %s", needle)
		}
	}
	if len(commands) < 10 {
		t.Fatalf("WHICH_DRIFT: compared only %d commands", len(commands))
	}
	if len(research["market snapshot"]) == 0 {
		t.Fatal("WHICH_DRIFT: market snapshot not compared")
	}
}
