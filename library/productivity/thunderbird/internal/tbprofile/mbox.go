package tbprofile

import (
	"bufio"
	"bytes"
	"crypto/sha1" // #nosec G505 -- ShortHash is a content id, not a security primitive
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// X-Mozilla-Status flag bits.
const (
	StatusRead      = 0x0001
	StatusReplied   = 0x0002
	StatusFlagged   = 0x0004
	StatusExpunged  = 0x0008
	StatusForwarded = 0x1000
)

// RawMessage is one mbox message: Offset/Length cover the RFC822 bytes, Start the "From " line.
type RawMessage struct {
	Start  int64
	Offset int64
	Length int64
	Data   []byte
}

// ErrMessageMoved reports that a stored offset no longer holds the stored message.
var ErrMessageMoved = errors.New("message moved inside its mbox")

var fromLineRE = regexp.MustCompile(`^From (- |\S+ +(Mon|Tue|Wed|Thu|Fri|Sat|Sun) |-?\r?\n$)`)

// IsFromLine reports whether line (with its terminator) is an mbox separator.
func IsFromLine(line []byte) bool {
	return bytes.HasPrefix(line, []byte("From ")) && fromLineRE.Match(line)
}

func isBlankLine(line []byte) bool {
	return len(line) == 1 && line[0] == '\n' || len(line) == 2 && line[0] == '\r' && line[1] == '\n'
}

// ScanMbox streams the mbox at path starting at byte offset start and calls
// fn for every message, expunged ones included. It returns the offset where
// the scan stopped (the file size when fn never errors).
func ScanMbox(path string, start int64, fn func(RawMessage) error) (int64, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return start, err
	}
	defer f.Close()
	prevBlank := true
	if start > 0 {
		prevBlank = blankLineBefore(f, start)
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return start, err
		}
	}
	return scanMboxReader(bufio.NewReaderSize(f, 1<<20), start, prevBlank, fn)
}

// blankLineBefore seeds a resumed scan so it splits messages exactly like a scan from 0.
func blankLineBefore(f io.ReaderAt, off int64) bool {
	n := min(off, 3)
	b := make([]byte, n)
	if _, err := f.ReadAt(b, off-n); err != nil {
		return false
	}
	if b[n-1] != '\n' {
		return false
	}
	if n == 1 || b[n-2] == '\n' {
		return true
	}
	return b[n-2] == '\r' && (n == 2 || b[n-3] == '\n')
}

// BoundaryAt reports whether offset is EOF, a blank line or a "From " line.
func BoundaryAt(path string, offset int64) (bool, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	defer f.Close()
	b := make([]byte, 256)
	n, err := f.ReadAt(b, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	b = b[:n]
	if n == 0 {
		return true, nil
	}
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[:i+1]
	}
	return isBlankLine(b) || IsFromLine(b), nil
}

// ReadMessageAt returns ErrMessageMoved on another Message-ID or, without one, no "From " line before offset.
func ReadMessageAt(path string, offset, length int64, messageID string) ([]byte, error) {
	raw, err := ReadRaw(path, offset, length)
	if err != nil {
		return nil, err
	}
	if messageID != "" {
		h, _ := SplitMessage(raw)
		if NormalizeMessageID(h.Get("Message-Id")) != messageID {
			return nil, ErrMessageMoved
		}
		return raw, nil
	}
	ok, err := fromLineBefore(path, offset)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrMessageMoved
	}
	return raw, nil
}

func fromLineBefore(path string, offset int64) (bool, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return false, err
	}
	defer f.Close()
	n := min(offset, 512)
	if n == 0 {
		return false, nil
	}
	b := make([]byte, n)
	if _, err := f.ReadAt(b, offset-n); err != nil {
		return false, err
	}
	if b[n-1] != '\n' {
		return false, nil
	}
	line := b
	if i := bytes.LastIndexByte(b[:n-1], '\n'); i >= 0 {
		line = b[i+1:]
	} else if n < offset {
		return false, nil
	}
	return IsFromLine(line), nil
}

