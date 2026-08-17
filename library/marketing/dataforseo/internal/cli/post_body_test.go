// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestGeneratedPostCommandAcceptsTopLevelArray(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DATAFORSEO_LOGIN", "test-login")
	t.Setenv("DATAFORSEO_PASSWORD", "test-password")

	stdin, err := os.CreateTemp(t.TempDir(), "post-body-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if _, err := stdin.WriteString(`[{"keywords":["tree service"],"location_code":2840,"language_code":"en"}]`); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	originalStdin := os.Stdin
	os.Stdin = stdin
	defer func() { os.Stdin = originalStdin }()

	flags := rootFlags{dryRun: true, asJSON: true, noInput: true}
	cmd := newKeywordsDataGoogleAdsSearchVolumeLiveCmd(&flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--stdin"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute generated POST command with array body: %v", err)
	}
	if !strings.Contains(output.String(), `"path": "/v3/keywords_data/google_ads/search_volume/live"`) {
		t.Fatalf("dry-run output does not identify the endpoint:\n%s", output.String())
	}
}
