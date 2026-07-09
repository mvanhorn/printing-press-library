// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: auth service-account (domain-wide delegation).
// Mints a Google access token from a service-account key by impersonating an
// admin (or target user) and prints it for use as GOOGLE_WORKSPACE_TOKEN.

package cli

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

// defaultSAScopes is a read-mostly admin scope set covering the audit commands.
// Offboard and other mutating commands need write scopes; override with --scopes.
var defaultSAScopes = []string{
	"https://www.googleapis.com/auth/admin.directory.user.readonly",
	"https://www.googleapis.com/auth/admin.directory.user.security",
	"https://www.googleapis.com/auth/admin.directory.group.readonly",
	"https://www.googleapis.com/auth/admin.directory.orgunit.readonly",
	"https://www.googleapis.com/auth/admin.reports.audit.readonly",
	"https://www.googleapis.com/auth/apps.alerts",
	"https://www.googleapis.com/auth/drive.readonly",
	"https://www.googleapis.com/auth/gmail.readonly",
}

type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func newAuthServiceAccountCmd(flags *rootFlags) *cobra.Command {
	var flagKey string
	var flagImpersonate string
	var flagScopes string

	cmd := &cobra.Command{
		Use:   "service-account",
		Short: "Mint an access token from a service-account key via domain-wide delegation.",
		Long: "Mint a Google access token from a service-account JSON key by impersonating an admin (or target\n" +
			"user) through domain-wide delegation, and print it for use as GOOGLE_WORKSPACE_TOKEN.\n\n" +
			"Authorize the service account's scopes once in Admin Console > Security > API Controls >\n" +
			"Domain-wide Delegation. Use --impersonate <admin> for Directory/Reports/Alert Center, or\n" +
			"--impersonate <user> to reach that user's Drive/Gmail data.",
		Example:     "  workspace-admin-pp-cli auth service-account --key sa.json --impersonate admin@yourdomain.com",
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would mint a service-account access token via domain-wide delegation")
				return nil
			}
			if flagKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--key (path to service-account JSON) is required"))
			}
			if flagImpersonate == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--impersonate (admin or user email to act as) is required"))
			}
			sa, tokenURI, err := loadSAKey(flagKey)
			if err != nil {
				return err
			}
			scopes := defaultSAScopes
			if strings.TrimSpace(flagScopes) != "" {
				scopes = splitCSV(flagScopes)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			token, err := mintServiceAccountToken(ctx, sa, scopes, flagImpersonate, tokenURI, time.Now())
			if err != nil {
				return authErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "export GOOGLE_WORKSPACE_TOKEN=%s\n", token)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKey, "key", "", "Path to the service-account JSON key file")
	cmd.Flags().StringVar(&flagImpersonate, "impersonate", "", "Admin or user email to impersonate (domain-wide delegation subject)")
	cmd.Flags().StringVar(&flagScopes, "scopes", "", "Comma-separated OAuth scopes (default: read-mostly admin set)")
	return cmd
}

// loadSAKey reads and parses a service-account JSON key file, returning the
// parsed key and the token endpoint (defaulting to Google's public OAuth2 URL
// when the key omits token_uri). Shared by the auth service-account command and
// the domain-wide email-exposure sweep.
func loadSAKey(path string) (serviceAccountKey, string, error) {
	if path == "" {
		return serviceAccountKey{}, "", usageErr(fmt.Errorf("--key (path to service-account JSON) is required"))
	}
	// #nosec G304 -- path is the user-supplied location of their own service-account JSON key (the documented purpose of --key)
	raw, err := os.ReadFile(path)
	if err != nil {
		return serviceAccountKey{}, "", configErr(fmt.Errorf("reading service-account key: %w", err))
	}
	var sa serviceAccountKey
	if err := json.Unmarshal(raw, &sa); err != nil {
		return serviceAccountKey{}, "", configErr(fmt.Errorf("parsing service-account key: %w", err))
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return serviceAccountKey{}, "", configErr(fmt.Errorf("service-account key missing client_email or private_key"))
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		// #nosec G101 -- public Google OAuth2 token endpoint URL, not a credential
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	return sa, tokenURI, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// b64url encodes without padding, per JWT.
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// jwtClaimsJSON builds the JWT claim set for a service-account assertion. Pure
// and unit-testable.
func jwtClaimsJSON(iss, scope, aud, sub string, iat, exp int64) ([]byte, error) {
	return json.Marshal(map[string]any{
		"iss":   iss,
		"scope": scope,
		"aud":   aud,
		"sub":   sub,
		"iat":   iat,
		"exp":   exp,
	})
}

// signedJWTAssertion builds and RS256-signs the assertion JWT.
func signedJWTAssertion(sa serviceAccountKey, scopes []string, subject, aud string, now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := jwtClaimsJSON(sa.ClientEmail, strings.Join(scopes, " "), aud, subject, now.Unix(), now.Add(time.Hour).Unix())
	if err != nil {
		return "", err
	}
	signingInput := b64url(header) + "." + b64url(claims)

	key, err := parseRSAPrivateKey(sa.PrivateKey)
	if err != nil {
		return "", err
	}
	hashed := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("signing assertion: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM private key")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	rsaKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA")
	}
	return rsaKey, nil
}

// mintServiceAccountToken performs the JWT-bearer grant exchange.
func mintServiceAccountToken(ctx context.Context, sa serviceAccountKey, scopes []string, subject, tokenURI string, now time.Time) (string, error) {
	assertion, err := signedJWTAssertion(sa, scopes, subject, tokenURI, now)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("no access_token returned (%s: %s)", tok.Error, tok.ErrorDesc)
	}
	return tok.AccessToken, nil
}
