// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Gmail batch HTTP API client. messages.list returns only ids;
// this is the only way to hydrate N messages without N round-trips.
package gmailmail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RawSender is the slice of the generated client the batch fetcher needs.
type RawSender interface {
	SendRaw(ctx context.Context, method, path string, params map[string]string, body []byte, contentType string, headers map[string]string) (json.RawMessage, int, error)
}

// DefaultMetadataHeaders are the high-gravity headers pull stores for every
// message: identity, threading, and the List-Unsubscribe / List-Id pair that
// powers the unsub audit.
var DefaultMetadataHeaders = []string{
	"From", "To", "Cc", "Subject", "Date",
	"Message-ID", "In-Reply-To", "References",
	"List-Unsubscribe", "List-Id",
}

// batchChunkSize stays well under Google's 50-call ceiling for Gmail. The
// batch endpoint fires every sub-request at once against the per-user quota,
// and messages.get costs 20 units each, so large chunks throttle themselves:
// a 40-wide chunk of format=full gets a third of its parts 429'd in practice.
const batchChunkSize = 20

// batchRetries bounds how many times throttled sub-requests are re-sent.
const batchRetries = 4

// BatchGetMessages hydrates message metadata (or full bodies) for ids via the
// /batch/gmail/v1 multipart endpoint, chunked at batchChunkSize per request.
// format is "metadata", "full", "minimal", or "raw". Results preserve no
// particular order. Individual per-part failures are skipped; the error return
// covers transport-level failures only, and skipped counts are reported.
func BatchGetMessages(ctx context.Context, c RawSender, ids []string, format string, metadataHeaders []string) ([]Message, int, error) {
	var out []Message
	// Track what came back so throttled ids can be retried rather than
	// silently dropped: an empty result and a rate-limited result must never
	// look the same to the caller.
	pending := append([]string(nil), ids...)
	terminalFailures := 0
	for attempt := 0; attempt <= batchRetries; attempt++ {
		if len(pending) == 0 {
			break
		}
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s.
			delay := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return out, len(pending) + terminalFailures, ctx.Err()
			case <-time.After(delay):
			}
		}
		var throttled []string
		for start := 0; start < len(pending); start += batchChunkSize {
			end := start + batchChunkSize
			if end > len(pending) {
				end = len(pending)
			}
			msgs, retry, term, err := batchGetChunk(ctx, c, pending[start:end], format, metadataHeaders)
			if err != nil {
				return out, len(pending) + terminalFailures, err
			}
			out = append(out, msgs...)
			throttled = append(throttled, retry...)
			terminalFailures += term
		}
		pending = throttled
	}
	return out, len(pending) + terminalFailures, nil
}

// batchGetChunk returns the messages it recovered plus the ids whose
// sub-request was rate-limited and should be retried.
func batchGetChunk(ctx context.Context, c RawSender, ids []string, format string, metadataHeaders []string) ([]Message, []string, int, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i, id := range ids {
		ph := textproto.MIMEHeader{}
		ph.Set("Content-Type", "application/http")
		ph.Set("Content-ID", fmt.Sprintf("<item-%d>", i))
		part, err := w.CreatePart(ph)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("building batch part: %w", err)
		}
		q := url.Values{}
		if format != "" {
			q.Set("format", format)
		}
		for _, h := range metadataHeaders {
			q.Add("metadataHeaders", h)
		}
		reqLine := fmt.Sprintf("GET /gmail/v1/users/me/messages/%s?%s HTTP/1.1\r\n\r\n", url.PathEscape(id), q.Encode())
		if _, err := part.Write([]byte(reqLine)); err != nil {
			return nil, nil, 0, fmt.Errorf("writing batch part: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, nil, 0, fmt.Errorf("closing batch body: %w", err)
	}

	contentType := "multipart/mixed; boundary=" + w.Boundary()
	resp, status, err := c.SendRaw(ctx, "POST", "/batch/gmail/v1", nil, body.Bytes(), contentType, map[string]string{"Accept": "multipart/mixed"})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("batch request: %w", err)
	}
	if status < 200 || status >= 300 {
		return nil, nil, 0, fmt.Errorf("batch request returned HTTP %d", status)
	}
	msgs, throttledIdx, terminal, err := parseBatchResponseIndexed([]byte(resp))
	if err != nil {
		return nil, nil, 0, err
	}
	// Map throttled part positions back to the ids that produced them.
	var retry []string
	for _, i := range throttledIdx {
		if i >= 0 && i < len(ids) {
			retry = append(retry, ids[i])
		}
	}
	return msgs, retry, terminal, nil
}

