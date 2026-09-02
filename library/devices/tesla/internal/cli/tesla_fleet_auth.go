// tesla auth fleet-* — Tesla Fleet API partner-app credential lifecycle.
//
// Five subcommands:
//   - fleet-setup     prints the developer.tesla.com onboarding walkthrough; no
//     network calls.
//   - fleet-register  client_credentials grant for the partner token, then POST
//     partner_accounts to register the public-key host domain.
//     Saves client_id, client_secret, and public_key_domain.
//   - fleet-login     authorization_code grant via a localhost callback server
//     bound to FIXED port 8585 (OAuth servers enforce
//     exact-match redirect_uri; port fallback is incompatible).
//   - fleet-refresh   refresh_token grant; re-mints the user access token.
//   - fleet-status    presence + JWT-decoded audience + scopes + expiry.
//     NEVER prints token literals or secrets.
//
// OAuth callback pattern ported from ~/snowflake-bypass/fleet-oauth/main.go.
// CSRF state is generated per-invocation, held in memory, bound to the handler
// closure, and compared before accepting the code. See KD2 in 2026-05-22-001.
//
// Hand-coded; lives outside the generator's emit set so it survives regens.
package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const (
	fleetAuthURL      = "https://auth.tesla.com/oauth2/v3/authorize"
	fleetTokenURL     = "https://auth.tesla.com/oauth2/v3/token"
	fleetRedirectURI  = "http://localhost:8585/callback"
	fleetRedirectPort = 8585
	// Full Fleet scope set. openid + offline_access yields a refresh token;
	// vehicle_device_data + vehicle_cmds + vehicle_charging_cmds covers the
	// command surfaces tesla-control exercises.
	fleetScopes = "openid offline_access vehicle_device_data vehicle_cmds vehicle_charging_cmds"
	// Regional Fleet API audience — North America. Other regions ship later;
	// for now this matches the fleet-oauth helper that produced the working
	// token on the reference user.
	fleetAPIAudience = "https://fleet-api.prd.na.vn.cloud.tesla.com"
)

// fleetAPIBase resolves the regional Fleet API root used for partner
// registration, token audience, and (when reads route through Fleet) data and
// command calls. Resolution order: the TESLA_FLEET_API_URL env override, then
// the persisted [fleet].api_base (recorded at register/login time), then the
// North America default. Non-NA owners — whose vehicles live in eu/cn/etc. —
// need the correct regional host, or Tesla returns HTTP 421 (misdirected) for
// the wrong region and HTTP 412 ("must be registered in the current region").
func fleetAPIBase(cfg *config.Config) string {
	if v := strings.TrimSpace(os.Getenv("TESLA_FLEET_API_URL")); v != "" {
		if base := normalizeFleetBase(v); base != "" {
			return base
		}
		fmt.Fprintf(os.Stderr, "warning: ignoring TESLA_FLEET_API_URL=%q (must be https on a tesla.com host)\n", v)
	}
	if cfg != nil && cfg.Fleet.APIBase != "" {
		if base := normalizeFleetBase(cfg.Fleet.APIBase); base != "" {
			return base
		}
		fmt.Fprintf(os.Stderr, "warning: ignoring persisted [fleet].api_base=%q (must be https on a tesla.com host)\n", cfg.Fleet.APIBase)
	}
	return fleetAPIAudience
}

// normalizeFleetBase validates and trims a Fleet API base URL. The Fleet bearer
// is sent to this host, so an arbitrary value — from a hostile creds bundle, a
// typo, or a poisoned env var — would be a token-exfiltration vector. Only
// https:// URLs on a tesla.com host are accepted; returns "" otherwise.
func normalizeFleetBase(raw string) string {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Hostname()
	// Loopback is always allowed: a local mock server (tests) or the local
	// tesla-http-proxy relay can't exfiltrate the bearer off-machine. Still
	// require http/https so an unsupported scheme is caught here instead of
	// failing opaquely at request time with "unsupported protocol scheme".
	if (host == "localhost" || host == "127.0.0.1" || host == "::1") &&
		(u.Scheme == "http" || u.Scheme == "https") {
		return s
	}
	// Any remote host must be https on tesla.com — the bearer's only legit
	// off-machine destination.
	if u.Scheme == "https" && (host == "tesla.com" || strings.HasSuffix(host, ".tesla.com")) {
		return s
	}
	return ""
}

