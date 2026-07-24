// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/habitica/internal/client"
)

// Habitica authenticates a request with both the API token handled by config
// and the account UUID below. Keeping the UUID in the environment avoids
// storing a second credential in command flags or command history.
func init() {
	registerClientHook(func(c *client.Client) error {
		userID := strings.TrimSpace(os.Getenv("HABITICA_USER_ID"))
		if userID == "" || c == nil || c.Config == nil {
			return nil
		}
		if c.Config.Headers == nil {
			c.Config.Headers = make(map[string]string)
		}
		c.Config.Headers["x-api-user"] = userID
		c.Config.Headers["x-client"] = "habitica-pp-cli"
		return nil
	})
}

func habiticaHeaders() (map[string]string, error) {
	userID := strings.TrimSpace(os.Getenv("HABITICA_USER_ID"))
	if userID == "" {
		return nil, configErr(fmt.Errorf("HABITICA_USER_ID is required; set it alongside HABITICA_API_TOKEN"))
	}
	return map[string]string{"x-api-user": userID, "x-client": "habitica-pp-cli"}, nil
}

func habiticaData(raw json.RawMessage) (json.RawMessage, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding Habitica response: %w", err)
	}
	if len(envelope.Data) == 0 {
		return nil, fmt.Errorf("Habitica response did not include data")
	}
	return envelope.Data, nil
}
