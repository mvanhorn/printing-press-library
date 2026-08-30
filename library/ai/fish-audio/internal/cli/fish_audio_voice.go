// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: clone, design, promote, and discover voices.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

// fishModelPath is the voice model collection: POST creates a clone, GET lists
// the public catalog.
const fishModelPath = "/model"

// fishVoiceDesignPath generates voice candidates from an instruction.
const fishVoiceDesignPath = "/v1/voice-design"

// voiceDesignModelHeader is the model header the voice-design endpoint expects.
const voiceDesignModelHeader = "voice-design-1"

// clonedVoice is the JSON contract `voice clone` and `voice design-save` print.
type clonedVoice struct {
	ModelID    string `json:"model_id"`
	Title      string `json:"title"`
	Visibility string `json:"visibility"`
	TrainMode  string `json:"train_mode"`
	Source     string `json:"source"`
}

// modelCreateResponse is the subset of POST /model this CLI reads back.
type modelCreateResponse struct {
	ID         string `json:"_id"`
	AltID      string `json:"id"`
	Title      string `json:"title"`
	Visibility string `json:"visibility"`
	TrainMode  string `json:"train_mode"`
}

// modelID resolves the id whichever key the server used.
func (r modelCreateResponse) modelID() string {
	if strings.TrimSpace(r.ID) != "" {
		return r.ID
	}
	return r.AltID
}

// createVoiceModel posts a multipart POST /model and decodes the new model.
// Both `voice clone` and `voice design-save` land here, so the two commands
// cannot disagree about how a model is created.
func createVoiceModel(ctx context.Context, c *client.Client, fields []multipartField, files []multipartFile) (clonedVoice, error) {
	body, contentType, err := buildMultipart(fields, files)
	if err != nil {
		return clonedVoice{}, err
	}
	data, _, err := c.PostRaw(ctx, fishModelPath, body, map[string]string{"Content-Type": contentType})
	if err != nil {
		return clonedVoice{}, err
	}
	var resp modelCreateResponse
	if len(data) > 0 {
		if err := json.Unmarshal(data, &resp); err != nil {
			return clonedVoice{}, fmt.Errorf("parsing the %s response: %w", fishModelPath, err)
		}
	}
	return clonedVoice{
		ModelID:    resp.modelID(),
		Title:      resp.Title,
		Visibility: resp.Visibility,
		TrainMode:  resp.TrainMode,
	}, nil
}

