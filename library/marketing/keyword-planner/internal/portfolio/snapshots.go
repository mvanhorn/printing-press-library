// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const snapshotColumns = `
    id, run_id, endpoint, api_version, discovery_revision,
    customer_id, login_customer_id, language, network, currency_code,
    currency_source, geo_set_id, geo_target_constants, submitted_seeds,
    submitted_keywords, requested_start, requested_end, request_body,
    request_body_present, request_body_sha256, source_variant, metadata,
    fetched_at, status, complete, finalized, finished_at, warning_flags,
    failed_receipts, missing_batches`

// StartSnapshot records the immutable request context before any response or
// account-helper lookup is made. Currency may be empty until SetAccountCurrency
// appends verified account metadata.
func (s *Store) StartSnapshot(ctx context.Context, in SnapshotInput, fetchedAt time.Time) (Snapshot, error) {
	endpoint, err := NormalizeEndpoint(in.Endpoint)
	if err != nil {
		return Snapshot{}, err
	}
	if endpoint == EndpointAccount {
		return Snapshot{}, fmt.Errorf("%w: account is a receipt endpoint, not a snapshot endpoint", ErrInvalidInput)
	}
	if _, _, err := validateRange(in.RequestedStart, in.RequestedEnd, fetchedAt); err != nil {
		return Snapshot{}, err
	}
	if in.Network != "" && in.Network != NetworkGoogleSearch {
		return Snapshot{}, fmt.Errorf("%w: network %q is not allowed; use %s", ErrInvalidInput, in.Network, NetworkGoogleSearch)
	}
	geoID, geos, err := geoSetID(in.GeoTargetConstants)
	if err != nil {
		return Snapshot{}, err
	}
	seeds, err := marshalStrings(in.SubmittedSeeds)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode submitted seeds: %w", err)
	}
	keywords, err := marshalStrings(in.SubmittedKeywords)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode submitted keywords: %w", err)
	}
	requestBody := cloneBytes(in.RequestBody)
	metadata := cloneBytes(in.Metadata)
	bodyHash := ""
	if requestBody != nil {
		bodyHash = bytesHash(requestBody)
	}
	fetchedAt = currentTime(fetchedAt)
	id, err := newID()
	if err != nil {
		return Snapshot{}, err
	}

	snapshot := Snapshot{
		ID: id, RunID: in.RunID, Endpoint: endpoint, APIVersion: in.APIVersion,
		DiscoveryRevision: in.DiscoveryRevision, CustomerID: in.CustomerID,
		LoginCustomerID: in.LoginCustomerID, Language: in.Language,
		Network: in.Network, CurrencyCode: in.CurrencyCode,
		CurrencySource: in.CurrencySource, GeoSetID: geoID,
		GeoTargetConstants: append([]string{}, geos...),
		SubmittedSeeds:     append([]string{}, in.SubmittedSeeds...),
		SubmittedKeywords:  append([]string{}, in.SubmittedKeywords...),
		RequestedStart:     in.RequestedStart, RequestedEnd: in.RequestedEnd,
		RequestBody: json.RawMessage(cloneBytes(requestBody)), SourceVariant: in.SourceVariant,
		Metadata: json.RawMessage(cloneBytes(metadata)), RequestBodyHash: bodyHash,
		FetchedAt: fetchedAt, Status: StatusInProgress, Complete: false,
		Finalized: false, WarningFlags: []string{}, FailedReceipts: []string{}, MissingBatches: []string{},
	}

	err = s.withWriteLock(ctx, func() error {
		_, err := s.db.ExecContext(ctx, `INSERT INTO snapshots (`+snapshotColumns+`)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshot.ID, snapshot.RunID, snapshot.Endpoint, snapshot.APIVersion,
			snapshot.DiscoveryRevision, snapshot.CustomerID, snapshot.LoginCustomerID,
			snapshot.Language, snapshot.Network, snapshot.CurrencyCode, snapshot.CurrencySource,
			snapshot.GeoSetID, string(mustJSON(snapshot.GeoTargetConstants)), string(seeds), string(keywords),
			snapshot.RequestedStart, snapshot.RequestedEnd, nullableBytes(requestBody),
			boolInt(requestBody != nil), snapshot.RequestBodyHash, snapshot.SourceVariant,
			nullableBytes(metadata), utcText(snapshot.FetchedAt), snapshot.Status, 0, 0, nil,
			`[]`, `[]`, `[]`)
		if err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// AppendReceipt commits the exact response bytes and request metadata before
