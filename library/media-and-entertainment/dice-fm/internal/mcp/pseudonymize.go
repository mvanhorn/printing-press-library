// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Pseudonymization layer for the MCP output boundary (review finding #4).
//
// dice-fm is frequently driven by an agent/MCP host (Claude Desktop / co-work /
// agy). Read-only MCP tools that return fan PII (email/phone/name/dob) pull that
// PII into the model context — host logs, transcripts, and the model provider.
// This layer replaces those direct identifiers with a deterministic, salted
// HMAC token ("fan:<16hex>") at the MCP boundary ONLY. The human CLI keeps
// emitting raw output to the operator's own terminal; only the MCP surface
// pseudonymizes by default. A per-tool include_pii=true bypasses the scrub and
// returns raw values AND the token (so an operator who explicitly opts in can
// still link rows).
//
// The token is deterministic for a given (salt, identityKey): the same person
// gets the same token across every tool/call, so an agent can still dedup and
// correlate without ever seeing a raw identifier. The salt is per-store, random,
// never emitted, and not cross-store correlatable.
package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// saltSidecarName is the per-store sidecar file holding the HMAC salt. It is
// deliberately outside SQLite so the read-only sql MCP tool cannot query it.
const saltSidecarName = "pseudonymize.salt"

// saltLen is the salt size in bytes.
const saltLen = 32

// piiDropKeys are direct-identifier keys removed inside a PII container subtree
// (fan/holder) by ScrubJSONBlob. dob is special-category and is dropped here so
// it never escapes even if a spec forgets to list it. `name` is included because
// inside a fan/holder object it is a person's name — but it is ONLY consulted
// while inside a container subtree, so a top-level event/venue `name` is never
// touched by the blob path.
var piiDropKeys = map[string]bool{
	"email":        true,
	"holder_email": true,
	"phone":        true,
	"phonenumber":  true,
	"firstname":    true,
	"first_name":   true,
	"lastname":     true,
	"last_name":    true,
	"name":         true,
	"holder_name":  true,
	"dob":          true,
}

// flatPIIColumns are direct-identifier column names redacted on flat sql rows
// once rowHasFlatPII has identified the row as carrying person data. `name` is
// included so mirrored fans/door person-name fields are redacted, but a bare
// analytics row with only an event/venue `name` is preserved because
// isFlatPersonIdentifierKey intentionally excludes bare `name`. Mixed sql rows
// that select an event `name` alongside a person identifier can over-redact the
// event name; that is an accepted best-effort sql tradeoff because redacting an
// event name is much less harmful than leaking a person name.
var flatPIIColumns = map[string]bool{
	"email":        true,
	"holder_email": true,
	"phone":        true,
	"phonenumber":  true,
	"firstname":    true,
	"first_name":   true,
	"lastname":     true,
	"last_name":    true,
	"name":         true,
	"holder_name":  true,
	"dob":          true,
}

// blobPIIContainerKeys names object keys whose contents are personal data, used
// by ScrubJSONBlob to know where to inject a fan_ref token and which identity
// key to derive it from.
var blobPIIContainerKeys = map[string]string{
	"fan":    "id",
	"holder": "email", // holder rows are keyed by email (holder id often absent)
}

// Token returns the pseudonymous reference for an identity key: "fan:" followed
// by the first 16 hex chars of HMAC-SHA256(salt, identityKey). Deterministic for
// a given (salt, identityKey).
func Token(salt []byte, identityKey string) string {
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(identityKey))
	sum := mac.Sum(nil)
	return "fan:" + hex.EncodeToString(sum)[:16]
}

// Opts controls a Scrub call.
type Opts struct {
	// Salt is the per-store HMAC salt. Resolve via SaltForStore or saltFromStore.
	Salt []byte
	// IncludePII, when true, returns raw identifier values AND the linking
	// token instead of redacting.
	IncludePII bool
}

// NestedSpec describes a nested PII container object to scrub recursively.
type NestedSpec struct {
	Key  string
	Spec FieldSpec
}

// FieldSpec declares which keys of a flat result row are identity fields, plus
// any nested PII containers. IdentityKey names the key whose value seeds the
// token (e.g. fan id, or holder email).
type FieldSpec struct {
	IdentityKey      string
	EmailKeys        []string
	PhoneKeys        []string
	NameKeys         []string
	NestedContainers []NestedSpec
}

// Scrub returns a copy of row with identity fields handled per opts. In the
// default (redacted) mode the email/phone/name keys are removed and a "fan_ref"
// token is added; dob is always removed. In include_pii mode the raw values are
// preserved and the token is still added for linking. Nested PII containers
// (e.g. a ticket's holder object) are scrubbed recursively. The input map is
// not mutated.
func Scrub(row map[string]any, spec FieldSpec, opts Opts) map[string]any {
	out := make(map[string]any, len(row)+1)
	for k, v := range row {
		out[k] = v
	}

	identityKeys := append(append(append([]string{}, spec.EmailKeys...), spec.PhoneKeys...), spec.NameKeys...)

	for k := range out {
		if canonicalPIIKey(k) == "dob" {
			delete(out, k)
		}
	}

	if !opts.IncludePII {
		for _, k := range identityKeys {
			delete(out, k)
		}
	}

	// Token from the identity key's value (fall back to the first available
	// email if IdentityKey is unset/empty).
	if id := identityValue(row, spec); id != "" {
		out["fan_ref"] = Token(opts.Salt, id)
	}

	for _, nc := range spec.NestedContainers {
		nested, ok := row[nc.Key].(map[string]any)
		if !ok {
			continue
		}
		out[nc.Key] = Scrub(nested, nc.Spec, opts)
	}
	return out
}

