package canvaswrite

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/socketio"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

type CardInspectOptions struct {
	WorkspaceID string
	DocID       string
	CardID      string
}

type CardChild struct {
	ID       string `json:"id"`
	Flavour  string `json:"flavour"`
	Type     string `json:"type,omitempty"`
	Text     string `json:"text,omitempty"`
	SourceID string `json:"source_id,omitempty"`
}

type CardInspectResult struct {
	DocID    string      `json:"doc_id"`
	CardID   string      `json:"card_id"`
	Children []CardChild `json:"children"`
}

func InspectCard(cfg *config.Config, opts CardInspectOptions) (CardInspectResult, error) {
	if opts.WorkspaceID == "" {
		return CardInspectResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.DocID == "" || opts.CardID == "" {
		return CardInspectResult{}, fmt.Errorf("--doc and --card are required")
	}

	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return CardInspectResult{}, err
	}
	defer client.Close()
	if err := joinWorkspace(client, opts.WorkspaceID); err != nil {
		return CardInspectResult{}, err
	}

	engine, err := yjs.NewEngine()
	if err != nil {
		return CardInspectResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return CardInspectResult{}, err
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return CardInspectResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return CardInspectResult{}, err
	}
	card := blocks[opts.CardID]
	if card == nil {
		return CardInspectResult{}, fmt.Errorf("card not found")
	}
	result := CardInspectResult{DocID: opts.DocID, CardID: opts.CardID}
	children, _ := card["sys:children"].([]any)
	for _, rawID := range children {
		id, _ := rawID.(string)
		block := blocks[id]
		if block == nil {
			continue
		}
		child := CardChild{
			ID:       id,
			Flavour:  stringField(block, "sys:flavour"),
			Type:     stringField(block, "sys:type"),
			Text:     stringField(block, "prop:text"),
			SourceID: stringField(block, "prop:sourceId"),
		}
		result.Children = append(result.Children, child)
	}
	return result, nil
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
