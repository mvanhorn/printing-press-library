package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/elevenlabs/internal/client"
	"github.com/spf13/cobra"
)

const defaultElevenLabsVoiceID = "JBFqnCBsd6RMkjVDRZzb"

type elevenVoice struct {
	VoiceID  string         `json:"voice_id"`
	Name     string         `json:"name,omitempty"`
	Category string         `json:"category,omitempty"`
	Labels   map[string]any `json:"labels,omitempty"`
	Source   string         `json:"source,omitempty"`
}

type elevenModel struct {
	ModelID             string `json:"model_id"`
	Name                string `json:"name,omitempty"`
	CanDoTextToSpeech   bool   `json:"can_do_text_to_speech,omitempty"`
	CanDoTextToDialogue bool   `json:"can_do_text_to_dialogue,omitempty"`
}

type renderManifest struct {
	Path         string        `json:"path,omitempty"`
	Bytes        int           `json:"bytes,omitempty"`
	OutputFormat string        `json:"output_format"`
	ModelID      string        `json:"model_id"`
	Voice        *elevenVoice  `json:"voice,omitempty"`
	Voices       []elevenVoice `json:"voices,omitempty"`
	TextChars    int           `json:"text_chars,omitempty"`
	LineCount    int           `json:"line_count,omitempty"`
	Endpoint     string        `json:"endpoint"`
	DryRun       bool          `json:"dry_run,omitempty"`
	Timestamps   any           `json:"timestamps,omitempty"`
}

func newVoiceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Agent-friendly voice discovery and selection",
	}
	cmd.AddCommand(newVoiceDiscoverCmd(flags))
	return cmd
}

func newVoiceDiscoverCmd(flags *rootFlags) *cobra.Command {
	var source, search, language, gender, category, voiceType string
	var limit int

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find usable voices across owned and shared ElevenLabs voice catalogs",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			voices, err := discoverVoices(c, source, search, language, gender, category, voiceType, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, map[string]any{
				"count":  len(voices),
				"voices": voices,
			})
		},
	}
	cmd.Flags().StringVar(&source, "source", "owned", "Voice source: owned, shared, or all")
	cmd.Flags().StringVar(&search, "search", "", "Search by voice name or description")
	cmd.Flags().StringVar(&language, "language", "", "Language filter")
	cmd.Flags().StringVar(&gender, "gender", "", "Gender label filter")
	cmd.Flags().StringVar(&category, "category", "", "Category filter")
	cmd.Flags().StringVar(&voiceType, "voice-type", "", "Voice type filter")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum voices to return")
	return cmd
}

func newTTSCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tts",
		Short: "Agent-friendly text-to-speech workflows",
	}
	cmd.AddCommand(newTTSResolveCmd(flags))
	cmd.AddCommand(newTTSRenderCmd(flags))
	return cmd
}

func newTTSResolveCmd(flags *rootFlags) *cobra.Command {
	var voice, model, language, outputFormat string

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a voice and model before rendering speech",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resolvedVoice, err := resolveVoice(c, voice)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			modelID, models, err := resolveModel(c, model, true)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			settings, _ := c.Get("/v1/voices/settings/default", nil)
			subscription, _ := c.Get("/v1/user/subscription", nil)
			return flags.printJSON(cmd, map[string]any{
				"voice":                  resolvedVoice,
				"model_id":               modelID,
				"output_format":          outputFormat,
				"language_code":          language,
				"candidate_model_count":  len(models),
				"default_voice_settings": json.RawMessage(settings),
				"subscription":           json.RawMessage(subscription),
			})
		},
	}
	cmd.Flags().StringVar(&voice, "voice", defaultElevenLabsVoiceID, "Voice ID, exact name, or search query")
	cmd.Flags().StringVar(&model, "model", "auto", "Model ID or auto")
	cmd.Flags().StringVar(&language, "language-code", "", "Optional language code")
	cmd.Flags().StringVar(&outputFormat, "output-format", "mp3_44100_128", "Audio output format")
	return cmd
}

