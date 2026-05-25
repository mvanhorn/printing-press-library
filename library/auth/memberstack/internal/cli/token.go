// Hand-written novel command: token decode (offline JWT inspection).
// Survives regen via the separate-file durability rule.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTokenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Decode or verify Memberstack JWTs",
		Long: `Token tools for Memberstack-issued member JWTs. 'decode' parses the token
locally without any API call (useful for frontend debugging). 'verify' performs
a signed verification by calling the Admin API's verify-token endpoint and
returns the decoded claims if valid.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTokenDecodeCmd(flags))
	return cmd
}

func newTokenDecodeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decode [jwt]",
		Short: "Decode a Memberstack JWT locally without calling the API.",
		Long: `Parses the JWT header and payload locally and prints them as JSON. Does NOT
verify the signature — for signed verification, use 'members verify-token'.
The Memberstack secret key is not needed for decode; only the token string is.`,
		Example: `  memberstack-pp-cli token decode eyJhbGciOiJIUzI1NiI...
  echo "eyJ..." | memberstack-pp-cli token decode --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && isTerminal(cmd.OutOrStdout()) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would decode JWT (offline, no network)")
				return nil
			}

			var raw string
			switch {
			case len(args) > 0:
				raw = strings.TrimSpace(args[0])
			default:
				stdin, err := readStdinTrimmed(cmd)
				if err != nil {
					return err
				}
				raw = stdin
			}
			if raw == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a JWT is required (as an argument or on stdin)"))
			}

			parts := strings.Split(raw, ".")
			if len(parts) != 3 {
				return fmt.Errorf("invalid JWT: expected three dot-separated segments, got %d", len(parts))
			}

			header, herr := decodeJWTSegment(parts[0])
			if herr != nil {
				return fmt.Errorf("decoding header: %w", herr)
			}
			payload, perr := decodeJWTSegment(parts[1])
			if perr != nil {
				return fmt.Errorf("decoding payload: %w", perr)
			}

			out := map[string]any{
				"header":             header,
				"payload":            payload,
				"signature_present":  parts[2] != "",
				"signature_verified": false,
				"note":               "Signature was not verified. Use 'members verify-token' for signed verification.",
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	return cmd
}

func decodeJWTSegment(seg string) (map[string]any, error) {
	// JWT uses URL-safe base64 without padding.
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		// Some encoders include padding; try the std URL encoder too.
		b, err = base64.URLEncoding.DecodeString(seg)
		if err != nil {
			return nil, err
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}
	return m, nil
}

func readStdinTrimmed(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 1024)
	for {
		n, err := in.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(string(buf)), nil
}
