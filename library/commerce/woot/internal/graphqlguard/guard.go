// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package graphqlguard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// ValidateReadOnly accepts exactly one GraphQL query operation.
func ValidateReadOnly(document string) error {
	if strings.TrimSpace(document) == "" {
		return fmt.Errorf("GraphQL query document is required")
	}
	doc, err := parser.ParseQuery(&ast.Source{Name: "query.graphql", Input: document})
	if err != nil {
		return fmt.Errorf("invalid GraphQL query document: %w", err)
	}
	if len(doc.Operations) != 1 {
		return fmt.Errorf("GraphQL document must contain exactly one query operation")
	}
	if doc.Operations[0].Operation != ast.Query {
		return fmt.Errorf("GraphQL %s operations are not allowed; only query operations are permitted", doc.Operations[0].Operation)
	}
	return nil
}

type responseEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []responseError `json:"errors"`
}

type responseError struct {
	Message string `json:"message"`
}

// ValidateResponse rejects GraphQL failures carried inside an HTTP 200 body.
func ValidateResponse(data json.RawMessage) error {
	var envelope responseEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("invalid GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, graphqlErr := range envelope.Errors {
			message := strings.TrimSpace(graphqlErr.Message)
			if message == "" {
				message = "unspecified GraphQL error"
			}
			messages = append(messages, message)
		}
		return fmt.Errorf("GraphQL returned errors: %s", strings.Join(messages, "; "))
	}
	if envelope.Data == nil || strings.TrimSpace(string(envelope.Data)) == "null" {
		return fmt.Errorf("GraphQL response is missing usable data")
	}
	return nil
}
