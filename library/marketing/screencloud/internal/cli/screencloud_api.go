// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/screencloud/internal/config"
)

func isHTTPStatus(err error, status int) bool {
	var apiError *client.APIError
	return errors.As(err, &apiError) && apiError.StatusCode == status
}

const defaultPlaygroundsBaseURL = "https://api-playgrounds.screencloudapps.com"

type graphQLErrorItem struct {
	Message string `json:"message"`
}

type graphQLEnvelope struct {
	Data   json.RawMessage    `json:"data"`
	Errors []graphQLErrorItem `json:"errors"`
	Meta   map[string]any     `json:"meta"`
}

func runGraphQL(ctx context.Context, flags *rootFlags, query string, variables map[string]any) (json.RawMessage, map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{"query": query}
	if len(variables) > 0 {
		body["variables"] = variables
	}
	raw, _, err := c.Post(ctx, "/graphql", body)
	if err != nil {
		return nil, nil, classifyAPIError(err, flags)
	}
	var envelope graphQLEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decoding GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			if strings.TrimSpace(item.Message) != "" {
				messages = append(messages, item.Message)
			}
		}
		if len(messages) == 0 {
			messages = append(messages, "ScreenCloud returned an unspecified GraphQL error")
		}
		return nil, envelope.Meta, apiErr(fmt.Errorf("GraphQL request failed: %s", strings.Join(messages, "; ")))
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, envelope.Meta, apiErr(fmt.Errorf("GraphQL response did not include data"))
	}
	return envelope.Data, envelope.Meta, nil
}

func graphqlDocumentHasMutation(document string) bool {
	for i := 0; i < len(document); {
		switch document[i] {
		case '#':
			for i < len(document) && document[i] != '\n' {
				i++
			}
		case '"':
			if i+2 < len(document) && document[i:i+3] == `"""` {
				i += 3
				for i+2 < len(document) && document[i:i+3] != `"""` {
					i++
				}
				if i+2 < len(document) {
					i += 3
				}
				continue
			}
			i++
			for i < len(document) {
				if document[i] == '\\' && i+1 < len(document) {
					i += 2
					continue
				}
				if document[i] == '"' {
					i++
					break
				}
				i++
			}
		default:
			if document[i] == '_' || document[i] >= 'A' && document[i] <= 'Z' || document[i] >= 'a' && document[i] <= 'z' {
				start := i
				for i < len(document) && (document[i] == '_' || document[i] >= 'A' && document[i] <= 'Z' || document[i] >= 'a' && document[i] <= 'z' || document[i] >= '0' && document[i] <= '9') {
					i++
				}
				if strings.EqualFold(document[start:i], "mutation") {
					return true
				}
				continue
			}
			i++
		}
	}
	return false
}

func newPlaygroundsClient(flags *rootFlags) (*client.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	baseURL := strings.TrimSpace(os.Getenv("SCREENCLOUD_PLAYGROUNDS_URL"))
	if baseURL == "" {
		baseURL = defaultPlaygroundsBaseURL
	}
	cfg.BaseURL = strings.TrimRight(baseURL, "/")
	// The organization API key is valid only for Studio GraphQL. Never send it
	// to the separate Playgrounds host; every authorized call supplies an
	// explicit short-lived JWT header below.
	cfg.ScreencloudApiKey = ""
	cfg.AuthHeaderVal = ""
	cfg.AccessToken = ""
	// Configured headers may contain service-specific secrets under arbitrary
	// names. Crossing from Studio to Playgrounds is a trust-boundary change, so
	// carry none of them; the caller supplies only the scoped JWT below.
	cfg.Headers = map[string]string{}
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.DryRun = flags.dryRun
	// Source, private data, and viewer HTML must never enter the generic cache.
	c.NoCache = true
	return c, nil
}

func mintScopedJWT(ctx context.Context, flags *rootFlags, kind, spaceID, screenID string) (string, map[string]any, error) {
	if !flags.yes {
		return "", nil, usageErr(fmt.Errorf("minting a short-lived %s JWT is a GraphQL mutation; rerun with --yes after reviewing the target", kind))
	}
	if strings.TrimSpace(spaceID) == "" {
		return "", nil, usageErr(fmt.Errorf("--space-id is required to mint a scoped %s token", kind))
	}
	input := map[string]any{"spaceId": spaceID}
	var query, field, tokenField string
	switch kind {
	case "management":
		query = `mutation MintManagementToken($input: AppManagementJwtRequestInput!) { createSignedAppManagementJwt(input: $input) { signedAppManagementToken tokenType expiresAt } }`
		field = "createSignedAppManagementJwt"
		tokenField = "signedAppManagementToken"
	case "viewer":
		if strings.TrimSpace(screenID) != "" {
			input["screenId"] = screenID
		}
		query = `mutation MintViewerToken($input: AppViewerJwtRequestInput!) { createSignedAppViewerJwt(input: $input) { signedAppViewerToken tokenType expiresAt } }`
		field = "createSignedAppViewerJwt"
		tokenField = "signedAppViewerToken"
	default:
		return "", nil, fmt.Errorf("unsupported scoped token kind %q", kind)
	}
	data, meta, err := runGraphQL(ctx, flags, query, map[string]any{"input": input})
	if err != nil {
		return "", meta, err
	}
	var root map[string]map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return "", meta, fmt.Errorf("decoding scoped token response: %w", err)
	}
	token, _ := root[field][tokenField].(string)
	if token == "" {
		return "", meta, apiErr(fmt.Errorf("ScreenCloud did not return a %s token", kind))
	}
	public := map[string]any{"kind": kind, "minted": true}
	if v, ok := root[field]["tokenType"]; ok {
		public["token_type"] = v
	}
	if v, ok := root[field]["expiresAt"]; ok {
		public["expires_at"] = v
	}
	if cost, ok := meta["graphqlQueryCost"]; ok {
		public["graphql_query_cost"] = cost
	}
	return token, public, nil
}

func bearerHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func decodeObject(raw json.RawMessage) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	return object, nil
}

func encodeRaw(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	return json.RawMessage(data), err
}
