// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
)

const defaultCacheTTL = 15 * time.Minute

// Client wraps the NotebookLM batchexecute RPC surface with cookie auth.
type Client struct {
	Config   *config.Config
	NLM      *nlm.Client
	DryRun   bool
	NoCache  bool
	cacheDir string
	cacheTTL time.Duration
}

// New builds a client from on-disk config.
func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	hc, err := cfg.HTTPClient()
	if err != nil {
		return nil, err
	}
	nlmClient, err := nlm.NewClient(ctx, hc)
	if err != nil {
		return nil, err
	}
	cacheDir, _ := cacheDirPath()
	return &Client{
		Config:   cfg,
		NLM:      nlmClient,
		cacheDir: cacheDir,
		cacheTTL: defaultCacheTTL,
	}, nil
}

func cacheDirPath() (string, error) {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, "notebooklm-pp-cli", "http"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "notebooklm-pp-cli", "http"), nil
}

// HTTPClient returns the underlying transport for doctor checks.
func (c *Client) HTTPClient() *http.Client {
	if c == nil || c.Config == nil {
		return http.DefaultClient
	}
	hc, err := c.Config.HTTPClient()
	if err != nil {
		return http.DefaultClient
	}
	return hc
}

// AuthHeader returns the stored Cookie header value.
func (c *Client) AuthHeader() string {
	if c == nil || c.Config == nil {
		return ""
	}
	return c.Config.AuthHeader()
}

// MaskAuthHeader redacts cookie values for safe logging (shows last 4 chars only).
func MaskAuthHeader(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ";")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if idx := strings.Index(p, "="); idx > 0 {
			name := p[:idx]
			val := strings.TrimSpace(p[idx+1:])
			if len(val) <= 4 {
				parts[i] = name + "=***"
			} else {
				parts[i] = name + "=***" + val[len(val)-4:]
			}
		}
	}
	return strings.Join(parts, "; ")
}

func (c *Client) cacheKey(method, url string) string {
	h := sha256.Sum256([]byte(method + ":" + url))
	return hex.EncodeToString(h[:])
}

func (c *Client) readCache(key string) ([]byte, bool) {
	if c == nil || c.NoCache || c.cacheDir == "" {
		return nil, false
	}
	path := filepath.Join(c.cacheDir, key)
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if c.cacheTTL > 0 && time.Since(info.ModTime()) > c.cacheTTL {
		_ = os.Remove(path)
		return nil, false
	}
	data, err := os.ReadFile(path) // #nosec G304 -- cache path derived from hash key
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *Client) writeCache(key string, body []byte) {
	if c == nil || c.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(c.cacheDir, 0o700)
	path := filepath.Join(c.cacheDir, key)
	_ = os.WriteFile(path, body, 0o600)
}

// CachedGet performs a GET with optional on-disk response cache.
func (c *Client) CachedGet(ctx context.Context, url string) ([]byte, int, error) {
	key := c.cacheKey(http.MethodGet, url)
	if body, ok := c.readCache(key); ok {
		return body, http.StatusOK, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode == http.StatusOK && !c.NoCache {
		c.writeCache(key, body)
	}
	return body, resp.StatusCode, nil
}
