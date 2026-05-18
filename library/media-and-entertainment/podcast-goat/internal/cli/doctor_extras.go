// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// Podcast-goat-specific doctor checks that extend the generator-emitted
// doctor.go. Lives in a separate file so regen of doctor.go preserves these
// checks (doctor.go is emitted with DO NOT EDIT; this file is hand-authored).

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/youtube"
)

// collectKeySources surfaces per-provider credential source so users can see
// whether a saved key (auth set-key) is being picked up. Without this, a
// user who saved a key via config has no signal that it worked — the
// env-vars block still says "missing required" because that block tracks
// env-var presence, not effective key presence.
func collectKeySources() map[string]string {
	report := map[string]string{}
	for _, provider := range config.KnownProviders {
		env := config.EnvVarFor(provider)
		if env == "" {
			continue
		}
		report[provider] = config.Source(env)
	}
	return report
}

// renderKeySources prints the per-provider key-source row.
func renderKeySources(w io.Writer, rep map[string]string) {
	if len(rep) == 0 {
		return
	}
	fmt.Fprintf(w, "  INFO Paid keys:\n")
	for _, provider := range config.KnownProviders {
		source, ok := rep[provider]
		if !ok {
			continue
		}
		label := source
		switch source {
		case "missing":
			label = "missing (run `auth set-key --provider " + provider + " --value <key>` to persist)"
		case "config":
			label = "config (persisted via auth set-key)"
		case "env":
			label = "env (" + config.EnvVarFor(provider) + ")"
		}
		fmt.Fprintf(w, "    %-14s %s\n", provider+":", label)
	}
}

// renderYtDlpReport emits the human-readable yt-dlp row for `doctor`.
// Indicator follows the same OK/INFO/FAIL convention as cache/auth.
func renderYtDlpReport(w io.Writer, rep map[string]any) {
	status, _ := rep["status"].(string)
	switch status {
	case "ok":
		fmt.Fprintf(w, "  OK yt-dlp: ok\n")
		if loc, ok := rep["location"]; ok {
			fmt.Fprintf(w, "    location: %v\n", loc)
		}
		if src, ok := rep["source"]; ok {
			fmt.Fprintf(w, "    source: %v\n", src)
		}
	default:
		fmt.Fprintf(w, "  INFO yt-dlp: %s\n", status)
		if hint, ok := rep["hint"]; ok {
			fmt.Fprintf(w, "    hint: %v\n", hint)
		}
	}
}

// collectYtDlpReport surfaces yt-dlp presence — PATH-installed, sidecar, or
// neither — so users see the gap at `doctor` time instead of at first
// `episode get <yt-url>` failure.
func collectYtDlpReport() map[string]any {
	report := map[string]any{}
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		report["status"] = "ok"
		report["location"] = path
		report["source"] = "PATH"
		return report
	}
	sidecar := youtube.SidecarPath()
	if _, err := os.Stat(sidecar); err == nil {
		report["status"] = "ok"
		report["location"] = sidecar
		report["source"] = "sidecar (auto-downloaded)"
		return report
	}
	report["status"] = "not-installed"
	report["hint"] = fmt.Sprintf("yt-dlp not on PATH and no sidecar at %s. The first `episode get <youtube-url>` will auto-download yt-dlp (~35MB one-time). For instant first-fetch, install via `brew install yt-dlp` or `pip install yt-dlp`.", sidecar)
	return report
}
