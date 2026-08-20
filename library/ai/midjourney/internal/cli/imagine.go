// Copyright 2026 Dave Fano and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Local Printing Press patch: submit Midjourney web-app jobs through the
// sniffed /api/submit-jobs endpoint.

package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/midjourney/internal/cliutil"

	"github.com/spf13/cobra"
)

type submitJobFlags struct {
	userID       string
	channelID    string
	speed        string
	private      bool
	version      string
	profile      string
	styleRefs    []string
	omniRefs     []string
	imagePrompts []string
	imageAliases []string
	aspectRatio  string
	stylize      string
	imageWeight  string
	omniWeight   string
	chaos        string
	weird        string
	seed         string
	quality      string
	style        string
	niji         string
	raw          bool
	hd           bool
	draft        bool
	tile         bool
	browser      bool
	browserCDP   string
}

type submitJobsRequest struct {
	F         submitJobRequestFlags `json:"f"`
	ChannelID string                `json:"channelId"`
	Metadata  submitJobMetadata     `json:"metadata"`
	T         string                `json:"t"`
	Prompt    string                `json:"prompt,omitempty"`
	NewPrompt *string               `json:"newPrompt,omitempty"`
	ID        string                `json:"id,omitempty"`
}

type submitJobRequestFlags struct {
	Mode    string `json:"mode"`
	Private bool   `json:"private"`
}

type submitJobMetadata struct {
	IsMobile            any `json:"isMobile"`
	ImagePrompts        any `json:"imagePrompts"`
	ImageReferences     any `json:"imageReferences"`
	CharacterReferences any `json:"characterReferences"`
	DepthReferences     any `json:"depthReferences"`
	LightboxOpen        any `json:"lightboxOpen"`
}

func newImagineCmd(flags *rootFlags) *cobra.Command {
	var submit submitJobFlags
	cmd := &cobra.Command{
		Use:   "imagine <prompt>",
		Short: "Submit a Midjourney imagine job",
		Long:  "Submit a Midjourney imagine job through the sniffed Midjourney web-app API, including image prompts, style references, and omni references.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelID, err := submit.resolveChannelID()
			if err != nil {
				return usageErr(err)
			}
			if err := submit.validate(); err != nil {
				return usageErr(err)
			}
			prompt, err := submit.buildPrompt(strings.Join(args, " "))
			if err != nil {
				return usageErr(err)
			}
			payload := submitJobsRequest{
				F:         submitJobRequestFlags{Mode: submit.speed, Private: submit.private},
				ChannelID: channelID,
				Metadata:  submit.metadata(),
				T:         "imagine",
				Prompt:    prompt,
			}
			return postSubmitJob(cmd, flags, payload, submit.browser, submit.browserCDP)
		},
	}
	addSubmitJobFlags(cmd, &submit)
	cmd.Flags().StringVar(&submit.version, "version", "7", "Midjourney model version to append as --v; empty disables")
	cmd.Flags().StringVar(&submit.profile, "profile-id", "", "Personalization profile to append as --profile; use auto to reuse --user-id")
	cmd.Flags().StringArrayVar(&submit.styleRefs, "sref", nil, "Style reference URL/code to append as --sref (repeatable)")
	cmd.Flags().StringArrayVar(&submit.omniRefs, "oref", nil, "Omni reference URL/code to append as --oref (repeatable)")
	cmd.Flags().StringArrayVar(&submit.imagePrompts, "image-prompt", nil, "Image prompt URL to prepend to the prompt (repeatable)")
	cmd.Flags().StringArrayVar(&submit.imageAliases, "image", nil, "Alias for --image-prompt")
	cmd.Flags().StringVar(&submit.aspectRatio, "ar", "", "Aspect ratio to append as --ar, for example 16:9")
	cmd.Flags().StringVar(&submit.stylize, "stylize", "", "Stylize value to append as --stylize")
	cmd.Flags().StringVar(&submit.imageWeight, "iw", "", "Image prompt weight to append as --iw")
	cmd.Flags().StringVar(&submit.omniWeight, "ow", "", "Omni reference weight to append as --ow")
	cmd.Flags().StringVar(&submit.chaos, "chaos", "", "Chaos value to append as --chaos")
	cmd.Flags().StringVar(&submit.weird, "weird", "", "Weird value to append as --weird")
	cmd.Flags().StringVar(&submit.seed, "seed", "", "Seed to append as --seed")
	cmd.Flags().StringVar(&submit.quality, "quality", "", "Quality to append as --q")
	cmd.Flags().StringVar(&submit.style, "style", "", "Style value to append as --style, for example raw or cute")
	cmd.Flags().StringVar(&submit.niji, "niji", "", "Niji model version to append as --niji; when set, --version is ignored")
	cmd.Flags().BoolVar(&submit.raw, "raw", false, "Append --style raw")
	cmd.Flags().BoolVar(&submit.hd, "hd", false, "Append --hd")
	cmd.Flags().BoolVar(&submit.draft, "draft", false, "Append --draft")
	cmd.Flags().BoolVar(&submit.tile, "tile", false, "Append --tile")
	return cmd
}

