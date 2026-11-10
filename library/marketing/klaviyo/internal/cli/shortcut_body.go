// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func readShortcutBodyOfType(cmd *cobra.Command, wantType string) (map[string]any, error) {
	body, err := readShortcutBody(cmd)
	if err != nil {
		return nil, err
	}
	data, _ := body["data"].(map[string]any)
	if data["type"] != wantType {
		return nil, fmt.Errorf("stdin data.type must be %q", wantType)
	}
	return body, nil
}

func shortcutBodyFlagsChanged(cmd *cobra.Command) bool {
	changed := false
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if strings.HasPrefix(flag.Name, "data-") && flag.Name != "data-source" {
			changed = true
		}
	})
	return changed
}

func readShortcutBody(cmd *cobra.Command) (map[string]any, error) {
	raw, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	var body map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("parsing stdin JSON object: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("stdin must contain a single JSON request object")
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("stdin must contain a non-empty JSON request object")
	}
	return body, nil
}
