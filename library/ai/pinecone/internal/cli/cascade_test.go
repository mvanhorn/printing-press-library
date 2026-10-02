// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestSelectCascadeIndexesKeepsReachableCompatibleIndexes(t *testing.T) {
	names := []string{"stale", "cosine-a", "euclidean", "cosine-b"}
	valid, base, failures, err := selectCascadeIndexes(names, func(name string) (pineconeIndexShape, error) {
		if name == "stale" {
			return pineconeIndexShape{}, fmt.Errorf("index unavailable")
		}
		if name == "euclidean" {
			return pineconeIndexShape{Dimension: 1024, Metric: "euclidean"}, nil
		}
		return pineconeIndexShape{Dimension: 1024, Metric: "cosine"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(valid, ",") != "cosine-a,cosine-b" || base.Metric != "cosine" {
		t.Fatalf("valid indexes=%v base=%#v", valid, base)
	}
	if len(failures) != 2 || failures[0].Index != "stale" || failures[1].Index != "euclidean" {
		t.Fatalf("failures=%#v, want stale and incompatible", failures)
	}
}

func TestSelectCascadeIndexesChoosesUniqueLargestGroupRegardlessOfOrder(t *testing.T) {
	shapes := map[string]pineconeIndexShape{
		"different": {Dimension: 512, Metric: "dotproduct"},
		"first":     {Dimension: 1024, Metric: "cosine"},
		"second":    {Dimension: 1024, Metric: "COSINE"},
	}
	for _, names := range [][]string{{"different", "first", "second"}, {"second", "different", "first"}} {
		valid, shape, failures, err := selectCascadeIndexes(names, func(name string) (pineconeIndexShape, error) { return shapes[name], nil })
		if err != nil || shape.Dimension != 1024 || len(valid) != 2 || len(failures) != 1 || failures[0].Index != "different" {
			t.Fatalf("order=%v selected=%v shape=%#v failures=%#v error=%v", names, valid, shape, failures, err)
		}
	}
}

func TestSelectCascadeIndexesRejectsTiedIncompatibleGroups(t *testing.T) {
	_, _, _, err := selectCascadeIndexes([]string{"cosine", "euclidean"}, func(name string) (pineconeIndexShape, error) {
		return pineconeIndexShape{Dimension: 1024, Metric: name}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "no unique largest") {
		t.Fatalf("tied group error = %v", err)
	}
}

func TestSelectCascadeIndexesDeduplicatesBeforeChoosingGroup(t *testing.T) {
	names := []string{"cosine-a", "euclidean", "euclidean", "cosine-b"}
	calls := map[string]int{}
	valid, _, failures, err := selectCascadeIndexes(names, func(name string) (pineconeIndexShape, error) {
		calls[name]++
		if name == "euclidean" {
			return pineconeIndexShape{Dimension: 1024, Metric: "euclidean"}, nil
		}
		return pineconeIndexShape{Dimension: 1024, Metric: "cosine"}, nil
	})
	if err != nil || strings.Join(valid, ",") != "cosine-a,cosine-b" || len(failures) != 1 || failures[0].Index != "euclidean" || calls["euclidean"] != 1 {
		t.Fatalf("duplicates distorted selection: valid=%v failures=%#v calls=%v error=%v", valid, failures, calls, err)
	}
}

// TestNovelCascadeHelpWires smoke-tests that the cascade command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCascadeHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"cascade", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cascade --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "cascade"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cascade --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestEnsureCascadeCompatibleRejectsMixedMetrics(t *testing.T) {
	base := pineconeIndexShape{Dimension: 1024, Metric: "cosine"}
	candidate := pineconeIndexShape{Dimension: 1024, Metric: "euclidean"}
	if err := ensureCascadeCompatible("cosine-index", base, "euclidean-index", candidate); err == nil || !strings.Contains(err.Error(), "euclidean") {
		t.Fatalf("mixed metric error = %v, want euclidean incompatibility", err)
	}
}

func TestEnsureCascadeCompatibleAcceptsMatchingShape(t *testing.T) {
	base := pineconeIndexShape{Dimension: 1024, Metric: "cosine"}
	candidate := pineconeIndexShape{Dimension: 1024, Metric: "COSINE"}
	if err := ensureCascadeCompatible("one", base, "two", candidate); err != nil {
		t.Fatalf("matching shape rejected: %v", err)
	}
}
