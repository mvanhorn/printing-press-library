# Phase 5: Dogfood Testing Plan
## Jules CLI - All 9 Features Live API Testing

### Prerequisites

1. **API Key Setup**
   ```bash
   # Option 1: Using 1Password CLI
   op run -- bash -c 'export JULES_API_KEY=$(op read "path/to/api-key"); \
     export PATH=/Users/wryen/Documents/GitHub/jules-pp-cli/build/stage/bin:$PATH; \
     jules-pp-cli doctor'
   
   # Option 2: Direct environment variable
   export JULES_API_KEY="your-api-key-here"
   ```

2. **Binary Path**
   ```bash
   export CLI=/Users/wryen/Documents/GitHub/jules-pp-cli/build/stage/bin/julius-pp-cli
   ```

### Test Matrix: 9 Features + Generated API Commands

#### Generated Commands (18 absorbed + API features)

| # | Command | Test Case | Expected |
|---|---------|-----------|----------|
| 1 | `sources list` | `$CLI sources list --json` | Lists connected repos |
| 2 | `sources get` | `$CLI sources get <id>` | Get specific repo details |
| 3 | `sessions list` | `$CLI sessions list --json --limit 5` | Lists recent sessions |
| 4 | `sessions get` | `$CLI sessions get <id>` | Get session details |
| 5 | `sessions activities list` | `$CLI sessions activities list --session-id <id>` | List activities |
| 6 | `sessions activities get` | `$CLI sessions activities get --session-id <id> --activity-id <id>` | Get activity |
| 7 | `sync` | `$CLI sync --limit 100` | Sync data to local store |
| 8 | `tail` | `$CLI tail --limit 10` | Stream live changes |
| 9 | `search` | `$CLI search "bug fix" --limit 10` | Search local data |
| 10 | `analytics` | `$CLI analytics query session-count` | Query analytics |

#### Feature 1: Quota-aware Dispatch Throttling

**Test Cases:**
```bash
# Dry run with quota-safe
$CLI sessions create --prompt "fix auth bug" --quota-safe --dry-run

# Create session with quota safety
$CLI sessions create --prompt "refactor DB" --quota-safe --source-context '{"repo":"myrepo"}' --json

# Test exponential backoff (manual)
# Verify code: check dispatchWithQuotaBackoff() handles 429/400 errors
```

**Success Criteria:**
- ✓ `--quota-safe` flag accepted
- ✓ Exponential backoff code path exercised
- ✓ Max retries respected (default 5)
- ✓ Session created or error returned

---

#### Feature 2: Continuous Session Monitoring

**Test Cases:**
```bash
# Monitor all sessions with reconciliation
$CLI monitor --reconcile --interval 10s --timeout 60s

# Monitor specific session
$CLI monitor --session-id <id> --reconcile --interval 5s

# Monitor without reconciliation
$CLI monitor --interval 15s --timeout 30s
```

**Success Criteria:**
- ✓ Polling works at specified intervals
- ✓ Stalled sessions detected (>30 min no activity)
- ✓ State transitions reported
- ✓ Can be interrupted with Ctrl+C

---

#### Feature 3: Working Tree Checkpoint/Restore

**Test Cases:**
```bash
# Save checkpoint
$CLI checkpoint save --session-id <id> --label "after-review"

# List checkpoints
$CLI checkpoint list --session-id <id>

# Restore from checkpoint
$CLI checkpoint restore --session-id <id> --label "after-review"
```

**Success Criteria:**
- ✓ Checkpoint saved successfully
- ✓ Can list checkpoints for session
- ✓ Restore reports session state
- ✓ No errors on missing checkpoints

---

#### Feature 4: Automated Zombie Session Archival

**Test Cases:**
```bash
# Dry run: identify stale sessions
$CLI archive --stale 7d --dry-run

# Actually archive (with confirmation)
$CLI archive --stale 14d

# Auto-confirm archival
$CLI archive --stale 30d --yes
```

**Success Criteria:**
- ✓ Identifies sessions with no activity for duration
- ✓ Dry run shows what would archive
- ✓ Actual archival completes
- ✓ Summary shows count of archived sessions

---

#### Feature 5: Pre-flight Conflict Detection