// fleetTokenResponse is the shape every Tesla fleet token endpoint returns
// (client_credentials, authorization_code, refresh_token grants all share this
// envelope; some fields are empty for some grants).
type fleetTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

// newFleetSetupCmd is the read-only walkthrough page. It prints the steps for
// registering an app at developer.tesla.com and the fields the user will need
// to capture for fleet-register. No network calls; no disk writes.
func newFleetSetupCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "fleet-setup",
		Short: "Print the developer.tesla.com app-registration walkthrough",
		Long: `Read-only intro page for the Tesla Fleet API. Walks you through
registering an app at developer.tesla.com so you can later run fleet-register
with the client_id and client_secret you obtain.

This command does not contact Tesla and does not write to disk.`,
		Example: "  tesla-pp-cli auth fleet-setup\n  tesla-pp-cli auth fleet-setup --launch",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"step":        "fleet-setup",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"step":    "fleet-setup",
				}, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Tesla Fleet API partner-app setup")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "  1. Visit https://developer.tesla.com and sign in with your Tesla account.")
			fmt.Fprintln(w, "  2. Create an app. You'll need:")
			fmt.Fprintln(w, "     - A domain that hosts your public key at:")
			fmt.Fprintln(w, "         https://<your-domain>/.well-known/appspecific/com.tesla.3p.public-key.pem")
			fmt.Fprintln(w, "       (run `tesla auth fleet-template` to scaffold a Vercel-ready host)")
			fmt.Fprintln(w, "     - Allowed Origin(s): https://<your-domain>")
			fmt.Fprintln(w, "     - Allowed Redirect URI(s): http://localhost:8585/callback")
			fmt.Fprintln(w, "       (the fleet-login flow binds this exact URI)")
			fmt.Fprintln(w, "     - Scopes: openid offline_access vehicle_device_data vehicle_cmds vehicle_charging_cmds")
			fmt.Fprintln(w, "  3. Capture the client_id and client_secret Tesla shows you.")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Next:")
			fmt.Fprintln(w, "  tesla-pp-cli auth fleet-register \\")
			fmt.Fprintln(w, "    --client-id <id> --client-secret-file <path> \\")
			fmt.Fprintln(w, "    --public-key-domain <your-domain>")
			if launch {
				if err := openBrowser("https://developer.tesla.com"); err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), "(couldn't auto-open browser: %v -- visit the URL manually)\n", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Open developer.tesla.com in the default browser")
	return cmd
}

