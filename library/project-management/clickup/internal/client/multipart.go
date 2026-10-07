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

// StatUploadFile checks that path exists, is a regular file and can be opened
// for reading, and resolves the part filename (base name), Content-Type (by
// extension, falling back to sniffing the first 512 bytes) and size. Opening
// every file here lets multi-file callers reject an unreadable file before
// any upload is sent. It never uploads anything.
func StatUploadFile(path string) (UploadFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return UploadFile{}, fmt.Errorf("file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return UploadFile{}, fmt.Errorf("file %q: not a regular file", path)
	}
	f, err := os.Open(path) // #nosec G304 -- the user names this local file to upload it; reading it is the command's purpose
	if err != nil {
		return UploadFile{}, fmt.Errorf("file %q: %w", path, err)
	}
	head := make([]byte, 512)
	n, readErr := io.ReadFull(f, head)
	_ = f.Close() // read-only handle; a close error cannot lose data
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return UploadFile{}, fmt.Errorf("file %q: %w", path, readErr)
	}
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if ct == "" {
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

// multipartEnvelope encodes everything in the multipart body except the file
// bytes: head holds the extra form fields (sorted by key) and the file part
// header for fieldName, tail holds the closing boundary. The file is streamed
// between them, so an upload never holds the file in memory and the request
// still carries an exact Content-Length. contentType is the request
// Content-Type (multipart/form-data with boundary).
func multipartEnvelope(fieldName string, file UploadFile, fields map[string]string) (head, tail []byte, contentType string, err error) {
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
			return nil, nil, "", err
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
	if _, err := w.CreatePart(h); err != nil {
		return nil, nil, "", err
	}
	head = append([]byte(nil), buf.Bytes()...)
	buf.Reset()
	if err := w.Close(); err != nil {
		return nil, nil, "", err
	}
	return head, buf.Bytes(), w.FormDataContentType(), nil
}

// openMultipartBody opens file and returns a reader over the full multipart
// body (head, file bytes, tail) with its exact length. The caller closes it.
// Each attempt reopens the file, so a 429 retry resends the whole body.
func openMultipartBody(head, tail []byte, path string) (io.ReadCloser, int64, error) {
	f, err := os.Open(path) // #nosec G304 -- the user names this local file to upload it; reading it is the command's purpose
	if err != nil {
		return nil, 0, fmt.Errorf("reading %q: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close() // read-only handle; a close error cannot lose data
		return nil, 0, fmt.Errorf("reading %q: %w", path, err)
	}
	body := struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), io.LimitReader(f, info.Size()), bytes.NewReader(tail)), f}
	return body, int64(len(head)) + info.Size() + int64(len(tail)), nil
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

	head, tail, contentType, err := multipartEnvelope(fieldName, file, fields)
	if err != nil {
		return nil, 0, err
	}

	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		c.limiter.Wait()
		body, size, err := openMultipartBody(head, tail, file.Path)
		if err != nil {
			return nil, 0, err
		}
		req, err := http.NewRequest(http.MethodPost, targetURL, body)
		if err != nil {
			_ = body.Close() // read-only handle; a close error cannot lose data
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}
		req.ContentLength = size
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