func newRerunCmd(flags *rootFlags) *cobra.Command {
	var submit submitJobFlags
	cmd := &cobra.Command{
		Use:   "rerun <job-id>",
		Short: "Rerun a Midjourney job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelID, err := submit.resolveChannelID()
			if err != nil {
				return usageErr(err)
			}
			if err := submit.validate(); err != nil {
				return usageErr(err)
			}
			payload := submitJobsRequest{
				F:         submitJobRequestFlags{Mode: submit.speed, Private: submit.private},
				ChannelID: channelID,
				Metadata: submitJobMetadata{
					IsMobile:            nil,
					ImagePrompts:        nil,
					ImageReferences:     nil,
					CharacterReferences: nil,
					DepthReferences:     nil,
					LightboxOpen:        nil,
				},
				T:         "reroll",
				NewPrompt: nil,
				ID:        strings.TrimSpace(args[0]),
			}
			return postSubmitJob(cmd, flags, payload, submit.browser, submit.browserCDP)
		},
	}
	addSubmitJobFlags(cmd, &submit)
	return cmd
}

func addSubmitJobFlags(cmd *cobra.Command, submit *submitJobFlags) {
	cmd.Flags().StringVar(&submit.userID, "user-id", "", "Midjourney user id; defaults to MIDJOURNEY_USER_ID")
	cmd.Flags().StringVar(&submit.channelID, "channel-id", "", "Midjourney channel id; defaults to singleplayer_<user-id>")
	cmd.Flags().StringVar(&submit.speed, "speed", "fast", "Generation speed: fast, relax, or turbo")
	cmd.Flags().BoolVar(&submit.private, "private", false, "Submit as private")
	cmd.Flags().BoolVar(&submit.browser, "browser", true, "Submit through the logged-in Chrome CDP browser session")
	cmd.Flags().StringVar(&submit.browserCDP, "browser-cdp", "http://127.0.0.1:18800", "Chrome DevTools Protocol URL for --browser")
}

func (f *submitJobFlags) resolveChannelID() (string, error) {
	if f.channelID = strings.TrimSpace(f.channelID); f.channelID != "" {
		return f.channelID, nil
	}
	userID := strings.TrimSpace(f.userID)
	if userID == "" {
		userID = strings.TrimSpace(os.Getenv("MIDJOURNEY_USER_ID"))
	}
	if userID == "" {
		return "", fmt.Errorf("--user-id or MIDJOURNEY_USER_ID is required")
	}
	if strings.HasPrefix(userID, "singleplayer_") {
		return userID, nil
	}
	return "singleplayer_" + userID, nil
}

func (f *submitJobFlags) validate() error {
	switch strings.TrimSpace(f.speed) {
	case "fast", "relax", "turbo":
		return nil
	default:
		return fmt.Errorf("invalid --speed %q: must be fast, relax, or turbo", f.speed)
	}
}

