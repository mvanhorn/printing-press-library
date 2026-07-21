// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/graphqlguard"
)

func validateReadOnlyGraphQLRequest(readOnly bool, method, path string, params map[string]string) error {
	if !readOnly || !strings.EqualFold(method, "GET") || path != "/graphql" {
		return nil
	}
	return graphqlguard.ValidateReadOnly(params["query"])
}

func validateReadOnlyGraphQLResponse(readOnly bool, method, path string, data json.RawMessage) error {
	if !readOnly || !strings.EqualFold(method, "GET") || path != "/graphql" {
		return nil
	}
	return graphqlguard.ValidateResponse(data)
}
