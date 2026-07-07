// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Bridges the cobra/flags world to internal/captcha: resolves the active
// profile from --captcha-profile / SUNO_CAPTCHA_PROFILE / config and builds
// captcha.Options (dedicated user-data-dir + CDP port).

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/captcha"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/config"
)

// captchaProfileFlag is set by the gated generate commands.
var captchaProfileFlag string

// captchaProfilesDir is the parent of all dedicated solver profiles. Never the
// user's real Chrome dir.
func captchaProfilesDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "suno-pp-cli", "chrome-profiles")
}

// resolveCaptchaOptions loads config, resolves the profile (env > flag handled
// by passing env-or-flag), ensures it exists (assigning a port, persisted so the
// port survives), and returns the Options.
func resolveCaptchaOptions(configPath string, interactive bool) (captcha.Options, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return captcha.Options{}, err
	}
	sel := captchaProfileFlag
	if sel == "" {
		sel = os.Getenv("SUNO_CAPTCHA_PROFILE")
	}
	name := cfg.ResolveCaptchaProfile(sel)
	prof := cfg.EnsureCaptchaProfile(name)

	dir := prof.UserDataDir
	if dir == "" {
		dir = filepath.Join(captchaProfilesDir(), name)
	}
	if err := cfg.SaveCaptcha(); err != nil {
		return captcha.Options{}, err
	}
	return captcha.Options{
		Profile:     name,
		UserDataDir: dir,
		CDPPort:     prof.CDPPort,
		Interactive: interactive,
	}, nil
}

// defaultSolver is the production Solver, overridable in tests.
var defaultSolver captcha.Solver = captcha.New()

// solveCaptchaToken runs the solver for the active profile and returns a fresh
// token.
func solveCaptchaToken(ctx context.Context, configPath string, interactive bool) (string, error) {
	opts, err := resolveCaptchaOptions(configPath, interactive)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "Solving hCaptcha in Chrome… if macOS asks to allow access to Chrome's data, approve it promptly (the solver waits up to "+captcha.DefaultTimeout.String()+").")
	tok, err := defaultSolver.Solve(ctx, opts)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("captcha solve timed out after %s — if macOS prompted to allow Chrome access, approve it and retry: %w", captcha.DefaultTimeout, err)
		}
		return "", err
	}
	return tok, nil
}
