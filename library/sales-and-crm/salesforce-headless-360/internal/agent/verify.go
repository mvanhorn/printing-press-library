package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

// VerifyResult captures the outcome of a bundle verification pass.
type VerifyResult struct {
	KID         string         `json:"kid"`
	Issuer      string         `json:"iss"`
	Subject     string         `json:"sub"`
	GeneratedAt time.Time      `json:"generated_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
	Expired     bool           `json:"expired"`
	SignatureOK bool           `json:"signature_ok"`
	Warnings    []string       `json:"warnings,omitempty"`
	ManifestSHA string         `json:"manifest_sha256"`
	Audience    string         `json:"aud"`
	Mode        string         `json:"mode"` // offline | live | deep | strict
	Redactions  map[string]int `json:"redactions,omitempty"`
}

// VerifyOptions configures verification strictness.
type VerifyOptions struct {
	// Live: re-fetch the org's key collection. Not yet implemented in v1 —
	// requires an authed org. Verify returns a warning when Live is set
	// without auth plumbed through.
	Live bool
	// Deep: re-fetch ContentVersion file bytes and rehash against manifest
	// entries. Not yet implemented in v1 — requires an authed org.
	Deep bool
	// Strict: combines Live + Deep and fails if Expired.
	Strict bool
	// FileFetcher re-fetches ContentVersion bytes for Deep verification.
	FileFetcher ContentVersionFetcher
	// KeyClient re-fetches the org key collection for Live verification.
	KeyClient trust.OrgClient
	OrgAlias  string
}

// VerifyBundle loads a bundle from disk, verifies its JWS against the cached
// public key in the keystore, and returns a structured result.
func VerifyBundle(path string, opts VerifyOptions) (*VerifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle: %w", err)
	}
	var bundle Bundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parse bundle: %w", err)
	}
	if bundle.Signature == "" {
		return nil, fmt.Errorf("bundle has no signature")
	}
	kid, err := trust.ExtractKIDUnsafe(bundle.Signature)
	if err != nil {
		return nil, fmt.Errorf("extract kid: %w", err)
	}
	record, err := trust.LoadKeyRecord(kid)
	if err != nil {
		return nil, fmt.Errorf("load key record for kid %s: %w", kid, err)
	}
	pub, err := trust.ParsePublicKeyPEM(record.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	payload, header, err := trust.VerifyJWS(bundle.Signature, pub)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	var claims struct {
		Iss                  string         `json:"iss"`
		Sub                  string         `json:"sub"`
		Exp                  int64          `json:"exp"`
		Iat                  int64          `json:"iat"`
		Aud                  string         `json:"aud"`
		ManifestSHA256       string         `json:"manifest_sha256"`
		ComplianceRedactions map[string]int `json:"compliance_redactions"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}

	result := &VerifyResult{
		KID:         kid,
		Issuer:      claims.Iss,
		Subject:     claims.Sub,
		GeneratedAt: time.Unix(claims.Iat, 0).UTC(),
		ExpiresAt:   time.Unix(claims.Exp, 0).UTC(),
		SignatureOK: true,
		ManifestSHA: claims.ManifestSHA256,
		Audience:    claims.Aud,
		Redactions:  claims.ComplianceRedactions,
		Mode:        "offline",
	}
	if header["alg"] != "EdDSA" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("unexpected alg: %v", header["alg"]))
	}
	if time.Now().After(result.ExpiresAt) {
		result.Expired = true
		if opts.Strict {
			return result, fmt.Errorf("bundle expired at %s", result.ExpiresAt)
		}
		result.Warnings = append(result.Warnings, "bundle is past its exp claim")
	}
	if record.RetiredAt != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("signing key retired at %s (cached PEM used)", record.RetiredAt.Format(time.RFC3339)))
	}
	if opts.Live || opts.Strict {
		result.Mode = "live"
		if opts.KeyClient != nil {
			rows, err := trust.ListKeys(opts.KeyClient, trust.KeyListOptions{OrgAlias: opts.OrgAlias, Live: true})
			if err != nil {
				return result, fmt.Errorf("live key lookup failed: %w", err)
			}
			found := false
			for _, row := range rows {
				if row.KID == kid {
					found = true
					if row.Status == "retired" {
						result.Warnings = append(result.Warnings, "signing key is retired in live key collection")
					}
					break
				}
			}
			if !found {
				return result, fmt.Errorf("live key collection does not include kid %s", kid)
			}
		} else {
			result.Warnings = append(result.Warnings, "live mode requires org auth; using cached key only")
		}
	}
	if opts.Deep || opts.Strict {
		if opts.Strict {
			result.Mode = "strict"
		} else {
			result.Mode = "deep"
		}
		if opts.FileFetcher == nil {
			result.Warnings = append(result.Warnings, "deep mode requires ContentVersion byte re-fetch; no fetcher configured")
		} else if err := VerifyFileAttestations(context.Background(), bundle.Manifest.Files, opts.FileFetcher); err != nil {
			return result, err
		}
	}
	return result, nil
}
