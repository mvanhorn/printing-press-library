// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package fishaudio

import "math"

const (
	// TTSPaidRatePerMillionBytes is the paid TTS rate: $15 per 1,000,000
	// UTF-8 bytes of input text.
	TTSPaidRatePerMillionBytes = 15.0

	// FreeModel bills at $0. Every render still records the paid-rate
	// equivalent so a later tier change does not erase the real value of the
	// work that was done for free.
	FreeModel = "s2.1-pro-free"

	// ASRRatePerAudioHour is the ASR rate: $0.36 per hour of audio, billed on
	// the duration rounded to the nearest second.
	ASRRatePerAudioHour = 0.36

	// VoiceDesignRatePerRequest is the flat voice-design charge per request.
	VoiceDesignRatePerRequest = 0.01
)

// TTSCost returns the billed cost and the paid-rate equivalent for a render.
// bytesIn is the UTF-8 byte length of the input text. On the free model the
// billed cost is zero while the paid equivalent still reflects what the same
// render would have cost on a paid model.
func TTSCost(bytesIn int, model string) (costUSD float64, paidEquivUSD float64) {
	if bytesIn < 0 {
		bytesIn = 0
	}
	paidEquivUSD = roundCents(float64(bytesIn) / 1_000_000.0 * TTSPaidRatePerMillionBytes)
	if model == FreeModel {
		return 0, paidEquivUSD
	}
	return paidEquivUSD, paidEquivUSD
}

// ASRCost returns the cost of transcribing durationSeconds of audio. The
// vendor bills on the duration rounded to the nearest second.
func ASRCost(durationSeconds float64) float64 {
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	seconds := math.Round(durationSeconds)
	return roundCents(seconds / 3600.0 * ASRRatePerAudioHour)
}

// VoiceDesignCost returns the cost of n voice-design requests.
func VoiceDesignCost(requests int) float64 {
	if requests < 0 {
		requests = 0
	}
	return roundCents(float64(requests) * VoiceDesignRatePerRequest)
}

// roundCents keeps money values from carrying float noise into JSON output.
// Six decimals, not two: a single short render can legitimately cost a
// fraction of a cent, and truncating it to zero would make the render log
// under-report a batch total.
func roundCents(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
