package canvaswrite

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/socketio"
)

func connect(cfg *config.Config, workspaceID string) (*socketio.Client, error) {
	bearer := cfg.AffineToken
	if bearer == "" {
		bearer = strings.TrimPrefix(cfg.AuthHeader(), "Bearer ")
	}
	client, err := socketio.Connect(socketio.WSURLFromGraphQL(strings.TrimRight(cfg.BaseURL, "/")+"/graphql"), "", bearer)
	if err != nil {
		return nil, err
	}
	if err := joinWorkspace(client, workspaceID); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}
