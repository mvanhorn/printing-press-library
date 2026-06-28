// Package config resolves filesystem paths and environment overrides for the
// biz-insurance-finder CLI. There is no network auth (this tool talks to no API);
// the environment variables here let users relocate the profile and registry
// files without flags.
package config

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
)

// Environment overrides.
const (
	EnvProfile   = "BIZ_INSURANCE_FINDER_PROFILE"   // path to profile.json
	EnvProviders = "BIZ_INSURANCE_FINDER_PROVIDERS" // path to providers.json
)

// ProfilePath resolves where the applicant profile lives, preferring an
// explicit flag value, then $BIZ_INSURANCE_FINDER_PROFILE, then ./profile.json.
func ProfilePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv(EnvProfile); env != "" {
		return env
	}
	return insurance.DefaultProfileFileName
}

// ProvidersPath resolves an explicit registry path: the flag, else
// $BIZ_INSURANCE_FINDER_PROVIDERS, else "" (which lets the registry loader fall
// back to ./providers.json or the embedded seed).
func ProvidersPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return os.Getenv(EnvProviders)
}
