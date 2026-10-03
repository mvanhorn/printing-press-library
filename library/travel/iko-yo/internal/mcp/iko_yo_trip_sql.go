// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"errors"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cacheguard"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Preserve the generated SQL validation, scan bounds and result envelope while
// binding this source's saved-fact SQL to a verified private cache snapshot.
func handleTripSQL(ctx context.Context, request mcplib.CallToolRequest) (result *mcplib.CallToolResult, callErr error) {
	query, ok := request.GetArguments()["query"].(string)
	if !ok || query == "" {
		return mcplib.NewToolResultError("query is required"), nil
	}
	if err := validateReadOnlyQuery(query); err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	path, err := mcpDBPath()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	guard, err := cacheguard.Read(path)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	if !guard.Exists() {
		return mcplib.NewToolResultError(mcpMissingStoreMessage(path)), nil
	}
	snapshot, err := guard.Clone(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	defer func() {
		if err := errors.Join(guard.Snapshot(), guard.Cleanup()); err != nil {
			result = mcplib.NewToolResultError(err.Error())
			callErr = nil
		}
	}()
	db, err := store.OpenReadOnlyContext(ctx, snapshot)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	defer func() {
		if err := db.Close(); err != nil {
			result = mcplib.NewToolResultError(err.Error())
			callErr = nil
		}
	}()
	queryCtx, cancel := bound.WithSQLQueryDeadline(ctx)
	defer cancel()
	conn, err := db.DB().Conn(queryCtx)
	if err != nil {
		return mcplib.NewToolResultError(mcpSQLQueryError(queryCtx, err)), nil
	}
	defer func() {
		if err := conn.Close(); err != nil {
			result = mcplib.NewToolResultError(err.Error())
			callErr = nil
		}
	}()
	if _, err := sqlite.Limit(conn, sqlite3.SQLITE_LIMIT_LENGTH, mcpSQLMaxValueBytes); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("setting SQL value length cap: %v", err)), nil
	}
	rows, err := conn.QueryContext(queryCtx, query)
	if err != nil {
		return mcplib.NewToolResultError(mcpSQLQueryError(queryCtx, err)), nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			result = mcplib.NewToolResultError(err.Error())
			callErr = nil
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	scan := bound.NewSQLScanState(columns)
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return mcplib.NewToolResultError(mcpSQLQueryError(queryCtx, err)), nil
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = values[i]
		}
		if !scan.Add(row) {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcplib.NewToolResultError(mcpSQLQueryError(queryCtx, err)), nil
	}
	status, err := mcpStoreStatus(db)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return toolResultJSON(mcpSQLEnvelope(scan.Rows, columns, status, scan.Truncated))
}
