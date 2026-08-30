package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUUIDArgumentValidation(t *testing.T) {
	validator := requireUUIDArgument("uuid")

	if err := validator(nil, []string{"550e8400-e29b-41d4-a716-446655440000"}); err != nil {
		t.Fatalf("valid UUID rejected: %v", err)
	}
	if err := validator(nil, nil); err != nil {
		t.Fatalf("missing UUID should reach command-specific handling: %v", err)
	}

	err := validator(nil, []string{"not-a-uuid"})
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "uuid must be a UUID") {
		t.Fatalf("invalid UUID error = %v, want usage error naming the UUID requirement", err)
	}
}

func TestMissingUUIDArgumentsKeepCommandSpecificResponses(t *testing.T) {
	t.Run("records get shows help", func(t *testing.T) {
		var flags rootFlags
		cmd := newRootCmd(&flags)
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{"records", "get"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("records get without UUID returned %v", err)
		}
		if !strings.Contains(output.String(), "Usage:") {
			t.Fatalf("records get output = %q, want help", output.String())
		}
	})

	t.Run("sheets JSON emits structured usage", func(t *testing.T) {
		var flags rootFlags
		cmd := newRootCmd(&flags)
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{"--json", "sheets"})

		err := cmd.Execute()
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("sheets without UUID error = %v, want usage error", err)
		}
		if !strings.Contains(output.String(), `"error": "uuid is required"`) {
			t.Fatalf("sheets output = %q, want structured UUID error", output.String())
		}
	})
}

func TestTailKnownResourcesIncludeDocumentedExamples(t *testing.T) {
	for _, resource := range []string{"messages", "events"} {
		if !isTailKnownResource(resource) {
			t.Errorf("documented tail resource %q is not accepted", resource)
		}
	}
	if isTailKnownResource("not-a-resource") {
		t.Error("unknown tail resource was accepted")
	}
}
