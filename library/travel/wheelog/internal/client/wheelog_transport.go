// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
)

const wheelogMaxBody = 4 << 20
const wheelogSearchPath = "/timeline/custom/user/getWebTimelineList"
const wheelogDetailPath = "/poi/custom/user/getWebSpotDetail"

// WheelogTransport preserves the generated AdaptiveLimiter and typed RateLimitError
// path. It strips private source fields before response caching, logging or MCP output.
type WheelogTransport struct{ Base http.RoundTripper }

func (t WheelogTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	host := request.URL.Hostname()
	if host != "app.wheelog.com" && host != "127.0.0.1" && host != "localhost" {
		return nil, fmt.Errorf("WheeLog supports only its public app origin")
	}
	if host == "app.wheelog.com" && request.URL.Scheme != "https" {
		return nil, fmt.Errorf("WheeLog source requests require HTTPS")
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	path := request.URL.Path
	if path == "/" && (request.Method == http.MethodGet || request.Method == http.MethodHead) {
		response, err := base.RoundTrip(request)
		if err != nil {
			return nil, err
		}
		_ = response.Body.Close() // Page bytes are discarded; retain the HTTP status.
		body := []byte(`{"source_page_content_omitted":true}`)
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header = response.Header.Clone()
		response.Header.Del("Content-Encoding")
		response.Header.Del("Set-Cookie")
		response.Header.Del("Content-Length")
		response.Header.Set("Content-Type", "application/json")
		return response, nil
	}
	if request.Method != http.MethodPost || (path != wheelogSearchPath && path != wheelogDetailPath) {
		return nil, fmt.Errorf("unsupported WheeLog operation; use public spot search or inspection")
	}
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	if request.Body == nil {
		return nil, fmt.Errorf("WheeLog request needs form fields")
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 8193))
	if err != nil {
		return nil, err
	}
	if len(body) > 8192 {
		return nil, fmt.Errorf("WheeLog request exceeds 8 KiB")
	}
	fields, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("invalid WheeLog form fields")
	}
	if path == wheelogDetailPath {
		id, err := strconv.ParseInt(fields.Get("spotId"), 10, 64)
		if err != nil || id <= 0 || id > 1000000000 {
			return nil, fmt.Errorf("spot ID must be between 1 and 1000000000")
		}
		fields = url.Values{"spotId": {strconv.FormatInt(id, 10)}}
	} else {
		page := 1
		if value := fields.Get("pagenum"); value != "" {
			page, err = strconv.Atoi(value)
			if err != nil || page < 1 || page > 5 {
				return nil, fmt.Errorf("WheeLog page must be between 1 and 5")
			}
		}
		if len([]rune(fields.Get("word"))) > 256 {
			return nil, fmt.Errorf("WheeLog query exceeds 256 characters")
		}
		safe := url.Values{"word": {fields.Get("word")}, "pagenum": {strconv.Itoa(page)}, "type": {"spot"}, "isDetail": {"1"}}
		for _, key := range []string{"startDatetime", "endDatetime", "categoryList[]", "questionList[]"} {
			for _, value := range fields[key] {
				if len(value) > 64 {
					return nil, fmt.Errorf("WheeLog filter exceeds 64 characters")
				}
				safe.Add(key, value)
			}
		}
		fields = safe
	}
	encoded := []byte(fields.Encode())
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	clone.ContentLength = int64(len(encoded))
	clone.Header.Del("Accept-Encoding")
	clone.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	for _, key := range []string{"OS_CODE", "APP_VERSION", "APP_LANGUAGE"} {
		clone.Header.Del(key)
	}
	clone.Header["OS_CODE"] = []string{"3"}
	clone.Header["APP_VERSION"] = []string{"1"}
	clone.Header["APP_LANGUAGE"] = []string{"ja"}
	clone.Header.Set("X-Requested-With", "XMLHttpRequest")
	clone.Header.Set("Referer", "https://app.wheelog.com/?la=ja")
	response, err := base.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, wheelogMaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("reading bounded WheeLog response: %w", err)
	}
	if len(raw) > wheelogMaxBody {
		return nil, fmt.Errorf("WheeLog response exceeds 4 MiB")
	}
	raw, err = decodeContentEncoding(response.Header.Get("Content-Encoding"), raw)
	if err != nil {
		return nil, fmt.Errorf("decoding WheeLog response encoding")
	}
	if len(raw) > wheelogMaxBody {
		return nil, fmt.Errorf("decoded WheeLog response exceeds 4 MiB")
	}
	var sanitized []byte
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		sanitized, err = wheelog.SanitizeEnvelope(raw, path == wheelogDetailPath)
		if err != nil {
			return nil, err
		}
	} else {
		sanitized = []byte(fmt.Sprintf(`{"error":"WheeLog returned HTTP %d"}`, response.StatusCode))
	}
	response.Body = io.NopCloser(bytes.NewReader(sanitized))
	response.ContentLength = int64(len(sanitized))
	response.Header = response.Header.Clone()
	response.Header.Del("Content-Encoding")
	response.Header.Del("Content-Length")
	response.Header.Set("Content-Type", "application/json")
	// Cookie/session headers never become a persisted source response artifact.
	for key := range response.Header {
		if strings.EqualFold(key, "Set-Cookie") {
			response.Header.Del(key)
		}
	}
	return response, nil
}
