package client

import "testing"

// TestCacheKeyDeterministicAcrossParamOrder guards against the non-deterministic
// cache key regression: Go map iteration order is randomised per runtime, so an
// unordered walk of params yielded a different SHA256 for the same logical
// (path, params) pair, defeating the GET cache entirely.
func TestCacheKeyDeterministicAcrossParamOrder(t *testing.T) {
	c := &Client{}
	a := c.cacheKey("/api/tags", map[string]string{"limit": "10", "cursor": "abc", "q": "z"})
	b := c.cacheKey("/api/tags", map[string]string{"q": "z", "cursor": "abc", "limit": "10"})
	if a != b {
		t.Errorf("cache key must be stable regardless of param insertion order: %q != %q", a, b)
	}

	// Different params must still produce different keys.
	d := c.cacheKey("/api/tags", map[string]string{"limit": "20"})
	if a == d {
		t.Errorf("distinct params should not collide: both %q", a)
	}

	// Empty params is stable too.
	if c.cacheKey("/api/tags", nil) != c.cacheKey("/api/tags", map[string]string{}) {
		t.Error("nil and empty params should hash identically")
	}
}