// newFleetRegisterCmd performs the partner-token client_credentials grant and
// the partner_accounts POST to register the public-key host domain. Stores
// client_id, client_secret, public_key_domain, and private_key_path in the
// [fleet] config block.
//
// Per KD2, the client secret can be supplied via --client-secret-file (mode
// 600 file, preferred) or --client-secret (env: TESLA_FLEET_CLIENT_SECRET; ps-
// visible). If both are given, --client-secret-file wins.
func newFleetRegisterCmd(flags *rootFlags) *cobra.Command {
	var (
		clientID         string
		clientSecret     string
		clientSecretFile string
		publicKeyDomain  string
		keyFile          string
	)
	cmd := &cobra.Command{
		Use:   "fleet-register",
		Short: "Mint a partner token and register your public-key host domain with Tesla",
		Long: `Performs two Tesla calls back-to-back:

  1. POST https://auth.tesla.com/oauth2/v3/token (client_credentials grant)
     -> mints a short-lived partner token
  2. POST .../api/1/partner_accounts {domain}
     -> Tesla fetches your public key at
        https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
     -> 200 means the key is registered

On success, saves client_id, client_secret, public_key_domain, and the signing
private key path (--key-file) to the [fleet] block of your config.toml. The
partner token itself (8h lifespan) is NOT stored; it's only needed for this
registration step. Run fleet-login next to mint the user-bound access token.

If --key-file is not specified, the command looks for a key at the default
location where 'fleet-template --gen-key' writes it (~/.tesla/<host>-private.pem).

Secret handling: pass --client-secret-file (mode 600 file) by preference.
--client-secret takes the secret on the command line, which is visible to
'ps aux' on multi-user systems. The env var TESLA_FLEET_CLIENT_SECRET is
honored when neither flag is set.`,
		Example: `  tesla-pp-cli auth fleet-register --client-id abc --client-secret-file ~/.tesla/cs --public-key-domain keys.example.com
  tesla-pp-cli auth fleet-register --client-id abc --client-secret-file ~/.tesla/cs --public-key-domain keys.example.com --key-file ~/.tesla/tesla-keys-host-private.pem`,
		Annotations: map[string]string{
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"step":        "fleet-register",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":           true,
					"step":              "fleet-register",
					"public_key_domain": publicKeyDomain,
				}, flags)
			}

			effClientID := firstNonEmpty(clientID, os.Getenv("TESLA_FLEET_CLIENT_ID"))
			if effClientID == "" {
				return usageErr(fmt.Errorf("missing --client-id (or env TESLA_FLEET_CLIENT_ID)"))
			}
			effSecret, err := resolveClientSecret(clientSecret, clientSecretFile)
			if err != nil {
				return err
			}
			if effSecret == "" {
				return usageErr(fmt.Errorf("missing --client-secret-file (preferred) or --client-secret (or env TESLA_FLEET_CLIENT_SECRET)"))
			}
			if publicKeyDomain == "" {
				return usageErr(fmt.Errorf("missing --public-key-domain"))
			}

			cfg, err := config.Load(flagsConfigPath(flags))
			if err != nil {
				return configErr(err)
			}

			// Resolve the signing private key path. Order: explicit --key-file,
			// then env TESLA_FLEET_KEY_FILE, then auto-detect from ~/.tesla/.
			effKeyFile, keyErr := resolveFleetKeyFileForRegister(keyFile, publicKeyDomain)
			if keyErr != nil {
				return usageErr(keyErr)
			}

			// Resolve the regional Fleet base once: TESLA_FLEET_API_URL env >
			// persisted [fleet].api_base > North America default. The partner
			// token audience, the partner_accounts endpoint, and the persisted
			// api_base must all agree on the region, or Tesla returns HTTP 412
			// ("must be registered in the current region").
			fleetBase := fleetAPIBase(cfg)
			partnerTokenURL := fleetTokenURL
			partnerAccountsURL := fleetBase + "/api/1/partner_accounts"
			if base := os.Getenv("TESLA_FLEET_AUTH_URL"); base != "" {
				partnerTokenURL = base + "/oauth2/v3/token"
			}

			// Step 1: client_credentials grant -> partner token (8h).
			partnerTok, _, err := fleetClientCredentialsGrant(partnerTokenURL, effClientID, effSecret, fleetBase)
			if err != nil {
				return apiErr(err)
			}
			// Step 2: register the public-key host domain.
			if err := fleetRegisterPartnerAccount(partnerAccountsURL, partnerTok, publicKeyDomain); err != nil {
				return apiErr(err)
			}

			// Persist client_id + secret + domain + key path + resolved region into [fleet].
			// Always overwrite PrivateKeyPath, including empty: SaveFleetTokens
			// skips blank fields, so a re-register with no resolved key would
			// otherwise keep the previous domain's signing path.
			cfg.Fleet.APIBase = fleetBase
			cfg.Fleet.PrivateKeyPath = effKeyFile
			if err := cfg.SaveFleetTokens(effClientID, effSecret, "", "", time.Time{}, publicKeyDomain, effKeyFile); err != nil {
				return err
			}

			result := map[string]any{
				"status":            "registered",
				"public_key_domain": publicKeyDomain,
				"storage_path":      cfg.Path,
				"next":              "Run `tesla auth fleet-login` to mint your user access token.",
			}
			if effKeyFile != "" {
				result["key_file"] = effKeyFile
			} else {
				result["key_file_warning"] = "No signing key found. Run `tesla auth fleet-template --gen-key` and re-run fleet-register with --key-file, or set TESLA_FLEET_KEY_FILE."
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "Fleet API client_id from developer.tesla.com (env: TESLA_FLEET_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Fleet API client_secret (PS-visible; prefer --client-secret-file)")
	cmd.Flags().StringVar(&clientSecretFile, "client-secret-file", "", "Path to a mode-600 file containing the client_secret (preferred over --client-secret)")
	cmd.Flags().StringVar(&publicKeyDomain, "public-key-domain", "", "Domain hosting your public key at .well-known/appspecific/com.tesla.3p.public-key.pem")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to the signing private key (default: auto-detect from ~/.tesla/ based on fleet-template output)")
	return cmd
}

const teslaWellKnownPublicKeyPath = "/.well-known/appspecific/com.tesla.3p.public-key.pem"

