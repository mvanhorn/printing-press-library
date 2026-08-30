// Copyright 2026 bricenice17 and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "os"

// PATCH: Keep endpoint template-variable lookups separate from generated
// credential discovery. Rider IDs scope URLs; they are not authentication.
func templateVarEnv(name string) string {
	return os.Getenv(name)
}
