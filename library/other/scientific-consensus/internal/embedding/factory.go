// internal/embedding/factory.go
package embedding

import (
	"fmt"
	"os"
	"sync"
)

var (
	mu     sync.Mutex
	models = map[string]func() (Embedder, error){
		// "pro" uses a remote Python server (any SentenceTransformer model)
		"pro": func() (Embedder, error) {
			serverURL := os.Getenv("KALM_SERVER_URL")
			if serverURL == "" {
				serverURL = "http://localhost:5000"
			}
			return NewKaLMRemoteEmbedder(serverURL)
		},
	}
	activeModel Embedder
	modelName   string
)

// Get returns the embedder for the given model name.
// It lazy-loads the model on first call.
func Get(model string) (Embedder, error) {
	mu.Lock()
	defer mu.Unlock()
	if activeModel != nil && modelName == model {
		return activeModel, nil
	}
	if activeModel != nil {
		activeModel = nil
	}
	modelName = model
	fn, ok := models[model]
	if !ok {
		return nil, fmt.Errorf("unknown embedding model: %q (supported: pro)", model)
	}
	var err error
	activeModel, err = fn()
	if err != nil {
		return nil, err
	}
	return activeModel, nil
}