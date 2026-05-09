// Package hfdata bundles static data (currently the backend-support matrix)
// into the binary. Override layering happens in package hfx; this package
// only owns the //go:embed bytes.
package hfdata

import (
	_ "embed"
)

//go:embed backend-support.json
var backendSupportJSON []byte

// BackendSupportJSON returns the embedded backend-support.json bytes.
func BackendSupportJSON() ([]byte, error) {
	return backendSupportJSON, nil
}