// fetchWellKnownPublicKey downloads the Tesla well-known partner public key
// for a domain. Tests replace this to avoid live HTTPS.
var fetchWellKnownPublicKey = fetchWellKnownPublicKeyHTTPS

func teslaWellKnownPublicKeyURL(domain string) string {
	return "https://" + strings.TrimSpace(domain) + teslaWellKnownPublicKeyPath
}

func fetchWellKnownPublicKeyHTTPS(domain string) ([]byte, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, errors.New("empty public-key domain")
	}
	if strings.ContainsAny(domain, "/\\") || strings.Contains(domain, "://") {
		return nil, fmt.Errorf("invalid public-key domain %q", domain)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(teslaWellKnownPublicKeyURL(domain))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	pub := parsePublicKeyPEM(data)
	if pub == nil {
		return nil, errors.New("response is not a PEM public key")
	}
	return pub, nil
}

// resolveRegisterPublicKeyMaterial returns PKIX public-key bytes for the
// domain being registered. Only the hosted well-known PEM is used; a local
// ~/.tesla/<domain>-public.pem is not a fallback (it may be stale).
func resolveRegisterPublicKeyMaterial(teslaDir, publicKeyDomain string) ([]byte, string, error) {
	domain := strings.TrimSpace(publicKeyDomain)
	if domain == "" {
		return nil, "", nil
	}
	hostedURL := teslaWellKnownPublicKeyURL(domain)
	b, fetchErr := fetchWellKnownPublicKey(domain)
	if fetchErr == nil && len(b) > 0 {
		return b, hostedURL, nil
	}
	if fetchErr == nil {
		fetchErr = errors.New("fetched public key is empty")
	}
	return nil, "", fmt.Errorf("cannot fetch public key from %s: %w; specify --key-file", hostedURL, fetchErr)
}

// resolveFleetKeyFileForRegister resolves the signing private key path for
// fleet-register. Order: explicit flag, env TESLA_FLEET_KEY_FILE, auto-detect
// from ~/.tesla/ based on common fleet-template output patterns.
//
// Returns ("", nil) when no key is found (caller warns but does not fail).
// Returns an error when an explicit path is provided but not a valid private
// key PEM, when a domain public key is available and no candidate matches it,
// or when multiple auto-detected candidates cannot be uniquely matched.
func resolveFleetKeyFileForRegister(flagValue, publicKeyDomain string) (string, error) {
	// Explicit flag wins — must be a valid private key PEM or we fail.
	if flagValue != "" {
		abs, err := filepath.Abs(flagValue)
		if err != nil {
			return "", fmt.Errorf("--key-file %q: %w", flagValue, err)
		}
		if err := validatePrivateKeyPEM(abs); err != nil {
			return "", fmt.Errorf("--key-file: %w", err)
		}
		return abs, nil
	}
	// Env override — must be a valid private key PEM or we fail.
	if v := strings.TrimSpace(os.Getenv("TESLA_FLEET_KEY_FILE")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("TESLA_FLEET_KEY_FILE=%q: %w", v, err)
		}
		if err := validatePrivateKeyPEM(abs); err != nil {
			return "", fmt.Errorf("TESLA_FLEET_KEY_FILE: %w", err)
		}
		return abs, nil
	}
	// Auto-detect from ~/.tesla/. fleet-template --gen-key writes to
	// ~/.tesla/<dest-basename>-private.pem.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", nil
	}
	teslaDir := filepath.Join(home, ".tesla")

	candidates := scanValidPrivateKeys(teslaDir)
	if len(candidates) == 0 {
		return "", nil
	}

	matched, err := matchCandidatesToDomain(teslaDir, candidates, publicKeyDomain)
	if err != nil {
		return "", err
	}
	if matched != "" {
		return matched, nil
	}
	// No hosted public key to bind against (empty domain). A lone candidate
	// is the only signing key on disk — identity matching cannot run without
	// target material. --key-file remains the explicit override.
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", errMultipleCandidates(teslaDir, candidates, "Specify --key-file <path> to select one.")
}

