// Package evals implements the uk-train-goat agent eval grader.
//
// v0.1 shipping scope: structural eval. Each fixture declares an
// expected tool name; the grader checks that the tool resolves to a
// real registered command in the CLI. Pass rate is reported as a
// percentage and the eval CLI command exits non-zero below the
// configured threshold.
//
// The full agent-in-the-loop run (configured LLM picks a tool; grader
// compares to expected) lands in v0.2 behind EVAL_AGENT_MODEL.
package evals

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Fixture is one row in the dataset YAML (we use TOML at parse time
// for zero new dependencies, but the file is structurally compatible
// with the YAML schema described in the design spec).
type Fixture struct {
	ID       string         `toml:"id" json:"id"`
	Prompt   string         `toml:"prompt" json:"prompt"`
	Expected ExpectedCall   `toml:"expected" json:"expected"`
	Rubric   []string       `toml:"rubric" json:"rubric"`
}

// ExpectedCall captures the tool name and partial-args the agent must
// produce for a given prompt.
type ExpectedCall struct {
	Tool       string         `toml:"tool" json:"tool"`
	ArgsSubset map[string]any `toml:"args_subset" json:"args_subset"`
	ExitCode   int            `toml:"exit_code" json:"exit_code"`
}

// Dataset is the top-level fixture list.
type Dataset struct {
	Fixtures []Fixture `toml:"fixtures"`
}

// LoadFixtures reads a fixture file from disk. The file is TOML for now
// (single dependency we already use); v0.2 may switch to YAML once
// gopkg.in/yaml.v3 is added to go.mod.
func LoadFixtures(path string) ([]Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ds Dataset
	if err := toml.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parsing fixtures (%s): %w", path, err)
	}
	return ds.Fixtures, nil
}

// FixtureResult captures the outcome of grading a single fixture.
type FixtureResult struct {
	ID     string `json:"id"`
	Tool   string `json:"tool"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// Results aggregates per-fixture outcomes for the eval CLI.
type Results struct {
	Passed     int             `json:"passed"`
	Failed     int             `json:"failed"`
	PerFixture []FixtureResult `json:"per_fixture"`
}

// PassRate returns the pass rate as a percentage (0-100).
func (r *Results) PassRate() float64 {
	total := r.Passed + r.Failed
	if total == 0 {
		return 0
	}
	return float64(r.Passed) / float64(total) * 100.0
}

// GradeStructural runs the v0.1 structural pass: for every fixture,
// confirm the expected tool exists in the registered command set.
//
// This catches a real class of bug — an absorb-LLM authored fixture
// that names a command we deleted, or a fixture that names a flag
// belonging to a different command. It does NOT (yet) invoke an LLM
// to pick a tool; that's the v0.2 deliverable.
func GradeStructural(fixtures []Fixture, registered map[string]bool) Results {
	res := Results{}
	for _, f := range fixtures {
		fr := FixtureResult{ID: f.ID, Tool: f.Expected.Tool}
		switch {
		case f.Expected.Tool == "":
			fr.Reason = "expected.tool not set"
		case !registered[f.Expected.Tool]:
			fr.Reason = fmt.Sprintf("expected tool %q not registered in CLI", f.Expected.Tool)
		default:
			fr.Passed = true
		}
		if fr.Passed {
			res.Passed++
		} else {
			res.Failed++
		}
		res.PerFixture = append(res.PerFixture, fr)
	}
	return res
}