// binaryEnvelope mirrors the generated client's base64 wrapper. The client
// treats any non-text Content-Type as binary, and multipart/mixed qualifies,
// so a batch response arrives base64-encoded inside this envelope rather than
// as raw multipart bytes.
type binaryEnvelope struct {
	PPBinary bool   `json:"_pp_binary"`
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

// unwrapBinaryEnvelope returns the decoded payload when body is a client
// binary envelope, and body unchanged otherwise.
func unwrapBinaryEnvelope(body []byte) []byte {
	trimmed := bytes.TrimLeft(body, "\r\n \t")
	if !bytes.HasPrefix(trimmed, []byte("{")) {
		return body
	}
	var env binaryEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil || !env.PPBinary || env.Data == "" {
		return body
	}
	decoded, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return body
	}
	return decoded
}

// ParseBatchResponse splits a multipart/mixed batch response into messages.
// The second return is the count of parts that did not yield a message.
func ParseBatchResponse(body []byte) ([]Message, int, error) {
	msgs, throttled, terminal, err := parseBatchResponseIndexed(body)
	return msgs, len(throttled) + terminal, err
}

// parseBatchResponseIndexed additionally reports the positions of parts whose
// sub-request was rate-limited, so the caller can retry exactly those ids.
// The boundary is recovered from the body's first delimiter line, since the
// generated client does not expose response headers.
func parseBatchResponseIndexed(body []byte) ([]Message, []int, int, error) {
	body = unwrapBinaryEnvelope(body)
	trimmed := bytes.TrimLeft(body, "\r\n \t")
	if !bytes.HasPrefix(trimmed, []byte("--")) {
		// Not multipart: either a top-level error envelope or a single JSON body.
		var single Message
		if err := json.Unmarshal(trimmed, &single); err == nil && single.ID != "" {
			return []Message{single}, nil, 0, nil
		}
		return nil, nil, 0, fmt.Errorf("unexpected batch response shape (no multipart boundary)")
	}
	firstLine, _, _ := bytes.Cut(trimmed, []byte("\n"))
	boundary := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(firstLine)), "--"), "--")
	if boundary == "" {
		return nil, nil, 0, fmt.Errorf("could not recover multipart boundary from batch response")
	}

	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	var out []Message
	var retryable []int
	terminal := 0
	for idx := 0; ; idx++ {
		p, err := mr.NextPart()
		if err != nil {
			break // io.EOF or trailing garbage: both end the walk
		}
		msg, status, ok := parseBatchPart(p)
		switch {
		case ok:
			out = append(out, msg)
		case isRetryableStatus(status):
			// The batch endpoint runs every sub-request at once against the
			// per-user quota, so a chunk can throttle itself. These are not
			// missing messages; they need to be asked for again.
			retryable = append(retryable, idx)
		default:
			// Terminal for this part (400, 404, malformed). Re-sending would
			// spend quota and backoff to arrive at the same answer, so count
			// it as failed now instead of feeding it back into the retry set.
			terminal++
		}
	}
	return out, retryable, terminal, nil
}

// isRetryableStatus reports whether an inner sub-request status warrants a retry.
func isRetryableStatus(status int) bool {
	return status == 429 || status == 403 || (status >= 500 && status < 600)
}

// parseBatchPart reads one application/http part: an HTTP status line,
// headers, then the JSON body. It returns the inner status code so callers can
// distinguish throttling from a genuine failure.
func parseBatchPart(p *multipart.Part) (Message, int, bool) {
	r := bufio.NewReader(p)
	statusLine, err := r.ReadString('\n')
	if err != nil {
		return Message{}, 0, false
	}
	// "HTTP/1.1 200 OK" -> fields[1] is the status code. A line that does not
	// have one is malformed; treat that part as failed rather than guessing.
	fields := strings.Fields(statusLine)
	status := 0
	if len(fields) >= 2 {
		status, _ = strconv.Atoi(fields[1])
	}
	ok2xx := status >= 200 && status < 300
	// Skip inner HTTP headers up to the blank line.
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return Message{}, status, false
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return Message{}, status, false
	}
	if !ok2xx {
		return Message{}, status, false
	}
	var msg Message
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &msg); err != nil || msg.ID == "" {
		return Message{}, status, false
	}
	return msg, status, true
}
