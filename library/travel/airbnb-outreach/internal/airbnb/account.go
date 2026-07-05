// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import "encoding/json"

// Account is the signed-in user's basic identity.
type Account struct {
	ID         string `json:"id"`
	FirstName  string `json:"first_name"`
	IsHomeHost bool   `json:"is_home_host"`
}

// Me returns the signed-in account's identity and host status.
func (c *Client) Me() (*Account, error) {
	uid, err := c.CurrentUserID()
	if err != nil {
		return nil, err
	}
	data, err := c.Query("IsHostQuery", map[string]any{})
	if err != nil {
		return nil, err
	}
	var env struct {
		Viewer struct {
			User struct {
				FirstName  string `json:"firstName"`
				IsHomeHost bool   `json:"isHomeHost"`
			} `json:"user"`
		} `json:"viewer"`
	}
	_ = json.Unmarshal(data, &env)
	return &Account{
		ID:         uid,
		FirstName:  env.Viewer.User.FirstName,
		IsHomeHost: env.Viewer.User.IsHomeHost,
	}, nil
}