func newTTSRenderCmd(flags *rootFlags) *cobra.Command {
	var voice, text, textFile, model, language, outputFormat, out string
	var timestamps bool

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render speech to an audio file and print a machine-readable manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if textFile != "" {
				data, err := os.ReadFile(textFile)
				if err != nil {
					return err
				}
				text = string(data)
			}
			if strings.TrimSpace(text) == "" && !dryRunOK(flags) {
				return usageErr(fmt.Errorf("--text or --text-file is required"))
			}
			if out == "" {
				out = "elevenlabs-tts." + extensionForOutputFormat(outputFormat)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resolvedVoice, err := resolveVoice(c, voice)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			modelID, _, err := resolveModel(c, model, true)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			endpoint := "/v1/text-to-speech/" + resolvedVoice.VoiceID
			if timestamps {
				endpoint += "/with-timestamps"
			}
			manifest := renderManifest{
				Path:         out,
				OutputFormat: outputFormat,
				ModelID:      modelID,
				Voice:        &resolvedVoice,
				TextChars:    len([]rune(text)),
				Endpoint:     endpoint,
				DryRun:       dryRunOK(flags),
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, manifest)
			}
			body := map[string]any{"text": text, "model_id": modelID}
			if language != "" {
				body["language_code"] = language
			}
			params := map[string]string{"output_format": outputFormat}
			data, _, err := c.PostWithParams(endpoint, params, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if timestamps {
				audio, timing, err := audioFromTimestampResponse(data)
				if err != nil {
					return err
				}
				data = audio
				manifest.Timestamps = timing
			}
			if err := writeAudioFile(out, data); err != nil {
				return err
			}
			manifest.Bytes = len(data)
			return flags.printJSON(cmd, manifest)
		},
	}
	cmd.Flags().StringVar(&voice, "voice", defaultElevenLabsVoiceID, "Voice ID, exact name, or search query")
	cmd.Flags().StringVar(&text, "text", "", "Text to render")
	cmd.Flags().StringVar(&textFile, "text-file", "", "Read text from a file")
	cmd.Flags().StringVar(&model, "model", "eleven_v3", "Model ID or auto")
	cmd.Flags().StringVar(&language, "language-code", "", "Optional language code")
	cmd.Flags().StringVar(&outputFormat, "output-format", "mp3_44100_128", "Audio output format")
	cmd.Flags().StringVar(&out, "out", "", "Audio output path")
	cmd.Flags().BoolVar(&timestamps, "timestamps", false, "Use the timestamp endpoint and save returned base64 audio")
	return cmd
}

func newDialogueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dialogue",
		Short: "Agent-friendly multi-speaker dialogue workflows",
	}
	cmd.AddCommand(newDialogueCastCmd(flags))
	return cmd
}

func newDialogueCastCmd(flags *rootFlags) *cobra.Command {
	var lines, casts []string
	var model, language, outputFormat, out string
	var timestamps bool

	cmd := &cobra.Command{
		Use:   "cast",
		Short: "Render speaker-labelled lines using resolved ElevenLabs voices",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(lines) == 0 && !dryRunOK(flags) {
				return usageErr(fmt.Errorf("at least one --line speaker=text value is required"))
			}
			if out == "" {
				out = "elevenlabs-dialogue." + extensionForOutputFormat(outputFormat)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			modelID, _, err := resolveModel(c, model, true)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			castMap := parseAssignments(casts)
			inputs := make([]map[string]any, 0, len(lines))
			voices := make([]elevenVoice, 0, len(lines))
			for _, line := range lines {
				speaker, text, ok := strings.Cut(line, "=")
				if !ok || strings.TrimSpace(speaker) == "" || strings.TrimSpace(text) == "" {
					return usageErr(fmt.Errorf("--line values must use speaker=text"))
				}
				voiceQuery := castMap[strings.TrimSpace(speaker)]
				if voiceQuery == "" {
					voiceQuery = strings.TrimSpace(speaker)
				}
				resolvedVoice, err := resolveVoice(c, voiceQuery)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				voices = append(voices, resolvedVoice)
				inputs = append(inputs, map[string]any{
					"text":     strings.TrimSpace(text),
					"voice_id": resolvedVoice.VoiceID,
				})
			}
			endpoint := "/v1/text-to-dialogue"
			if timestamps {
				endpoint += "/with-timestamps"
			}
			manifest := renderManifest{
				Path:         out,
				OutputFormat: outputFormat,
				ModelID:      modelID,
				Voices:       voices,
				LineCount:    len(lines),
				Endpoint:     endpoint,
				DryRun:       dryRunOK(flags),
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, manifest)
			}
			body := map[string]any{"inputs": inputs, "model_id": modelID}
			if language != "" {
				body["language_code"] = language
			}
			data, _, err := c.PostWithParams(endpoint, map[string]string{"output_format": outputFormat}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if timestamps {
				audio, timing, err := audioFromTimestampResponse(data)
				if err != nil {
					return err
				}
				data = audio
				manifest.Timestamps = timing
			}
			if err := writeAudioFile(out, data); err != nil {
				return err
			}
			manifest.Bytes = len(data)
			return flags.printJSON(cmd, manifest)
		},
	}
	cmd.Flags().StringArrayVar(&lines, "line", nil, "Dialogue line as speaker=text; repeat for multiple turns")
	cmd.Flags().StringArrayVar(&casts, "cast", nil, "Speaker voice assignment as speaker=voice_id_or_query; repeat as needed")
	cmd.Flags().StringVar(&model, "model", "eleven_v3", "Model ID or auto")
	cmd.Flags().StringVar(&language, "language-code", "", "Optional language code")
	cmd.Flags().StringVar(&outputFormat, "output-format", "mp3_44100_128", "Audio output format")
	cmd.Flags().StringVar(&out, "out", "", "Audio output path")
	// PATCH: Dialogue cast should use the non-timestamp endpoint unless explicitly requested.
	cmd.Flags().BoolVar(&timestamps, "timestamps", false, "Use the timestamp endpoint and save returned base64 audio")
	return cmd
}

