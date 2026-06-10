package cli

import "testing"

func TestWorkspaceDoctorVerdict(t *testing.T) {
	// no workspace configured, sentinel slug
	if got := workspaceDoctorVerdict("my-workspace", "https://api.plane.so/api/v1/workspaces/{slug}", nil); got == "" {
		t.Fatal("expected a nudge when no workspace configured")
	}
	// configured workspace, templated base -> OK (empty verdict)
	cfg := []string{"acme"}
	if got := workspaceDoctorVerdict("acme", "https://api.plane.so/api/v1/workspaces/{slug}", cfg); got != "" {
		t.Fatalf("configured+templated should be OK, got %q", got)
	}
	// literal-slug base -> migration warning regardless of slug
	if got := workspaceDoctorVerdict("acme", "https://h/api/v1/workspaces/acme", cfg); got == "" {
		t.Fatal("expected a migration warning for literal-slug base_url")
	}
}
