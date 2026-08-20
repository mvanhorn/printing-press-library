// Package wpupload implements WordPress's raw-binary media upload protocol.
package wpupload

import (
	"context"
	"crypto/md5" // #nosec G501 -- WordPress Content-MD5 is a transport integrity check, not a password hash.
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

const maxUploadResponseBody = 8 << 20

// maxUploadAttempts bounds 429 retries. Uploads are single interactive
// requests, so one limiter-paced retry pair is enough before surfacing the
// typed error.
const maxUploadAttempts = 3

// uploadStartRate matches the generated client's conservative auto-mode
// starting pace; the limiter self-adjusts from 429/success feedback.
const uploadStartRate = 2.0

var ErrEmptyFile = errors.New("the file was empty")

type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return fmt.Sprintf("WordPress media upload returned HTTP %d", e.StatusCode)
}

type Client struct {
	baseURL    string
	authHeader string
	headers    map[string]string
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

// New builds an uploader from the same resolved config and transport used by
// the generated JSON client, keeping endpoint and auth selection in one place.
func New(cfg *config.Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := &Client{
		httpClient: httpClient,
		headers:    make(map[string]string),
		limiter:    cliutil.NewAdaptiveLimiterAuto(uploadStartRate),
	}
	if cfg != nil {
		client.baseURL = cfg.BaseURL
		client.authHeader = cfg.AuthHeader()
		for key, value := range cfg.Headers {
			client.headers[key] = value
		}
	}
	return client
}

// UploadFile sends filePath as the request body and returns WordPress's JSON
// media object unchanged.
func (c *Client) UploadFile(ctx context.Context, filePath string) (json.RawMessage, int, error) {
	file, err := os.Open(filepath.Clean(filePath)) // #nosec G304 -- explicit CLI file argument.
	if err != nil {
		return nil, 0, fmt.Errorf("opening upload file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("reading upload file metadata: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("upload path is not a regular file: %s", filePath)
	}
	if info.Size() == 0 {
		return nil, 0, ErrEmptyFile
	}

	first := make([]byte, 512)
	n, readErr := io.ReadFull(file, first)
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		return nil, 0, fmt.Errorf("reading upload file header: %w", readErr)
	}
	contentType := detectMIMEType(filePath, first[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewinding upload file: %w", err)
	}
	hash := md5.New() // #nosec G401 -- required by WordPress's Content-MD5 upload contract.
	if _, err := io.Copy(hash, file); err != nil {
		return nil, 0, fmt.Errorf("hashing upload file: %w", err)
	}
	contentMD5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewinding upload file: %w", err)
	}

	target, err := mediaUploadEndpoint(c.baseURL)
	if err != nil {
		return nil, 0, err
	}
	baseName := strings.NewReplacer("\r", "_", "\n", "_", `\`, `\\`, `"`, `\"`).Replace(filepath.Base(filePath))

	for attempt := 1; ; attempt++ {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, 0, fmt.Errorf("rewinding upload file: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, file)
		if err != nil {
			return nil, 0, fmt.Errorf("creating media upload request: %w", err)
		}
		req.ContentLength = info.Size()
		for key, value := range c.headers {
			req.Header.Set(key, value)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Content-Disposition", `attachment; filename="`+baseName+`"`)
		req.Header.Set("Content-MD5", contentMD5)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "wordpress-pp-cli/0.1.0")
		if c.authHeader != "" {
			req.Header.Set("Authorization", c.authHeader)
		}

		c.limiter.Wait()
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("uploading media: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxUploadResponseBody))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading media upload response: %w", readErr)
		}
		if closeErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("closing media upload response: %w", closeErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			wait := cliutil.RetryAfter(resp)
			if attempt >= maxUploadAttempts {
				// Typed error so callers can distinguish throttling from
				// "upload rejected" — empty-on-throttle silently corrupts
				// downstream state.
				return nil, resp.StatusCode, &cliutil.RateLimitError{
					URL:        target,
					RetryAfter: wait,
					Body:       strings.TrimSpace(string(body)),
				}
			}
			if err := sleepContext(ctx, wait); err != nil {
				return nil, resp.StatusCode, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, resp.StatusCode, decodeUploadError(resp.StatusCode, body)
		}
		c.limiter.OnSuccess()
		if !json.Valid(body) {
			return nil, resp.StatusCode, fmt.Errorf("WordPress media upload returned invalid JSON")
		}
		return json.RawMessage(body), resp.StatusCode, nil
	}
}

// sleepContext waits for d or until ctx is canceled, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func detectMIMEType(fileName string, first512 []byte) string {
	detected := http.DetectContentType(first512)
	if extensionType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName))); extensionType != "" {
		return extensionType
	}
	return detected
}

func mediaUploadEndpoint(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("invalid WordPress REST base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid WordPress REST base URL %q", baseURL)
	}
	query := u.Query()
	if _, ok := query["rest_route"]; ok {
		query.Set("rest_route", strings.TrimRight(query.Get("rest_route"), "/")+"/wp/v2/media")
		u.RawQuery = query.Encode()
		return u.String(), nil
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/wp/v2/media"
	return u.String(), nil
}

func decodeUploadError(statusCode int, body []byte) error {
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &response) == nil && (response.Code != "" || response.Message != "") {
		return &APIError{StatusCode: statusCode, Code: response.Code, Message: response.Message}
	}
	message := strings.TrimSpace(string(body))
	if len(message) > 512 {
		message = message[:512] + "..."
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}
	return &APIError{StatusCode: statusCode, Message: message}
}
