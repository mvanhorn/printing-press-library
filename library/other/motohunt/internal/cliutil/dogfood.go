// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: dogfood-mode detection for bounding side-effectful enrichment.

package cliutil

import "os"

// DogfoodEnvVar is the env var the printing-press dogfood harness sets when it
// exercises the CLI against the live site. Commands that fan out many extra
// network calls (e.g. `deal` enriching listings via per-id `get`) should curtail
// that work under dogfood so a smoke run stays cheap and fast.
const DogfoodEnvVar = "PRINTING_PRESS_DOGFOOD"

// IsDogfoodEnv reports whether the current process runs under the dogfood
// harness. Pair with a small enrich cap so dogfood runs don't hammer the site.
func IsDogfoodEnv() bool {
	return os.Getenv(DogfoodEnvVar) == "1"
}
