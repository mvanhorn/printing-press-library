// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestSkillArgsAlwaysIncludeAgentPoolSkill(t *testing.T) {
	got := skillArgs([]string{"--json"})
	want := []string{"skills", "get", "agentpool", "--json"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skillArgs() = %v, want %v", got, want)
	}
}
