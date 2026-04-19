// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// Package linkedin wraps the stickerdaniel/linkedin-mcp-server Python MCP
// subprocess (installable via `uvx linkedin-scraper-mcp@latest`). It speaks
// the MCP 2024-11-05 JSON-RPC protocol over stdin/stdout.
//
// Tool argument shapes are modeled as typed Go structs for the common case but
// ultimately cross the wire as map[string]any, and tool results are returned
// as json.RawMessage so the caller can pass them through unchanged. This keeps
// the client forward-compatible when the upstream server adds fields.
package linkedin

import "encoding/json"

// ProtocolVersion is the MCP revision this client speaks.
const ProtocolVersion = "2024-11-05"

// JSONRPCRequest is a single JSON-RPC 2.0 request frame.
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse is a single JSON-RPC 2.0 response frame.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError is the error body of a JSON-RPC response.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Implementation identifies the client or server party in an initialize
// exchange.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities is sent in the initialize request. We send an empty
// object because we don't register roots or sampling.
type ClientCapabilities struct {
	Experimental map[string]any `json:"experimental,omitempty"`
	Roots        *RootsCap      `json:"roots,omitempty"`
	Sampling     *struct{}      `json:"sampling,omitempty"`
}

// RootsCap describes roots support.
type RootsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities is returned by the server in the initialize response.
type ServerCapabilities struct {
	Experimental map[string]any `json:"experimental,omitempty"`
	Logging      map[string]any `json:"logging,omitempty"`
	Prompts      *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"prompts,omitempty"`
	Resources *struct {
		Subscribe   bool `json:"subscribe,omitempty"`
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"resources,omitempty"`
	Tools *struct {
		ListChanged bool `json:"listChanged,omitempty"`
	} `json:"tools,omitempty"`
}

// InitializeRequest matches `initialize` params.
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeResult is the server's reply.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// Tool describes a single tool returned by `tools/list`.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ListToolsResult is returned by `tools/list`.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolRequest is `tools/call` params.
type CallToolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ContentBlock is one entry in CallToolResult.Content. MCP defines several
// types (text, image, resource); we only read text blocks.
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Data     string          `json:"data,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
}

// CallToolResult is returned by `tools/call`.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ---------------------------------------------------------------------------
// LinkedIn tool argument shapes.
//
// These are convenience structs that marshal to the `arguments` map the MCP
// server expects. The CLI layer builds map[string]any directly from cobra
// flags; these are primarily for documentation and for any caller that wants
// type safety.
// ---------------------------------------------------------------------------

// SearchPeopleArgs corresponds to the `search_people` tool.
type SearchPeopleArgs struct {
	Keywords string `json:"keywords"`
	Location string `json:"location,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// SearchJobsArgs corresponds to the `search_jobs` tool.
type SearchJobsArgs struct {
	Keywords string `json:"keywords"`
	Location string `json:"location,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// GetPersonArgs corresponds to `get_person_profile`.
type GetPersonArgs struct {
	LinkedInURL string   `json:"linkedin_url"`
	Sections    []string `json:"sections,omitempty"`
}

// GetCompanyArgs corresponds to `get_company_profile`.
type GetCompanyArgs struct {
	CompanySlug string   `json:"company_slug"`
	Sections    []string `json:"sections,omitempty"`
}

// InboxArgs corresponds to `get_inbox`.
type InboxArgs struct {
	Limit int `json:"limit,omitempty"`
}

// ConversationArgs corresponds to `get_conversation`. NOTE: upstream issue
// #307 tracks intermittent failures here.
type ConversationArgs struct {
	UserOrThreadID string `json:"user_or_thread_id"`
}

// SearchMessagesArgs corresponds to `search_messages`.
type SearchMessagesArgs struct {
	Query string `json:"query"`
}

// SendMessageArgs corresponds to `send_message`.
type SendMessageArgs struct {
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
}

// CompanyPostsArgs corresponds to `get_company_posts`.
type CompanyPostsArgs struct {
	CompanySlug string `json:"company_slug"`
	Limit       int    `json:"limit,omitempty"`
}

// JobArgs corresponds to `get_job`.
type JobArgs struct {
	JobID string `json:"job_id"`
}

// SidebarArgs corresponds to `get_sidebar` (people also viewed).
type SidebarArgs struct {
	PersonURL string `json:"person_url"`
}

// ConnectArgs corresponds to `send_connection_request`. NOTE: upstream issue
// #365 tracks a bug where the note is sometimes dropped.
type ConnectArgs struct {
	PersonURL string `json:"person_url"`
	Note      string `json:"note,omitempty"`
}

// ToolNames enumerates the 13 LinkedIn tools wired in the CLI.
var ToolNames = struct {
	SearchPeople   string
	SearchJobs     string
	GetPerson      string
	GetCompany     string
	Inbox          string
	Conversation   string
	SearchMessages string
	SendMessage    string
	CompanyPosts   string
	Job            string
	Sidebar        string
	Connect        string
	Close          string
}{
	SearchPeople:   "search_people",
	SearchJobs:     "search_jobs",
	GetPerson:      "get_person_profile",
	GetCompany:     "get_company_profile",
	Inbox:          "get_inbox",
	Conversation:   "get_conversation",
	SearchMessages: "search_messages",
	SendMessage:    "send_message",
	CompanyPosts:   "get_company_posts",
	Job:            "get_job",
	Sidebar:        "get_sidebar",
	Connect:        "send_connection_request",
	Close:          "close_session",
}
