package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type loopsIntentKey struct{}

// WithLoopsMutationIntent marks one invocation as explicitly approved for the
// named Loops team. The transport still verifies the team for every mutation.
func WithLoopsMutationIntent(ctx context.Context, team string) context.Context {
	return context.WithValue(ctx, loopsIntentKey{}, strings.TrimSpace(team))
}

func loopsSendPath(method, path string) bool {
	return method == http.MethodPost && (path == "/v1/events/send" || path == "/v1/transactional" ||
		strings.HasSuffix(path, "/preview") && strings.HasPrefix(path, "/v1/email-messages/"))
}

func (c *Client) guardLoopsMutation(ctx context.Context, method, path string, body any, headers map[string]string) (string, error) {
	if c.DryRun {
		return "", nil
	}
	team, _ := ctx.Value(loopsIntentKey{}).(string)
	if team == "" {
		return "", errors.New("Loops mutation requires --execute and --team; no request sent")
	}
	data, err := c.GetNoCache(ctx, "/v1/api-key", nil)
	if err != nil {
		return "", errors.New("could not verify the selected Loops team; no request sent")
	}
	var identity struct {
		Success  bool   `json:"success"`
		TeamName string `json:"teamName"`
	}
	if json.Unmarshal(data, &identity) != nil || !identity.Success || identity.TeamName == "" || identity.TeamName != team {
		return "", errors.New("selected Loops team does not match --team; no request sent")
	}
	// Team names can be reused. Include a one-way credential fingerprint in
	// the local journal scope so two keys never share a send claim.
	credentialHash := sha256.Sum256([]byte(c.Config.AuthHeader()))
	journalScope := team + "\x00" + hex.EncodeToString(credentialHash[:])
	if !loopsSendPath(method, path) {
		return "", nil
	}
	if strings.HasSuffix(path, "/preview") {
		fingerprint, err := json.Marshal(body)
		if err != nil {
			return "", errors.New("cannot fingerprint preview send")
		}
		hash := sha256.Sum256(append([]byte(path+"\x00"), fingerprint...))
		return claimLoopsSend(journalScope, "preview:"+hex.EncodeToString(hash[:]), body)
	}
	key := ""
	for name, value := range headers {
		if strings.EqualFold(name, "Idempotency-Key") {
			key = strings.TrimSpace(value)
		}
	}
	if key == "" || len(key) > 100 || strings.ContainsAny(key, "@\r\n\t") {
		return "", errors.New("send requires an explicit --idempotency-key of 1–100 characters without private identifiers")
	}
	return claimLoopsSend(journalScope, key, body)
}

func loopsJournalDir() (string, error) {
	dir := strings.TrimSpace(os.Getenv("LOOPS_SEND_JOURNAL_DIR"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("cannot locate private send journal")
		}
		dir = filepath.Join(home, ".local", "state", "loops-pp-cli", "send-journal")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", errors.New("cannot create private send journal")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("send journal must be a private directory (mode 0700)")
	}
	return dir, nil
}

func claimLoopsSend(team, key string, body any) (string, error) {
	dir, err := loopsJournalDir()
	if err != nil {
		return "", err
	}
	request, err := json.Marshal(body)
	if err != nil {
		return "", errors.New("cannot fingerprint send request")
	}
	nameHash := sha256.Sum256([]byte(team + "\x00" + key))
	bodyHash := sha256.Sum256(request)
	name := filepath.Join(dir, hex.EncodeToString(nameHash[:])+".json")
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return "", errors.New("send key already attempted for this team; inspect Loops before any manual retry")
	}
	if err != nil {
		return "", errors.New("cannot record send attempt")
	}
	entry := map[string]string{
		"claimed_at":     time.Now().UTC().Format(time.RFC3339),
		"request_sha256": hex.EncodeToString(bodyHash[:]),
	}
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		_ = f.Close()
		return "", errors.New("cannot record send attempt")
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return "", errors.New("cannot sync send journal")
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("cannot close send journal: %w", err)
	}
	return name, nil
}

func releaseLoopsRateLimitClaim(name string) error {
	if name == "" {
		return nil
	}
	if err := os.Remove(name); err != nil {
		return errors.New("rate limit rejected send, but journal cleanup failed; inspect before retry")
	}
	return nil
}