// newFleetLoginCmd runs the authorization_code grant via a localhost callback
// server bound to FIXED port 8585. OAuth servers enforce exact-match
// redirect_uri, so port fallback is incompatible with Tesla's partner-app
// registration. If 8585 is in use we surface a clear error rather than fall
// through silently.
//
// CSRF state is generated per-invocation, held in memory only, bound to the
// handler closure, and compared before accepting the code. See
// ~/snowflake-bypass/fleet-oauth/main.go for the working pattern.
func newFleetLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		noOpen          bool
		vehicleLocation bool
		audience        string
		clientIDArg     string
	)
	cmd := &cobra.Command{
		Use:   "fleet-login",
		Short: "OAuth-login to Tesla Fleet API (browser flow on localhost:8585)",
		Long: `Opens your browser at Tesla's OAuth authorize URL, catches the redirect on
http://localhost:8585/callback, exchanges the code for access + refresh tokens,
and persists them to the [fleet] block of config.toml.

Port 8585 is fixed; Tesla enforces exact-match redirect_uri. If 8585 is in
use, free it before retrying (or update your partner app's registered redirect
URI to a different port — but the CLI default expects 8585).`,
		Example: "  tesla-pp-cli auth fleet-login",
		Annotations: map[string]string{
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"step":        "fleet-login",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"step":    "fleet-login",
				}, flags)
			}
			cfg, err := config.Load(flagsConfigPath(flags))
			if err != nil {
				return configErr(err)
			}
			effClientID := firstNonEmpty(clientIDArg, os.Getenv("TESLA_FLEET_CLIENT_ID"), cfg.Fleet.ClientID)
			if effClientID == "" {
				return usageErr(fmt.Errorf("no Fleet client_id available; run `tesla auth fleet-register` first or pass --client-id"))
			}
			effAudience := firstNonEmpty(audience, fleetAPIBase(cfg))
			effAuthURL := fleetAuthURL
			effTokenURL := fleetTokenURL
			if base := os.Getenv("TESLA_FLEET_AUTH_URL"); base != "" {
				effAuthURL = base + "/oauth2/v3/authorize"
				effTokenURL = base + "/oauth2/v3/token"
			}
			effScope := fleetScopes
			if vehicleLocation {
				effScope += " vehicle_location"
			}
			tok, err := runFleetLoginFlow(cmd, cfg, effClientID, effAuthURL, effTokenURL, effAudience, effScope, !noOpen)
			if err != nil {
				return err
			}
			expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
			// Sticky region: set the audience we logged in against on the
			// in-memory struct BEFORE the single SaveFleetTokens write, so the
			// tokens and api_base are persisted atomically. Two separate saves
			// would leave api_base unwritten if the process died between them,
			// silently reverting later reads to North America (the bug this
			// whole change fixes). Mirrors fleet-register.
			cfg.Fleet.APIBase = effAudience
			if err := cfg.SaveFleetTokens("", "", tok.AccessToken, tok.RefreshToken, expiresAt, "", ""); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":       "logged_in",
				"expires_at":   expiresAt.Format(time.RFC3339),
				"expires_in":   tok.ExpiresIn,
				"scope":        tok.Scope,
				"storage_path": cfg.Path,
				"hint":         "Run `tesla auth fleet-status` to verify; `tesla auth fleet-refresh` re-mints the access token.",
			}, flags)
		},
	}
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the auth URL but don't auto-open the browser")
	cmd.Flags().BoolVar(&vehicleLocation, "vehicle-location", false, "Also request the vehicle_location scope (GPS) — the app must be registered for it at developer.tesla.com")
	cmd.Flags().StringVar(&audience, "audience", "", "Override the token audience (default: regional Fleet API)")
	cmd.Flags().StringVar(&clientIDArg, "client-id", "", "Client ID override (default: stored value from fleet-register, env: TESLA_FLEET_CLIENT_ID)")
	return cmd
}

