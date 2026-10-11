package config

import "os"

// AgencyUID returns the Hostfully agency UID from the environment.
// This is required for all API calls.
func (c *Config) AgencyUID() string {
	if v := os.Getenv("HOSTFULLY_AGENCY_ID"); v != "" {
		return v
	}
	return ""
}
