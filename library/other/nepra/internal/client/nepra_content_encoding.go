// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-decompress-content-encoding.json.

package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
)

// maxDecompressedBytes bounds a decompressed body so a malicious or
// misconfigured server cannot exhaust memory with a compression bomb. The
// largest artifact this CLI legitimately reads is a State of Industry Report
// PDF at roughly 316 MB, and the biggest HTML surface is a 493 KB generation
// workbook, so 512 MB clears real payloads with room to spare.
const maxDecompressedBytes = 512 << 20

// decodeContentEncoding decompresses a response body according to its
// Content-Encoding header.
//
// This exists because the client sets Accept-Encoding EXPLICITLY (the spec
// declares it a required header, and NEPRA silently truncates some pages when
// gzip is not requested — the Wind determinations page returns 1,028 of 2,440
// rows under an HTTP 200). Go's net/http only decompresses transparently when
// IT chose the encoding; the moment a caller sets Accept-Encoding by hand,
// net/http hands back the raw compressed stream and leaves Content-Encoding in
// place. Nothing in the generated client did that decompression, so every
// gzipped response arrived as binary.
//
// The failure was completely silent: HTTP 200, provenance "source: live", and
// an empty result. MEASURED on the FCA table — 6,019 compressed bytes reached
// the HTML parser instead of 79,843 uncompressed ones, and the command printed
// `{"results": {}}` for a page holding 53 rows and 572 cells.
//
// A body Go already decompressed is unaffected: net/http strips
// Content-Encoding when it decodes transparently, so this becomes a no-op.
func decodeContentEncoding(encoding string, body []byte) ([]byte, error) {
	enc := strings.ToLower(strings.TrimSpace(encoding))
	if enc == "" || enc == "identity" || len(body) == 0 {
		return body, nil
	}
	// Content-Encoding is a list; the last applied encoding comes last. These
	// surfaces only ever send a single encoding, so anything stacked is
	// refused rather than guessed at.
	if strings.Contains(enc, ",") {
		return nil, fmt.Errorf("unsupported stacked Content-Encoding %q", encoding)
	}
	switch enc {
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("response declared Content-Encoding %q but is not valid gzip: %w", encoding, err)
		}
		defer func() { _ = zr.Close() }()
		return readCapped(zr, encoding)
	case "deflate":
		// Servers disagree about what "deflate" means: RFC 1950 (zlib
		// wrapper) is what the HTTP spec intends, but raw RFC 1951 streams
		// are common. Try the correct one, then fall back.
		if zr, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			defer func() { _ = zr.Close() }()
			return readCapped(zr, encoding)
		}
		fr := flate.NewReader(bytes.NewReader(body))
		defer func() { _ = fr.Close() }()
		out, err := readCapped(fr, encoding)
		if err != nil {
			return nil, fmt.Errorf("response declared Content-Encoding %q but is neither zlib nor raw deflate: %w", encoding, err)
		}
		return out, nil
	default:
		// br, zstd and anything else are NOT silently passed through as if
		// they were plaintext: that is what produced an empty result under a
		// 200. The caller sees an error naming the encoding instead.
		return nil, fmt.Errorf("unsupported Content-Encoding %q (this client requests only gzip and deflate)", encoding)
	}
}

func readCapped(r io.Reader, encoding string) ([]byte, error) {
	out, err := io.ReadAll(io.LimitReader(r, maxDecompressedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompressing %q response: %w", encoding, err)
	}
	if len(out) > maxDecompressedBytes {
		return nil, fmt.Errorf("decompressed %q response exceeds the %d-byte cap", encoding, maxDecompressedBytes)
	}
	return out, nil
}
