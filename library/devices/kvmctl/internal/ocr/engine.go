package ocr

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrOCRUnavailable means no production OCR executable was configured.
	ErrOCRUnavailable = errors.New("ocr unavailable")
	// ErrOCRFailed means an explicitly configured OCR executable could not return valid OCR.
	ErrOCRFailed = errors.New("ocr failed")
)

const (
	defaultOCRTimeout = 5 * time.Second
	maxOCRInputBytes  = 16 << 20
	maxOCROutputBytes = 4 << 20
)

type ocrProtocol string

const (
	tesseractTSVProtocol ocrProtocol = "tesseract-tsv"
	jsonProtocol         ocrProtocol = "json"
)

// CommandEngine invokes an explicitly configured OCR executable without a shell.
// KVMCTL_OCR_COMMAND is an executable path, never a shell command line. By default
// it is called as: <path> stdin stdout tsv, which is Tesseract's stdin-to-TSV
// protocol. Set KVMCTL_OCR_PROTOCOL=json to call <path> with no arguments; that
// program must write one strict JSON object to stdout:
// {"width":1920,"height":1080,"words":[{"text":"Advanced","confidence":95.2,"x":1,"y":2,"width":3,"height":4}]}.
type CommandEngine struct {
	command  string
	protocol ocrProtocol
	timeout  time.Duration
}

// CommandEngineFromEnvironment returns a real engine only when KVMCTL_OCR_COMMAND
// names an executable. It never falls back to a globally installed OCR binary.
func CommandEngineFromEnvironment() (*CommandEngine, error) {
	command := os.Getenv("KVMCTL_OCR_COMMAND")
	if command == "" {
		return nil, ErrOCRUnavailable
	}
	protocol := ocrProtocol(os.Getenv("KVMCTL_OCR_PROTOCOL"))
	switch protocol {
	case "", "tesseract", tesseractTSVProtocol:
		protocol = tesseractTSVProtocol
	case jsonProtocol:
	default:
		return nil, fmt.Errorf("%w: unsupported OCR protocol %q", ErrOCRUnavailable, protocol)
	}
	return &CommandEngine{command: command, protocol: protocol, timeout: defaultOCRTimeout}, nil
}

// Name identifies the configured executable path without probing it or executing a shell.
func (e *CommandEngine) Name() string { return e.command }

