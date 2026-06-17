// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewNovelWebhookServeCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelWebhookServeCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}

func TestNewNovelWebhookServeCmdRegisterValidation(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelWebhookServeCmd(flags)
	if err := cmd.Flags().Parse([]string{"--register"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Error("expected usage error when --register is set without callback/event/data type")
	}
}
