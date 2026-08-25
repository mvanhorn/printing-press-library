// internal/embedding/kalm_remote.go
package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KaLMRemoteEmbedder is an HTTP client that talks to the Python embedding server.
type KaLMRemoteEmbedder struct {
	serverURL  string
	httpClient *http.Client
	dimensions int
	normalize  bool
}

// NewKaLMRemoteEmbedder creates a new embedder that connects to the given server URL.
// It first checks the /health endpoint to ensure the server is reachable.
func NewKaLMRemoteEmbedder(serverURL string) (*KaLMRemoteEmbedder, error) {
	if serverURL == "" {
		serverURL = "http://127.0.0.1:5000"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(serverURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("KaLM server unreachable: %w (start kalm_server.py)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KaLM server health check returned %d", resp.StatusCode)
	}
	return &KaLMRemoteEmbedder{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		normalize:  true,
	}, nil
}

// Embed sends the text to the Python server and returns the embedding vector.
// The server truncates long texts to 512 tokens, and this client adds a
// safety character limit to prevent extremely large requests.
func (k *KaLMRemoteEmbedder) Embed(text string) ([]float64, error) {
	// Safety: check if the embedder is nil
	if k == nil {
		return nil, fmt.Errorf("embedder is nil")
	}
	if k.httpClient == nil {
		return nil, fmt.Errorf("http client not initialized (server unreachable)")
	}

	// Safety guard: limit text to 10,000 characters to avoid excessive memory/bandwidth.
	// The server will further truncate at the token level (512 tokens).
	const maxTextLen = 10000
	if len(text) > maxTextLen {
		text = text[:maxTextLen]
	}

	payload := map[string]interface{}{
		"text":      text,
		"normalize": k.normalize,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("JSON marshal: %w", err)
	}

	resp, err := k.httpClient.Post(k.serverURL+"/embed", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedding  []float64 `json:"embedding"`
		Dimensions int       `json:"dimensions"`
		Error      string    `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("server error: %s", result.Error)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding received")
	}

	k.dimensions = result.Dimensions
	return result.Embedding, nil
}

// Dimensions returns the dimensionality of the embedding vectors.
// Default is 768 for BioBERT-NLI (gsarti/biobert-nli); updated after first Embed call.
func (k *KaLMRemoteEmbedder) Dimensions() int {
	if k.dimensions == 0 {
		return 768 // default for gsarti/biobert-nli (BioBERT-NLI)
	}
	return k.dimensions
}