// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

// Package odoo implements a generic Odoo 18 XML-RPC client.
package odoo

import (
	"fmt"
	"os"

	"github.com/kolo/xmlrpc"
)

// Client holds connection parameters for an Odoo instance.
type Client struct {
	URL    string
	DB     string
	User   string
	APIKey string
	UID    int
}

// NewFromEnv builds a Client from ODOO_URL, ODOO_DB, ODOO_USER, ODOO_API_KEY.
func NewFromEnv() (*Client, error) {
	url := os.Getenv("ODOO_URL")
	db := os.Getenv("ODOO_DB")
	user := os.Getenv("ODOO_USER")
	apiKey := os.Getenv("ODOO_API_KEY")
	if url == "" || db == "" || user == "" || apiKey == "" {
		return nil, fmt.Errorf("ODOO_URL, ODOO_DB, ODOO_USER, and ODOO_API_KEY must all be set")
	}
	return &Client{URL: url, DB: db, User: user, APIKey: apiKey}, nil
}

// NewFromFlags builds a Client from explicit values (flag overrides), falling
// back to env vars for any empty parameter.
func NewFromFlags(url, db, user string) (*Client, error) {
	if url == "" {
		url = os.Getenv("ODOO_URL")
	}
	if db == "" {
		db = os.Getenv("ODOO_DB")
	}
	if user == "" {
		user = os.Getenv("ODOO_USER")
	}
	apiKey := os.Getenv("ODOO_API_KEY")
	if url == "" || db == "" || user == "" || apiKey == "" {
		return nil, fmt.Errorf("set ODOO_URL, ODOO_DB, ODOO_USER, and ODOO_API_KEY (or use --url, --db, --user flags)")
	}
	return &Client{URL: url, DB: db, User: user, APIKey: apiKey}, nil
}

// Authenticate calls /xmlrpc/2/common authenticate and stores the UID.
func (c *Client) Authenticate() error {
	rpc, err := xmlrpc.NewClient(c.URL+"/xmlrpc/2/common", nil)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", c.URL, err)
	}
	defer rpc.Close()
	var uid int
	err = rpc.Call("authenticate", []interface{}{c.DB, c.User, c.APIKey, map[string]interface{}{}}, &uid)
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	if uid == 0 {
		return fmt.Errorf("authentication failed: invalid credentials for user %q on database %q", c.User, c.DB)
	}
	c.UID = uid
	return nil
}

// ServerVersion returns the Odoo server version string.
func (c *Client) ServerVersion() (string, error) {
	rpc, err := xmlrpc.NewClient(c.URL+"/xmlrpc/2/common", nil)
	if err != nil {
		return "", err
	}
	defer rpc.Close()
	var info map[string]interface{}
	if err := rpc.Call("version", []interface{}{}, &info); err != nil {
		return "", err
	}
	if sv, ok := info["server_version"]; ok {
		return fmt.Sprintf("%v", sv), nil
	}
	return "unknown", nil
}

// executeKw calls execute_kw on /xmlrpc/2/object.
func (c *Client) executeKw(model, method string, args []interface{}, kwargs map[string]interface{}, out interface{}) error {
	rpc, err := xmlrpc.NewClient(c.URL+"/xmlrpc/2/object", nil)
	if err != nil {
		return fmt.Errorf("connecting to object endpoint: %w", err)
	}
	defer rpc.Close()
	params := []interface{}{c.DB, c.UID, c.APIKey, model, method, args, kwargs}
	return rpc.Call("execute_kw", params, out)
}

// SearchRead calls search_read on the given model with domain, fields, limit, offset, order.
func (c *Client) SearchRead(model string, domain []interface{}, fields []string, limit, offset int, order string) ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{
		"fields": fields,
		"limit":  limit,
		"offset": offset,
		"order":  order,
	}
	var result []map[string]interface{}
	err := c.executeKw(model, "search_read", []interface{}{domain}, kwargs, &result)
	return result, err
}

// Read calls read on the given model for the given IDs.
func (c *Client) Read(model string, ids []int, fields []string) ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{"fields": fields}
	var result []map[string]interface{}
	err := c.executeKw(model, "read", []interface{}{ids}, kwargs, &result)
	return result, err
}

// Create calls create on the given model with vals. Returns the new record ID.
func (c *Client) Create(model string, vals map[string]interface{}) (int, error) {
	var id int
	err := c.executeKw(model, "create", []interface{}{vals}, map[string]interface{}{}, &id)
	return id, err
}

// Write calls write on the given model for IDs with vals.
func (c *Client) Write(model string, ids []int, vals map[string]interface{}) error {
	var ok bool
	return c.executeKw(model, "write", []interface{}{ids, vals}, map[string]interface{}{}, &ok)
}

// SearchCount calls search_count on the given model with domain.
func (c *Client) SearchCount(model string, domain []interface{}) (int, error) {
	var count int
	err := c.executeKw(model, "search_count", []interface{}{domain}, map[string]interface{}{}, &count)
	return count, err
}

// Call invokes an arbitrary method on a model (e.g. action_confirm).
func (c *Client) Call(model, method string, ids []int) error {
	var result interface{}
	return c.executeKw(model, method, []interface{}{ids}, map[string]interface{}{}, &result)
}