// any decoder is called. A nil Body is a preserved no-body receipt.
func (s *Store) AppendReceipt(ctx context.Context, in ReceiptInput) (ResponseIDs, error) {
	if strings.TrimSpace(in.SnapshotID) == "" {
		return ResponseIDs{}, fmt.Errorf("%w: snapshot ID is empty", ErrInvalidInput)
	}
	endpoint := strings.TrimSpace(in.Endpoint)
	if endpoint != "" {
		var err error
		endpoint, err = NormalizeEndpoint(endpoint)
		if err != nil {
			return ResponseIDs{}, err
		}
	}
	if in.PageNumber < 0 || in.BatchNumber < 0 || in.Attempt < 0 {
		return ResponseIDs{}, fmt.Errorf("%w: page, batch and attempt must be non-negative", ErrInvalidInput)
	}
	keywords, err := marshalStrings(in.SubmittedKeywords)
	if err != nil {
		return ResponseIDs{}, fmt.Errorf("encode submitted keywords: %w", err)
	}
	requestBody := cloneBytes(in.RequestBody)
	body := cloneBytes(in.Body)
	fetchedAt := currentTime(in.FetchedAt)
	bodyHash := bytesHash(body)
	receiptID, err := newID()
	if err != nil {
		return ResponseIDs{}, err
	}
	result := ResponseIDs{SnapshotID: in.SnapshotID, ReceiptID: receiptID,
		PageNumber: in.PageNumber, BatchNumber: in.BatchNumber, Attempt: in.Attempt,
		BodySHA256: bodyHash}

	err = s.withWriteLock(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin receipt transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		var status, snapshotEndpoint string
		var finalized int
		if err := tx.QueryRowContext(ctx, `SELECT status, finalized, endpoint FROM snapshots WHERE id = ?`, in.SnapshotID).Scan(&status, &finalized, &snapshotEndpoint); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read snapshot before receipt: %w", err)
		}
		if finalized != 0 {
			return ErrFinalized
		}
		if status != StatusInProgress {
			return fmt.Errorf("%w: snapshot status is %s", ErrIncompleteSnapshot, status)
		}
		if endpoint == "" {
			endpoint = snapshotEndpoint
		}
		if endpoint != snapshotEndpoint && endpoint != EndpointAccount {
			return fmt.Errorf("%w: receipt endpoint %q does not match snapshot endpoint %q", ErrInvalidInput, endpoint, snapshotEndpoint)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO response_receipts
            (id, snapshot_id, endpoint, page_token, page_number, batch_number, attempt,
             request_body, request_body_present, submitted_keywords, http_status,
             google_request_id, body, body_present, body_sha256, fetched_at,
             error_code, error_message, normalized)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
			receiptID, in.SnapshotID, endpoint, in.PageToken, in.PageNumber,
			in.BatchNumber, in.Attempt, nullableBytes(requestBody), boolInt(requestBody != nil),
			string(keywords), in.HTTPStatus, in.GoogleRequestID, nullableBytes(body), boolInt(body != nil),
			bodyHash, utcText(fetchedAt), in.ErrorCode, in.ErrorMessage); err != nil {
			return fmt.Errorf("insert response receipt: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit response receipt: %w", err)
		}
		return nil
	})
	if err != nil {
		return ResponseIDs{}, err
	}
	return result, nil
}

// SetAccountCurrency appends verified account currency provenance and updates
// only the still-in-progress snapshot's display metadata. Normalization reads
// the append-only event, so an input snapshot with an unknown currency remains
// auditable while the verified value is used for consumer rows.
func (s *Store) SetAccountCurrency(ctx context.Context, snapshotID, currencyCode, source, receiptID string) error {
	if strings.TrimSpace(snapshotID) == "" || strings.TrimSpace(receiptID) == "" {
		return fmt.Errorf("%w: snapshot and account receipt IDs are required", ErrInvalidInput)
	}
	currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
	source = strings.TrimSpace(source)
	if currencyCode == "" || source == "" {
		return fmt.Errorf("%w: currency code and source are required", ErrInvalidInput)
	}
	id, err := newID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.withWriteLock(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin account metadata transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		var finalized int
		if err := tx.QueryRowContext(ctx, `SELECT finalized FROM snapshots WHERE id = ?`, snapshotID).Scan(&finalized); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read snapshot for currency: %w", err)
		}
		if finalized != 0 {
			return ErrFinalized
		}
		var receiptSnapshot, receiptEndpoint string
		if err := tx.QueryRowContext(ctx, `SELECT snapshot_id, endpoint FROM response_receipts WHERE id = ?`, receiptID).Scan(&receiptSnapshot, &receiptEndpoint); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read account receipt: %w", err)
		}
		if receiptSnapshot != snapshotID {
			return fmt.Errorf("%w: account receipt belongs to another snapshot", ErrInvalidInput)
		}
		if receiptEndpoint != EndpointAccount {
			return fmt.Errorf("%w: receipt %q is not an account-helper receipt", ErrInvalidInput, receiptID)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_metadata
            (id, snapshot_id, receipt_id, currency_code, source, recorded_at)
            VALUES (?, ?, ?, ?, ?, ?)`, id, snapshotID, receiptID, currencyCode, source, utcText(now)); err != nil {
			return fmt.Errorf("append account metadata: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE snapshots
            SET currency_code = ?, currency_source = ?
            WHERE id = ? AND finalized = 0`, currencyCode, source, snapshotID); err != nil {
			return fmt.Errorf("update snapshot currency metadata: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit account metadata: %w", err)
		}
		return nil
	})
}

