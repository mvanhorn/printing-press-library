// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
)

func TestCommandAuthRequiredForMissingEnvAndConfig(t *testing.T) {
	out, err := executeTestCommand(t, "shops", "info", "--dry-run")
	if err == nil {
		t.Fatalf("shops info succeeded with missing auth; output: %s", out)
	}
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode(err) = %d, want 4; err = %v", ExitCode(err), err)
	}
	for _, want := range []string{config.EnvAppKey, config.EnvAppSecret, config.EnvAccessToken} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %s", err.Error(), want)
		}
	}
}

func TestLimitValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "orders list lower bound",
			args: []string{"orders", "list", "--limit", "0"},
			want: "--limit must be between 1 and 100",
		},
		{
			name: "products list upper bound",
			args: []string{"products", "list", "--limit", "101"},
			want: "--limit must be between 1 and 100",
		},
		{
			name: "fulfillment list lower bound",
			args: []string{"fulfillment", "list", "--limit", "0"},
			want: "--limit must be between 1 and 50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executeTestCommand(t, tt.args...)
			if err == nil {
				t.Fatal("command succeeded, want validation error")
			}
			if ExitCode(err) != 2 {
				t.Fatalf("ExitCode(err) = %d, want 2; err = %v", ExitCode(err), err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRequiredIDValidation(t *testing.T) {
	tests := [][]string{
		{"orders", "get"},
		{"products", "get"},
		{"inventory", "get"},
		{"fulfillment", "get"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := executeTestCommand(t, args...)
			if err == nil {
				t.Fatal("command succeeded, want missing ID validation error")
			}
			if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
				t.Fatalf("error = %q, want missing required ID validation", err.Error())
			}
		})
	}
}

func TestInventoryListRequiresExplicitIDs(t *testing.T) {
	_, err := executeTestCommand(t, "inventory", "list")
	if err == nil {
		t.Fatal("inventory list succeeded without IDs")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode(err) = %d, want 2; err = %v", ExitCode(err), err)
	}
	if !strings.Contains(err.Error(), "provide --product-id or --sku-id") {
		t.Fatalf("error = %q, want explicit ID validation", err.Error())
	}
}

func executeTestCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	clearTikTokEnv(t)

	configPath := filepath.Join(t.TempDir(), "missing-config.toml")
	allArgs := append([]string{"--config", configPath}, args...)

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(allArgs)

	err := cmd.Execute()
	return out.String(), err
}

func clearTikTokEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		config.EnvConfigPath,
		config.EnvAppKey,
		config.EnvAppSecret,
		config.EnvAccessToken,
		config.EnvRefreshToken,
		config.EnvShopID,
		config.EnvShopCipher,
		config.EnvBaseURL,
		config.EnvAuthBaseURL,
	} {
		t.Setenv(name, "")
	}
}
