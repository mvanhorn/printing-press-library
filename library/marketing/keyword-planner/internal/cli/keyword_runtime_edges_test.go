// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlannerEnvOnlyWithAlternateHome(t *testing.T) {
	home, alternate := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(plannerEnvFileEnv, "")
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"GOOGLE_ADS_CLIENT_ID", "GOOGLE_ADS_CLIENT_SECRET", "GOOGLE_ADS_REFRESH_TOKEN", "GOOGLE_ADS_DEVELOPER_TOKEN"} {
		t.Setenv(key, "fixture")
	}
	t.Setenv("GOOGLE_ADS_CUSTOMER_ID", "123-456-7890")
	t.Setenv("GOOGLE_ADS_LOGIN_CUSTOMER_ID", "")
	cfg, err := plannerLiveConfig("", "", &rootFlags{homePath: alternate, rateLimit: -1})
	if err != nil || cfg.EnvPath != filepath.Join(alternate, ".env") || cfg.CustomerID != "1234567890" {
		t.Fatalf("isolated env config: path=%q error=%v", cfg.EnvPath, err)
	}
}

func TestPlannerQuotedTildeHomeUsesCanonicalPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(plannerDBEnv, "")
	t.Setenv(plannerEnvFileEnv, "")
	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"doctor", "--live", "--dry-run", "--home", "~/isolated", "--json", "--no-learn"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "isolated")
	if !strings.Contains(output.String(), filepath.Join(want, plannerDBDefaultDir, plannerDBFilename)) || !strings.Contains(output.String(), filepath.Join(want, ".env")) {
		t.Fatalf("noncanonical home paths: %s", output.String())
	}
	if !strings.Contains(root.PersistentFlags().Lookup("home").Usage, "request pacing remains shared") {
		t.Fatal("home help omits shared pacing exception")
	}
}

func TestPlannerExplicitNonpositiveTimeoutFailsBeforeIO(t *testing.T) {
	for _, timeout := range []string{"0s", "-1s"} {
		t.Run(timeout, func(t *testing.T) {
			root := RootCmd()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs([]string{"doctor", "--live", "--timeout", timeout, "--env-file", filepath.Join(t.TempDir(), "absent.env"), "--no-learn"})
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--timeout must be a positive") {
				t.Fatalf("timeout error=%v", err)
			}
		})
	}
}
