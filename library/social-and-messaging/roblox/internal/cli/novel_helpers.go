// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/internal/client"
	"github.com/spf13/cobra"
)

func fetchBundle(cmd *cobra.Command, c *client.Client, out map[string]any, key, path string, params map[string]string) {
	raw, err := c.Get(cmd.Context(), path, params)
	if err != nil {
		out[key+"_error"] = err.Error()
		return
	}
	var v any
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		out[key+"_error"] = err.Error()
		return
	}
	out[key] = v
}
func printBundle(cmd *cobra.Command, flags *rootFlags, out map[string]any) error {
	raw, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}
func fetchArray(cmd *cobra.Command, c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	raw, err := c.Get(cmd.Context(), path, params)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", path, err)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding response envelope from %s: %w", path, err)
	}
	data, ok := env["data"]
	if !ok || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, fmt.Errorf("response from %s is missing a data array", path)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("decoding data array from %s: %w", path, err)
	}
	return rows, nil
}
func intersectByNestedID(a, b []map[string]any, path string) []map[string]any {
	seen := map[string]bool{}
	for _, v := range b {
		if id := nestedString(v, path); id != "" {
			seen[id] = true
		}
	}
	out := []map[string]any{}
	for _, v := range a {
		if seen[nestedString(v, path)] {
			out = append(out, v)
		}
	}
	return out
}
func nestedString(v map[string]any, path string) string {
	var cur any = v
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if cur == nil {
		return ""
	}
	return fmt.Sprint(cur)
}

func validateRobloxID(value, name string) error {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return usageErr(fmt.Errorf("%s must be a positive numeric Roblox ID", name))
	}
	return nil
}
