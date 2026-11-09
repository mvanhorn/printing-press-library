// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlatformClientMutationDryRunNoEffects(t *testing.T) {
	home := t.TempDir()
	for name, value := range map[string]string{
		"HOME": home, "USERPROFILE": home,
		"XDG_CONFIG_HOME": filepath.Join(home, "config"), "XDG_DATA_HOME": filepath.Join(home, "data"),
		"XDG_CACHE_HOME": filepath.Join(home, "cache"), "XDG_STATE_HOME": filepath.Join(home, "state"),
		"PRINTING_PRESS_CLIENT_PROFILE": "", "PRINTING_PRESS_MCP_BOUND_PROFILE": "",
	} {
		t.Setenv(name, value)
	}
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = &platformSourceRegistration{Source: "test-source", Adapter: platformCLIAdapter{}}
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "preview-tenant", Sources: map[string]platform.SourceProfile{
		"test-source": {CredentialRef: "op://Synthetic/Profile/token", ExpectedAccountID: "before"},
	}}
	if err := platform.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "sentinel"), []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		files := map[string]string{}
		err := filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[path] = string(data)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	for _, action := range []string{"add", "source set", "set-default", "migrate", "delete", "cache clear"} {
		t.Run(action, func(t *testing.T) {
			root := RootCmd()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&bytes.Buffer{})
			args := append([]string{"client"}, strings.Fields(action)...)
			root.SetArgs(append(args, "preview-tenant", "--dry-run", "--json"))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "\"dry_run\":true") && !strings.Contains(output.String(), "\"dry_run\": true") {
				t.Fatalf("missing preview: %s", output.String())
			}
			if !reflect.DeepEqual(before, snapshot()) {
				t.Fatal("dry-run changed a decoy profile, cache, state, or sentinel")
			}
		})
	}
}
