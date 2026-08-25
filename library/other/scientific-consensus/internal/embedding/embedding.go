// internal/embedding/embedding.go
package embedding

import "math"

// Embedder defines the interface for text embedding models.
type Embedder interface {
	// Embed returns a vector representation of the input text.
	Embed(text string) ([]float64, error)
	// Dimensions returns the size of the embedding vector.
	Dimensions() int
}

// CosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1, but for our use case it's 0..1.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}