func discoverVoices(c *client.Client, source, search, language, gender, category, voiceType string, limit int) ([]elevenVoice, error) {
	if limit <= 0 {
		limit = 10
	}
	voices := []elevenVoice{}
	switch source {
	case "owned", "all", "":
		params := map[string]string{"page_size": fmt.Sprintf("%d", limit)}
		addParam(params, "search", search)
		addParam(params, "language", language)
		addParam(params, "gender", gender)
		addParam(params, "category", category)
		addParam(params, "voice_type", voiceType)
		data, err := c.Get("/v2/voices", params)
		if err != nil {
			return nil, err
		}
		voices = append(voices, parseVoices(data, "owned")...)
	case "shared":
	default:
		return nil, usageErr(fmt.Errorf("--source must be owned, shared, or all"))
	}
	if source == "shared" || source == "all" {
		params := map[string]string{"page_size": fmt.Sprintf("%d", limit)}
		addParam(params, "search", search)
		addParam(params, "language", language)
		addParam(params, "gender", gender)
		addParam(params, "category", category)
		data, err := c.Get("/v1/shared-voices", params)
		if err != nil {
			return nil, err
		}
		voices = append(voices, parseVoices(data, "shared")...)
	}
	if len(voices) > limit {
		voices = voices[:limit]
	}
	return voices, nil
}

func resolveVoice(c *client.Client, query string) (elevenVoice, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		query = defaultElevenLabsVoiceID
	}
	if looksLikeVoiceID(query) {
		return elevenVoice{VoiceID: query}, nil
	}
	voices, err := discoverVoices(c, "owned", query, "", "", "", "", 10)
	if err != nil {
		return elevenVoice{}, err
	}
	for _, voice := range voices {
		if strings.EqualFold(voice.Name, query) {
			return voice, nil
		}
	}
	if len(voices) == 0 {
		return elevenVoice{}, notFoundErr(fmt.Errorf("no voice matched %q", query))
	}
	return voices[0], nil
}

func resolveModel(c *client.Client, requested string, tts bool) (string, []elevenModel, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" && requested != "auto" {
		return requested, nil, nil
	}
	data, err := c.Get("/v1/models", nil)
	if err != nil {
		return "", nil, err
	}
	var models []elevenModel
	if err := json.Unmarshal(extractResponseData(data), &models); err != nil {
		return "eleven_v3", models, nil
	}
	for _, model := range models {
		if model.ModelID == "eleven_v3" && (!tts || model.CanDoTextToSpeech) {
			return model.ModelID, models, nil
		}
	}
	for _, model := range models {
		if !tts || model.CanDoTextToSpeech {
			return model.ModelID, models, nil
		}
	}
	return "eleven_v3", models, nil
}

func parseVoices(data json.RawMessage, source string) []elevenVoice {
	var envelope struct {
		Voices []elevenVoice `json:"voices"`
	}
	_ = json.Unmarshal(extractResponseData(data), &envelope)
	for i := range envelope.Voices {
		envelope.Voices[i].Source = source
	}
	return envelope.Voices
}

func addParam(params map[string]string, key, value string) {
	if value != "" {
		params[key] = value
	}
}

func looksLikeVoiceID(value string) bool {
	return len(value) >= 16 && !strings.ContainsAny(value, " \t\n")
}

func parseAssignments(values []string) map[string]string {
	out := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if ok {
			out[strings.TrimSpace(key)] = strings.TrimSpace(val)
		}
	}
	return out
}

func audioFromTimestampResponse(data json.RawMessage) ([]byte, any, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, err
	}
	audioValue, _ := payload["audio_base64"].(string)
	if audioValue == "" {
		return nil, nil, fmt.Errorf("timestamp response did not include audio_base64")
	}
	audio, err := base64.StdEncoding.DecodeString(audioValue)
	if err != nil {
		return nil, nil, err
	}
	delete(payload, "audio_base64")
	return audio, payload, nil
}

func writeAudioFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func extensionForOutputFormat(format string) string {
	switch {
	case strings.HasPrefix(format, "wav_"):
		return "wav"
	case strings.HasPrefix(format, "pcm_"):
		return "pcm"
	case strings.HasPrefix(format, "opus_"):
		return "opus"
	case strings.HasPrefix(format, "ulaw_"), strings.HasPrefix(format, "alaw_"):
		return "wav"
	default:
		return "mp3"
	}
}