func newFishVoiceCloneCmd(flags *rootFlags) *cobra.Command {
	var (
		flagTitle       string
		flagAudio       []string
		flagText        []string
		flagDescription string
		flagVisibility  string
		flagTags        []string
		flagNoEnhance   bool
	)

	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a voice from audio samples and get an instantly usable model_id",
		Long: `Uploads one or more audio samples and creates a TTS voice model.

train_mode is always fast, so the model is usable the moment it is created.
That is the iteration case this CLI is built for; the API's own entity default
is full, which is archival quality and not instantly available.

Pass --text once per --audio, in the same order, to supply the transcript of
each sample. Without it the service transcribes the samples itself, which
costs time and can misread a noisy recording.

New models are private by default. Public requests are downgraded to private
by the API; publish through the web flow instead.`,
		Example: strings.Trim(`
  fish-audio-pp-cli voice clone --title "Pearl concierge" --audio sample1.wav
  fish-audio-pp-cli voice clone --title "Pearl concierge" --audio a.wav --audio b.wav --text "First line." --text "Second line." --json
  fish-audio-pp-cli voice clone --title "Narrator" --audio read.wav --tags narration --tags calm --visibility unlist
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--title=Pearl concierge;--audio=sample.wav;--dry-run",
			"pp:typed-exit-codes": "0,2,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voice clone")
			}
			if strings.TrimSpace(flagTitle) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--title is required: name the voice model you are creating"))
			}
			if len(flagAudio) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--audio is required: pass at least one audio sample, repeat the flag for more"))
			}
			if len(flagText) > 0 && len(flagText) != len(flagAudio) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--text was passed %d time(s) for %d --audio file(s): pass one transcript per sample, in the same order, or none at all", len(flagText), len(flagAudio)))
			}
			visibility, err := fishaudio.ValidateVisibility(flagVisibility)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			fields := []multipartField{
				{Name: "type", Value: "tts"},
				{Name: "train_mode", Value: "fast"},
				{Name: "title", Value: flagTitle},
				{Name: "visibility", Value: visibility},
				{Name: "enhance_audio_quality", Value: strconv.FormatBool(!flagNoEnhance)},
			}
			if strings.TrimSpace(flagDescription) != "" {
				fields = append(fields, multipartField{Name: "description", Value: flagDescription})
			}
			for _, tag := range flagTags {
				fields = append(fields, multipartField{Name: "tags", Value: tag})
			}
			for _, text := range flagText {
				fields = append(fields, multipartField{Name: "texts", Value: text})
			}

			files := make([]multipartFile, 0, len(flagAudio))
			for _, path := range flagAudio {
				content, readErr := readUploadFile(path)
				if readErr != nil {
					return usageErr(readErr)
				}
				files = append(files, multipartFile{Name: "voices", FileName: filepath.Base(path), Content: content})
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			voice, err := createVoiceModel(ctx, c, fields, files)
			if err != nil {
				return classifyRawAPIError(err)
			}
			voice.Source = "voice clone"
			if voice.Title == "" {
				voice.Title = flagTitle
			}
			if voice.Visibility == "" {
				voice.Visibility = visibility
			}
			if voice.TrainMode == "" {
				voice.TrainMode = "fast"
			}
			return emitClonedVoice(cmd, flags, voice)
		},
	}
	cmd.Flags().StringVar(&flagTitle, "title", "", "Name of the voice model to create")
	cmd.Flags().StringArrayVar(&flagAudio, "audio", nil, "Audio sample to train the voice from; repeatable, up to 20")
	cmd.Flags().StringArrayVar(&flagText, "text", nil, "Transcript of the matching --audio sample, in the same order; repeatable")
	cmd.Flags().StringVar(&flagDescription, "description", "", "Description stored on the voice model")
	cmd.Flags().StringVar(&flagVisibility, "visibility", "private", "Who can see the model (one of: private, unlist, public)")
	cmd.Flags().StringArrayVar(&flagTags, "tags", nil, "Tag to attach to the model; repeatable")
	cmd.Flags().BoolVar(&flagNoEnhance, "no-enhance", false, "Skip the audio-quality enhancement pass on the uploaded samples")
	return cmd
}

// emitClonedVoice prints a created model through the shared output helpers.
func emitClonedVoice(cmd *cobra.Command, flags *rootFlags, voice clonedVoice) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), voice, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "created voice %s\n  title %s, visibility %s, train mode %s\n",
		orDash(voice.ModelID), orDash(voice.Title), orDash(voice.Visibility), orDash(voice.TrainMode))
	return nil
}

// designCandidate is one generated voice candidate as this CLI stores it. The
// signature is what POST /model needs to promote the candidate to a permanent
// model, so it is captured verbatim even though the published candidate schema
// does not name the field.
type designCandidate struct {
	ID         string `json:"id"`
	Index      int    `json:"index"`
	File       string `json:"file"`
	SampleRate int    `json:"sample_rate"`
	DurationMS int    `json:"duration_ms"`
	Text       string `json:"text,omitempty"`
	Language   string `json:"language,omitempty"`
	Signature  string `json:"signature,omitempty"`
}

// candidatesManifest is the candidates.json `voice design` writes and
// `voice design-save` reads back.
type candidatesManifest struct {
	Instruction   string            `json:"instruction"`
	ReferenceText string            `json:"reference_text,omitempty"`
	Language      string            `json:"language,omitempty"`
	Seed          int               `json:"seed,omitempty"`
	CreatedAt     string            `json:"created_at"`
	CostUSD       float64           `json:"cost_usd"`
	Dir           string            `json:"dir"`
	Candidates    []designCandidate `json:"candidates"`
}

// candidatesFileName is the manifest a design run leaves behind.
const candidatesFileName = "candidates.json"

func newFishVoiceDesignCmd(flags *rootFlags) *cobra.Command {
	var (
		flagInstruction          string
		flagReferenceText        string
		flagLanguage             string
		flagN                    int
		flagSpeed                float64
		flagNumStep              int
		flagGuidanceScale        float64
		flagInstructGuidanceScal float64
		flagSeed                 int
		flagOutDir               string
	)

	cmd := &cobra.Command{
		Use:   "design",
		Short: "Generate voice candidates from a written instruction and save them to disk",
		Long: `Generates candidate voices from a plain-language instruction, writes each
candidate's audio to --out-dir, and writes a candidates.json naming them.

Listen to the files, then promote the one you want with 'voice design-save
--candidates-dir <dir> --pick <n> --title <name>'. The signature captured in
candidates.json is what makes that promotion possible: the API matches it
against the uploaded audio and rejects a mismatch.

Each request costs $0.01 regardless of --n.`,
		Example: strings.Trim(`
  fish-audio-pp-cli voice design --instruction "Warm, confident studio narrator with a natural tone"
  fish-audio-pp-cli voice design --instruction "Bright, upbeat concierge" --n 4 --language en --out-dir ./candidates --json
  fish-audio-pp-cli voice design --instruction "Calm night-shift dispatcher" --reference-text "Your ride is two minutes away." --seed 42
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--instruction=Warm, confident studio narrator;--dry-run",
			"pp:typed-exit-codes": "0,2,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voice design")
			}
			if strings.TrimSpace(flagInstruction) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--instruction is required: describe the voice you want in plain language"))
			}
			if flagN < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--n must be at least 1"))
			}
			outDir := flagOutDir
			if strings.TrimSpace(outDir) == "" {
				outDir = "voice-candidates"
			}

			body := map[string]any{"instruction": flagInstruction, "n": flagN}
			if strings.TrimSpace(flagReferenceText) != "" {
				body["reference_text"] = flagReferenceText
			}
			if strings.TrimSpace(flagLanguage) != "" {
				body["language"] = flagLanguage
			}
			if flagSpeed != 0 {
				body["speed"] = flagSpeed
			}
			if flagNumStep > 0 {
				body["num_step"] = flagNumStep
			}
			if flagGuidanceScale != 0 {
				body["guidance_scale"] = flagGuidanceScale
			}
			if flagInstructGuidanceScal != 0 {
				body["instruct_guidance_scale"] = flagInstructGuidanceScal
			}
			if cmd.Flags().Changed("seed") {
				body["seed"] = flagSeed
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("encoding the voice-design request: %w", err)
			}
			data, _, err := c.PostRaw(ctx, fishVoiceDesignPath, payload, map[string]string{
				"Content-Type": client.RawContentTypeJSON,
				"model":        voiceDesignModelHeader,
			})
			if err != nil {
				return classifyRawAPIError(err)
			}

			manifest := candidatesManifest{
				Instruction:   flagInstruction,
				ReferenceText: flagReferenceText,
				Language:      flagLanguage,
				CreatedAt:     time.Now().UTC().Format(time.RFC3339),
				CostUSD:       fishaudio.VoiceDesignCost(1),
				Dir:           outDir,
				Candidates:    make([]designCandidate, 0),
			}
			if cmd.Flags().Changed("seed") {
				manifest.Seed = flagSeed
			}
			if len(data) > 0 {
				parsed, parseErr := parseDesignCandidates(data, outDir)
				if parseErr != nil {
					return parseErr
				}
				manifest.Candidates = parsed
			}
			if len(manifest.Candidates) > 0 {
				if err := os.MkdirAll(outDir, 0o700); err != nil {
					return fmt.Errorf("creating --out-dir %s: %w", outDir, err)
				}
				if err := writeCandidateFiles(data, manifest.Candidates); err != nil {
					return err
				}
				encoded, marshalErr := json.MarshalIndent(manifest, "", "  ")
				if marshalErr != nil {
					return fmt.Errorf("encoding %s: %w", candidatesFileName, marshalErr)
				}
				if err := os.WriteFile(filepath.Join(outDir, candidatesFileName), encoded, 0o600); err != nil {
					return fmt.Errorf("writing %s: %w", candidatesFileName, err)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), manifest, flags)
			}
			out := cmd.OutOrStdout()
			if len(manifest.Candidates) == 0 {
				fmt.Fprintln(out, "The voice-design request returned no candidates.")
				return nil
			}
			fmt.Fprintf(out, "%d candidate(s) in %s ($%.2f)\n", len(manifest.Candidates), outDir, manifest.CostUSD)
			w := newTabWriter(out)
			fmt.Fprintln(w, "PICK\tFILE\tDURATION\tSAMPLE RATE")
			for _, cand := range manifest.Candidates {
				fmt.Fprintf(w, "%d\t%s\t%dms\t%dHz\n", cand.Index, cand.File, cand.DurationMS, cand.SampleRate)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(out, "\npromote one with: %s voice design-save --candidates-dir %s --pick <PICK> --title \"<name>\"\n", cmd.Root().Name(), outDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagInstruction, "instruction", "", "Plain-language description of the voice to generate, 1 to 2000 characters")
	cmd.Flags().StringVar(&flagReferenceText, "reference-text", "", "Preview text the candidates speak, up to 150 characters")
	cmd.Flags().StringVar(&flagLanguage, "language", "", "Language hint for the generated voice, as an ISO 639-1 code")
	cmd.Flags().IntVar(&flagN, "n", 2, "How many candidates to generate in this one request")
	cmd.Flags().Float64Var(&flagSpeed, "speed", 0, "Speaking-rate multiplier for the candidates; 0 keeps the default")
	cmd.Flags().IntVar(&flagNumStep, "num-step", 0, "Diffusion steps per candidate; higher is slower and cleaner, 0 keeps the default")
	cmd.Flags().Float64Var(&flagGuidanceScale, "guidance-scale", 0, "How closely generation follows the reference text; 0 keeps the default")
	cmd.Flags().Float64Var(&flagInstructGuidanceScal, "instruct-guidance-scale", 0, "How closely generation follows the instruction; 0 keeps the default")
	cmd.Flags().IntVar(&flagSeed, "seed", 0, "Seed that makes a generation reproducible")
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "voice-candidates", "Directory to write the candidate audio and candidates.json to")
	return cmd
}