// identityValue resolves the token seed for a row: the IdentityKey's value if
// present, else the first non-empty email key value.
func identityValue(row map[string]any, spec FieldSpec) string {
	if spec.IdentityKey != "" {
		if v, ok := row[spec.IdentityKey]; ok {
			if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	for _, ek := range spec.EmailKeys {
		if v, ok := row[ek]; ok {
			if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// ScrubJSONBlob recursively scrubs an arbitrary decoded-JSON value (map, slice,
// scalar). It is the best-effort path for the search and sql tools, where the
// row shape is not statically known: inside any object under a known PII
// container key (fan/holder) it drops the direct-identifier keys and injects a
// fan_ref; it also drops a top-level/anywhere bare identifier key set as a
// backstop. Returns a new value; the input is not mutated. include_pii preserves
// raw identifiers except dob, which is always removed.
func ScrubJSONBlob(v any, opts Opts) any {
	return scrubBlobValue(v, "", opts)
}

// scrubBlobValue walks v. containerKey is the PII-container key (fan/holder)
// whose subtree we are currently inside, or "" when not inside one.
func scrubBlobValue(v any, containerKey string, opts Opts) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		// If this object is itself a PII container, derive its token first.
		if containerKey != "" {
			idKey := blobPIIContainerKeys[containerKey]
			if seed := identitySeed(t, idKey, "id"); seed != "" {
				out["fan_ref"] = Token(opts.Salt, seed)
			}
		}
		for k, child := range t {
			canon := canonicalPIIKey(k)
			if canon == "dob" {
				continue
			}
			// Drop direct identifiers when inside a PII container.
			if containerKey != "" && !opts.IncludePII && piiDropKeys[canon] {
				continue
			}
			// Recurse: a nested fan/holder marks its subtree as a container.
			nextContainer := ""
			if _, isContainer := blobPIIContainerKeys[k]; isContainer {
				nextContainer = k
			}
			out[k] = scrubBlobValue(child, nextContainer, opts)
		}
		if containerKey == "" {
			scrubFlatPIIObject(out, t, opts)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = scrubBlobValue(child, containerKey, opts)
		}
		return out
	default:
		return v
	}
}

func scrubFlatPIIObject(out, row map[string]any, opts Opts) {
	// dob is special-category data and is never emitted, even with include_pii.
	for k := range out {
		if canonicalPIIKey(k) == "dob" {
			delete(out, k)
		}
	}
	hasFlatPII := rowHasFlatPII(row)
	if opts.IncludePII {
		if hasFlatPII {
			if seed := flatIdentitySeed(row); seed != "" {
				out["fan_ref"] = Token(opts.Salt, seed)
			}
		}
		return
	}
	if hasFlatPII {
		if seed := flatIdentitySeed(row); seed != "" {
			out["fan_ref"] = Token(opts.Salt, seed)
		}
	}
	for k := range out {
		canon := canonicalPIIKey(k)
		if hasFlatPII && shouldRedactFlatPIIKey(canon) {
			delete(out, k)
		}
	}
}

func normalizedNameSeed(parts ...string) string {
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "<nil>" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return strings.ToLower(strings.Join(cleaned, " "))
}

func canonicalPIIKey(k string) string {
	return strings.ToLower(k)
}

// identitySeed picks a token seed using the same preference order as flat rows:
// email columns, then fan/holder ids, then normalized names and phone fallbacks.
// Container callers can pass a local id key such as "id" for fan/holder blobs.
func identitySeed(row map[string]any, containerIDKeys ...string) string {
	for _, want := range []string{"email", "holder_email"} {
		for k, v := range row {
			if canonicalPIIKey(k) != want {
				continue
			}
			if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	for k, v := range row {
		if !isFanHolderIDKey(k) {
			continue
		}
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return s
		}
	}
	for _, k := range containerIDKeys {
		if k == "" {
			continue
		}
		for rowKey, v := range row {
			if canonicalPIIKey(rowKey) != canonicalPIIKey(k) {
				continue
			}
			if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	var first, last string
	for k, v := range row {
		switch canonicalPIIKey(k) {
		case "firstname", "first_name":
			if first == "" {
				first = fmt.Sprintf("%v", v)
			}
		case "lastname", "last_name":
			if last == "" {
				last = fmt.Sprintf("%v", v)
			}
		}
	}
	if s := normalizedNameSeed(first, last); s != "" {
		return s
	}
	for _, want := range []string{"holder_name", "name", "phone", "phonenumber"} {
		for k, v := range row {
			if canonicalPIIKey(k) != want {
				continue
			}
			if s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v))); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

// SaltForStore lazily reads (or creates) the per-store HMAC salt from a 0600
// sidecar file next to the SQLite database. The salt is 32 random bytes,
// created on first use, never emitted, and stable within the store.
func SaltForStore(s *store.Store) ([]byte, error) {
	return readOrCreateSalt(saltPathForStore(s))
}

func readOrCreateSalt(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating pseudonymize salt directory: %w", err)
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)

	if salt, err := os.ReadFile(path); err == nil {
		if len(salt) != saltLen {
			return nil, fmt.Errorf("pseudonymize salt file %q exists but has wrong length %d (expected %d); refusing to silently rotate — remove it to regenerate", path, len(salt), saltLen)
		}
		return salt, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading pseudonymize salt: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generating pseudonymize salt: %w", err)
	}
	if err := os.WriteFile(path, salt, 0o600); err != nil {
		return nil, fmt.Errorf("persisting pseudonymize salt: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return salt, nil
}

func saltPathForStore(s *store.Store) string {
	return saltPathForDB(s.Path())
}

func saltPathForDB(path string) string {
	return filepath.Join(filepath.Dir(path), saltSidecarName)
}