// newFleetRefreshCmd re-mints the Fleet user access token using the stored
// refresh token. Idempotent. Surfaces "refresh token expired" cleanly when
// Tesla returns 401.
func newFleetRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet-refresh",
		Short: "Re-mint the Fleet API access token from the stored refresh token",
		Long: `Reads the Fleet refresh token from the [fleet] block of config.toml,
exchanges it for a fresh access token, and writes both back. Run after a 401
on a Fleet API call, or whenever you want to confirm refresh works.`,
		Example: "  tesla-pp-cli auth fleet-refresh --json",
		Annotations: map[string]string{
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"step":        "fleet-refresh",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"step":    "fleet-refresh",
				}, flags)
			}
			cfg, err := config.Load(flagsConfigPath(flags))
			if err != nil {
				return configErr(err)
			}
			ft := cfg.FleetTokens()
			if ft.RefreshToken == "" {
				return fmt.Errorf("no Fleet refresh token stored; run `tesla auth fleet-login` first")
			}
			effClientID := firstNonEmpty(os.Getenv("TESLA_FLEET_CLIENT_ID"), ft.ClientID)
			if effClientID == "" {
				return fmt.Errorf("no Fleet client_id stored; run `tesla auth fleet-register` first")
			}
			effTokenURL := fleetTokenURL
			if base := os.Getenv("TESLA_FLEET_AUTH_URL"); base != "" {
				effTokenURL = base + "/oauth2/v3/token"
			}
			_, curScope, _ := decodeJWTClaims(ft.AccessToken)
			tok, err := fleetRefreshGrant(effTokenURL, effClientID, ft.RefreshToken, curScope)
			if err != nil {
				// Tesla's refresh-token-expired response is a 401 with
				// invalid_grant. Surface a friendly hint to re-run fleet-login.
				msg := err.Error()
				if strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "401") {
					return fmt.Errorf("refresh token expired or invalid; run `tesla auth fleet-login` again")
				}
				return err
			}
			expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
			finalRefresh := tok.RefreshToken
			if strings.TrimSpace(finalRefresh) == "" {
				finalRefresh = ft.RefreshToken // Tesla sometimes omits a fresh one; keep the old.
			}
			if err := cfg.SaveFleetTokens("", "", tok.AccessToken, finalRefresh, expiresAt, "", ""); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":       "refreshed",
				"expires_at":   expiresAt.Format(time.RFC3339),
				"expires_in":   tok.ExpiresIn,
				"storage_path": cfg.Path,
			}, flags)
		},
	}
	return cmd
}

// newFleetStatusCmd reports presence, JWT-decoded audience, scopes, and
// expiry. CRITICAL: it MUST NOT print token literals or client_secret. A
// regression test asserts no secret leak.
func newFleetStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet-status",
		Short: "Show Fleet API auth status (presence, audience, scopes, expiry; never prints secrets)",
		Long: `Reports whether Fleet API credentials are present and unexpired. Decodes
the JWT payload to surface audience and scopes. NEVER prints the access token,
refresh token, or client_secret literal.`,
		Example: "  tesla-pp-cli auth fleet-status --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"step":        "fleet-status",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"step":    "fleet-status",
				}, flags)
			}
			cfg, err := config.Load(flagsConfigPath(flags))
			if err != nil {
				return configErr(err)
			}
			ft := cfg.FleetTokens()
			envOverride := os.Getenv("TESLA_FLEET_TOKEN")
			effectiveTok := firstNonEmpty(envOverride, ft.AccessToken)
			status := map[string]any{
				"access_token_present":  effectiveTok != "",
				"refresh_token_present": ft.RefreshToken != "",
				"client_id_present":     ft.ClientID != "",
				"client_secret_present": ft.ClientSecret != "",
				"public_key_domain":     ft.PublicKeyDomain,
				"source":                fleetStatusSource(envOverride, ft),
			}
			if !ft.TokenExpiry.IsZero() {
				status["expires_at"] = ft.TokenExpiry.Format(time.RFC3339)
				status["expired"] = time.Now().After(ft.TokenExpiry)
			}
			if effectiveTok != "" {
				if aud, scopes, claimErr := decodeJWTClaims(effectiveTok); claimErr == nil {
					if len(aud) > 0 {
						status["audience"] = aud
					}
					if scopes != "" {
						status["scopes"] = scopes
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), status, flags)
		},
	}
	return cmd
}