func (f *submitJobFlags) buildPrompt(base string) (string, error) {
	imagePrompts := f.allImagePrompts()
	parts := make([]string, 0, 8+len(imagePrompts)+len(f.styleRefs)+len(f.omniRefs))
	for _, imagePrompt := range imagePrompts {
		if imagePrompt = strings.TrimSpace(imagePrompt); imagePrompt != "" {
			parts = append(parts, imagePrompt)
		}
	}
	if base = strings.TrimSpace(base); base == "" {
		return "", fmt.Errorf("prompt is required")
	}
	parts = append(parts, base)
	appendParam := func(name, value string) {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, "--"+name, value)
		}
	}
	appendParam("ar", f.aspectRatio)
	for _, ref := range f.styleRefs {
		appendParam("sref", ref)
	}
	for _, ref := range f.omniRefs {
		appendParam("oref", ref)
	}
	if strings.TrimSpace(f.profile) == "auto" {
		profile := strings.TrimSpace(f.userID)
		if profile == "" {
			profile = strings.TrimSpace(os.Getenv("MIDJOURNEY_USER_ID"))
		}
		appendParam("profile", strings.TrimPrefix(profile, "singleplayer_"))
	} else {
		appendParam("profile", f.profile)
	}
	if strings.TrimSpace(f.niji) == "" {
		appendParam("v", f.version)
	}
	appendParam("stylize", f.stylize)
	appendParam("iw", f.imageWeight)
	appendParam("ow", f.omniWeight)
	appendParam("chaos", f.chaos)
	appendParam("weird", f.weird)
	appendParam("seed", f.seed)
	appendParam("q", f.quality)
	if strings.TrimSpace(f.style) != "" {
		appendParam("style", f.style)
	} else if f.raw {
		parts = append(parts, "--style", "raw")
	}
	if strings.TrimSpace(f.niji) != "" {
		appendParam("niji", f.niji)
	}
	if f.hd {
		parts = append(parts, "--hd")
	}
	if f.draft {
		parts = append(parts, "--draft")
	}
	if f.tile {
		parts = append(parts, "--tile")
	}
	return strings.Join(parts, " "), nil
}

func (f *submitJobFlags) metadata() submitJobMetadata {
	return submitJobMetadata{
		IsMobile:            nil,
		ImagePrompts:        countOrZero(f.allImagePrompts()),
		ImageReferences:     countOrZero(f.styleRefs),
		CharacterReferences: countOrZero(f.omniRefs),
		DepthReferences:     0,
		LightboxOpen:        nil,
	}
}

func (f *submitJobFlags) allImagePrompts() []string {
	if len(f.imageAliases) == 0 {
		return f.imagePrompts
	}
	values := make([]string, 0, len(f.imagePrompts)+len(f.imageAliases))
	values = append(values, f.imagePrompts...)
	values = append(values, f.imageAliases...)
	return values
}