// parseDesignCandidates reads the candidate list without decoding the audio
// twice. The signature field is picked up defensively: the published schema
// does not name it, but POST /model requires it, so whatever key carries it is
// preserved.
func parseDesignCandidates(data []byte, outDir string) ([]designCandidate, error) {
	var envelope struct {
		Candidates []map[string]json.RawMessage `json:"candidates"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing the %s response: %w", fishVoiceDesignPath, err)
	}
	out := make([]designCandidate, 0, len(envelope.Candidates))
	for i, raw := range envelope.Candidates {
		cand := designCandidate{Index: i}
		_ = json.Unmarshal(raw["id"], &cand.ID)
		if v, ok := raw["index"]; ok {
			_ = json.Unmarshal(v, &cand.Index)
		}
		_ = json.Unmarshal(raw["sample_rate"], &cand.SampleRate)
		_ = json.Unmarshal(raw["duration_ms"], &cand.DurationMS)
		_ = json.Unmarshal(raw["text"], &cand.Text)
		_ = json.Unmarshal(raw["language"], &cand.Language)
		for _, key := range []string{"signature", "voice_design_signature", "sig"} {
			if v, ok := raw[key]; ok {
				var sig string
				if json.Unmarshal(v, &sig) == nil && sig != "" {
					cand.Signature = sig
					break
				}
			}
		}
		cand.File = filepath.Join(outDir, fmt.Sprintf("candidate-%d.wav", cand.Index))
		out = append(out, cand)
	}
	return out, nil
}

// writeCandidateFiles decodes each candidate's base64 audio to its file.
func writeCandidateFiles(data []byte, candidates []designCandidate) error {
	var envelope struct {
		Candidates []struct {
			AudioBase64 string `json:"audio_base64"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parsing the %s audio payload: %w", fishVoiceDesignPath, err)
	}
	for i, cand := range candidates {
		if i >= len(envelope.Candidates) {
			break
		}
		audio, err := base64.StdEncoding.DecodeString(envelope.Candidates[i].AudioBase64)
		if err != nil {
			return fmt.Errorf("decoding candidate %d audio: %w", cand.Index, err)
		}
		repaired, _ := fishaudio.RepairWAVHeader(audio)
		if _, err := writeAudioFile(cand.File, repaired); err != nil {
			return err
		}
	}
	return nil
}

