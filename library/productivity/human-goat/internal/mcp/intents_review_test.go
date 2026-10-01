// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"reflect"
	"testing"
)

func TestDispatchRemoteErrandUsesCallerPrompt(t *testing.T) {
	args, err := dispatchRemoteErrandArgs(map[string]any{
		"id":  "+12065550100",
		"ask": "- Ask whether curbside pickup is available.",
	})
	if err != nil {
		t.Fatalf("dispatchRemoteErrandArgs() error: %v", err)
	}
	want := []string{"call", "+12065550100", "--ask=- Ask whether curbside pickup is available."}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestDispatchRemoteErrandRequiresPrompt(t *testing.T) {
	if _, err := dispatchRemoteErrandArgs(map[string]any{"id": "+12065550100"}); err == nil {
		t.Fatal("missing ask should fail")
	}
}