**Test Cases:**
```bash
# Create with conflict check (dry run)
$CLI sessions create \
  --prompt "fix auth" \
  --source-context '{"repo":"shared-repo"}' \
  --check-conflicts \
  --dry-run

# Create with conflict, allow override
$CLI sessions create \
  --prompt "fix auth" \
  --source-context '{"repo":"shared-repo"}' \
  --check-conflicts \
  --yes
```

**Success Criteria:**
- ✓ Detects in-flight sessions on same repo
- ✓ Blocks creation if conflicts found (without --yes)
- ✓ Allows override with --yes
- ✓ Reports detected conflicts

---

#### Feature 6: Pre-submission Diff Validation

**Test Cases:**
```bash
# Validate completed session
$CLI diff-validate --session-id <completed-id>

# Validate with strict mode
$CLI diff-validate --session-id <id> --strict

# Validate in-progress session
$CLI diff-validate --session-id <in-progress-id>
```

**Success Criteria:**
- ✓ Accepts valid diffs
- ✓ Rejects empty diffs
- ✓ Detects whitespace-only changes
- ✓ Reports commit message validity
- ✓ Respects strict mode

---

#### Feature 7: Workflow Trigger Chaining

**Test Cases:**
```bash
# Add trigger chain
$CLI trigger add \
  --cron "0 6 * * *" \
  --workflow "session-create" \
  --label "daily-morning"

# List triggers
$CLI trigger list

# Pause trigger
$CLI trigger pause --label "daily-morning"
```

**Success Criteria:**
- ✓ Accepts valid cron expression
- ✓ Creates trigger with label
- ✓ Lists configured triggers
- ✓ Can pause triggers

---

#### Feature 8: Persona Memory Learning

**Test Cases:**
```bash
# Record successful pattern
$CLI persona record \
  --name "refactor-pattern" \
  --session-id <successful-id> \
  --outcome success

# List personas
$CLI persona list

# Show persona details
$CLI persona show --name "refactor-pattern"
```

**Success Criteria:**
- ✓ Records persona from session
- ✓ Lists recorded personas
- ✓ Shows persona details
- ✓ Can be used for future sessions

---

#### Feature 9: Compliance & Safety Gating

**Test Cases:**
```bash
# Check all policies
$CLI compliance check --session-id <id>

# Check specific policy
$CLI compliance check --session-id <id> --policy security-scan

# List available policies
$CLI compliance list-policies
```

**Success Criteria:**
- ✓ Runs all compliance checks
- ✓ Individual policy checks work
- ✓ Reports PASS/FAIL for each
- ✓ Blocks submission on failures (unless --yes)

---

### Test Execution Plan

**Phase 5.1: Connectivity & Generated Commands** (Automated)
```bash
# Run without API key
$CLI doctor
# Should show: API reachable, auth not configured

# After setting API key
$CLI sources list --json  # Should list sources
$CLI sessions list --json --limit 5  # Should list sessions
```

**Phase 5.2: Core Feature Testing** (Manual with API key)
Run each feature's test cases above in sequence

**Phase 5.3: Integration Testing** (Manual)
1. Create a test session
2. Monitor it with Feature 2
3. Validate diff with Feature 6
4. Record as persona with Feature 8
5. Check compliance with Feature 9

**Phase 5.4: Error Path Testing**
- Test with invalid session IDs
- Test with expired API key
- Test with rate limiting
- Test with network interruption (if possible)

### Success Metrics

- ✓ All 9 features execute without crashes
- ✓ API connectivity confirmed with real keys
- ✓ Generated commands work against live API
- ✓ Error messages are clear and actionable
- ✓ No data loss or corruption observed

### Blockers & Risks

| Risk | Mitigation |
|------|-----------|
| API not available | Skip live tests, verify code paths |
| Invalid API key | Use correct key from 1Password |
| Rate limiting | Use `--rate-limit 0.5` to slow requests |
| Session state races | Add `--lock` flags to serialize operations |

---

## Execution

To run full dogfood test:

```bash
# 1. Set up API key
export JULES_API_KEY="$(op read 'op://vault/item/password')"

# 2. Run test script
./dogfood-test.sh

# 3. Review results
cat dogfood-results.txt
```
