package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"
)

// phGraphQLProbe runs an authenticated `viewer` GraphQL ping against Product
// Hunt and reports an auth-stage-aware verdict so the user knows what to do
// next. Returns a map suitable for inclusion in the doctor `report`.
func phGraphQLProbe(ctx context.Context, cfg *config.Config) map[string]any {
	out := map[string]any{}
	if cfg == nil {
		out["status"] = "no config"
		return out
	}
	auth := cfg.AuthHeader()
	hasOAuth := cfg.ClientID != "" && cfg.ClientSecret != ""
	if auth == "" && !hasOAuth {
		out["status"] = "no token"
		out["next_step"] = "run `producthunt-pp-cli auth onboard` (developer-token flow) or set PRODUCT_HUNT_TOKEN"
		return out
	}
	c := phgql.New(cfg)
	var resp phgql.ViewerResponse
	rate, err := c.Query(ctx, phgql.ViewerQuery, nil, &resp)
	if err != nil {
		if pe, ok := err.(*phgql.Error); ok {
			out["status"] = pe.Code
			out["message"] = pe.Message
			out["next_step"] = nextStepForCode(pe.Code)
			return out
		}
		out["status"] = "error"
		out["message"] = err.Error()
		return out
	}
	if resp.Viewer.User == nil {
		out["status"] = "valid (oauth_client_credentials, no viewer scope)"
		out["next_step"] = "switch to a developer token to unlock `whoami` (https://www.producthunt.com/v2/oauth/applications)"
	} else {
		out["status"] = fmt.Sprintf("valid as @%s (%s)", resp.Viewer.User.Username, resp.Viewer.User.Name)
	}
	out["rate_limit"] = rate
	out["budget_remaining"] = rate.Remaining
	out["budget_limit"] = rate.Limit
	out["budget_reset_seconds"] = rate.ResetSecs
	return out
}

func nextStepForCode(code string) string {
	switch code {
	case "invalid_oauth_token":
		return "regenerate your developer token at https://www.producthunt.com/v2/oauth/applications and run `auth set-token <new>`"
	case "RATE_LIMITED":
		return "wait for the X-Rate-Limit-Reset window to elapse and try again; consider --budget-aware on heavy commands"
	case "GRAPHQL_ERROR":
		return "the credentials are valid but the query was rejected (check field names / variable types)"
	case "TRANSPORT":
		return "network failure — check your connection or try again"
	default:
		return "run `producthunt-pp-cli auth status` and double-check your config"
	}
}