// Recognize writes image bytes to the configured executable's standard input and
// validates bounded TSV or JSON output. It never invents OCR output on failure.
func (e *CommandEngine) Recognize(imageBytes []byte) (int, int, []Word, error) {
	if e == nil || e.command == "" {
		return 0, 0, nil, ErrOCRUnavailable
	}
	if len(imageBytes) == 0 || len(imageBytes) > maxOCRInputBytes {
		return 0, 0, nil, fmt.Errorf("%w: image input size is invalid", ErrOCRFailed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	args := []string{}
	if e.protocol == tesseractTSVProtocol {
		args = []string{"stdin", "stdout", "tsv"}
	}
	command := exec.CommandContext(ctx, e.command, args...)
	command.Stdin = bytes.NewReader(imageBytes)
	stdout := &limitedBuffer{limit: maxOCROutputBytes}
	stderr := &limitedBuffer{limit: maxOCROutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return 0, 0, nil, fmt.Errorf("%w: command timed out", ErrOCRFailed)
		}
		if stdout.exceeded || stderr.exceeded {
			return 0, 0, nil, fmt.Errorf("%w: command output exceeded limit", ErrOCRFailed)
		}
		return 0, 0, nil, fmt.Errorf("%w: command execution failed: %v", ErrOCRFailed, err)
	}
	if stdout.exceeded || stderr.exceeded {
		return 0, 0, nil, fmt.Errorf("%w: command output exceeded limit", ErrOCRFailed)
	}
	if e.protocol == jsonProtocol {
		return parseJSONResponse(stdout.Bytes())
	}
	return parseTesseractTSV(stdout.Bytes())
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 || len(p) > remaining {
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return 0, errors.New("OCR output limit exceeded")
	}
	return b.Buffer.Write(p)
}

type jsonResponse struct {
	Width  *int        `json:"width"`
	Height *int        `json:"height"`
	Words  *[]jsonWord `json:"words"`
}

type jsonWord struct {
	Text       *string  `json:"text"`
	Confidence *float64 `json:"confidence"`
	X          *int     `json:"x"`
	Y          *int     `json:"y"`
	Width      *int     `json:"width"`
	Height     *int     `json:"height"`
}

func parseJSONResponse(data []byte) (int, int, []Word, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var response jsonResponse
	if err := decoder.Decode(&response); err != nil {
		return 0, 0, nil, fmt.Errorf("%w: invalid JSON response", ErrOCRFailed)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return 0, 0, nil, fmt.Errorf("%w: invalid JSON response", ErrOCRFailed)
	}
	if response.Width == nil || response.Height == nil || response.Words == nil || *response.Width <= 0 || *response.Height <= 0 {
		return 0, 0, nil, fmt.Errorf("%w: JSON response lacks valid dimensions or words", ErrOCRFailed)
	}
	words := make([]Word, 0, len(*response.Words))
	for _, word := range *response.Words {
		if word.Text == nil || word.Confidence == nil || word.X == nil || word.Y == nil || word.Width == nil || word.Height == nil {
			return 0, 0, nil, fmt.Errorf("%w: JSON word lacks required fields", ErrOCRFailed)
		}
		words = append(words, Word{Text: *word.Text, Confidence: *word.Confidence, X: *word.X, Y: *word.Y, Width: *word.Width, Height: *word.Height})
	}
	if err := validateWords(words, *response.Width, *response.Height); err != nil {
		return 0, 0, nil, err
	}
	return *response.Width, *response.Height, words, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON data")
	}
	return nil
}

func parseTesseractTSV(data []byte) (int, int, []Word, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = '	'
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV", ErrOCRFailed)
	}
	header := []string{"level", "page_num", "block_num", "par_num", "line_num", "word_num", "left", "top", "width", "height", "conf", "text"}
	if len(records[0]) != len(header) {
		return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV header", ErrOCRFailed)
	}
	for i := range header {
		if records[0][i] != header[i] {
			return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV header", ErrOCRFailed)
		}
	}
	width, height := 0, 0
	words := make([]Word, 0)
	for _, record := range records[1:] {
		if len(record) != len(header) {
			return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV row", ErrOCRFailed)
		}
		level, err := strconv.Atoi(record[0])
		if err != nil {
			return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV level", ErrOCRFailed)
		}
		x, xErr := strconv.Atoi(record[6])
		y, yErr := strconv.Atoi(record[7])
		w, wErr := strconv.Atoi(record[8])
		h, hErr := strconv.Atoi(record[9])
		if xErr != nil || yErr != nil || wErr != nil || hErr != nil {
			return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV bounds", ErrOCRFailed)
		}
		if level == 1 {
			width, height = w, h
			continue
		}
		if level != 5 || strings.TrimSpace(record[11]) == "" {
			continue
		}
		confidence, err := strconv.ParseFloat(record[10], 64)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("%w: invalid Tesseract TSV confidence", ErrOCRFailed)
		}
		words = append(words, Word{Text: record[11], Confidence: confidence, X: x, Y: y, Width: w, Height: h})
	}
	if err := validateWords(words, width, height); err != nil {
		return 0, 0, nil, err
	}
	return width, height, words, nil
}

func validateWords(words []Word, width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: invalid image dimensions", ErrOCRFailed)
	}
	for _, word := range words {
		if word.X < 0 || word.Y < 0 || word.Width <= 0 || word.Height <= 0 || word.X+word.Width > width || word.Y+word.Height > height {
			return fmt.Errorf("%w: word outside image bounds", ErrOCRFailed)
		}
	}
	return nil
}