func newFishVoiceDesignSaveCmd(flags *rootFlags) *cobra.Command {
	var (
		flagCandidatesDir string
		flagPick          int
		flagTitle         string
		flagDescription   string
		flagVisibility    string
		flagTags          []string
	)

	cmd := &cobra.Command{
		Use:   "design-save",
		Short: "Promote one voice-design candidate to a permanent voice model",
		Long: `Uploads the chosen candidate's audio together with its voice-design
signature and creates a permanent voice model.

Raw, this is a two-step, order-sensitive sequence: the signature array must
line up position by position with the uploaded voices, and an invalid signature
rejects the whole request. This command reads both out of the candidates.json
that 'voice design' wrote, so the order is never yours to get wrong.

--pick takes the PICK value from the 'voice design' table.`,
		Example: strings.Trim(`
  fish-audio-pp-cli voice design-save --candidates-dir ./voice-candidates --pick 0 --title "Pearl concierge"
  fish-audio-pp-cli voice design-save --candidates-dir ./voice-candidates --pick 1 --title "Night dispatcher" --visibility unlist --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:happy-args":       "--candidates-dir=voice-candidates;--pick=0;--title=Pearl concierge;--dry-run",
			"pp:typed-exit-codes": "0,2,3,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voice design-save")
			}
			if strings.TrimSpace(flagCandidatesDir) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--candidates-dir is required: pass the --out-dir a 'voice design' run wrote"))
			}
			if strings.TrimSpace(flagTitle) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--title is required: name the voice model you are creating"))
			}
			visibility, err := fishaudio.ValidateVisibility(flagVisibility)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			manifestPath := filepath.Join(flagCandidatesDir, candidatesFileName)
			// #nosec G304 -- manifestPath is built from the operator's own
			// --candidates-dir plus a fixed file name.
			raw, err := os.ReadFile(manifestPath)
			if err != nil {
				return notFoundErr(fmt.Errorf("reading %s: %w\nrun 'voice design --out-dir %s' first", manifestPath, err, flagCandidatesDir))
			}
			var manifest candidatesManifest
			if err := json.Unmarshal(raw, &manifest); err != nil {
				return fmt.Errorf("parsing %s: %w", manifestPath, err)
			}
			var picked *designCandidate
			for i := range manifest.Candidates {
				if manifest.Candidates[i].Index == flagPick {
					picked = &manifest.Candidates[i]
					break
				}
			}
			if picked == nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--pick %d is not in %s: it holds %d candidate(s)", flagPick, manifestPath, len(manifest.Candidates)))
			}
			content, err := readUploadFile(picked.File)
			if err != nil {
				return notFoundErr(err)
			}

			fields := []multipartField{
				{Name: "type", Value: "tts"},
				{Name: "train_mode", Value: "fast"},
				{Name: "title", Value: flagTitle},
				{Name: "visibility", Value: visibility},
			}
			if strings.TrimSpace(flagDescription) != "" {
				fields = append(fields, multipartField{Name: "description", Value: flagDescription})
			}
			for _, tag := range flagTags {
				fields = append(fields, multipartField{Name: "tags", Value: tag})
			}
			if strings.TrimSpace(manifest.ReferenceText) != "" {
				fields = append(fields, multipartField{Name: "texts", Value: manifest.ReferenceText})
			}
			if picked.Signature != "" {
				fields = append(fields, multipartField{Name: "voice_design_signatures", Value: picked.Signature})
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: candidate %d carries no voice-design signature; the model will be created as an ordinary clone, not stamped source=voice_design\n", picked.Index)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			voice, err := createVoiceModel(ctx, c, fields, []multipartFile{{
				Name:     "voices",
				FileName: filepath.Base(picked.File),
				Content:  content,
			}})
			if err != nil {
				return classifyRawAPIError(err)
			}
			voice.Source = "voice design-save"
			if voice.Title == "" {
				voice.Title = flagTitle
			}
			if voice.Visibility == "" {
				voice.Visibility = visibility
			}
			if voice.TrainMode == "" {
				voice.TrainMode = "fast"
			}
			return emitClonedVoice(cmd, flags, voice)
		},
	}
	cmd.Flags().StringVar(&flagCandidatesDir, "candidates-dir", "", "Directory holding candidates.json from a 'voice design' run")
	cmd.Flags().IntVar(&flagPick, "pick", 0, "Index of the candidate to promote, from the 'voice design' PICK column")
	cmd.Flags().StringVar(&flagTitle, "title", "", "Name of the voice model to create")
	cmd.Flags().StringVar(&flagDescription, "description", "", "Description stored on the voice model")
	cmd.Flags().StringVar(&flagVisibility, "visibility", "private", "Who can see the model (one of: private, unlist, public)")
	cmd.Flags().StringArrayVar(&flagTags, "tags", nil, "Tag to attach to the model; repeatable")
	return cmd
}

// voiceDiscoverView is the JSON contract `voice discover` prints.
type voiceDiscoverView struct {
	Query   string                  `json:"query"`
	Source  string                  `json:"source"`
	Cached  int64                   `json:"cached_voices"`
	Synced  int                     `json:"synced_voices"`
	Results []store.VoiceCatalogRow `json:"results"`
	Note    string                  `json:"note,omitempty"`
}

// voiceDiscoverSources lists the accepted --source values.
var voiceDiscoverSources = []string{"self", "public", "all"}

// discoverSyncPages bounds how many catalog pages one --refresh walks. Output
// size is bounded separately by --limit: this caps scan effort so an empty
// result is never the product of an unbounded crawl that timed out.
const discoverSyncPages = 5

// discoverPageSize is the catalog page size one sync request asks for.
const discoverPageSize = 100

func newFishVoiceDiscoverCmd(flags *rootFlags) *cobra.Command {
	var (
		flagQuery        string
		flagSource       string
		flagLimit        int
		flagRefresh      bool
		flagMaxScanPages int
		flagDB           string
	)

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Search a local cache of the voice catalog by title, description, or tag",
		Long: `Searches voices by text. Fish Audio has no server-side voice search, so this
command caches pages of the voice catalog locally and runs full-text search
against that cache.

Pass --refresh to re-sync before searching. The first run needs it; after that
the cache answers instantly and offline.

--source self limits the search to your own models, public to the shared
catalog, all to both.

--limit bounds how many matches come back; --max-scan-pages bounds how many
catalog pages a --refresh walks, so an empty result never hides an unbounded
crawl.`,
		Example: strings.Trim(`
  fish-audio-pp-cli voice discover --query "narrator" --refresh
  fish-audio-pp-cli voice discover --query "calm female english" --source public --limit 10 --json
  fish-audio-pp-cli voice discover --source self --refresh --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "voice discover")
			}
			source := strings.TrimSpace(flagSource)
			if source == "" {
				source = "all"
			}
			valid := false
			for _, s := range voiceDiscoverSources {
				if s == source {
					valid = true
					break
				}
			}
			if !valid {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid value %q for --source: must be one of %s", flagSource, strings.Join(voiceDiscoverSources, ", ")))
			}
			if flagQuery == "" && len(args) > 0 {
				flagQuery = strings.Join(args, " ")
			}

			dbPath := fishRenderDBPath(flagDB)
			if !flagRefresh {
				if stop, mirrorErr := fishMissingMirror(cmd, flags, dbPath, voiceDiscoverView{
					Query: flagQuery, Source: source, Results: make([]store.VoiceCatalogRow, 0),
					Note: "the voice cache is empty; run 'voice discover --refresh' to populate it",
				}); stop {
					return mirrorErr
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureVoiceCatalog(ctx); err != nil {
				return err
			}

			view := voiceDiscoverView{Query: flagQuery, Source: source, Results: make([]store.VoiceCatalogRow, 0)}
			if flagRefresh {
				maxPages := flagMaxScanPages
				if cliutil.IsDogfoodEnv() && maxPages > 1 {
					maxPages = 1
				}
				c, clientErr := flags.newClient()
				if clientErr != nil {
					return clientErr
				}
				synced, syncErr := syncVoiceCatalog(ctx, c, db, source, maxPages)
				if syncErr != nil {
					return syncErr
				}
				view.Synced = synced
			}

			cached, err := db.VoiceCatalogCount(ctx)
			if err != nil {
				return err
			}
			view.Cached = cached
			results, err := db.SearchVoiceCatalog(ctx, flagQuery, source, flagLimit)
			if err != nil {
				return err
			}
			view.Results = results
			if len(results) == 0 {
				if cached == 0 {
					view.Note = "the local voice cache is empty; run 'voice discover --refresh' to populate it"
				} else {
					view.Note = fmt.Sprintf("no voice in the %d cached entries matched %q; try --refresh or a broader query", cached, flagQuery)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, view.Note)
				return nil
			}
			w := newTabWriter(out)
			fmt.Fprintln(w, "MODEL ID\tTITLE\tTAGS\tSOURCE")
			for _, row := range results {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", row.ID, truncate(row.Title, 40), truncate(row.Tags, 30), row.Source)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Text to match against voice titles, descriptions, and tags")
	cmd.Flags().StringVar(&flagSource, "source", "all", "Which catalog to search (one of: self, public, all)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum matching voices to return")
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Re-sync the catalog from the API before searching")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", discoverSyncPages, "Maximum catalog pages one --refresh walks")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// syncVoiceCatalog pages GET /model into the local cache and returns how many
// rows were written.
func syncVoiceCatalog(ctx context.Context, c *client.Client, db *store.Store, source string, maxPages int) (int, error) {
	wanted := []string{"public"}
	switch source {
	case "self":
		wanted = []string{"self"}
	case "all":
		wanted = []string{"self", "public"}
	}
	written := 0
	for _, scope := range wanted {
		for page := 1; page <= maxPages; page++ {
			params := map[string]string{
				"page_size":   strconv.Itoa(discoverPageSize),
				"page_number": strconv.Itoa(page),
				"sort_by":     "created_at",
			}
			if scope == "self" {
				params["self"] = "true"
			}
			data, err := c.Get(ctx, fishModelPath, params)
			if err != nil {
				return written, apiErr(fmt.Errorf("syncing the %s voice catalog: %w", scope, err))
			}
			rows, more := parseCatalogPage(data, scope)
			count, upsertErr := db.UpsertVoiceCatalog(ctx, rows)
			if upsertErr != nil {
				return written, upsertErr
			}
			written += count
			if !more {
				break
			}
		}
	}
	return written, nil
}

// parseCatalogPage reads one GET /model page into catalog rows and reports
// whether the page was full, which is the only "there may be more" signal the
// endpoint gives.
func parseCatalogPage(data []byte, scope string) ([]store.VoiceCatalogRow, bool) {
	var envelope struct {
		Items []struct {
			ID          string   `json:"_id"`
			AltID       string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Tags        []string `json:"tags"`
			Author      struct {
				Nickname string `json:"nickname"`
			} `json:"author"`
			Languages  []string `json:"languages"`
			Visibility string   `json:"visibility"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false
	}
	rows := make([]store.VoiceCatalogRow, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		id := item.ID
		if id == "" {
			id = item.AltID
		}
		if id == "" {
			continue
		}
		rows = append(rows, store.VoiceCatalogRow{
			ID:          id,
			Title:       item.Title,
			Description: item.Description,
			Tags:        strings.Join(item.Tags, " "),
			Author:      item.Author.Nickname,
			Languages:   strings.Join(item.Languages, " "),
			Visibility:  item.Visibility,
			Source:      scope,
		})
	}
	return rows, len(envelope.Items) >= discoverPageSize
}
