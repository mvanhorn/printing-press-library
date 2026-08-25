package cli

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/layers/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/layers/internal/config"
)

type accountTokenClaims struct {
	JTI string `json:"jti"`
	Sub string `json:"sub"`
}

type portalSessionClaims struct {
	Kind        string         `json:"kind"`
	UserID      string         `json:"userId"`
	Community   string         `json:"community"`
	PortalAlias string         `json:"portalAlias"`
	Params      map[string]any `json:"params"`
	IssuedAt    int64          `json:"iat"`
	Issuer      string         `json:"iss"`
	JTI         string         `json:"jti"`
	Subject     string         `json:"sub"`
}

func mintPortalSession(accountToken, userID, community, portalAlias string, now time.Time) (string, error) {
	parts := strings.Split(accountToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("Layers account token is not a three-part JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decoding Layers account token: %w", err)
	}
	var account accountTokenClaims
	if err := json.Unmarshal(payload, &account); err != nil {
		return "", fmt.Errorf("parsing Layers account token: %w", err)
	}
	if account.JTI == "" || account.Sub == "" {
		return "", fmt.Errorf("Layers account token is missing jti or sub claims")
	}
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(community) == "" {
		return "", fmt.Errorf("portal session requires user ID and community")
	}

	header, _ := json.Marshal(map[string]string{"alg": "HS256"})
	claims, _ := json.Marshal(portalSessionClaims{
		Kind:        "session:portal",
		UserID:      userID,
		Community:   community,
		PortalAlias: portalAlias,
		Params:      map[string]any{},
		IssuedAt:    now.Unix(),
		Issuer:      "https://id.layers.digital/",
		JTI:         account.JTI,
		Subject:     account.Sub,
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(parts[2]))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func rawBearerToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.LayersToken != "" {
		return cfg.LayersToken
	}
	return strings.TrimSpace(strings.TrimPrefix(cfg.AuthHeader(), "Bearer "))
}

func resolvePortalScope(ctx context.Context, c *client.Client, community, userID string, dryRun bool) (string, string, error) {
	if community == "" && c != nil && c.Config != nil {
		community = strings.TrimSpace(c.Config.Headers["community-id"])
	}
	if dryRun {
		if community == "" {
			community = "<community-id>"
		}
		if userID == "" {
			userID = "<user-id>"
		}
		return community, userID, nil
	}
	if community == "" {
		return "", "", fmt.Errorf("community is required; set LAYERS_COMMUNITY_ID or pass --community")
	}
	if userID != "" {
		return community, userID, nil
	}
	data, err := c.Get(ctx, "/v1/context", map[string]string{"platform": "web", "version": "0.1.0"})
	if err != nil {
		return "", "", fmt.Errorf("resolving portal user ID: %w", err)
	}
	var result struct {
		UserID string `json:"userId"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", fmt.Errorf("parsing portal context: %w", err)
	}
	if result.UserID == "" {
		return "", "", fmt.Errorf("portal context did not return userId")
	}
	return community, result.UserID, nil
}

func (f *rootFlags) newPortalAPIClient(ctx context.Context, portalAlias, community, userID string) (*client.Client, string, string, error) {
	accountClient, err := f.newClient()
	if err != nil {
		return nil, "", "", err
	}
	community, userID, err = resolvePortalScope(ctx, accountClient, community, userID, f.dryRun)
	if err != nil {
		return nil, "", "", err
	}
	// A dry run never sends Authorization. Keep the account client so
	// documentation validation also works with Printing Press placeholders
	// that intentionally are not JWT-shaped credentials.
	if f.dryRun {
		return accountClient, community, userID, nil
	}
	session, err := mintPortalSession(rawBearerToken(accountClient.Config), userID, community, portalAlias, time.Now())
	if err != nil {
		return nil, "", "", fmt.Errorf("deriving in-memory portal session: %w", err)
	}
	portalConfig := *accountClient.Config
	portalConfig.AuthHeaderVal = ""
	portalConfig.AccessToken = ""
	portalConfig.RefreshToken = ""
	portalConfig.LayersToken = session
	portalConfig.AuthSource = "derived portal session"
	portalConfig.CredentialSource = "process memory"
	portalClient := client.New(&portalConfig, f.timeout, f.rateLimit)
	portalClient.DryRun = f.dryRun
	portalClient.NoCache = f.noCache
	return portalClient, community, userID, nil
}
