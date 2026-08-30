// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package manifest loads and validates calendars.yaml — the approved-calendar
// manifest that scopes every cross-account verdict command (conflicts, slots,
// changes, events exceptions) and is the ground truth `manifest check` diffs
// live reality against.
//
// The file lives in the gauth config dir (next to profiles.yaml):
//
//	calendars:
//	  - account: personal          # gauth profile name from profiles.yaml
//	    id: derik@example.com      # Google calendar id
//	    role: write                # read | write (intent; OAuth scope still governs)
//	    default_write: true        # optional: default target for creates
//	    note: main personal cal    # optional free text
package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// FileName is the manifest file name inside the gauth config dir.
	FileName = "calendars.yaml"

	// RoleRead marks a calendar the CLI may only read.
	RoleRead = "read"
	// RoleWrite marks a calendar the CLI may mutate (subject to the
	// client-side event-safety barrier and the profile's OAuth scope).
	RoleWrite = "write"
)

// Entry is one approved (account, calendar) pair.
type Entry struct {
	Account      string `yaml:"account" json:"account"`
	ID           string `yaml:"id" json:"id"`
	Role         string `yaml:"role" json:"role"`
	DefaultWrite bool   `yaml:"default_write,omitempty" json:"default_write,omitempty"`
	Note         string `yaml:"note,omitempty" json:"note,omitempty"`
}

// Manifest is the parsed calendars.yaml.
type Manifest struct {
	Calendars []Entry `yaml:"calendars" json:"calendars"`
}

// Path returns the manifest file path inside dir.
func Path(dir string) string {
	return filepath.Join(dir, FileName)
}

// Load reads and structurally validates calendars.yaml in dir. A missing file
// is an explicit, actionable error (mirroring gauth.LoadProfiles), never a
// panic: test environments routinely lack the file.
//
// Structural rules enforced here (account-name existence is checked separately
// by Validate, which needs the profile list):
//   - at least one entry
//   - account and id non-empty on every entry
//   - role is read|write (normalized to lowercase)
//   - no duplicate (account, id) pair
func Load(dir string) (*Manifest, error) {
	p := Path(dir)
	b, err := os.ReadFile(p) // #nosec G304 -- path derived from the gauth config dir.
	if err != nil {
		return nil, fmt.Errorf("calendar manifest not found at %s — create it with calendars: [{account, id, role}] entries, or run 'manifest check --emit-skeleton' to generate one from live discovery: %w", p, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	if len(m.Calendars) == 0 {
		return nil, fmt.Errorf("%s contains no calendars", p)
	}
	seen := map[string]int{}
	for i := range m.Calendars {
		e := &m.Calendars[i]
		e.Account = strings.TrimSpace(e.Account)
		e.ID = strings.TrimSpace(e.ID)
		e.Role = strings.ToLower(strings.TrimSpace(e.Role))
		if e.Account == "" || e.ID == "" {
			return nil, fmt.Errorf("%s: calendar entry %d missing account or id", p, i)
		}
		if e.Role != RoleRead && e.Role != RoleWrite {
			return nil, fmt.Errorf("%s: calendar %q (account %q): unknown role %q (want %s|%s)", p, e.ID, e.Account, e.Role, RoleRead, RoleWrite)
		}
		key := e.Account + "\x00" + e.ID
		if prev, dup := seen[key]; dup {
			return nil, fmt.Errorf("%s: duplicate calendar entry (account %q, id %q) at indexes %d and %d", p, e.Account, e.ID, prev, i)
		}
		seen[key] = i
	}
	return &m, nil
}

// Validate checks that every entry's account is a known gauth profile name.
func (m *Manifest) Validate(accountNames []string) error {
	known := make(map[string]struct{}, len(accountNames))
	for _, n := range accountNames {
		known[n] = struct{}{}
	}
	for _, e := range m.Calendars {
		if _, ok := known[e.Account]; !ok {
			return fmt.Errorf("%s: calendar %q references unknown account %q (profiles.yaml has: %s)", FileName, e.ID, e.Account, strings.Join(accountNames, ", "))
		}
	}
	return nil
}

// LoadValidated is Load followed by Validate against the given profile names.
func LoadValidated(dir string, accountNames []string) (*Manifest, error) {
	m, err := Load(dir)
	if err != nil {
		return nil, err
	}
	if err := m.Validate(accountNames); err != nil {
		return nil, err
	}
	return m, nil
}

// ByAccount groups entries by account, preserving manifest order both across
// accounts (first-appearance order) and within an account.
func (m *Manifest) ByAccount() ([]string, map[string][]Entry) {
	var order []string
	grouped := map[string][]Entry{}
	for _, e := range m.Calendars {
		if _, ok := grouped[e.Account]; !ok {
			order = append(order, e.Account)
		}
		grouped[e.Account] = append(grouped[e.Account], e)
	}
	return order, grouped
}

// Find returns the entries whose calendar id equals id, across all accounts.
func (m *Manifest) Find(id string) []Entry {
	var out []Entry
	for _, e := range m.Calendars {
		if e.ID == id {
			out = append(out, e)
		}
	}
	return out
}
