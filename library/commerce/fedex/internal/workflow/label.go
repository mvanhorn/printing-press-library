// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/secureio"
)

const MaxPDFLabelBytes int64 = 10 << 20

var (
	ErrLabelMissing   = errors.New("PDF label is missing")
	ErrLabelMalformed = errors.New("PDF label base64 is malformed")
	ErrLabelTooLarge  = errors.New("PDF label exceeds size limit")
	ErrLabelNotPDF    = errors.New("decoded label is not a PDF")
)

// DecodePDFLabel decodes one base64 label while bounding decoded memory. A
// non-positive limit selects MaxPDFLabelBytes. The decoded artifact must begin
// with the PDF file signature.
func DecodePDFLabel(encoded string, maxBytes int64) ([]byte, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, ErrLabelMissing
	}
	if maxBytes <= 0 {
		maxBytes = MaxPDFLabelBytes
	}
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
	decoded, err := io.ReadAll(io.LimitReader(decoder, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLabelMalformed, err)
	}
	if int64(len(decoded)) > maxBytes {
		return nil, fmt.Errorf("%w: maximum is %d bytes", ErrLabelTooLarge, maxBytes)
	}
	if !bytes.HasPrefix(decoded, []byte("%PDF-")) {
		return nil, ErrLabelNotPDF
	}
	return decoded, nil
}

// WritePDFLabelAtomic validates and atomically writes a PDF label. secureio
// enforces owner-only file mode and rejects unsafe target paths/symlinks.
func WritePDFLabelAtomic(path, encoded string, maxBytes int64) error {
	decoded, err := DecodePDFLabel(encoded, maxBytes)
	if err != nil {
		return err
	}
	if err := secureio.WriteFileAtomic(path, decoded); err != nil {
		return fmt.Errorf("writing PDF label: %w", err)
	}
	return nil
}
