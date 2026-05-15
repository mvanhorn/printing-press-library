// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

// Package odoo implements an Odoo XML-RPC client for helpdesk ticket access.
package odoo

import (
	"fmt"
	"os"
	"strconv"

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

// Ticket represents a helpdesk.ticket record from Odoo.
type Ticket struct {
	ID              int         `json:"id"`
	Number          string      `json:"number"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	PartnerID       interface{} `json:"partner_id"`
	PartnerName     string      `json:"partner_name"`
	PartnerEmail    string      `json:"partner_email"`
	UserID          interface{} `json:"user_id"`
	TeamID          interface{} `json:"team_id"`
	StageID         interface{} `json:"stage_id"`
	Priority        string      `json:"priority"`
	KanbanState     string      `json:"kanban_state"`
	TagIDs          []int       `json:"tag_ids"`
	CategoryID      interface{} `json:"category_id"`
	ChannelID       interface{} `json:"channel_id"`
	CompanyID       interface{} `json:"company_id"`
	AssignedDate    string      `json:"assigned_date"`
	ClosedDate      string      `json:"closed_date"`
	Closed          bool        `json:"closed"`
	Unattended      bool        `json:"unattended"`
	LastStageUpdate string      `json:"last_stage_update"`
	WriteDate       string      `json:"write_date"`
	Active          bool        `json:"active"`
}

// Message represents a mail.message chatter entry.
type Message struct {
	ID          int         `json:"id"`
	Body        string      `json:"body"`
	AuthorID    interface{} `json:"author_id"`
	Date        string      `json:"date"`
	MessageType string      `json:"message_type"`
	SubtypeID   interface{} `json:"subtype_id"`
}

// Team represents a helpdesk.ticket.team record.
type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Stage represents a helpdesk.ticket.stage record.
type Stage struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Closed     bool   `json:"closed"`
	Unattended bool   `json:"unattended"`
	Sequence   int    `json:"sequence"`
}

// Tag represents a helpdesk.ticket.tag record.
type Tag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Category represents a helpdesk.ticket.category record.
type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Partner represents a res.partner record (customer).
type Partner struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// User represents a res.users record (agent).
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Login string `json:"login"`
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

// executeKw calls execute_kw on /xmlrpc/2/object and decodes the result into out.
func (c *Client) executeKw(model, method string, args []interface{}, kwargs map[string]interface{}, out interface{}) error {
	rpc, err := xmlrpc.NewClient(c.URL+"/xmlrpc/2/object", nil)
	if err != nil {
		return fmt.Errorf("connecting to object endpoint: %w", err)
	}
	defer rpc.Close()
	params := []interface{}{c.DB, c.UID, c.APIKey, model, method, args, kwargs}
	return rpc.Call("execute_kw", params, out)
}

var ticketFields = []string{
	"id", "number", "name", "description",
	"partner_id", "partner_name", "partner_email",
	"user_id", "team_id", "stage_id",
	"priority", "kanban_state", "tag_ids",
	"category_id", "channel_id", "company_id",
	"assigned_date", "closed_date", "closed", "unattended",
	"last_stage_update", "write_date", "active",
}

// SearchTickets runs search_read on helpdesk.ticket with the given domain.
func (c *Client) SearchTickets(domain []interface{}, limit, offset int, order string) ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{
		"fields": ticketFields,
		"limit":  limit,
		"offset": offset,
		"order":  order,
	}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket", "search_read", []interface{}{domain}, kwargs, &result)
	return result, err
}

// GetTicket returns one ticket by ID.
func (c *Client) GetTicket(id int) (map[string]interface{}, error) {
	kwargs := map[string]interface{}{"fields": ticketFields}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket", "read", []interface{}{[]int{id}}, kwargs, &result)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("ticket %d not found", id)
	}
	return result[0], nil
}

// GetTicketByNumber searches for a ticket by its number field (e.g. "T0042").
func (c *Client) GetTicketByNumber(number string) (map[string]interface{}, error) {
	domain := []interface{}{[]interface{}{"number", "=", number}}
	results, err := c.SearchTickets(domain, 1, 0, "id desc")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("ticket %q not found", number)
	}
	return results[0], nil
}

// CountTickets returns the number of tickets matching domain.
func (c *Client) CountTickets(domain []interface{}) (int, error) {
	var count int
	err := c.executeKw("helpdesk.ticket", "search_count", []interface{}{domain}, map[string]interface{}{}, &count)
	return count, err
}

// GetTicketMessages returns chatter messages for a ticket.
func (c *Client) GetTicketMessages(ticketID int) ([]map[string]interface{}, error) {
	domain := []interface{}{
		[]interface{}{"res_model", "=", "helpdesk.ticket"},
		[]interface{}{"res_id", "=", ticketID},
	}
	kwargs := map[string]interface{}{
		"fields": []string{"id", "body", "author_id", "date", "message_type", "subtype_id"},
		"order":  "date asc",
	}
	var result []map[string]interface{}
	err := c.executeKw("mail.message", "search_read", []interface{}{domain}, kwargs, &result)
	return result, err
}

// ListTeams returns all helpdesk teams.
func (c *Client) ListTeams() ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{"fields": []string{"id", "name"}}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket.team", "search_read", []interface{}{[]interface{}{}}, kwargs, &result)
	return result, err
}

// ListStages returns all helpdesk stages.
func (c *Client) ListStages() ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{
		"fields": []string{"id", "name", "closed", "unattended", "sequence"},
		"order":  "sequence asc",
	}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket.stage", "search_read", []interface{}{[]interface{}{}}, kwargs, &result)
	return result, err
}

// ListTags returns all helpdesk tags.
func (c *Client) ListTags() ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{"fields": []string{"id", "name"}}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket.tag", "search_read", []interface{}{[]interface{}{}}, kwargs, &result)
	return result, err
}

// ListCategories returns all helpdesk categories.
func (c *Client) ListCategories() ([]map[string]interface{}, error) {
	kwargs := map[string]interface{}{"fields": []string{"id", "name"}}
	var result []map[string]interface{}
	err := c.executeKw("helpdesk.ticket.category", "search_read", []interface{}{[]interface{}{}}, kwargs, &result)
	return result, err
}

// CreateTicket creates a new helpdesk ticket. Returns the new ticket ID.
func (c *Client) CreateTicket(vals map[string]interface{}) (int, error) {
	var id int
	err := c.executeKw("helpdesk.ticket", "create", []interface{}{vals}, map[string]interface{}{}, &id)
	return id, err
}

// UpdateTicket updates fields on an existing ticket.
func (c *Client) UpdateTicket(id int, vals map[string]interface{}) error {
	var ok bool
	return c.executeKw("helpdesk.ticket", "write", []interface{}{[]int{id}, vals}, map[string]interface{}{}, &ok)
}

// PostNote posts an internal note on a ticket.
func (c *Client) PostNote(ticketID int, body string) (int, error) {
	// Use message_post method on helpdesk.ticket — this respects followers and sends notifications.
	kwargs := map[string]interface{}{
		"body":         body,
		"message_type": "comment",
		"subtype_xmlid": "mail.mt_note",
	}
	var msgID int
	err := c.executeKw("helpdesk.ticket", "message_post", []interface{}{[]int{ticketID}}, kwargs, &msgID)
	return msgID, err
}

// ServerVersion returns the Odoo server version string for doctor/verify.
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

// IDFromMany2one extracts the integer ID from an Odoo Many2one field value.
// Odoo returns Many2one as [id, "name"] or false.
func IDFromMany2one(v interface{}) int {
	if v == nil {
		return 0
	}
	switch arr := v.(type) {
	case []interface{}:
		if len(arr) >= 1 {
			switch id := arr[0].(type) {
			case int:
				return id
			case int64:
				return int(id)
			case float64:
				return int(id)
			case string:
				n, _ := strconv.Atoi(id)
				return n
			}
		}
	}
	return 0
}

// NameFromMany2one extracts the display name from an Odoo Many2one field value.
func NameFromMany2one(v interface{}) string {
	if v == nil {
		return ""
	}
	switch arr := v.(type) {
	case []interface{}:
		if len(arr) >= 2 {
			return fmt.Sprintf("%v", arr[1])
		}
	}
	return ""
}

// StringVal safely converts an interface{} to string (handles false from Odoo).
func StringVal(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case bool:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// BoolVal safely converts an interface{} to bool.
func BoolVal(v interface{}) bool {
	if v == nil {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// IntVal safely converts an interface{} to int.
func IntVal(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// IntSliceVal safely converts an interface{} to []int (for Many2many).
func IntSliceVal(v interface{}) []int {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]int, 0, len(arr))
	for _, item := range arr {
		result = append(result, IntVal(item))
	}
	return result
}
