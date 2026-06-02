// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored absorbed feature: offline Svix webhook signature verification
// for Sybill's meeting.new_recording.v1 events. No competitor CLI ships this.

package cli

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWebhookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "webhook",
		Short:       "Work with Sybill webhook events (Svix signature verification).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebhookVerifyCmd(flags))
	return cmd
}

func newWebhookVerifyCmd(flags *rootFlags) *cobra.Command {
	var secret string
	var id string
	var timestamp string
	var signature string
	var payloadFile string
	var tolerance time.Duration

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify a Sybill (Svix) webhook signature offline.",
		Long: `Verify the signature on a Sybill webhook delivery without any network call.
Sybill signs webhooks with Svix: HMAC-SHA256 over "{svix-id}.{svix-timestamp}.{body}"
using your endpoint secret (the "whsec_..." value from the dashboard).

Provide the three Svix headers and the raw request body. The body is read from
--payload <file>, or from stdin when --payload is omitted. The secret may be
passed with --secret or the SYBILL_WEBHOOK_SECRET environment variable.

Exit code 0 means the signature is valid; a non-zero exit means it is not.`,
		Example: strings.Trim(`
  # Verify a captured delivery (body on stdin)
  cat body.json | sybill-pp-cli webhook verify \
    --secret whsec_XXXX \
    --id msg_2abc --timestamp 1717320000 \
    --signature "v1,g0b6S..."

  # Body from a file, secret from the environment
  export SYBILL_WEBHOOK_SECRET=whsec_XXXX
  sybill-pp-cli webhook verify --payload body.json \
    --id msg_2abc --timestamp 1717320000 --signature "v1,g0b6S..."
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := cmd.OutOrStdout()

			if secret == "" {
				secret = os.Getenv("SYBILL_WEBHOOK_SECRET")
			}
			if secret == "" {
				return fmt.Errorf("--secret (or SYBILL_WEBHOOK_SECRET) is required: the whsec_ value from Settings > Integrations")
			}
			if id == "" || timestamp == "" || signature == "" {
				return fmt.Errorf("--id, --timestamp, and --signature are all required (the svix-id, svix-timestamp, svix-signature headers)")
			}

			var body []byte
			var err error
			if payloadFile != "" {
				body, err = os.ReadFile(payloadFile)
				if err != nil {
					return fmt.Errorf("reading --payload %s: %w", payloadFile, err)
				}
			} else {
				body, err = io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading body from stdin: %w", err)
				}
			}

			result := verifySvixSignature(secret, id, timestamp, signature, body, tolerance, time.Now())

			if novelMachineOutput(out, flags) {
				if jerr := printJSONFiltered(out, result, flags); jerr != nil {
					return jerr
				}
			} else if result.Valid {
				fmt.Fprintln(out, "valid: signature matches")
			} else {
				fmt.Fprintf(out, "invalid: %s\n", result.Reason)
			}
			if !result.Valid {
				return fmt.Errorf("signature verification failed: %s", result.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&secret, "secret", "", "Endpoint secret (whsec_...); falls back to SYBILL_WEBHOOK_SECRET")
	cmd.Flags().StringVar(&id, "id", "", "The svix-id header value")
	cmd.Flags().StringVar(&timestamp, "timestamp", "", "The svix-timestamp header value (unix seconds)")
	cmd.Flags().StringVar(&signature, "signature", "", "The svix-signature header value (space-separated v1,... entries)")
	cmd.Flags().StringVar(&payloadFile, "payload", "", "File containing the raw request body (default: stdin)")
	cmd.Flags().DurationVar(&tolerance, "tolerance", 5*time.Minute, "Max timestamp skew before the delivery is rejected as stale (0 disables)")
	return cmd
}

// svixResult is the structured outcome of a verification.
type svixResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

// verifySvixSignature implements the Svix verification scheme used by Sybill
// webhooks. The signed content is "{id}.{timestamp}.{body}", HMAC-SHA256 with
// the base64-decoded secret (after the whsec_ prefix), compared in constant
// time against each v1 entry in the signature header.
func verifySvixSignature(secret, id, timestamp, signatureHeader string, body []byte, tolerance time.Duration, now time.Time) svixResult {
	if tolerance > 0 {
		ts, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
		if err != nil {
			return svixResult{Valid: false, Reason: "timestamp is not a unix integer"}
		}
		delta := now.Sub(time.Unix(ts, 0))
		if delta < 0 {
			delta = -delta
		}
		if delta > tolerance {
			return svixResult{Valid: false, Reason: fmt.Sprintf("timestamp outside tolerance (%s skew)", delta.Round(time.Second))}
		}
	}

	key := secret
	if i := strings.IndexByte(key, '_'); strings.HasPrefix(key, "whsec_") && i >= 0 {
		key = key[i+1:]
	}
	secretBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return svixResult{Valid: false, Reason: "secret is not valid base64 (expected whsec_<base64>)"}
	}

	signedContent := id + "." + strings.TrimSpace(timestamp) + "." + string(body)
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedContent))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	for _, part := range strings.Fields(signatureHeader) {
		// Each entry is "version,signature"; we support v1.
		comma := strings.IndexByte(part, ',')
		sig := part
		if comma >= 0 {
			if !strings.EqualFold(part[:comma], "v1") {
				continue
			}
			sig = part[comma+1:]
		}
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return svixResult{Valid: true}
		}
	}
	return svixResult{Valid: false, Reason: "no v1 signature entry matched"}
}
