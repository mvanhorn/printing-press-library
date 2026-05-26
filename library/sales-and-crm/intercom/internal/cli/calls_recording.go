// PATCH(intercom-calls-v2-14): see calls.go.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/client"
)

func newCallsRecordingCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "recording <id>",
		Short: "Download the audio recording for a call",
		Long: `Downloads the MP3 audio recording for the given call. The CLI calls
GET /calls/{id}/recording, follows the 302 redirect to the signed S3 URL (Go's
stdlib drops the Authorization header on cross-domain redirects, so the S3
query-param auth wins), and streams the response body to --out.

If --out is omitted, the file is written to ./<call-id>.mp3 in the current
working directory.`,
		Example: `  # Download to ./35553520.mp3
  intercom-pp-cli calls recording 35553520

  # Download to a specific path
  intercom-pp-cli calls recording 35553520 --out /tmp/call.mp3`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			callID := strings.TrimSpace(args[0])
			if callID == "" {
				return usageErr(fmt.Errorf("call id is required"))
			}
			out := outPath
			if out == "" {
				out = callID + ".mp3"
			}
			// Resolve to an absolute path so the success message is unambiguous.
			absOut, err := filepath.Abs(out)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The CLI's client wraps non-JSON responses in a `_pp_binary` JSON
			// envelope by default. For raw audio we opt out by setting the
			// X-Printing-Press-Binary-Response header (defined as
			// client.BinaryResponseHeader); the client strips it before
			// sending and returns the response body as-is.
			//
			// Recordings are typically a few MB, well under the limits we
			// already accept for other endpoints. If this ever grows past
			// a streaming threshold, the right fix is a new
			// client.GetStream method that writes the body to an io.Writer.
			headers := callsHeaderOverrides()
			headers[client.BinaryResponseHeader] = "true"
			data, err := c.GetWithHeaders(cmd.Context(), "/calls/"+callID+"/recording", nil, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// The client wraps non-JSON content-types in a `_pp_binary` envelope
			// (`{"_pp_binary":true,"encoding":"base64","bytes":"..."}`) even when
			// the caller opts into binary mode via X-Printing-Press-Binary-Response.
			// That's a generator-template bug worth a retro; for now, detect the
			// envelope and decode the base64 payload so the file on disk is real
			// audio bytes, not a JSON wrapper.
			raw, err := unwrapBinaryEnvelope(data)
			if err != nil {
				return fmt.Errorf("decoding recording: %w", err)
			}
			if err := os.WriteFile(absOut, raw, 0o644); err != nil {
				return fmt.Errorf("writing recording: %w", err)
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"call_id":    callID,
					"out":        absOut,
					"size_bytes": len(raw),
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %d bytes to %s\n", len(raw), absOut)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (default: <call-id>.mp3 in current dir)")
	return cmd
}

// unwrapBinaryEnvelope detects the `_pp_binary` JSON envelope the generated
// client wraps non-JSON responses in and decodes the base64 payload back to
// raw bytes. If the input isn't a recognized envelope, returns it unchanged
// so future client-template fixes that drop the wrapper Just Work.
func unwrapBinaryEnvelope(data []byte) ([]byte, error) {
	trim := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trim, "{") || !strings.Contains(trim, `"_pp_binary"`) {
		return data, nil
	}
	var env struct {
		PPBinary    bool   `json:"_pp_binary"`
		ContentType string `json:"content_type"`
		Encoding    string `json:"encoding"`
		Bytes       int    `json:"bytes"` // declared count (not the payload)
		Data        string `json:"data"`  // base64 payload
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return data, nil
	}
	if !env.PPBinary || env.Encoding != "base64" || env.Data == "" {
		return data, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 payload: %w", err)
	}
	return decoded, nil
}
