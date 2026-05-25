package client

import "fmt"

// AuthError signals missing or invalid credentials.
type AuthError struct{ Message string }

func (e *AuthError) Error() string {
	if e.Message == "" {
		return "no credentials configured — run `jinko auth login --key jnk_...` or set JINKO_API_KEY"
	}
	return e.Message
}

// APIError wraps a non-2xx response from the Jinko BFF.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code,omitempty"`
	Message    string `json:"message,omitempty"`
	DocURL     string `json:"doc_url,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
}