// Finish seals a snapshot. A complete finish is rejected while any Planner
// receipt remains unnormalized, preventing success after a persistence or
// decoder failure.
func (s *Store) Finish(ctx context.Context, in FinishInput) error {
	if strings.TrimSpace(in.SnapshotID) == "" {
		return fmt.Errorf("%w: snapshot ID is empty", ErrInvalidInput)
	}
	status := strings.TrimSpace(in.Status)
	if in.Complete {
		if status != "" && status != StatusComplete {
			return fmt.Errorf("%w: complete snapshot must use status %q", ErrInvalidInput, StatusComplete)
		}
		status = StatusComplete
	} else {
		if status == "" {
			status = StatusIncomplete
		}
		if status != StatusIncomplete && status != StatusFailed {
			return fmt.Errorf("%w: incomplete snapshot status must be %q or %q", ErrInvalidInput, StatusIncomplete, StatusFailed)
		}
	}
	warnings, err := flagsJSON(in.WarningFlags)
	if err != nil {
		return fmt.Errorf("encode finish warnings: %w", err)
	}
	failed, err := marshalStrings(in.FailedReceipts)
	if err != nil {
		return fmt.Errorf("encode failed receipts: %w", err)
	}
	missing, err := marshalStrings(in.MissingBatches)
	if err != nil {
		return fmt.Errorf("encode missing batches: %w", err)
	}
	return s.withWriteLock(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin finish transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		var current string
		var finalized int
		if err := tx.QueryRowContext(ctx, `SELECT status, finalized FROM snapshots WHERE id = ?`, in.SnapshotID).Scan(&current, &finalized); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("read snapshot before finish: %w", err)
		}
		if finalized != 0 {
			return ErrFinalized
		}
		if current != StatusInProgress {
			return fmt.Errorf("%w: snapshot status is %s", ErrIncompleteSnapshot, current)
		}
		if in.Complete {
			var plannerReceipts int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM response_receipts
				WHERE snapshot_id = ? AND endpoint IN (?, ?)`,
				in.SnapshotID, EndpointIdeas, EndpointHistorical).Scan(&plannerReceipts); err != nil {
				return fmt.Errorf("check Planner receipts: %w", err)
			}
			if plannerReceipts == 0 {
				return fmt.Errorf("%w: no Planner receipt was recorded", ErrIncompleteSnapshot)
			}
			var pendingGroups int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM (
				SELECT page_number, batch_number
				FROM response_receipts
				WHERE snapshot_id = ? AND endpoint IN (?, ?)
				GROUP BY page_number, batch_number
				HAVING SUM(normalized) = 0
			)`, in.SnapshotID, EndpointIdeas, EndpointHistorical).Scan(&pendingGroups); err != nil {
				return fmt.Errorf("check pending receipt normalization: %w", err)
			}
			if pendingGroups > 0 {
				return fmt.Errorf("%w: %d Planner page or batch group(s) remain unnormalized", ErrIncompleteSnapshot, pendingGroups)
			}
			if len(in.MissingBatches) > 0 {
				return fmt.Errorf("%w: %d requested batch(es) are missing", ErrIncompleteSnapshot, len(in.MissingBatches))
			}
		}
		finishedAt := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE snapshots SET status = ?, complete = ?, finalized = 1,
            finished_at = ?, warning_flags = ?, failed_receipts = ?, missing_batches = ?
            WHERE id = ? AND finalized = 0`, status, boolInt(in.Complete), utcText(finishedAt),
			string(warnings), string(failed), string(missing), in.SnapshotID); err != nil {
			return fmt.Errorf("finalize snapshot: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit snapshot finish: %w", err)
		}
		return nil
	})
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate portfolio ID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(b[0:4]), hex.EncodeToString(b[4:6]), hex.EncodeToString(b[6:8]), hex.EncodeToString(b[8:10]), hex.EncodeToString(b[10:16])), nil
}

func bytesHash(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	copyValue := make([]byte, len(value))
	copy(copyValue, value)
	return copyValue
}

func nullableBytes(value []byte) any {
	if value == nil {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
