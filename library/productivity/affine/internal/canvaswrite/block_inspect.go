package canvaswrite

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/socketio"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

type BlockInspectOptions struct {
	WorkspaceID string
	DocID       string
	BlockID     string
}

type BlockInspectResult struct {
	DocID   string         `json:"doc_id"`
	BlockID string         `json:"block_id"`
	Block   map[string]any `json:"block"`
}

func InspectBlock(cfg *config.Config, opts BlockInspectOptions) (BlockInspectResult, error) {
	if opts.WorkspaceID == "" {
		return BlockInspectResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.DocID == "" || opts.BlockID == "" {
		return BlockInspectResult{}, fmt.Errorf("--doc and --block are required")
	}

	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return BlockInspectResult{}, err
	}
	defer client.Close()
	if err := joinWorkspace(client, opts.WorkspaceID); err != nil {
		return BlockInspectResult{}, err
	}

	engine, err := yjs.NewEngine()
	if err != nil {
		return BlockInspectResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return BlockInspectResult{}, err
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return BlockInspectResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return BlockInspectResult{}, err
	}
	block := blocks[opts.BlockID]
	if block == nil {
		return BlockInspectResult{}, fmt.Errorf("block not found")
	}
	return BlockInspectResult{DocID: opts.DocID, BlockID: opts.BlockID, Block: block}, nil
}
