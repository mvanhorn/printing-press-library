package traveloka

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

func decodeJSON(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	}
	return apiError("MALFORMED_RESPONSE", "multiple or malformed JSON values", 0, false)
}
func copyMap(value map[string]any) (map[string]any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = decodeJSON(b, &result)
	return result, err
}
func secretKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	return strings.Contains(key, "token") || strings.Contains(key, "cookie") || strings.Contains(key, "sentinel") || strings.Contains(key, "password") || key == "authorization" || key == "proxyauthorization" || strings.Contains(key, "secret") || key == "usercontext" || key == "marketingcontextcapsule" || key == "inventoryratekey" || key == "requestheaders" || key == "headers" || key == "tvclientsessionid" || key == "xdid" || key == "tav" || key == "tvmccid"
}
func sanitizeValue(value any, secrets []string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			if !secretKey(k) {
				out[k] = sanitizeValue(x, secrets)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = sanitizeValue(x, secrets)
		}
		return out
	case string:
		return redactPublicText(v, secrets)
	default:
		return value
	}
}
func redactText(value string, secrets []string) string {
	for _, s := range secrets {
		if s != "" {
			value = strings.ReplaceAll(value, s, "[REDACTED]")
		}
	}
	return value
}
func collectStrings(value any, out *[]string) {
	switch v := value.(type) {
	case string:
		if v != "" {
			*out = append(*out, v)
		}
	case map[string]any:
		for _, x := range v {
			collectStrings(x, out)
		}
	case []any:
		for _, x := range v {
			collectStrings(x, out)
		}
	}
}

// Sanitize strips credential-like fields recursively from a public response or snapshot.
func Sanitize(value any) any {
	var secrets []string
	collectCredentialStrings(value, &secrets)
	return sanitizeValue(value, secrets)
}

// Opaque credential values are redacted from public strings. Tiny preference cookie
// values (for example country codes or 0/1) are not credentials in public data fields.
func redactPublicText(value string, secrets []string) string {
	for _, secret := range secrets {
		if len(secret) >= 8 {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}
func collectCredentialStrings(value any, out *[]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if secretKey(key) {
				collectStrings(child, out)
			} else {
				collectCredentialStrings(child, out)
			}
		}
	case []any:
		for _, child := range v {
			collectCredentialStrings(child, out)
		}
	}
}
