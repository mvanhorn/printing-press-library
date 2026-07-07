package gplay

import (
	"context"
	"fmt"
)

// Suggest returns up to ~5 search autocomplete completions for a term via the
// IJ4APc RPC.
func (c *Client) Suggest(ctx context.Context, term string) ([]string, error) {
	if term == "" {
		return nil, fmt.Errorf("term is required")
	}
	inner := fmt.Sprintf(`[[null,[%q],[10],[2],4]]`, term)
	payload, err := c.batchExecute(ctx, "IJ4APc", inner, "", "")
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, nil
	}
	root := decode(payload)
	var out []string
	for _, s := range root.path(0, 0).arr() {
		if t := s.path(0).str(); t != "" {
			out = append(out, t)
		}
	}
	return out, nil
}
