// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
)

// Inspections retrieve public HTTP facts and write only the CLI's observation cache.
func applyHGJInspectionHints(s *server.MCPServer) {
	for _, name := range []string{"restaurants_get", "prayer_get"} {
		entry := s.GetTool(name)
		if entry == nil {
			continue
		}
		tool := entry.Tool
		for _, option := range []mcplib.ToolOption{mcplib.WithReadOnlyHintAnnotation(false), mcplib.WithDestructiveHintAnnotation(false), mcplib.WithIdempotentHintAnnotation(false), mcplib.WithOpenWorldHintAnnotation(true)} {
			option(&tool)
		}
		s.AddTool(tool, entry.Handler)
	}
}
func hgjMCPStoreStatus(ctx context.Context, db *store.Store) (mcpStoreStatusKind, error) {
	var exists, count int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='hgj_detail_snapshots'`).Scan(&exists); err != nil {
		return "", err
	}
	if exists > 0 {
		if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM hgj_detail_snapshots WHERE position=0`).Scan(&count); err != nil {
			return "", err
		}
		if count > 0 {
			return mcpStoreStatusReady, nil
		}
	}
	// Preserve the framework's status for manually populated generic tables.
	return mcpStoreStatus(db)
}
