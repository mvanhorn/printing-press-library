// Copyright 2026 zjsng. Licensed under Apache-2.0.
package client

import (
	"errors"
	"io"
	"net/http"
)

const maxHTMLResponseBytes = 5 << 20

var ErrResponseBodyTooLarge = errors.New("response body exceeds size limit")

// Declared HTML workflows have a five-MiB wire and inflation budget regardless
// of server MIME labels. Ordinary API/error bodies share the existing 32-MiB
// decoded budget. Successful generic binary/stream envelopes retain their
// original semantics; they are not HTML directory operations.
func responseBodyBudget(html, binary bool, resp *http.Response) int {
	if html {
		return maxHTMLResponseBytes
	}
	if resp.StatusCode < 400 && (binary || isBinaryResponseContentType(resp.Header.Get("Content-Type"))) {
		return 0
	}
	return maxDecodedBodyBytes
}

func decodedResponseBodyBudget(html bool) int {
	if html {
		return maxHTMLResponseBytes
	}
	return maxDecodedBodyBytes
}

func readResponseBody(r io.Reader, limit int) ([]byte, error) {
	if limit == 0 {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, ErrResponseBodyTooLarge
	}
	return data, nil
}
