// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrEmptyResponse = errors.New("empty batchexecute response")
	ErrNoFrames      = errors.New("no wrb.fr frames in batchexecute response")
	xssiPrefix       = regexp.MustCompile(`^\)\]\}'\r?\n`)
)

// StripXSSIPrefix removes Google's )]}' anti-XSSI prefix and following newline.
func StripXSSIPrefix(body string) string {
	body = strings.TrimSpace(body)
	if loc := xssiPrefix.FindStringIndex(body); loc != nil {
		return body[loc[1]:]
	}
	if strings.HasPrefix(body, ")]}'") {
		return strings.TrimSpace(body[4:])
	}
	return body
}

// Frame is one decoded batchexecute response frame.
type Frame struct {
	RPCID   string
	Payload json.RawMessage
	Raw     []any
}

// ParseFrames decodes a batchexecute response into wrb.fr frames.
func ParseFrames(body string) ([]Frame, error) {
	body = StripXSSIPrefix(body)
	if body == "" {
		return nil, ErrEmptyResponse
	}

	chunks, err := parseChunkedResponse(body)
	if err == nil && len(chunks) > 0 {
		return framesFromChunks(chunks)
	}

	// Fallback: single JSON array (non-chunked responses).
	var outer []json.RawMessage
	if err := json.Unmarshal([]byte(body), &outer); err != nil {
		return nil, fmt.Errorf("decode batchexecute response: %w", err)
	}
	return framesFromChunks(rawMessagesToAny(outer))
}

func parseChunkedResponse(body string) ([]any, error) {
	lines := strings.Split(body, "\n")
	var chunks []any
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		i++
		if line == "" {
			continue
		}
		if _, err := strconv.Atoi(line); err == nil {
			if i >= len(lines) {
				break
			}
			payload := strings.TrimSpace(lines[i])
			i++
			if payload == "" {
				continue
			}
			var chunk any
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil {
				chunks = append(chunks, chunk)
			}
			continue
		}
		// Bare JSON line without byte-count prefix.
		var chunk any
		if err := json.Unmarshal([]byte(line), &chunk); err == nil {
			chunks = append(chunks, chunk)
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks parsed")
	}
	return chunks, nil
}

func framesFromChunks(chunks []any) ([]Frame, error) {
	var frames []Frame
	var walk func(any)
	walk = func(node any) {
		row, ok := node.([]any)
		if !ok {
			return
		}
		if len(row) >= 3 {
			if tag, _ := row[0].(string); tag == "wrb.fr" {
				rpcid, _ := row[1].(string)
				payload := frameData(row)
				frames = append(frames, Frame{RPCID: rpcid, Payload: payload, Raw: row})
				return
			}
		}
		for _, item := range row {
			walk(item)
		}
	}
	for _, chunk := range chunks {
		walk(chunk)
	}
	if len(frames) == 0 {
		return nil, ErrNoFrames
	}
	return frames, nil
}

func rawMessagesToAny(outer []json.RawMessage) []any {
	out := make([]any, len(outer))
	for i, raw := range outer {
		var v any
		_ = json.Unmarshal(raw, &v)
		out[i] = v
	}
	return out
}

// DecodeInnerJSON unwraps the doubly-encoded string payload common in batchexecute.
func DecodeInnerJSON(payload json.RawMessage) (json.RawMessage, error) {
	if len(payload) == 0 || string(payload) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(payload, &s); err == nil {
		if s == "" {
			return nil, nil
		}
		return json.RawMessage(s), nil
	}
	return payload, nil
}

// ExtractRPCResult returns the last non-null decoded payload for an RPC id.
func ExtractRPCResult(frames []Frame, rpcid string) (json.RawMessage, error) {
	var last json.RawMessage
	var seen bool
	for _, frame := range frames {
		if frame.RPCID != rpcid {
			continue
		}
		seen = true
		if len(frame.Raw) >= 3 {
			if tag, _ := frame.Raw[0].(string); tag == "er" {
				code := frame.Raw[2]
				return nil, fmt.Errorf("batchexecute %s: rpc error %v", rpcid, code)
			}
		}
		inner, err := DecodeInnerJSON(frame.Payload)
		if err != nil {
			return nil, err
		}
		if len(inner) == 0 || string(inner) == "null" {
			continue
		}
		last = inner
	}
	if !seen {
		return nil, fmt.Errorf("batchexecute %s: response missing frame", rpcid)
	}
	if len(last) == 0 {
		return nil, fmt.Errorf("batchexecute %s: null result", rpcid)
	}
	return last, nil
}

// frameData reads the batchexecute payload slot from a wrb.fr frame.
func frameData(row []any) json.RawMessage {
	if len(row) <= 2 {
		return nil
	}
	switch v := row[2].(type) {
	case string:
		if v == "" {
			return nil
		}
		return json.RawMessage(v)
	default:
		if v == nil {
			return nil
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return b
	}
}

// framePayload reads the batchexecute payload slot from a wrb.fr frame.
// Google services usually place the JSON string at index 2; some fixtures use index 3.
func framePayload(row []any) json.RawMessage {
	if len(row) > 2 {
		if s, ok := row[2].(string); ok && s != "" {
			return json.RawMessage(s)
		}
	}
	if len(row) > 3 {
		if s, ok := row[3].(string); ok && s != "" {
			return json.RawMessage(s)
		}
		if row[3] != nil {
			b, _ := json.Marshal(row[3])
			return b
		}
	}
	return nil
}
