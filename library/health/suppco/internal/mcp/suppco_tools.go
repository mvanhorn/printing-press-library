package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/provider"
)

const printingPressMockAuthorization = "Bearer mock-token-for-testing"

func allowPrintingPressMockOrigin(c *client.Client) bool {
	if !cliutil.IsVerifyEnv() || !cliutil.IsVerifyLiveHTTPEnv() || c == nil || c.Config == nil {
		return false
	}
	authorization := strings.TrimSpace(c.Config.AuthHeader())
	return authorization == printingPressMockAuthorization
}

func newMCPProvider() (*provider.Service, error) {
	c, err := newMCPClient()
	if err != nil {
		return nil, err
	}
	return provider.New(c), nil
}

func handleStackProducts(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	service, err := newMCPProvider()
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	projection, err := service.Products(ctx)
	if err != nil {
		return mcpProviderError(err), nil
	}
	return toolResultJSON(projection)
}

func handleStackNutrients(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	service, err := newMCPProvider()
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	projection, err := service.Nutrients(ctx)
	if err != nil {
		return mcpProviderError(err), nil
	}
	return toolResultJSON(projection)
}

func handleScheduleShow(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	date, err := mcpDate(req)
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	service, err := newMCPProvider()
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	projection, err := service.Schedule(ctx, date)
	if err != nil {
		return mcpProviderError(err), nil
	}
	return toolResultJSON(projection)
}

func handleRegimenSnapshot(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	date, err := mcpDate(req)
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	service, err := newMCPProvider()
	if err != nil {
		return mcpToolError(err.Error()), nil
	}
	snapshot, err := service.Snapshot(ctx, date)
	if err != nil {
		return mcpProviderError(err), nil
	}
	return toolResultJSON(snapshot)
}

func mcpProviderError(err error) *mcplib.CallToolResult {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return mcpToolError("authentication failed: SuppCo rejected the bearer token; replace SUPPCO_ACCESS_TOKEN or pipe a current token to suppco-pp-cli auth set-token")
		case http.StatusForbidden:
			return mcpToolError("permission denied: the SuppCo bearer token does not have access to this read")
		case http.StatusTooManyRequests:
			return mcpToolError("rate limited: retry the SuppCo read later")
		}
	}
	return mcpToolError(err.Error())
}

func mcpDate(req mcplib.CallToolRequest) (string, error) {
	value, ok := req.GetArguments()["date"]
	if !ok {
		return "", fmt.Errorf("date is required in YYYY-MM-DD format")
	}
	date, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("date must be a string in YYYY-MM-DD format")
	}
	if err := provider.ValidateDate(date); err != nil {
		return "", err
	}
	return date, nil
}
