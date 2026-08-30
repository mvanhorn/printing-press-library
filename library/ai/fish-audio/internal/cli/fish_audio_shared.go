// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored support code for the Fish Audio novel commands.
// pp:data-source computed
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
	"github.com/vmihailenco/msgpack/v5"
)

// fishCLIName is the store scope every Fish Audio command shares, so a render
// written by `tts render` is the same row `render log` reads back.
const fishCLIName = "fish-audio-pp-cli"

// fishRenderDBPath resolves the --db flag against the default data directory.
func fishRenderDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath(fishCLIName)
}

// fishMissingMirror is the missing-mirror guard for the render-log commands. A
// database that does not exist yet is an empty local state, not a failure: the
// machine surface gets a valid empty result and the human surface gets the
// command that would populate it.
//
// It reports whether the caller should stop.
func fishMissingMirror(cmd *cobra.Command, flags *rootFlags, dbPath string, empty any) (bool, error) {
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		return false, nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "no local render log at %s\nrun: %s tts render --text \"...\" --voice <model_id> --out out.mp3 --db %s\n",
		dbPath, cmd.Root().Name(), dbPath)
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return true, printJSONFiltered(cmd.OutOrStdout(), empty, flags)
	}
	return true, nil
}

// fishSince turns a --since value into an RFC3339 lower bound. It accepts a
// loose duration ("30d", "12h") or an absolute date ("2026-08-01"), because
// both read naturally on a spend report.
func fishSince(since string) (string, error) {
	s := strings.TrimSpace(since)
	if s == "" {
		return "", nil
	}
	if d, err := cliutil.ParseDurationLoose(s); err == nil && d > 0 {
		return time.Now().UTC().Add(-d).Format(time.RFC3339), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("invalid --since value %q: use a duration (30d, 12h) or a date (2026-08-01)", since)
}

// renderManifest is the JSON contract `tts render` prints and `tts batch`
// collects per line.
type renderManifest struct {
	ID               int64   `json:"id"`
	File             string  `json:"file"`
	BytesIn          int64   `json:"bytes_in"`
	BytesOut         int64   `json:"bytes_out"`
	SHA256           string  `json:"sha256"`
	Model            string  `json:"model"`
	Voice            string  `json:"voice"`
	Format           string  `json:"format"`
	CostUSD          float64 `json:"cost_usd"`
	CostUSDPaidEquiv float64 `json:"cost_usd_paid_equiv"`
	Skipped          bool    `json:"skipped"`
	// WAVHeaderRepaired is true when the streamed WAV arrived with a zeroed
	// frame count and this command rewrote it. Surfaced so a caller chasing a
	// player bug can see the fix happened.
	WAVHeaderRepaired bool `json:"wav_header_repaired,omitempty"`
}

// fishTTSPath is the synthesis endpoint every render rides.
const fishTTSPath = "/v1/tts"

// encodeRenderBody serializes a render request and names its content type.
// Inline reference audio forces MessagePack: the API rejects raw bytes inside
// JSON, and base64 is not what the field expects.
func encodeRenderBody(req fishaudio.RenderRequest) ([]byte, string, error) {
	if req.NeedsMsgpack() {
		body, err := msgpack.Marshal(req.Body())
		if err != nil {
			return nil, "", fmt.Errorf("encoding msgpack body: %w", err)
		}
		return body, client.RawContentTypeMsgpack, nil
	}
	body, err := json.Marshal(req.Body())
	if err != nil {
		return nil, "", fmt.Errorf("encoding JSON body: %w", err)
	}
	return body, client.RawContentTypeJSON, nil
}

// synthesize posts one render and returns the audio bytes. The WAV frame-count
// repair runs here so every caller (single render, batch worker, voice verify)
// writes a playable file.
func synthesize(ctx context.Context, c *client.Client, req fishaudio.RenderRequest) ([]byte, bool, error) {
	body, contentType, err := encodeRenderBody(req)
	if err != nil {
		return nil, false, err
	}
	headers := map[string]string{
		"Content-Type": contentType,
		"model":        req.Model,
	}
	audio, _, err := c.PostRaw(ctx, fishTTSPath, body, headers)
	if err != nil {
		return nil, false, err
	}
	if req.Format == "wav" {
		repaired, changed := fishaudio.RepairWAVHeader(audio)
		return repaired, changed, nil
	}
	return audio, false, nil
}

// writeAudioFile writes audio to path, creating parent directories, and
// returns its SHA-256. The digest is what makes a render log row verifiable
// after the fact: a file that was replaced no longer matches its row.
func writeAudioFile(path string, audio []byte) (string, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("creating output directory %s: %w", dir, err)
		}
	}
	// #nosec G703 -- path is the operator's own --out value; writing the rendered
	// audio where the caller asked is the purpose of the command.
	if err := os.WriteFile(path, audio, 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	sum := sha256.Sum256(audio)
	return hex.EncodeToString(sum[:]), nil
}

