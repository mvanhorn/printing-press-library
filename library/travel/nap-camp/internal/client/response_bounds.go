// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package client

import (
	"errors"
	"io"
)

// Nap Camp planning and raw public contracts share the same finite transport
// envelope. Reading one byte past the cap distinguishes exact-size EOF from a
// response that must fail; a partial body is never returned as successful data.
const maxResponseBodyBytes = 4 << 20

var ErrResponseBodyTooLarge = errors.New("response exceeds the 4 MiB transport bound")

func readResponseBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, int64(maxResponseBodyBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxResponseBodyBytes {
		return nil, ErrResponseBodyTooLarge
	}
	return body, nil
}
