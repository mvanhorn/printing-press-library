package socketio

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	client "github.com/zishang520/engine.io-client-go/transports"
	"github.com/zishang520/engine.io/v2/types"
	sio "github.com/zishang520/socket.io-client-go/socket"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultAckTimeout     = 10 * time.Second
)

// Client wraps a Socket.io connection to an AFFiNE server.
type Client struct {
	socket *sio.Socket
	mu     sync.Mutex
}

// WSURLFromGraphQL converts a GraphQL endpoint URL to a WebSocket base URL.
func WSURLFromGraphQL(endpoint string) string {
	u := endpoint
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	// Strip /graphql suffix. Socket.io connects to the root.
	u = strings.TrimSuffix(u, "/graphql")
	u = strings.TrimSuffix(u, "/graphql/")
	// Convert back to http(s) for socket.io-client-go (it handles ws upgrade)
	u = strings.Replace(u, "wss://", "https://", 1)
	u = strings.Replace(u, "ws://", "http://", 1)
	return u
}

// Connect establishes a Socket.io connection to the AFFiNE server.
func Connect(baseURL, cookie, bearer string) (*Client, error) {
	opts := sio.DefaultOptions()
	opts.SetForceNew(true)
	opts.SetReconnection(false)
	opts.SetTimeout(defaultConnectTimeout)

	// ponytail: avoid polling zstd crash in engine.io-client-go; drop patch when upstream applies transports.
	opts.SetTransports(types.NewSet(client.WebSocket))

	// Set auth headers
	headers := http.Header{}
	if cookie != "" {
		headers.Set("Cookie", cookie)
	}
	if bearer != "" {
		headers.Set("Authorization", "Bearer "+bearer)
	}
	if len(headers) > 0 {
		opts.SetExtraHeaders(headers)
	}

	// Set up connect event handlers before connecting
	connected := make(chan struct{}, 1)
	connectErr := make(chan error, 1)

	opts.SetAutoConnect(false) // Connect manually after setting up handlers

	socket, err := sio.Io(baseURL, opts)
	if err != nil {
		return nil, fmt.Errorf("socket.io create: %w", err)
	}

	socket.On("connect", func(args ...any) {
		select {
		case connected <- struct{}{}:
		default:
		}
	})
	socket.On("connect_error", func(args ...any) {
		var e error
		if len(args) > 0 {
			if err, ok := args[0].(error); ok {
				e = err
			} else {
				e = fmt.Errorf("connect_error: %v", args[0])
			}
		} else {
			e = fmt.Errorf("connect_error")
		}
		select {
		case connectErr <- e:
		default:
		}
	})

	// If already connected (race), handle it
	if socket.Connected() {
		return &Client{socket: socket}, nil
	}

	socket.Connect()

	select {
	case <-connected:
		return &Client{socket: socket}, nil
	case err := <-connectErr:
		socket.Disconnect()
		return nil, err
	case <-time.After(defaultConnectTimeout):
		// The socket may still be usable even if we missed the connect event
		if socket.Connected() {
			return &Client{socket: socket}, nil
		}
		socket.Disconnect()
		return nil, fmt.Errorf("socket.io connect timeout after %v", defaultConnectTimeout)
	}
}

// Close disconnects the socket.
func (c *Client) Close() {
	c.socket.Disconnect()
}

// JoinWorkspace joins a workspace room.
func (c *Client) JoinWorkspace(workspaceID, clientVersion string) error {
	payload := map[string]any{
		"spaceType":     "workspace",
		"spaceId":       workspaceID,
		"clientVersion": clientVersion,
	}
	ack, err := c.emitWithAck("space:join", payload)
	if err != nil {
		return err
	}
	if errMsg := ackError(ack); errMsg != "" {
		return fmt.Errorf("join workspace: %s", errMsg)
	}
	return nil
}

// LoadDocResult contains the result of loading a document.
type LoadDocResult struct {
	Missing   string // base64-encoded Y.js state (named "missing" in the AFFiNE protocol)
	Timestamp int64
}

// LoadDoc loads a document's Y.js state.
func (c *Client) LoadDoc(workspaceID, docID string) (*LoadDocResult, error) {
	payload := map[string]any{
		"spaceType": "workspace",
		"spaceId":   workspaceID,
		"docId":     docID,
	}
	ack, err := c.emitWithAck("space:load-doc", payload)
	if err != nil {
		return nil, err
	}
	if errMsg := ackError(ack); errMsg != "" {
		return nil, fmt.Errorf("load-doc: %s", errMsg)
	}
	data, _ := ack["data"].(map[string]any)
	if data == nil {
		return &LoadDocResult{}, nil
	}
	result := &LoadDocResult{}
	if s, ok := data["missing"].(string); ok {
		result.Missing = s
	}
	if ts, ok := data["timestamp"].(float64); ok {
		result.Timestamp = int64(ts)
	}
	return result, nil
}

// PushDocUpdate pushes a Y.js update to the server.
func (c *Client) PushDocUpdate(workspaceID, docID, updateBase64 string) error {
	payload := map[string]any{
		"spaceType": "workspace",
		"spaceId":   workspaceID,
		"docId":     docID,
		"update":    updateBase64,
	}
	ack, err := c.emitWithAck("space:push-doc-update", payload)
	if err != nil {
		return err
	}
	if errMsg := ackError(ack); errMsg != "" {
		return fmt.Errorf("push-doc-update: %s", errMsg)
	}
	return nil
}

// PushDocUpdateNoAck pushes a Y.js update without waiting for a Socket.IO ack.
// This is useful for large initial document snapshots where the AFFiNE server
// persists the update but the Go socket client can stall while waiting for ack.
func (c *Client) PushDocUpdateNoAck(workspaceID, docID, updateBase64 string) {
	c.socket.Emit("space:push-doc-update", map[string]any{
		"spaceType": "workspace",
		"spaceId":   workspaceID,
		"docId":     docID,
		"update":    updateBase64,
	})
}

// DeleteDoc sends a delete doc event (fire-and-forget).
func (c *Client) DeleteDoc(workspaceID, docID string) {
	c.socket.Emit("space:delete-doc", map[string]any{
		"spaceType": "workspace",
		"spaceId":   workspaceID,
		"docId":     docID,
	})
}

// emitWithAck emits an event and waits for an acknowledgement.
func (c *Client) emitWithAck(event string, payload map[string]any) (map[string]any, error) {
	ch := make(chan struct {
		data []any
		err  error
	}, 1)

	c.socket.Timeout(defaultAckTimeout).EmitWithAck(event, payload)(func(data []any, err error) {
		ch <- struct {
			data []any
			err  error
		}{data, err}
	})

	select {
	case result := <-ch:
		if result.err != nil {
			return nil, fmt.Errorf("%s: %w", event, result.err)
		}
		if len(result.data) > 0 {
			if m, ok := result.data[0].(map[string]any); ok {
				return m, nil
			}
		}
		return map[string]any{}, nil
	case <-time.After(defaultAckTimeout + 2*time.Second):
		return nil, fmt.Errorf("%s timeout", event)
	}
}

// ackError extracts an error message from an ack response.
func ackError(ack map[string]any) string {
	errObj, ok := ack["error"]
	if !ok {
		return ""
	}
	if errMap, ok := errObj.(map[string]any); ok {
		if msg, ok := errMap["message"].(string); ok && msg != "" {
			return msg
		}
		return "unknown error"
	}
	if errStr, ok := errObj.(string); ok {
		return errStr
	}
	return ""
}
