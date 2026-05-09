// Package mcp implements a Model Context Protocol server for govprocure-pp-cli.
// It exposes the SQLite database (grants, SAM, awards) as MCP tools over stdio.
// Protocol: https://spec.modelcontextprotocol.io/specification/
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
)

// --- JSON-RPC 2.0 types ---

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func errResp(id interface{}, code int, msg string) Response {
	return Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}

// --- MCP capability types ---

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools *struct{} `json:"tools,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string) ToolResult {
	return ToolResult{Content: []ContentBlock{{Type: "text", Text: text}}}
}

func errorResult(msg string) ToolResult {
	return ToolResult{IsError: true, Content: []ContentBlock{{Type: "text", Text: msg}}}
}

// --- Tool definitions ---

var tools = []Tool{
	{
		Name:        "search_grants",
		Description: "Full-text search grants.gov opportunities in the local SQLite mirror. Returns matching grants with title, agency, close date, award range, and synopsis.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Search query (keywords, CFDA number, agency name, eligibility terms)"},
				"limit": {Type: "integer", Description: "Max results to return (default: 10, max: 50)"},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "search_sam",
		Description: "Full-text search SAM.gov contract opportunities in the local SQLite mirror. Returns matching notices with title, agency, NAICS code, set-aside type, and response deadline.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Search query (keywords, NAICS code, agency, solicitation number)"},
				"limit": {Type: "integer", Description: "Max results to return (default: 10, max: 50)"},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "search_awards",
		Description: "Full-text search USASpending.gov award records. Returns past awards with recipient, agency, amount, and date range.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {Type: "string", Description: "Search query (recipient name, agency, program description)"},
				"limit": {Type: "integer", Description: "Max results to return (default: 10, max: 50)"},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "get_grant",
		Description: "Fetch a single grants.gov opportunity by its opportunity ID.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{"id": {Type: "string", Description: "Opportunity ID from grants.gov"}},
			Required:   []string{"id"},
		},
	},
	{
		Name:        "get_sam_notice",
		Description: "Fetch a single SAM.gov contract notice by its notice ID.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{"id": {Type: "string", Description: "Notice ID from SAM.gov"}},
			Required:   []string{"id"},
		},
	},
	{
		Name:        "grants_closing_soon",
		Description: "List grants.gov opportunities closing within the next N days, ordered by close date. Use this to find urgent application deadlines.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"days": {Type: "integer", Description: "Look-ahead window in days (default: 30)"},
			},
		},
	},
	{
		Name:        "sam_by_set_aside",
		Description: "List SAM.gov opportunities filtered by set-aside type. Common codes: SDVOSB (Service-Disabled Veteran-Owned), WOSB (Women-Owned), SBA (8a), HZC (HUBZone), SBP (Small Business).",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"set_aside": {Type: "string", Description: "Set-aside code (e.g. SDVOSB, WOSB, SBA, HZC, SBP)"},
				"limit":     {Type: "integer", Description: "Max results (default: 20)"},
			},
			Required: []string{"set_aside"},
		},
	},
	{
		Name:        "agency_profile",
		Description: "Get a full intelligence profile for a federal agency: open grants, open SAM notices, total historical awards, top recipients. Useful for competitive analysis before bidding.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{"agency": {Type: "string", Description: "Agency name or partial name (e.g. 'NSF', 'Department of Defense', 'HHS')"}},
			Required:   []string{"agency"},
		},
	},
	{
		Name:        "sync_status",
		Description: "Show the last sync timestamp and record counts for each data source (grants.gov, SAM.gov, USASpending.gov). Use this to understand how fresh the local data is.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
}

// --- Server ---

type Server struct {
	db *db.DB
}

func NewServer(database *db.DB) *Server {
	return &Server{db: database}
}

func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = encoder.Encode(errResp(nil, -32700, "parse error"))
			continue
		}

		resp := s.handle(req)
		_ = encoder.Encode(resp)
	}

	return scanner.Err()
}

func (s *Server) handle(req Request) Response {
	switch req.Method {
	case "initialize":
		return Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo:      ServerInfo{Name: "govprocure", Version: "1.0.0"},
				Capabilities:    Capabilities{Tools: &struct{}{}},
			},
		}

	case "initialized":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}}

	case "tools/list":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: ToolsListResult{Tools: tools}}

	case "tools/call":
		var p CallToolParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(req.ID, -32602, "invalid params")
		}
		result, err := s.callTool(p.Name, p.Arguments)
		if err != nil {
			return Response{JSONRPC: "2.0", ID: req.ID, Result: errorResult(err.Error())}
		}
		return Response{JSONRPC: "2.0", ID: req.ID, Result: result}

	case "ping":
		return Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]string{"status": "ok"}}

	default:
		return errResp(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) callTool(name string, args json.RawMessage) (ToolResult, error) {
	var a map[string]interface{}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &a)
	}
	if a == nil {
		a = map[string]interface{}{}
	}

	getString := func(key, def string) string {
		if v, ok := a[key]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
		return def
	}
	getInt := func(key string, def int) int {
		if v, ok := a[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
		return def
	}

	switch name {
	case "search_grants":
		query := getString("query", "")
		if query == "" {
			return errorResult("query is required"), nil
		}
		limit := getInt("limit", 10)
		if limit > 50 {
			limit = 50
		}
		grants, err := s.db.SearchGrants(query, limit)
		if err != nil {
			return errorResult(fmt.Sprintf("search error: %v", err)), nil
		}
		if len(grants) == 0 {
			return textResult("No grants found matching: " + query), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d grant(s) matching '%s':\n\n", len(grants), query)
		for i, g := range grants {
			fmt.Fprintf(&sb, "%d. **%s**\n", i+1, g.Title)
			fmt.Fprintf(&sb, "   ID: %s | Agency: %s\n", g.OpportunityID, g.Agency)
			fmt.Fprintf(&sb, "   Close: %s | Posted: %s\n", g.CloseDate, g.PostDate)
			if g.AwardCeiling > 0 {
				fmt.Fprintf(&sb, "   Award: $%.0f – $%.0f\n", g.AwardFloor, g.AwardCeiling)
			}
			if g.Eligibility != "" {
				fmt.Fprintf(&sb, "   Eligibility: %s\n", g.Eligibility)
			}
			if g.Synopsis != "" {
				syn := g.Synopsis
				if len(syn) > 200 {
					syn = syn[:200] + "..."
				}
				fmt.Fprintf(&sb, "   Synopsis: %s\n", syn)
			}
			fmt.Fprintln(&sb)
		}
		return textResult(sb.String()), nil

	case "search_sam":
		query := getString("query", "")
		if query == "" {
			return errorResult("query is required"), nil
		}
		limit := getInt("limit", 10)
		if limit > 50 {
			limit = 50
		}
		notices, err := s.db.SearchSAM(query, limit)
		if err != nil {
			return errorResult(fmt.Sprintf("search error: %v", err)), nil
		}
		if len(notices) == 0 {
			return textResult("No SAM opportunities found matching: " + query), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d SAM opportunity/ies matching '%s':\n\n", len(notices), query)
		for i, n := range notices {
			fmt.Fprintf(&sb, "%d. **%s**\n", i+1, n.Title)
			fmt.Fprintf(&sb, "   Notice ID: %s | Solicitation: %s\n", n.NoticeID, n.SolicitationNumber)
			fmt.Fprintf(&sb, "   Agency: %s | Sub-tier: %s\n", n.Agency, n.SubTier)
			fmt.Fprintf(&sb, "   NAICS: %s | Set-aside: %s\n", n.NAICSCode, n.SetAside)
			fmt.Fprintf(&sb, "   Response Deadline: %s | Posted: %s\n", n.ResponseDeadline, n.PostedDate)
			if n.Description != "" {
				desc := n.Description
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}
				fmt.Fprintf(&sb, "   Description: %s\n", desc)
			}
			fmt.Fprintln(&sb)
		}
		return textResult(sb.String()), nil

	case "search_awards":
		query := getString("query", "")
		if query == "" {
			return errorResult("query is required"), nil
		}
		limit := getInt("limit", 10)
		if limit > 50 {
			limit = 50
		}
		awards, err := s.db.SearchAwards(query, limit)
		if err != nil {
			return errorResult(fmt.Sprintf("search error: %v", err)), nil
		}
		if len(awards) == 0 {
			return textResult("No awards found matching: " + query), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d award(s) matching '%s':\n\n", len(awards), query)
		for i, a := range awards {
			fmt.Fprintf(&sb, "%d. **%s** → %s\n", i+1, a.Agency, a.RecipientName)
			fmt.Fprintf(&sb, "   Award ID: %s | Amount: $%.0f\n", a.AwardID, a.Amount)
			fmt.Fprintf(&sb, "   Period: %s to %s\n", a.StartDate, a.EndDate)
			if a.Description != "" {
				desc := a.Description
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}
				fmt.Fprintf(&sb, "   Description: %s\n", desc)
			}
			fmt.Fprintln(&sb)
		}
		return textResult(sb.String()), nil

	case "get_grant":
		id := getString("id", "")
		if id == "" {
			return errorResult("id is required"), nil
		}
		g, err := s.db.GetGrant(id)
		if err != nil {
			return errorResult(fmt.Sprintf("db error: %v", err)), nil
		}
		if g == nil {
			return textResult("No grant found with ID: " + id), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "**%s**\n\n", g.Title)
		fmt.Fprintf(&sb, "Opportunity ID: %s\n", g.OpportunityID)
		fmt.Fprintf(&sb, "Agency: %s\n", g.Agency)
		fmt.Fprintf(&sb, "CFDA: %s\n", g.CFDANumber)
		fmt.Fprintf(&sb, "Posted: %s | Closes: %s\n", g.PostDate, g.CloseDate)
		fmt.Fprintf(&sb, "Award: $%.0f – $%.0f\n", g.AwardFloor, g.AwardCeiling)
		fmt.Fprintf(&sb, "Eligibility: %s\n\n", g.Eligibility)
		fmt.Fprintf(&sb, "Synopsis:\n%s\n", g.Synopsis)
		return textResult(sb.String()), nil

	case "get_sam_notice":
		id := getString("id", "")
		if id == "" {
			return errorResult("id is required"), nil
		}
		n, err := s.db.GetSAM(id)
		if err != nil {
			return errorResult(fmt.Sprintf("db error: %v", err)), nil
		}
		if n == nil {
			return textResult("No SAM notice found with ID: " + id), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "**%s**\n\n", n.Title)
		fmt.Fprintf(&sb, "Notice ID: %s\n", n.NoticeID)
		fmt.Fprintf(&sb, "Solicitation: %s\n", n.SolicitationNumber)
		fmt.Fprintf(&sb, "Agency: %s | Sub-tier: %s\n", n.Agency, n.SubTier)
		fmt.Fprintf(&sb, "NAICS: %s | Set-aside: %s\n", n.NAICSCode, n.SetAside)
		fmt.Fprintf(&sb, "Posted: %s | Response Deadline: %s\n\n", n.PostedDate, n.ResponseDeadline)
		fmt.Fprintf(&sb, "Description:\n%s\n", n.Description)
		return textResult(sb.String()), nil

	case "grants_closing_soon":
		days := getInt("days", 30)
		grants, err := s.db.GrantsClosingWithin(days)
		if err != nil {
			return errorResult(fmt.Sprintf("db error: %v", err)), nil
		}
		if len(grants) == 0 {
			return textResult(fmt.Sprintf("No grants closing within the next %d days.", days)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d grant(s) closing within %d days:\n\n", len(grants), days)
		for i, g := range grants {
			fmt.Fprintf(&sb, "%d. **%s** — closes %s\n", i+1, g.Title, g.CloseDate)
			fmt.Fprintf(&sb, "   %s | Award: $%.0f–$%.0f\n", g.Agency, g.AwardFloor, g.AwardCeiling)
			fmt.Fprintf(&sb, "   ID: %s\n\n", g.OpportunityID)
		}
		return textResult(sb.String()), nil

	case "sam_by_set_aside":
		setAside := getString("set_aside", "")
		if setAside == "" {
			return errorResult("set_aside is required (e.g. SDVOSB, WOSB, SBA, HZC, SBP)"), nil
		}
		limit := getInt("limit", 20)
		notices, err := s.db.SAMBySetAside(setAside, limit)
		if err != nil {
			return errorResult(fmt.Sprintf("db error: %v", err)), nil
		}
		if len(notices) == 0 {
			return textResult(fmt.Sprintf("No SAM opportunities found with set-aside: %s", setAside)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d opportunity/ies with set-aside '%s':\n\n", len(notices), setAside)
		for i, n := range notices {
			fmt.Fprintf(&sb, "%d. **%s**\n", i+1, n.Title)
			fmt.Fprintf(&sb, "   %s | NAICS: %s\n", n.Agency, n.NAICSCode)
			fmt.Fprintf(&sb, "   Deadline: %s | Notice: %s\n\n", n.ResponseDeadline, n.NoticeID)
		}
		return textResult(sb.String()), nil

	case "agency_profile":
		agency := getString("agency", "")
		if agency == "" {
			return errorResult("agency is required"), nil
		}
		profile, err := s.db.GetAgencyProfile(agency)
		if err != nil {
			return errorResult(fmt.Sprintf("db error: %v", err)), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "**Agency Intelligence: %s**\n\n", profile.Agency)
		fmt.Fprintf(&sb, "Total Historical Awards: $%.0f across %d awards\n", profile.TotalAwarded, profile.AwardCount)
		if len(profile.TopRecipients) > 0 {
			fmt.Fprintf(&sb, "Top Recipients:\n")
			for _, r := range profile.TopRecipients {
				fmt.Fprintf(&sb, "  • %s\n", r)
			}
		}
		fmt.Fprintf(&sb, "\nOpen Grants (%d):\n", len(profile.OpenGrants))
		for _, g := range profile.OpenGrants {
			fmt.Fprintf(&sb, "  • %s — closes %s (ID: %s)\n", g.Title, g.CloseDate, g.OpportunityID)
		}
		fmt.Fprintf(&sb, "\nOpen SAM Notices (%d):\n", len(profile.OpenSAM))
		for _, n := range profile.OpenSAM {
			fmt.Fprintf(&sb, "  • %s — deadline %s [%s]\n", n.Title, n.ResponseDeadline, n.SetAside)
		}
		return textResult(sb.String()), nil

	case "sync_status":
		sources := []string{"grants", "sam", "awards"}
		var sb strings.Builder
		fmt.Fprintf(&sb, "**govprocure sync status:**\n\n")
		for _, src := range sources {
			entry, err := s.db.LastSync(src)
			if err != nil || entry == nil {
				fmt.Fprintf(&sb, "%-12s  never synced\n", src)
				continue
			}
			fmt.Fprintf(&sb, "%-12s  last synced: %s | %d records | status: %s\n",
				src, entry.SyncedAt, entry.RecordsSynced, entry.Status)
			if entry.Error != "" {
				fmt.Fprintf(&sb, "              error: %s\n", entry.Error)
			}
		}
		return textResult(sb.String()), nil

	default:
		return errorResult("unknown tool: " + name), nil
	}
}

// Serve opens the DB and runs the MCP server. Called from the CLI.
func Serve(cfg *config.Config) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	srv := NewServer(database)
	return srv.Run()
}