// runFleetLoginFlow binds a localhost callback server to the fixed
// fleetRedirectPort, generates a CSRF state, opens the browser, and waits for
// the redirect. The handler closure compares state before accepting any code.
//
// Patterned after ~/snowflake-bypass/fleet-oauth/main.go.
func runFleetLoginFlow(cmd *cobra.Command, cfg *config.Config, clientID, authURL, tokenURL, audience, scope string, openBrowserFlag bool) (*fleetTokenResponse, error) {
	if scope == "" {
		scope = fleetScopes
	}
	// Pre-flight: refuse to bind if 8585 is already in use. We surface a
	// clear error rather than try a different port because Tesla enforces
	// exact-match redirect_uri.
	addr := fmt.Sprintf("127.0.0.1:%d", fleetRedirectPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("port %d in use: %w. Free it (lsof -i :%d) or update your partner app's registered redirect URI", fleetRedirectPort, err, fleetRedirectPort)
	}

	state, err := randomURLSafe(16)
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("generate state: %w", err)
	}

	q := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {fleetRedirectURI},
		"scope":         {scope},
		"state":         {state},
		// "login consent" forces both re-authentication AND a fresh consent
		// screen. Without consent, Tesla silently grants only the user's
		// previously-consented scopes and ignores newly-requested ones (e.g.
		// vehicle_location), so a scope added later never reaches the token.
		"prompt": {"login consent"},
	}
	loginURL := authURL + "?" + q.Encode()

	type callbackResult struct {
		code string
		err  error
	}
	done := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		qs := r.URL.Query()
		gotState := qs.Get("state")
		gotCode := qs.Get("code")
		gotErr := qs.Get("error")

		if gotErr != "" {
			fmt.Fprintf(w, "Auth error: %s\n%s\nClose this tab and check the terminal.\n", gotErr, qs.Get("error_description"))
			done <- callbackResult{"", fmt.Errorf("tesla returned error: %s (%s)", gotErr, qs.Get("error_description"))}
			return
		}
		// CSRF state check — closure-bound to the per-invocation `state`.
		if gotState != state {
			fmt.Fprintln(w, "State mismatch. Possible CSRF. Close this tab.")
			done <- callbackResult{"", fmt.Errorf("CSRF state mismatch: got %q want %q", gotState, state)}
			return
		}
		if gotCode == "" {
			fmt.Fprintln(w, "No code in callback. Close this tab.")
			done <- callbackResult{"", fmt.Errorf("no code in callback")}
			return
		}
		fmt.Fprintln(w, "Code received. You can close this tab.")
		done <- callbackResult{gotCode, nil}
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			done <- callbackResult{"", serveErr}
		}
	}()

	w := cmd.OutOrStderr()
	fmt.Fprintln(w, "Tesla Fleet OAuth login URL:")
	fmt.Fprintln(w, loginURL)
	fmt.Fprintln(w, "")
	if openBrowserFlag {
		fmt.Fprintln(w, "Opening browser. If it doesn't open, copy the URL above.")
		_ = openBrowser(loginURL)
	}
	fmt.Fprintf(w, "Waiting for callback on %s ...\n", fleetRedirectURI)

	res := <-done
	_ = srv.Close()
	if res.err != nil {
		return nil, res.err
	}

	// Exchange the code for tokens.
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {clientID},
		"code":         {res.code},
		"audience":     {audience},
		"redirect_uri": {fleetRedirectURI},
	}
	// The reference helper also sends client_secret. Tesla's authorization_code
	// grant for confidential clients requires it; use the cfg the caller already
	// loaded with its --config path (do NOT re-load with config.Load("") — that
	// silently ignores any --config override and resolves the default path).
	if cfg != nil && cfg.Fleet.ClientSecret != "" {
		form.Set("client_secret", cfg.Fleet.ClientSecret)
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok fleetTokenResponse
	if jerr := json.Unmarshal(body, &tok); jerr != nil {
		return nil, fmt.Errorf("parse token response: %w", jerr)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	return &tok, nil
}

// fleetClientCredentialsGrant mints the partner token (8h lifespan). Used by
// fleet-register; not stored.
func fleetClientCredentialsGrant(tokenURL, clientID, clientSecret, audience string) (string, *fleetTokenResponse, error) {
	if audience == "" {
		audience = fleetAPIAudience
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {fleetScopes},
		"audience":      {audience},
	}
	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return "", nil, fmt.Errorf("client_credentials grant: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 {
		return "", nil, fmt.Errorf("client_credentials grant 401 invalid_credentials: %s", strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("client_credentials grant http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok fleetTokenResponse
	if jerr := json.Unmarshal(body, &tok); jerr != nil {
		return "", nil, fmt.Errorf("parse partner token response: %w", jerr)
	}
	if tok.AccessToken == "" {
		return "", nil, fmt.Errorf("partner token response missing access_token")
	}
	return tok.AccessToken, &tok, nil
}

// fleetRegisterPartnerAccount POSTs the domain to partner_accounts. Tesla then
// scans <domain>/.well-known/appspecific/com.tesla.3p.public-key.pem.
func fleetRegisterPartnerAccount(partnerAccountsURL, partnerToken, domain string) error {
	body := map[string]string{"domain": domain}
	bodyJSON, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", partnerAccountsURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return fmt.Errorf("build partner_accounts request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+partnerToken)
	req.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("partner_accounts POST: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("partner_accounts http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// fleetRefreshGrant re-mints the access token via refresh_token grant.
func fleetRefreshGrant(tokenURL, clientID, refreshToken, scope string) (*fleetTokenResponse, error) {
	// Re-request the scopes the current token already carries so a refresh
	// never silently narrows the grant (e.g. dropping vehicle_location).
	if scope == "" {
		scope = fleetScopes
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"scope":         {scope},
	}
	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("refresh_token grant: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("refresh_token grant http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok fleetTokenResponse
	if jerr := json.Unmarshal(body, &tok); jerr != nil {
		return nil, fmt.Errorf("parse refresh response: %w", jerr)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access_token")
	}
	return &tok, nil
}

// decodeJWTClaims pulls audience and scope from the middle segment of a JWT.
// Returns ([]string{}, "", err) for unparseable input. Best-effort: this is
// for surfacing fleet-status, not for verification.
func decodeJWTClaims(tok string) ([]string, string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("not a JWT shape (%d segments)", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Tesla pads sometimes; try standard URL encoding.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, "", fmt.Errorf("decode payload: %w", err)
		}
	}
	// `aud` may be a string or a []string; handle both.
	var raw map[string]any
	if jerr := json.Unmarshal(payload, &raw); jerr != nil {
		return nil, "", fmt.Errorf("parse claims: %w", jerr)
	}
	var aud []string
	switch v := raw["aud"].(type) {
	case string:
		aud = []string{v}
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				aud = append(aud, s)
			}
		}
	}
	scope, _ := raw["scope"].(string)
	if scope == "" {
		// Tesla returns the granted scopes under `scp`, and as a JSON array
		// (not a space-delimited string), so handle both shapes.
		switch v := raw["scp"].(type) {
		case string:
			scope = v
		case []any:
			parts := make([]string, 0, len(v))
			for _, e := range v {
				if s, ok := e.(string); ok {
					parts = append(parts, s)
				}
			}
			scope = strings.Join(parts, " ")
		}
	}
	return aud, scope, nil
}

// resolveClientSecret applies the secret-source precedence: --client-secret-file
// (preferred, mode 600 file), then --client-secret, then env
// TESLA_FLEET_CLIENT_SECRET. Returns "" with no error when no source is set so
// the caller can emit a usage error with the right wording.
func resolveClientSecret(flag, filePath string) (string, error) {
	if filePath != "" {
		info, err := os.Stat(filePath)
		if err != nil {
			return "", usageErr(fmt.Errorf("client-secret-file %s: %w", filePath, err))
		}
		// Mode 600 (or 400) only. The file should not be group/world readable
		// because it carries a long-lived partner-app secret. Skip the check
		// on Windows (mode bits are advisory there).
		mode := info.Mode().Perm()
		if mode&0o077 != 0 {
			return "", usageErr(fmt.Errorf("client-secret-file %s has mode %o (group/world readable); chmod 600 it first", filePath, mode))
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", usageErr(fmt.Errorf("read client-secret-file %s: %w", filePath, err))
		}
		return strings.TrimSpace(string(data)), nil
	}
	if flag != "" {
		return flag, nil
	}
	return os.Getenv("TESLA_FLEET_CLIENT_SECRET"), nil
}

// teslaFleetHasLocationScope reports whether the active Fleet token (env
// override or [fleet] block) carries the vehicle_location scope, decoded from
// the JWT. drive_state reads only request the location_data endpoint when this
// is true, since Tesla 403s the whole call if it's requested without the scope.
func teslaFleetHasLocationScope(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	tok := firstNonEmpty(os.Getenv("TESLA_FLEET_TOKEN"), cfg.Fleet.AccessToken)
	_, scopes, err := decodeJWTClaims(tok)
	return err == nil && strings.Contains(scopes, "vehicle_location")
}

// firstNonEmpty returns the first non-empty string in vals, or "" if all empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// randomURLSafe returns a url-safe random string of n bytes (encoded length
// will be larger). Used for the OAuth state parameter.
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "="), nil
}

// fleetStatusSource reports where fleet-status read its access token from.
// "env" wins over "config" when TESLA_FLEET_TOKEN is set, "none" when neither
// is set. Never includes the token literal.
func fleetStatusSource(envTok string, ft config.FleetConfig) string {
	if envTok != "" {
		return "env:TESLA_FLEET_TOKEN"
	}
	if ft.AccessToken != "" {
		return "config"
	}
	return "none"
}
