// Package safety enforces the write-command guardrails documented in the spec.
//
// Defense in depth — every write command goes through this package:
//
//  1. Live-mode gate: for real accounts (account.trade_mode == 2), MT5_LIVE=1
//     in env AND --i-understand-this-is-live on the command must BOTH be
//     present. Demo and contest accounts skip the live gate.
//
//  2. Per-command guardrails from ~/.config/mt5-pp-cli/config.toml:
//     - kill_switch_file: a single touched file refuses all writes
//     - max_volume_per_order: rejects on volume > N (0 = no limit)
//     - max_open_positions: rejects if currently open >= N (0 = no limit)
//     - max_daily_loss: rejects if today's realized P&L < -N (0 = disabled)
//
//  3. Hash-confirm dry-run: the first invocation computes a deterministic
//     SHA-256 of the canonical request bucketed to a rolling 60s window and
//     prints it. The user re-invokes with --confirm <hash> to execute.
//
//  4. Audit log append to <store>/audit.jsonl AND the audit DB table for
//     every attempt (confirmed, dry-run, or rejected).

package safety

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// Window is how long a printed hash remains valid for --confirm.
const Window = 60 * time.Second

// Mode enumerates the three trading modes the CLI knows about.
type Mode int

const (
	ModePaper  Mode = iota // not on a real account, or MT5_PAPER=1
	ModeLive               // real account + MT5_LIVE=1 + --i-understand-this-is-live
	ModeDryRun             // --dry-run explicitly set (always show hash, never send)
)

// CurrentMode reports the trading mode the current PROCESS is in based on env
// vars only. Per-command flags + the account.trade_mode are checked by
// PrecheckWrite for the actual gate enforcement.
func CurrentMode() Mode {
	if os.Getenv("MT5_LIVE") == "1" {
		return ModeLive
	}
	return ModePaper
}

// ModeDescription is a one-line human description for `mt5-pp-cli doctor` and
// `mt5-pp-cli connect status`.
func ModeDescription() string {
	if CurrentMode() == ModeLive {
		return "MT5_LIVE=1 — live writes ENABLED; per-command --i-understand-this-is-live still required for every write on a real account"
	}
	return "PAPER (default) — live writes refused. Set MT5_LIVE=1 AND pass --i-understand-this-is-live per command to unlock real-account writes."
}

// AddWriteFlags adds --confirm and --i-understand-this-is-live to a command.
// Call once per write command in its constructor.
func AddWriteFlags(cmd *cobra.Command) {
	cmd.Flags().String("confirm", "", "SHA-256 hash from a prior dry-run; arms the actual send")
	cmd.Flags().Bool("i-understand-this-is-live", false, "Required (with MT5_LIVE=1) for real-account writes")
}

// PrecheckWrite enforces:
//  1. kill switch file
//  2. live-mode gate (skipped when accountTradeMode != 2)
//
// Per-command volume/positions/loss guardrails are checked by CheckGuardrails
// once the caller knows the request shape.
//
// accountTradeMode is the int from mt5.account_info() — 0=demo 1=contest 2=real.
func PrecheckWrite(cmd *cobra.Command, g Guardrails, accountTradeMode int) error {
	if g.KillSwitchFile != "" {
		if _, err := os.Stat(g.KillSwitchFile); err == nil {
			return fmt.Errorf("%w: %s", ErrKillSwitch, g.KillSwitchFile)
		}
	}
	if accountTradeMode == 2 {
		if CurrentMode() != ModeLive {
			return errors.New("real-account writes require MT5_LIVE=1 in the environment")
		}
		live, _ := cmd.Flags().GetBool("i-understand-this-is-live")
		if !live {
			return errors.New("real-account writes also require --i-understand-this-is-live on the command line")
		}
	}
	return nil
}

// CheckGuardrails enforces volume-only thresholds against the resolved
// request shape. Position-count and daily-loss checks live as separate
// functions (CheckPositionCap, CheckDailyLoss) because they need state
// the safety package shouldn't own (bridge, DB) — the callers compose
// them in writeCtx.openAll.
func CheckGuardrails(g Guardrails, volume float64) error {
	if g.MaxVolumePerOrder > 0 && volume > g.MaxVolumePerOrder {
		return fmt.Errorf("volume %g exceeds max_volume_per_order=%g (config.toml)", volume, g.MaxVolumePerOrder)
	}
	return nil
}

