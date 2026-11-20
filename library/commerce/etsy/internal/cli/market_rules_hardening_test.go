// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesCompilationCaps(t *testing.T) {
	for _, row := range []struct {
		name        string
		count, size int
		bad         bool
	}{
		{"too_many", 257, 1, true}, {"too_long", 1, 513, true}, {"boundary", 256, 512, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			r := &mktRawTarget{name: "linen apron"}
			for i := 0; i < row.count; i++ {
				if i%2 == 0 {
					r.include = append(r.include, strings.Repeat("a", row.size))
				} else {
					r.exclude = append(r.exclude, strings.Repeat("a", row.size))
				}
			}
			_, err := mktCompileTargets([]*mktRawTarget{r})
			if (err != nil) != row.bad {
				t.Fatal(err)
			}
			if err != nil && len(err.Error()) > 250 {
				t.Fatal("unbounded diagnostic")
			}
		})
	}
	t.Run("cap_accumulates_across_targets", func(t *testing.T) {
		targets := []*mktRawTarget{{name: "linen apron"}, {name: "ceramic mug"}}
		for _, target := range targets {
			for i := 0; i < 129; i++ {
				target.include = append(target.include, "linen apron")
			}
		}
		if _, err := mktCompileTargets(targets); err == nil || !strings.Contains(err.Error(), "256 patterns") {
			t.Fatalf("expected aggregate pattern cap: %v", err)
		}
	})
}

func TestRulesCapsUsageExit(t *testing.T) {
	mktTestEnv(t)
	src, _ := json.Marshal(map[string]any{"targets": []map[string]any{{"name": "linen apron", "include": []string{strings.Repeat("a", 513)}}}})
	rulesFile := filepath.Join(t.TempDir(), "rules.json")
	if err := os.WriteFile(rulesFile, src, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"market", "gaps", "--rules-json", string(src)},
		{"market", "gaps", "--rules", rulesFile},
		{"market", "price", "--group-by", "class", "--rules", rulesFile},
		{"market", "price", "--group-by", "class", "--rules-json", string(src)},
	} {
		_, err := mktRunCLI(t, args...)
		if ExitCode(err) != 2 || !strings.Contains(err.Error(), "512 bytes") || strings.Count(err.Error(), "rules:")+strings.Count(err.Error(), "--rules-json:") != 1 {
			t.Fatal(err)
		}
	}
}

func TestRulesParseUsageSinglePrefix(t *testing.T) {
	mktTestEnv(t)
	for _, row := range []struct{ name, src, want string }{
		{"json", "{", "rules: JSON:"},
		{"top_level", "linen apron", "rules: line 1:"},
		{"target_entry", "targets:\n name: linen apron", "rules: line 2:"},
		{"key_value", "targets:\n - name: linen apron\n missing", "rules: line 3:"},
		{"unknown_key", "targets:\n - name: linen apron\n unknown: linen apron", "rules: line 3:"},
	} {
		t.Run(row.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "rules")
			if err := os.WriteFile(path, []byte(row.src), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"market", "gaps", "--rules", path}, {"market", "price", "--group-by", "class", "--rules", path}} {
				_, err := mktRunCLI(t, args...)
				if ExitCode(err) != 2 || !strings.HasPrefix(err.Error(), row.want) || strings.Count(err.Error(), "rules:") != 1 {
					t.Fatalf("expected single prefix %q: %v", row.want, err)
				}
			}
		})
	}
}
