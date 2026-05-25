package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveRuntimePrefersExplicitBlenderBin(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"BLENDER_BIN": "/opt/blender/blender",
	}
	rt := resolveRuntime(func(key string) string { return env[key] }, func(name string) (string, error) {
		if name == "blender" || name == "blender.exe" {
			t.Fatalf("lookPath should not be called for blender when BLENDER_BIN is set")
		}
		return "", nil
	})

	if rt.BlenderPath != "/opt/blender/blender" {
		t.Fatalf("BlenderPath = %q, want explicit BLENDER_BIN", rt.BlenderPath)
	}
}

func TestBuildBlenderPythonArgsUsesBackgroundBeforeFile(t *testing.T) {
	t.Parallel()

	args := buildBlenderPythonArgs("scene.blend", "job.py", true, []string{"--one", "two"})
	want := []string{"--background", "scene.blend", "--python", "job.py", "--", "--one", "two"}

	if len(args) != len(want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q; full args %#v", i, args[i], want[i], args)
		}
	}
}

func TestBuildBlenderPythonArgsForWindowsBlenderTranslatesAbsolutePaths(t *testing.T) {
	t.Parallel()

	args := buildBlenderPythonArgsForRuntime(
		"/mnt/c/Program Files/Blender Foundation/Blender 5.1/blender.exe",
		"/tmp/scene.blend",
		"/tmp/job.py",
		true,
		nil,
	)

	if args[1] == "/tmp/scene.blend" || args[3] == "/tmp/job.py" {
		t.Fatalf("Windows Blender args were not translated: %#v", args)
	}
	if !strings.HasSuffix(args[3], `\job.py`) && !strings.HasSuffix(args[3], "/job.py") {
		t.Fatalf("script arg = %q, want translated path ending in job.py", args[3])
	}
}

func TestHandoffPlanFromAssetUsesStableDefaults(t *testing.T) {
	t.Parallel()

	plan := buildHandoffPlan("part.step", "", "Cycles", "turntable", 24)

	if plan.SourceAsset != "part.step" {
		t.Fatalf("SourceAsset = %q", plan.SourceAsset)
	}
	if plan.OutputBlend != filepath.Base("part.blend") {
		t.Fatalf("OutputBlend = %q, want part.blend", plan.OutputBlend)
	}
	if plan.Render.Engine != "Cycles" {
		t.Fatalf("Render.Engine = %q", plan.Render.Engine)
	}
	if plan.Animation.Frames != 24 {
		t.Fatalf("Animation.Frames = %d", plan.Animation.Frames)
	}
}

func TestValidationRequiresFlagValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "asset", err: requireNonEmptyFlag("asset", ""), want: "--asset is required"},
		{name: "script", err: requireNonEmptyFlag("script", "  "), want: "--script is required"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err == nil || !strings.Contains(tt.err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", tt.err, tt.want)
			}
		})
	}
}

func TestValidationAcceptsSupportedAssetExtensions(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"assembly.glb", "assembly.gltf", "part.obj", "part.fbx", "part.stl"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if err := validateAssetPath(path); err != nil {
				t.Fatalf("validateAssetPath(%q) returned %v", path, err)
			}
		})
	}
}

func TestValidationRejectsUnsupportedAssetExtensions(t *testing.T) {
	t.Parallel()

	err := validateAssetPath("part.step")
	if err == nil || !strings.Contains(err.Error(), "unsupported asset extension") {
		t.Fatalf("error = %v, want unsupported asset extension", err)
	}
}

func TestValidationRejectsBadRenderSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "image extension", err: validateImageOutputPath("render.blend"), want: "unsupported render output extension"},
		{name: "resolution shape", err: validateResolution("1024"), want: "resolution must be WIDTHxHEIGHT"},
		{name: "resolution width", err: validateResolution("0x720"), want: "resolution width must be positive"},
		{name: "frames", err: validatePositiveIntFlag("frames", 0), want: "--frames must be greater than 0"},
		{name: "mode", err: validateChoice("mode", "panorama", "still", "turntable"), want: "--mode must be one of: still, turntable"},
		{name: "transport", err: validateChoice("transport", "tcp", "stdio", "http"), want: "--transport must be one of: stdio, http"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err == nil || !strings.Contains(tt.err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", tt.err, tt.want)
			}
		})
	}
}

func TestJSONOutputAlwaysPresent(t *testing.T) {
	t.Parallel()

	// Commands with human-readable output mode must advertise --json.
	mustHaveJSON := []*cobra.Command{
		newResolveblenderCmd(),
		newDoctorCmd(),
	}
	for _, cmd := range mustHaveJSON {
		cmd := cmd
		t.Run(cmd.Use, func(t *testing.T) {
			t.Parallel()
			if cmd.Flags().Lookup("json") == nil {
				t.Fatalf("%s does not advertise --json", cmd.Use)
			}
		})
	}

	// Commands that always output JSON should not advertise a misleading --json flag.
	alwaysJSON := []*cobra.Command{
		newAssetimportCmd(),
		newHandoffplanCmd(),
		newMcpserveCmd(),
		newMcpstatusCmd(),
		newRenderstillCmd(),
		newRenderturntableCmd(),
		newSceneinspectCmd(),
	}
	for _, cmd := range alwaysJSON {
		cmd := cmd
		t.Run(cmd.Use, func(t *testing.T) {
			t.Parallel()
			if cmd.Flags().Lookup("json") != nil {
				t.Fatalf("%s should not advertise --json (always JSON)", cmd.Use)
			}
		})
	}
}