// CheckPositionCap rejects when adding one more position would exceed the
// configured limit. Pass the current count (from b.PositionsGet(nil)).
// A new order_send adds 1, position close subtracts 1, position modify is
// net-zero — callers signal intent via 'wouldAdd' (true for new opens).
func CheckPositionCap(g Guardrails, currentOpen int, wouldAdd bool) error {
	if g.MaxOpenPositions <= 0 {
		return nil
	}
	next := currentOpen
	if wouldAdd {
		next++
	}
	if next > g.MaxOpenPositions {
		return fmt.Errorf("opening this position would put open count at %d, exceeds max_open_positions=%d (config.toml)",
			next, g.MaxOpenPositions)
	}
	return nil
}

// CheckDailyLoss rejects when today's realized P&L is already below the
// configured floor. Pass realizedToday as a SIGNED number (negative =
// loss). max_daily_loss is the loss floor in the account's currency, so
// a configured 500 means "stop trading if today's realized PnL <= -500".
// 0 (the default) disables the check.
func CheckDailyLoss(g Guardrails, realizedToday float64) error {
	if g.MaxDailyLoss <= 0 {
		return nil
	}
	if realizedToday <= -g.MaxDailyLoss {
		return fmt.Errorf("today's realized P&L %.2f reached max_daily_loss=%.2f floor (config.toml); trading halted for the day",
			realizedToday, g.MaxDailyLoss)
	}
	return nil
}

// Guardrails mirrors the [guardrails] config block. Defined here so packages
// don't need to import internal/config to enforce them.
type Guardrails struct {
	MaxVolumePerOrder float64
	MaxOpenPositions  int
	MaxDailyLoss      float64
	KillSwitchFile    string
}

// ── hash / confirm flow ──────────────────────────────────────────────────────

// Known limitation: the bucket is derived from time.Now() so an operator
// rolling the system clock backwards could revalidate a previously-printed
// hash within whatever window the rollback exposes. We accept this for v1
// because (a) clock rollback requires local admin which is already past
// the safety perimeter, (b) every confirmed write is audited regardless,
// so reconstruction is possible. Future hardening would record issued
// hashes in the DB with a monotonic-clock-derived expiry and verify
// against the record.
//
// Hash returns the canonical SHA-256 hash of a request bucketed to the
// current Window. Same request inside the same 60s bucket produces the same
// hash so a user can copy/paste the value they just saw.
func Hash(request any) (string, error) {
	canon, err := canonicalize(request)
	if err != nil {
		return "", err
	}
	bucket := time.Now().UTC().Truncate(Window).Unix()
	payload := struct {
		Bucket  int64           `json:"bucket"`
		Request json.RawMessage `json:"request"`
	}{bucket, canon}
	buf, _ := json.Marshal(payload)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:]), nil
}

// Verify returns true if 'provided' matches the hash for the current window
// or the previous one (so a user has the full Window to type the hash).
func Verify(provided string, request any) (bool, error) {
	canon, err := canonicalize(request)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC()
	for _, t := range []time.Time{now, now.Add(-Window)} {
		bucket := t.Truncate(Window).Unix()
		payload := struct {
			Bucket  int64           `json:"bucket"`
			Request json.RawMessage `json:"request"`
		}{bucket, canon}
		buf, _ := json.Marshal(payload)
		sum := sha256.Sum256(buf)
		if hex.EncodeToString(sum[:]) == provided {
			return true, nil
		}
	}
	return false, nil
}

func canonicalize(v any) (json.RawMessage, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonicalize: %w", err)
	}
	return buf, nil
}

// Confirmed reports whether the user has armed the write via --confirm <hash>
// matching the canonical request. Caller uses this to gate the actual bridge call.
func Confirmed(cmd *cobra.Command, request any) (bool, error) {
	provided, _ := cmd.Flags().GetString("confirm")
	if provided == "" {
		return false, nil
	}
	return Verify(provided, request)
}

// CurrentHash returns the hash a write handler should print on its dry-run.
func CurrentHash(request any) string {
	h, _ := Hash(request)
	return h
}
