// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// profileRecord is the minimal profile shape the API returns.
type profileRecord struct {
	ProfileID string `json:"profileId"`
	Name      string `json:"name"`
}

// resolveProfileID finds the profile UUID every Tiimo path is scoped to.
//
// The generated endpoint commands take it as a required positional, which is
// correct for a faithful API mirror but hostile for daily-driver commands:
// nobody remembers their own profile UUID. Resolution order, cheapest first:
//
//  1. an explicit --profile value (UUID or name)
//  2. the TIIMO_PROFILE_ID environment variable
//  3. the local mirror's profiles table (no network)
//  4. a live GET /api/profiles
//
// Most accounts hold exactly one profile, so in practice this resolves from
// the mirror without a round trip.
func resolveProfileID(ctx context.Context, cmd *cobra.Command, flags *rootFlags, explicit, dbPath string) (string, error) {
	if v := strings.TrimSpace(explicit); v != "" {
		if looksLikeUUID(v) {
			return v, nil
		}
		// A name was given; fall through to a lookup that can match it.
		if id, err := lookupProfileByName(ctx, cmd, flags, v, dbPath); err == nil && id != "" {
			return id, nil
		}
		return "", usageErr(fmt.Errorf("no profile matching %q; run `tiimo-pp-cli profiles list` to see available profiles", v))
	}
	if v := strings.TrimSpace(os.Getenv("TIIMO_PROFILE_ID")); v != "" {
		return v, nil
	}

	if profiles, err := profilesFromMirror(ctx, cmd, flags, dbPath); err == nil && len(profiles) > 0 {
		return pickSingleProfile(profiles)
	}

	profiles, err := profilesFromAPI(ctx, flags)
	if err != nil {
		return "", err
	}
	if len(profiles) == 0 {
		return "", notFoundErr(fmt.Errorf("no Tiimo profiles found for this account"))
	}
	return pickSingleProfile(profiles)
}

// pickSingleProfile returns the only profile, or asks the caller to
// disambiguate rather than silently guessing which of several shared
// profiles the user meant.
func pickSingleProfile(profiles []profileRecord) (string, error) {
	if len(profiles) == 1 {
		return profiles[0].ProfileID, nil
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return "", usageErr(fmt.Errorf(
		"this account has %d profiles (%s); pass --profile <name-or-uuid> to choose one",
		len(profiles), strings.Join(names, ", ")))
}

func lookupProfileByName(ctx context.Context, cmd *cobra.Command, flags *rootFlags, name, dbPath string) (string, error) {
	profiles, err := profilesFromMirror(ctx, cmd, flags, dbPath)
	if err != nil || len(profiles) == 0 {
		profiles, err = profilesFromAPI(ctx, flags)
		if err != nil {
			return "", err
		}
	}
	want := strings.ToLower(name)
	for _, p := range profiles {
		if strings.ToLower(p.Name) == want {
			return p.ProfileID, nil
		}
	}
	for _, p := range profiles {
		if strings.Contains(strings.ToLower(p.Name), want) {
			return p.ProfileID, nil
		}
	}
	return "", fmt.Errorf("no profile named %q", name)
}

func profilesFromMirror(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) ([]profileRecord, error) {
	st, ok, err := openLocalMirror(ctx, cmd, flags, dbPath)
	if err != nil || !ok {
		return nil, fmt.Errorf("no local mirror")
	}
	defer st.Close()

	items, err := st.List("profiles", 100)
	if err != nil {
		return nil, err
	}
	out := make([]profileRecord, 0, len(items))
	for _, raw := range items {
		var p profileRecord
		if err := json.Unmarshal(raw, &p); err != nil || p.ProfileID == "" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func profilesFromAPI(ctx context.Context, flags *rootFlags) ([]profileRecord, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/api/profiles", nil)
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}
	var out []profileRecord
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	return out, nil
}

// looksLikeUUID is a shape check, not a validator: it only decides whether a
// --profile value should be treated as an ID or looked up as a name.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
