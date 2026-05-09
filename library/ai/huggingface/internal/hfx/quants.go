package hfx

import (
	"regexp"
	"strings"
)

// QuantPattern represents a recognized GGUF quantization label.
type QuantPattern struct {
	Code   string // canonical label, e.g. "Q4_K_M", "IQ4_NL", "UD-Q4_K_XL"
	Family string // grouping: "K", "I", "UD", "F", "Legacy", "MXFP"
	Bits   int    // approx bits-per-weight (best-effort)
}

// quantRegex matches a GGUF quant suffix in a filename. Order matters: more
// specific patterns first.
var quantRegex = regexp.MustCompile(
	`(?i)\b(?:UD-(?:Q\d_K_(?:XL|L|M|S))|(?:IQ\d_(?:NL|XS|S|M|XXS|))|(?:Q\d_K_(?:M|S|L|XL))|(?:Q\d_(?:0|1))|MXFP\d|F16|BF16|F32)\b`,
)

// DetectQuant returns the recognized quant code from a filename or sibling
// path. Returns ("", false) when no recognized pattern is present (e.g.
// safetensors, raw .bin).
func DetectQuant(filename string) (QuantPattern, bool) {
	m := quantRegex.FindString(filename)
	if m == "" {
		return QuantPattern{}, false
	}
	upper := strings.ToUpper(m)
	pat := QuantPattern{Code: upper}
	switch {
	case strings.HasPrefix(upper, "UD-"):
		pat.Family = "UD"
		pat.Bits = bitsFromCode(strings.TrimPrefix(upper, "UD-"))
	case strings.HasPrefix(upper, "IQ"):
		pat.Family = "I"
		pat.Bits = bitsFromCode(upper)
	case strings.HasPrefix(upper, "Q") && strings.Contains(upper, "_K_"):
		pat.Family = "K"
		pat.Bits = bitsFromCode(upper)
	case strings.HasPrefix(upper, "Q"):
		pat.Family = "Legacy"
		pat.Bits = bitsFromCode(upper)
	case strings.HasPrefix(upper, "MXFP"):
		pat.Family = "MXFP"
		pat.Bits = bitsFromCode(upper)
	case upper == "F16" || upper == "BF16":
		pat.Family = "F"
		pat.Bits = 16
	case upper == "F32":
		pat.Family = "F"
		pat.Bits = 32
	}
	return pat, true
}

// bitsFromCode extracts the bits-per-weight digit from a Q*/IQ* code.
// Best-effort; a Q4_K_M is "approximately 4 bits per weight" — exact bits
// depend on K-quant block layout. Used for sort ordering and rough size
// estimation only.
func bitsFromCode(code string) int {
	for i, ch := range code {
		if ch >= '0' && ch <= '9' {
			rest := code[i:]
			if len(rest) >= 1 {
				return int(rest[0] - '0')
			}
		}
	}
	return 0
}

// IsGGUF reports whether a sibling/filename is a GGUF artifact.
func IsGGUF(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".gguf")
}

// TrustedUploaders is Rick's allowlist of trusted GGUF re-uploaders. Encoded
// once here, applied wherever uploader rep matters (find-quants, eval-candidates).
// To extend: add entries here; do NOT spread the list across multiple commands.
var TrustedUploaders = map[string]bool{
	"unsloth":      true,
	"bartowski":    true,
	"mradermacher": true,
}

// IsTrustedUploader is case-insensitive.
func IsTrustedUploader(user string) bool {
	return TrustedUploaders[strings.ToLower(user)]
}
