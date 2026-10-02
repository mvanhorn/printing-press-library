// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/mcp/bound"
)

// RegisterAirportTools keeps generated framework and Cobra tools, then replaces
// typed HTML endpoint handlers with domain-safe link extraction. Generated tools
// otherwise return raw SSR HTML, including embedded reservation/inventory data.
func RegisterAirportTools(s *server.MCPServer) {
	RegisterTools(s)
	for _, kind := range []string{"routes", "guide", "stop", "timetable"} {
		options := []mcplib.ToolOption{mcplib.WithDescription(airportPageDescription(kind)), mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false), mcplib.WithOpenWorldHintAnnotation(true)}
		switch kind {
		case "stop":
			options = append(options, mcplib.WithString("stop-id", mcplib.Required(), mcplib.Description("Exact provider stop identifier from stops find")))
		case "timetable":
			options = append(options, mcplib.WithString("route-id", mcplib.Required(), mcplib.Description("Exact provider area identifier from routes")), mcplib.WithNumber("direction", mcplib.Description("Source direction 1 from airport or 2 to airport")), mcplib.WithString("date", mcplib.Description("JST service date YYYY-MM-DD; defaults to today")))
		}
		s.AddTool(mcplib.NewTool("pages_"+kind, options...), airportPageHandler(kind))
	}
}

func airportPageDescription(kind string) string {
	switch kind {
	case "routes":
		return "Read the official route-area page and return canonical timetable links. No parameters are required. Embedded application data is omitted; prefer routes for structured airport and area filtering."
	case "guide":
		return "Read the official bus guide and return canonical baggage and boarding links. No parameters are required. Prefer conditions for structured published facts and current notices."
	case "stop":
		return "Read a stop page and return canonical route links. Required: stop-id, an exact provider identifier from stops find. Embedded application data is omitted; prefer stops get for structured boarding maps and connections."
	default:
		return "Read a timetable page and return canonical stop links without inventory data. Required: route-id. Optional: direction (1 from airport, 2 to airport) and date (JST YYYY-MM-DD, today by default). Prefer timetable for structured dated journeys and fares."
	}
}

func airportPageHandler(kind string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		path, prefixes, err := airportPagePath(kind, req.GetArguments())
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		p := limousine.New(30*time.Second, defaultMCPRateLimit)
		env, err := p.PageLinks(ctx, path, prefixes)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		data, err := json.Marshal(env)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcplib.NewToolResultText(bound.EndpointResponse("GET", data)), nil
	}
}

func airportPagePath(kind string, args map[string]any) (string, []string, error) {
	allowed := map[string]bool{}
	switch kind {
	case "stop":
		allowed["stop-id"] = true
	case "timetable":
		allowed["route-id"] = true
		allowed["direction"] = true
		allowed["date"] = true
	}
	for k := range args {
		if !allowed[k] {
			return "", nil, fmt.Errorf("unsupported page parameter %q", k)
		}
	}
	switch kind {
	case "routes":
		return "/en/timetable/list/", []string{"/en/timetable/detail"}, nil
	case "guide":
		return "/en/guide/", []string{"/en/guide/terms"}, nil
	case "stop":
		id, ok := args["stop-id"].(string)
		if !ok {
			return "", nil, fmt.Errorf("stop-id is required")
		}
		if err := limousine.ValidateID(id); err != nil {
			return "", nil, err
		}
		return "/en/busstop/detail/" + id + "/", []string{"/en/timetable/detail", "/en/line/detail"}, nil
	case "timetable":
		id, ok := args["route-id"].(string)
		if !ok {
			return "", nil, fmt.Errorf("route-id is required")
		}
		if err := limousine.ValidateID(id); err != nil {
			return "", nil, err
		}
		dir := 1
		if v, ok := args["direction"]; ok {
			n, ok := v.(float64)
			if !ok || (n != 1 && n != 2) {
				return "", nil, fmt.Errorf("direction must be numeric 1 or 2")
			}
			dir = int(n)
		}
		date := ""
		if v, ok := args["date"]; ok {
			date, ok = v.(string)
			if !ok {
				return "", nil, fmt.Errorf("date must be a JST YYYY-MM-DD string")
			}
		}
		date, err := limousine.ServiceDate(date, time.Now())
		if err != nil {
			return "", nil, err
		}
		q := url.Values{"dir": {strconv.Itoa(dir)}, "d": {date}}
		return "/en/timetable/detail/" + id + "/?" + q.Encode(), []string{"/en/busstop/detail"}, nil
	}
	return "", nil, fmt.Errorf("unsupported page kind")
}
