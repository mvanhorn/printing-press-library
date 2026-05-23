// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Thin wrapper around time.Timer so WaitMinedByHash stays readable without
// importing time everywhere. Kept separate from erc20.go to avoid polluting
// that file with stdlib glue.

package onchain

import "time"

const defaultPollInterval = 2 * time.Second

type pollTimer struct{ *time.Timer }

func (t *pollTimer) Stop() { t.Timer.Stop() }

func newTimer(d time.Duration) *pollTimer { return &pollTimer{Timer: time.NewTimer(d)} }