func countOrZero(values []string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func postSubmitJob(cmd *cobra.Command, flags *rootFlags, payload submitJobsRequest, useBrowser bool, browserCDP string) error {
	if cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv() {
		data, err := submitJobVerifyShortCircuitEnvelope(payload.T)
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	if useBrowser && !flags.dryRun {
		data, err := browserSubmitJob(cmd.Context(), browserCDP, payload)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	data, _, err := c.Post(cmd.Context(), "/api/submit-jobs", raw)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

func submitJobVerifyShortCircuitEnvelope(jobType string) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"__pp_verify_synthetic__": true,
		"status":                  "noop",
		"reason":                  "verify_short_circuit",
		"method":                  "POST",
		"path":                    "/api/submit-jobs",
		"job_type":                jobType,
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

type cdpTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type cdpEvalResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func browserSubmitJob(ctx context.Context, cdpBase string, payload submitJobsRequest) (json.RawMessage, error) {
	target, err := findMidjourneyCDPTarget(ctx, cdpBase)
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadLiteral := jsStringLiteral(string(payloadJSON))
	expression := fmt.Sprintf("fetch('/api/submit-jobs', {method: 'POST', headers: {'accept': 'application/json', 'content-type': 'application/json', 'x-csrf-protection': '1'}, credentials: 'include', body: %s}).then(async r => ({ok: r.ok, status: r.status, text: await r.text()}))", payloadLiteral)
	eval := map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expression,
			"awaitPromise":  true,
			"returnByValue": true,
		},
	}
	response, err := cdpRoundTrip(ctx, target.WebSocketDebuggerURL, eval)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result struct {
			Result struct {
				Value struct {
					OK     bool   `json:"ok"`
					Status int    `json:"status"`
					Text   string `json:"text"`
				} `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return nil, err
	}
	value := envelope.Result.Result.Value
	if value.Status < 200 || value.Status >= 300 {
		return nil, fmt.Errorf("POST /api/submit-jobs returned HTTP %d: %s", value.Status, value.Text)
	}
	if json.Valid([]byte(value.Text)) {
		return json.RawMessage(value.Text), nil
	}
	return json.Marshal(map[string]any{
		"status": value.Status,
		"body":   value.Text,
	})
}

func jsStringLiteral(value string) string {
	value = strings.NewReplacer("\u2028", "\\u2028", "\u2029", "\\u2029").Replace(value)
	return strconv.Quote(value)
}

func findMidjourneyCDPTarget(ctx context.Context, cdpBase string) (*cdpTarget, error) {
	cdpBase = strings.TrimRight(cdpBase, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdpBase+"/json", nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to Chrome CDP at %s: %w", cdpBase, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Chrome CDP %s returned HTTP %d", cdpBase, res.StatusCode)
	}
	var targets []cdpTarget
	if err := json.NewDecoder(res.Body).Decode(&targets); err != nil {
		return nil, err
	}
	for _, target := range targets {
		if target.Type == "page" && strings.Contains(target.URL, "midjourney.com/jobs/") && target.WebSocketDebuggerURL != "" {
			return &target, nil
		}
	}
	for _, target := range targets {
		if target.Type == "page" && strings.Contains(target.URL, "midjourney.com") && target.WebSocketDebuggerURL != "" {
			return &target, nil
		}
	}
	return nil, fmt.Errorf("no open Midjourney page found in Chrome CDP at %s", cdpBase)
}

func cdpRoundTrip(ctx context.Context, wsURL string, message any) (json.RawMessage, error) {
	conn, reader, err := dialLocalWebSocket(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	body, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	if err := writeWebSocketFrame(conn, body); err != nil {
		return nil, err
	}
	for {
		opcode, payload, err := readWebSocketFrame(reader)
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 1:
			var response cdpEvalResponse
			if err := json.Unmarshal(payload, &response); err != nil {
				continue
			}
			if response.ID != 1 {
				continue
			}
			if response.Error != nil {
				return nil, fmt.Errorf("Chrome CDP error %d: %s", response.Error.Code, response.Error.Message)
			}
			return payload, nil
		case 8:
			return nil, fmt.Errorf("Chrome CDP websocket closed")
		case 9:
			_ = writeWebSocketControlFrame(conn, 10, payload)
		}
	}
}

func dialLocalWebSocket(ctx context.Context, wsURL string) (net.Conn, *bufio.Reader, error) {
	parsed, err := url.Parse(wsURL)
	if err != nil {
		return nil, nil, err
	}
	if parsed.Scheme != "ws" {
		return nil, nil, fmt.Errorf("unsupported Chrome CDP websocket scheme %q", parsed.Scheme)
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, nil, err
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	crlf := string([]byte{13, 10})
	request := strings.Join([]string{
		"GET " + path + " HTTP/1.1",
		"Host: " + parsed.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Key: " + key,
		"Sec-WebSocket-Version: 13",
	}, crlf) + crlf + crlf
	if _, err := io.WriteString(conn, request); err != nil {
		conn.Close()
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if !strings.Contains(status, " 101 ") {
		conn.Close()
		return nil, nil, fmt.Errorf("Chrome CDP websocket upgrade failed: %s", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
		if line == crlf {
			break
		}
	}
	return conn, reader, nil
}

func writeWebSocketFrame(w io.Writer, payload []byte) error {
	return writeWebSocketFrameWithOpcode(w, 1, payload)
}

func writeWebSocketControlFrame(w io.Writer, opcode byte, payload []byte) error {
	return writeWebSocketFrameWithOpcode(w, opcode, payload)
}

func writeWebSocketFrameWithOpcode(w io.Writer, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	switch {
	case len(payload) < 126:
		header = append(header, 0x80|byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 0x80|127, 0, 0, 0, 0, byte(len(payload)>>24), byte(len(payload)>>16), byte(len(payload)>>8), byte(len(payload)))
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(mask); err != nil {
		return err
	}
	_, err := w.Write(masked)
	return err
}

func readWebSocketFrame(r *bufio.Reader) (byte, []byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	opcode := first & 0x0f
	masked := second&0x80 != 0
	length := uint64(second & 0x7f)
	switch length {
	case 126:
		b, err := readN(r, 2)
		if err != nil {
			return 0, nil, err
		}
		length = uint64(b[0])<<8 | uint64(b[1])
	case 127:
		b, err := readN(r, 8)
		if err != nil {
			return 0, nil, err
		}
		length = 0
		for _, part := range b {
			length = length<<8 | uint64(part)
		}
	}
	var mask []byte
	if masked {
		mask, err = readN(r, 4)
		if err != nil {
			return 0, nil, err
		}
	}
	if length > uint64(math.MaxInt) {
		return 0, nil, fmt.Errorf("websocket frame too large: %d bytes", length)
	}
	payload, err := readN(r, int(length))
	if err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func readN(r *bufio.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(r, buf)
	return buf, err
}
