// Package profile stores the user's body weight and estimates walking calories,
// the one metric a WalkingPad never reports. Energy is computed with the MET
// (metabolic equivalent) method: kcal = MET(speed) * weight_kg * hours.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile holds the inputs needed to estimate calories. Weight is the only
// field the MET model needs; nothing speculative is stored.
type Profile struct {
	WeightKg float64 `json:"weight_kg"`
}

// Path returns the profile file path for the CLI.
func Path(cliName string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, cliName, "profile.json"), nil
}

// Load reads the profile. A missing profile is not an error: it returns a zero
// Profile and ok=false so callers can prompt the user to set a weight.
func Load(path string) (Profile, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, false, nil
		}
		return Profile{}, false, fmt.Errorf("read profile: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, false, fmt.Errorf("decode profile: %w", err)
	}
	return p, p.WeightKg > 0, nil
}

// Save writes the profile, creating the parent directory.
func Save(path string, p Profile) error {
	if p.WeightKg <= 0 {
		return fmt.Errorf("weight must be greater than 0")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	return nil
}

// metPoint is one (speed km/h, MET) reference value for walking.
type metPoint struct {
	kmh float64
	met float64
}

// Compendium-of-Physical-Activities walking MET values, interpolated between.
var metTable = []metPoint{
	{3.2, 2.8},
	{4.0, 3.0},
	{4.8, 3.5},
	{5.6, 4.3},
	{6.4, 5.0},
}

// METForSpeed returns the walking MET for an average speed in km/h, linearly
// interpolated within the table and clamped at the ends. Non-positive speed
// returns 0 (no activity).
func METForSpeed(kmh float64) float64 {
	if kmh <= 0 {
		return 0
	}
	if kmh <= metTable[0].kmh {
		return metTable[0].met
	}
	last := metTable[len(metTable)-1]
	if kmh >= last.kmh {
		return last.met
	}
	for i := 1; i < len(metTable); i++ {
		hi := metTable[i]
		if kmh <= hi.kmh {
			lo := metTable[i-1]
			frac := (kmh - lo.kmh) / (hi.kmh - lo.kmh)
			return lo.met + frac*(hi.met-lo.met)
		}
	}
	return last.met
}

// Calories estimates kcal burned for a walk of avgSpeedKmh over durationS at
// the given body weight.
func Calories(weightKg, avgSpeedKmh float64, durationS int) float64 {
	if weightKg <= 0 || durationS <= 0 {
		return 0
	}
	met := METForSpeed(avgSpeedKmh)
	hours := float64(durationS) / 3600
	return met * weightKg * hours
}