func scanMboxReader(r *bufio.Reader, start int64, prevBlank bool, fn func(RawMessage) error) (int64, error) {
	pos := start
	msgStart, fromStart := int64(-1), int64(-1)
	var buf, long []byte
	var lastBlankLen int64
	for {
		line, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			long = append(long[:0], line...)
			for errors.Is(err, bufio.ErrBufferFull) {
				line, err = r.ReadSlice('\n')
				long = append(long, line...)
			}
			line = long
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return pos, err
		}
		if len(line) == 0 {
			break
		}
		if prevBlank && IsFromLine(line) {
			if msgStart >= 0 {
				if e := emit(fn, fromStart, msgStart, pos, buf, prevBlank, lastBlankLen); e != nil {
					return pos, e
				}
			}
			fromStart = pos
			msgStart = pos + int64(len(line))
			buf = buf[:0]
		} else if msgStart >= 0 {
			buf = append(buf, line...)
		}
		prevBlank = isBlankLine(line)
		if prevBlank {
			lastBlankLen = int64(len(line))
		}
		pos += int64(len(line))
		if err != nil {
			break
		}
	}
	if msgStart >= 0 {
		if e := emit(fn, fromStart, msgStart, pos, buf, prevBlank, lastBlankLen); e != nil {
			return pos, e
		}
	}
	return pos, nil
}

func emit(fn func(RawMessage) error, fromStart, msgStart, end int64, buf []byte, trailingBlank bool, blankLen int64) error {
	n := end - msgStart
	data := buf
	if trailingBlank && n >= blankLen && int64(len(data)) >= blankLen {
		n -= blankLen
		data = data[:int64(len(data))-blankLen]
	}
	if n <= 0 {
		return nil
	}
	return fn(RawMessage{Start: fromStart, Offset: msgStart, Length: n, Data: data})
}

// ReadRaw returns length bytes at offset of an mbox file.
func ReadRaw(mboxPath string, offset, length int64) ([]byte, error) {
	if offset < 0 || length <= 0 {
		return nil, fmt.Errorf("invalid mbox range offset=%d length=%d", offset, length)
	}
	f, err := os.Open(filepath.Clean(mboxPath))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, length)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return nil, fmt.Errorf("reading message at offset %d: %w", offset, err)
	}
	return buf, nil
}

// MozillaStatus parses an X-Mozilla-Status hex value.
func MozillaStatus(v string) uint32 {
	n, err := strconv.ParseUint(strings.TrimSpace(v), 16, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}

// NormalizeMessageID strips angle brackets and whitespace from a Message-ID.
func NormalizeMessageID(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '<'); i >= 0 {
		if j := strings.IndexByte(s[i:], '>'); j > 0 {
			return strings.TrimSpace(s[i+1 : i+j])
		}
	}
	return strings.Trim(s, "<> \t")
}

var msgIDTokenRE = regexp.MustCompile(`<([^<>\s]+)>`)

// ParseMessageIDList extracts the <id> tokens of a References header.
func ParseMessageIDList(s string) []string {
	out := make([]string, 0)
	for _, m := range msgIDTokenRE.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		for _, f := range strings.Fields(s) {
			out = append(out, strings.Trim(f, "<>"))
		}
	}
	return out
}

// ThreadRoot returns the root Message-ID of a message's thread: the first
// References entry, else In-Reply-To, else the message's own Message-ID.
func ThreadRoot(messageID, inReplyTo string, references []string) string {
	for _, r := range references {
		if r != "" {
			return r
		}
	}
	if inReplyTo != "" {
		return inReplyTo
	}
	return messageID
}

// ShortHash returns 12 hex chars of sha1(parts joined by "|"), avoiding
// values that parse as a zero number.
func ShortHash(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, "|"))) // #nosec G401 -- non-security id; another hash would rekey every stored message
	h := hex.EncodeToString(sum[:])
	for i := 0; i+12 <= len(h); i += 12 {
		if f, err := strconv.ParseFloat(h[i:i+12], 64); err != nil || f != 0 {
			return h[i : i+12]
		}
	}
	return "m" + h[:11]
}

// MessageKey is the stable store id of a message.
func MessageKey(account, folderPath, messageID string, offset int64) string {
	if messageID == "" {
		messageID = "offset:" + strconv.FormatInt(offset, 10)
	}
	return ShortHash(account, folderPath, messageID)
}

// ThreadID is the stable id of the thread rooted at rootMessageID.
func ThreadID(rootMessageID string) string {
	return ShortHash("thread", rootMessageID)
}
