// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Embedded "starter" normalization config and its layered loader. The starter
// declares the recommended entities and shapes with EMPTY rules; an operator
// config (if present at the default config path) merges over it per entity.
// This file is NOT generated and survives `generate --force`.
package cli

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

//go:embed normalize_starter.yaml
var starterConfigYAML []byte

// loadNormalizeConfig returns the active normalization config: the embedded
// starter (recommended entities + shapes, empty rules) optionally overlaid by an
// operator config at the default config path. The operator layer wins per entity.
//
// When no operator file exists the starter is returned alone — the common case
// in CI and on a fresh install. A present-but-unreadable or malformed operator
// file is surfaced as an error rather than silently ignored.
func loadNormalizeConfig() (*normalizecfg.Config, error) {
	return loadNormalizeConfigFrom(starterConfigYAML, defaultConfigPath(diceCLIName))
}

// loadNormalizeConfigFrom is the testable seam behind loadNormalizeConfig: it
// parses the embedded starter bytes and, if an operator config exists at
// operatorPath, merges it over the starter (operator wins per entity). A missing
// operator file yields the starter alone; an unreadable or malformed operator
// file is surfaced as an error.
func loadNormalizeConfigFrom(starterBytes []byte, operatorPath string) (*normalizecfg.Config, error) {
	starter, err := normalizecfg.Parse(starterBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing embedded starter config: %w", err)
	}

	opData, err := os.ReadFile(operatorPath)
	if os.IsNotExist(err) {
		return starter, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading operator config %q: %w", operatorPath, err)
	}
	operator, err := normalizecfg.Parse(opData)
	if err != nil {
		return nil, fmt.Errorf("parsing operator config %q: %w", operatorPath, err)
	}
	return normalizecfg.Merge(starter, operator)
}
