package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/config"
)

// Client makes JSON-RPC 1.0 calls to a Dogecoin Core node.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	idCounter  atomic.Int64
	DryRun     bool
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	ID     json.RawMessage `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Call invokes a JSON-RPC method on the node and returns the result field.
// params may be nil (treated as empty array), a []any, or any JSON-marshalable value.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.idCounter.Add(1)

	if params == nil {
		params = []any{}
	}

	reqBody := rpcRequest{
		JSONRPC: "1.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling rpc request: %w", err)
	}

	if c.DryRun {
		return json.RawMessage(fmt.Sprintf(`{"dry_run":true,"method":%q,"params":%s}`,
			method, mustMarshal(params))), nil
	}

	url := c.cfg.BaseURL + "/"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.cfg.RPCUser != "" {
		req.SetBasicAuth(c.cfg.RPCUser, c.cfg.RPCPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", method, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed: check DOGECOIN_RPC_USER and DOGECOIN_RPC_PASS")
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return nil, fmt.Errorf("rate limited by node (HTTP 429); retry after %s", retryAfter)
		}
		return nil, fmt.Errorf("rate limited by node (HTTP 429); back off and retry")
	case http.StatusOK:
		// proceed
	default:
		return nil, fmt.Errorf("node returned HTTP %d for method %s", resp.StatusCode, method)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// newRPCClientFromConfig creates a Client directly from a config (bypasses rootFlags).
// Used by doctor and other commands that need RPC outside the normal command flow.
func NewFromConfig(cfg *config.Config) *Client {
	return New(cfg)
}
