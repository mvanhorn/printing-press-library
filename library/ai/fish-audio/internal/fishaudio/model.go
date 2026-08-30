// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

// Package fishaudio holds the pure request/pricing/format logic behind the
// hand-written Fish Audio commands. Nothing here touches the network, the
// filesystem, or Cobra: the CLI layer supplies inputs and renders outputs, so
// every rule in this package is unit-testable on its own.
package fishaudio

import (
	"fmt"
	"sort"
	"strings"
)

// SupportedModels is the closed set the `model` HTTP header accepts. The API
// silently falls back to s2.1-pro on an unrecognized value instead of
// returning an error, so the CLI validates the value before it is sent.
var SupportedModels = []string{"s1", "s2-pro", "s2.1-pro", "s2.1-pro-free"}

// DefaultModel is the model the API itself defaults to.
const DefaultModel = "s2.1-pro"

// deprecatedModels are vendor model strings that still resolve but are on the
// vendor's deprecation list. They are accepted with a warning, not rejected.
var deprecatedModels = map[string]string{
	"speech-1.5": "speech-1.5 is deprecated; use s2.1-pro",
	"speech-1.6": "speech-1.6 is deprecated; use s2.1-pro",
}

// ValidateModel checks a --model value. It returns the value to send and a
// non-empty warning when the value is deprecated. An unknown value is an
// error, naming the accepted set.
func ValidateModel(model string) (string, string, error) {
	m := strings.TrimSpace(model)
	if m == "" {
		return DefaultModel, "", nil
	}
	for _, ok := range SupportedModels {
		if m == ok {
			return m, "", nil
		}
	}
	if warn, ok := deprecatedModels[m]; ok {
		return m, warn, nil
	}
	return "", "", fmt.Errorf("invalid value %q for --model: must be one of %s", model, strings.Join(SupportedModels, ", "))
}

// IsS2Family reports whether a model belongs to the s2 family. Multi-speaker
// `<|speaker:N|>` tags and array reference_id values work only on s2 models.
func IsS2Family(model string) bool {
	return strings.HasPrefix(model, "s2")
}

// SupportedFormats is the closed set of output container formats.
var SupportedFormats = []string{"mp3", "wav", "pcm", "opus"}

// ValidateFormat checks a --format value against the closed set.
func ValidateFormat(format string) (string, error) {
	f := strings.TrimSpace(format)
	if f == "" {
		return "mp3", nil
	}
	for _, ok := range SupportedFormats {
		if f == ok {
			return f, nil
		}
	}
	return "", fmt.Errorf("invalid value %q for --format: must be one of %s", format, strings.Join(SupportedFormats, ", "))
}

// SupportedLatencies is the closed set of latency/quality trade-off modes.
var SupportedLatencies = []string{"normal", "balanced", "low"}

// ValidateLatency checks a --latency value against the closed set.
func ValidateLatency(latency string) (string, error) {
	l := strings.TrimSpace(latency)
	if l == "" {
		return "normal", nil
	}
	for _, ok := range SupportedLatencies {
		if l == ok {
			return l, nil
		}
	}
	return "", fmt.Errorf("invalid value %q for --latency: must be one of %s", latency, strings.Join(SupportedLatencies, ", "))
}

// SupportedVisibilities is the closed set of model visibility values.
var SupportedVisibilities = []string{"private", "unlist", "public"}

// ValidateVisibility checks a --visibility value against the closed set.
func ValidateVisibility(visibility string) (string, error) {
	v := strings.TrimSpace(visibility)
	if v == "" {
		return "private", nil
	}
	for _, ok := range SupportedVisibilities {
		if v == ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("invalid value %q for --visibility: must be one of %s", visibility, strings.Join(SupportedVisibilities, ", "))
}

// canonicalKV renders a map as a stable `k=v` list joined by newlines. Sorting
// the keys is what makes a request hash independent of flag order.
func canonicalKV(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	return strings.Join(parts, "\n")
}