// openRenderStore opens the local store and makes sure render_log exists.
func openRenderStore(ctx context.Context, dbPath string) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := s.EnsureRenderLog(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// readTextInput resolves --text and --text-file into the text to render.
// Exactly one of the two must be set.
func readTextInput(text, textFile string) (string, error) {
	hasText := strings.TrimSpace(text) != ""
	hasFile := strings.TrimSpace(textFile) != ""
	switch {
	case hasText && hasFile:
		return "", fmt.Errorf("pass either --text or --text-file, not both")
	case hasFile:
		// #nosec G304 -- textFile is the operator's own --text-file value; reading it
		// is the flag's purpose.
		data, err := os.ReadFile(textFile)
		if err != nil {
			return "", fmt.Errorf("reading --text-file %s: %w", textFile, err)
		}
		return string(data), nil
	case hasText:
		return text, nil
	default:
		return "", fmt.Errorf("--text or --text-file is required")
	}
}

// classifyRawAPIError maps a PostRaw failure onto the CLI's typed exit codes.
// 429 becomes exit 7 so a batch driver can back off instead of retrying blind.
func classifyRawAPIError(err error) error {
	var upstream *client.APIError
	if As(err, &upstream) {
		switch {
		case upstream.StatusCode == 429:
			return rateLimitErr(err)
		case upstream.StatusCode == 401 || upstream.StatusCode == 403:
			return authErr(err)
		case upstream.StatusCode == 404:
			return notFoundErr(err)
		case upstream.StatusCode == 402:
			return apiErr(fmt.Errorf("%w\nhint: this call bills against Fish Audio dev API credit, which is separate from the web-app subscription credit."+
				"\n      Check both ledgers with: fish-audio-pp-cli wallet balance"+
				"\n      Top up at https://fish.audio/app/developers, or use --model s2.1-pro-free for TTS renders (ASR and cloning are always billed).", err))
		}
	}
	return apiErr(err)
}

// multipartField is one name/value pair of a multipart form. A slice, not a
// map, because several Fish Audio fields repeat: `voices`, `texts`, `tags`,
// and `voice_design_signatures` each appear once per uploaded voice and their
// order is what pairs them up.
type multipartField struct {
	Name  string
	Value string
}

// multipartFile is one uploaded file part.
type multipartFile struct {
	Name     string
	FileName string
	Content  []byte
}

// buildMultipart encodes a multipart/form-data body that preserves field
// order and allows repeated names. The generated client's encoder takes maps,
// so it can neither repeat a name nor guarantee order.
func buildMultipart(fields []multipartField, files []multipartFile) ([]byte, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, field := range fields {
		if err := writer.WriteField(field.Name, field.Value); err != nil {
			_ = writer.Close()
			return nil, "", fmt.Errorf("writing multipart field %q: %w", field.Name, err)
		}
	}
	for _, file := range files {
		part, err := writer.CreateFormFile(file.Name, file.FileName)
		if err != nil {
			_ = writer.Close()
			return nil, "", fmt.Errorf("creating multipart file field %q: %w", file.Name, err)
		}
		if _, err := part.Write(file.Content); err != nil {
			_ = writer.Close()
			return nil, "", fmt.Errorf("writing multipart file field %q: %w", file.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finalizing multipart body: %w", err)
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

// maxUploadBytes caps one uploaded sample. Voice clone samples are short; a
// larger file is a mistake worth naming before it is sent over the wire.
const maxUploadBytes = 64 << 20 // 64 MiB

// readUploadFile reads a file destined for a multipart upload, refusing one
// that is implausibly large.
func readUploadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if info.Size() > maxUploadBytes {
		return nil, fmt.Errorf("%s is %d bytes, over the %d byte upload limit", path, info.Size(), int64(maxUploadBytes))
	}
	// #nosec G304 -- path names the sample the operator chose to upload; the size
	// ceiling above is the guard that matters here.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return data, nil
}
