package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newRelayCmd(flags *rootFlags) *cobra.Command {
	var toGroup, keyPrefix string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Pipe stdin lines into a Roam group via /chat.post with deterministic idempotency keys",
		Long: `Reads lines from stdin and posts each as a chat message to --to <group>.
Each line's client-msg-id is a SHA-256 hash of [keyPrefix + line] so retries are idempotent.
Honors Retry-After on 429 with exponential backoff.

Example:
  tail -F /var/log/app.log | roam-pp-cli relay --to G123 --idempotent-key-prefix deploy-`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if toGroup == "" {
				return usageErr(fmt.Errorf("--to <group-address> is required"))
			}
			if dryRun || dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "(dry-run) would post stdin lines to %s with key-prefix %q\n", toGroup, keyPrefix)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			scanner := bufio.NewScanner(os.Stdin)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			sent := 0
			failed := 0
			for scanner.Scan() {
				line := strings.TrimRight(scanner.Text(), "\r\n")
				if line == "" {
					continue
				}
				sum := sha256.Sum256([]byte(keyPrefix + line))
				clientMsgID := hex.EncodeToString(sum[:])[:32]
				body := map[string]any{
					"address":       toGroup,
					"text":          line,
					"client_msg_id": clientMsgID,
				}

				backoff := time.Second
				for attempt := 0; attempt < 5; attempt++ {
					_, code, err := c.Post("/chat.post", body)
					if err == nil && code < 400 {
						sent++
						break
					}
					if code == 429 {
						time.Sleep(backoff)
						backoff *= 2
						continue
					}
					failed++
					if flags.asJSON {
						out, _ := json.Marshal(map[string]any{"line": line, "error": err.Error(), "status": code})
						fmt.Fprintln(cmd.ErrOrStderr(), string(out))
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "FAIL line=%q status=%d err=%v\n", truncate(line, 80), code, err)
					}
					break
				}
			}
			if err := scanner.Err(); err != nil {
				return apiErr(fmt.Errorf("stdin read: %w", err))
			}
			out, _ := json.Marshal(map[string]any{"sent": sent, "failed": failed, "to": toGroup})
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if failed > 0 {
				return apiErr(fmt.Errorf("%d lines failed to post", failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&toGroup, "to", "", "Target group/chat address ID (required)")
	cmd.Flags().StringVar(&keyPrefix, "idempotent-key-prefix", "relay-", "Prefix for client_msg_id hashing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be sent without calling the API")
	return cmd
}
