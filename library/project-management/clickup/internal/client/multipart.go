// Hand-authored. Do not regenerate over this file with `printing-press generate`
// without merging — it owns multipart/form-data uploads for ClickUp attachments.
//
// PATCH(multipart-attachment-upload): the generated client only speaks JSON,
// but ClickUp's attachment endpoints (v2 POST /task/{id}/attachment and v3
// POST /workspaces/{ws}/{type}/{id}/attachments) require multipart/form-data.
// This file adds a multipart POST path that reuses the generated client's
// auth, configured headers, User-Agent, adaptive limiter and cache
// invalidation, without editing generated client.go.

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup/internal/cliutil"
)

// UploadFile describes one local file to send as a multipart file part.
type UploadFile struct {
	Path        string `json:"file"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// StatUploadFile checks that path exists and is a regular file, and resolves
// the part filename (base name), Content-Type (by extension, falling back to
// sniffing the first 512 bytes) and size. It never uploads anything.
func StatUploadFile(path string) (UploadFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return UploadFile{}, fmt.Errorf("file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return UploadFile{}, fmt.Errorf("file %q: not a regular file", path)
	}
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if ct == "" {
		f, err := os.Open(path) // #nosec G304 -- the user names this local file to upload it; reading it is the command's purpose
		if err != nil {
			return UploadFile{}, fmt.Errorf("file %q: %w", path, err)
		}
		head := make([]byte, 512)
		n, _ := io.ReadFull(f, head)
		_ = f.Close() // read-only handle; a close error cannot lose data
		ct = http.DetectContentType(head[:n])
	}
	return UploadFile{
		Path:        path,
		Name:        filepath.Base(path),
		ContentType: ct,
		Size:        info.Size(),
	}, nil
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// BuildMultipartBody encodes the extra form fields (sorted by key) followed by
// one file part named fieldName. It returns the encoded body and the request
// Content-Type (multipart/form-data with boundary).
func BuildMultipartBody(fieldName string, file UploadFile, fields map[string]string) ([]byte, string, error) {
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, "", fmt.Errorf("reading %q: %w", file.Path, err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := w.WriteField(k, fields[k]); err != nil {
			return nil, "", err
		}
	}

	name := file.Name
	if name == "" {
		name = filepath.Base(file.Path)
	}
	ct := file.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		quoteEscaper.Replace(fieldName), quoteEscaper.Replace(name)))
	h.Set("Content-Type", ct)
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// MaskedAuthHeader returns the resolved Authorization header with all but the
// last 4 characters masked, for dry-run previews. Empty when no auth is set.
func (c *Client) MaskedAuthHeader() string {
	h, err := c.authHeader()
	if err != nil {
		return ""
	}
	return maskToken(h)
}

// PostMultipart uploads one file as a multipart/form-data POST. 429 responses
// are retried (adaptive limiter + Retry-After); 5xx and transport errors are
// NOT retried, because the upload may already have landed and a retry would
// create a duplicate attachment. In DryRun mode it prints a preview to stderr
// and sends nothing.
func (c *Client) PostMultipart(path string, params map[string]string, fieldName string, file UploadFile, fields map[string]string) (json.RawMessage, int, error) {
	targetURL := c.BaseURL + path

	authHeader, err := c.authHeader()
	if err != nil {
		return nil, 0, err
	}

	if c.DryRun {
		fmt.Fprintf(os.Stderr, "POST %s\n", targetURL)
		keys := make([]string, 0, len(params))
		for k, v := range params {
			if v != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for i, k := range keys {
			sep := "&"
			if i == 0 {
				sep = "?"
			}
			fmt.Fprintf(os.Stderr, "  %s%s=%s\n", sep, k, params[k])
		}
		fmt.Fprintf(os.Stderr, "  Content-Type: multipart/form-data\n")
		for k, v := range fields {
			if v != "" {
				fmt.Fprintf(os.Stderr, "  field %s=%s\n", k, v)
			}
		}
		fmt.Fprintf(os.Stderr, "  part %s: %s (%s, %d bytes)\n", fieldName, file.Name, file.ContentType, file.Size)
		if authHeader != "" {
			fmt.Fprintf(os.Stderr, "  Authorization: %s\n", maskToken(authHeader))
		}
		fmt.Fprintf(os.Stderr, "\n(dry run - no request sent)\n")
		return json.RawMessage(`{"dry_run": true}`), 0, nil
	}

	body, contentType, err := BuildMultipartBody(fieldName, file, fields)
	if err != nil {
		return nil, 0, err
	}

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		if len(params) > 0 {
			q := req.URL.Query()
			for k, v := range params {
				if v != "" {
					q.Set(k, v)
				}
			}
			req.URL.RawQuery = q.Encode()
		}
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		if c.Config != nil {
			for k, v := range c.Config.Headers {
				if strings.EqualFold(k, "Content-Type") {
					continue
				}
				req.Header.Set(k, v)
			}
		}
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "clickup-pp-cli/v2+v3")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("POST %s: %w", path, err)
		}
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // body already fully read; a close error changes nothing
		if err != nil {
			return nil, 0, fmt.Errorf("reading response: %w", err)
		}
		respBody = sanitizeJSONResponse(respBody)

		if resp.StatusCode < 400 {
			c.limiter.OnSuccess()
			c.invalidateCache()
			return json.RawMessage(respBody), resp.StatusCode, nil
		}

		apiErr := &APIError{Method: http.MethodPost, Path: path, StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
		if resp.StatusCode == 429 && attempt < maxRetries {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			fmt.Fprintf(os.Stderr, "rate limited, waiting %s (attempt %d/%d)\n", wait, attempt+1, maxRetries)
			time.Sleep(wait)
			lastErr = apiErr
			continue
		}
		return nil, resp.StatusCode, apiErr
	}
	return nil, 0, lastErr
}
