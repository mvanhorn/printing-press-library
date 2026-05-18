// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 whisperapi paid audio adapter (provider-pluggable).

package whisperapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

const adapterName = "whisperapi"

// Provider enumerates which audio-transcription backend to use.
type Provider string

const (
	ProviderElevenLabs Provider = "elevenlabs"
	ProviderOpenAI     Provider = "openai"
	ProviderDeepgram   Provider = "deepgram"
)

// Adapter is the audio-transcription fallback. v0.1 returns a typed
// KeyMissingError or a NotImplementedError pointing at the audio-extraction
// path that ships in v0.2.
type Adapter struct {
	Client   *http.Client
	Provider Provider
}

func New() *Adapter {
	return &Adapter{
		Client:   &http.Client{Timeout: 60 * time.Second},
		Provider: ProviderElevenLabs,
	}
}

func (a *Adapter) Name() string          { return adapterName }
func (a *Adapter) Tier() transcript.Tier { return transcript.TierPaid }

// Match accepts anything; tier-10 last-resort.
func (a *Adapter) Match(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func (a *Adapter) Fetch(ctx context.Context, url string) (*transcript.Transcript, error) {
	prov := a.Provider
	if prov == "" {
		prov = ProviderElevenLabs
	}
	switch prov {
	case ProviderElevenLabs:
		if config.Resolve("ELEVENLABS_API_KEY") == "" {
			return nil, &source.KeyMissingError{EnvVar: "ELEVENLABS_API_KEY", URL: "https://elevenlabs.io/app/account"}
		}
	case ProviderOpenAI:
		if config.Resolve("OPENAI_API_KEY") == "" {
			return nil, &source.KeyMissingError{EnvVar: "OPENAI_API_KEY", URL: "https://platform.openai.com/api-keys"}
		}
	case ProviderDeepgram:
		if config.Resolve("DEEPGRAM_API_KEY") == "" {
			return nil, &source.KeyMissingError{EnvVar: "DEEPGRAM_API_KEY", URL: "https://console.deepgram.com/"}
		}
	default:
		return nil, fmt.Errorf("whisperapi: unknown provider %q", prov)
	}
	return nil, &source.NotImplementedError{
		Adapter: adapterName,
		Detail: fmt.Sprintf(
			"%s key detected, but the yt-dlp audio extract -> upload -> diarize pipeline ships in v0.2. "+
				"For now please use `--provider spoken` or `--provider taddy` for paid fallback.",
			prov,
		),
	}
}

var _ source.Adapter = (*Adapter)(nil)
