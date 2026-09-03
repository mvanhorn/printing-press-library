// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package approval persists short-lived, one-time mutation approvals without
// storing full shipment or pickup requests.
package approval

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/secureio"
)

const (
	StatusPending        = "pending"
	StatusExecuting      = "executing"
	StatusSucceeded      = "succeeded"
	StatusRejected       = "rejected"
	StatusOutcomeUnknown = "outcome_unknown"
	StatusExpired        = "expired"
)

var (
	ErrAlreadyConsumed = errors.New("pending operation was already consumed")
	ErrDigestMismatch  = errors.New("confirmation digest does not match pending operation")
	ErrExpired         = errors.New("pending operation expired")
	ErrInvalidID       = errors.New("invalid pending operation ID")
	ErrNotFound        = errors.New("pending operation not found")
	ErrOperationBusy   = errors.New("pending operation is being consumed")
	ErrRequestMismatch = errors.New("request does not match pending operation")
)

// ReviewSummary contains only redacted fields needed to identify the target,
// scope, timing, and cost-sensitive characteristics of a mutation.
type ReviewSummary struct {
	AccountSuffix      string `json:"account_suffix,omitempty"`
	CarrierCode        string `json:"carrier_code,omitempty"`
	ConfirmationSuffix string `json:"confirmation_suffix,omitempty"`
	DeletionControl    string `json:"deletion_control,omitempty"`
	DestinationRegion  string `json:"destination_region,omitempty"`
	LocationSuffix     string `json:"location_suffix,omitempty"`
	PackageCount       int    `json:"package_count,omitempty"`
	PickupWindow       string `json:"pickup_window,omitempty"`
	ScheduledDate      string `json:"scheduled_date,omitempty"`
	SenderCountry      string `json:"sender_country,omitempty"`
	ServiceType        string `json:"service_type,omitempty"`
	TrackingSuffix     string `json:"tracking_suffix,omitempty"`
	WeightSummary      string `json:"weight_summary,omitempty"`
}

// Mutation is the exact operation covered by a confirmation digest.
type Mutation struct {
	Action  string
	Origin  string
	Method  string
	Path    string
	Request any
}

// Permit is produced only by a successful one-time Consume. Its fields are
// private so callers cannot fabricate a protected-operation authorization.
type Permit struct {
	mutation    Mutation
	requestHash string
}

func (p *Permit) Allows(method, path string, request any) bool {
	if p == nil || method != p.mutation.Method || path != p.mutation.Path {
		return false
	}
	mutation := p.mutation
	mutation.Request = request
	hash, err := hashRequest(mutation)
	return err == nil && hash == p.requestHash
}

// Record is the durable approval state. RequestHash cryptographically binds
// version, action, origin, method, path, and exact request without persisting
// the request itself.
type Record struct {
	ID                 string        `json:"id"`
	Action             string        `json:"action"`
	Environment        string        `json:"environment"`
	AccountSuffix      string        `json:"account_suffix,omitempty"`
	RequestHash        string        `json:"request_hash"`
	Review             ReviewSummary `json:"review"`
	Status             string        `json:"status"`
	CreatedAt          time.Time     `json:"created_at"`
	ExpiresAt          time.Time     `json:"expires_at"`
	ConsumedAt         *time.Time    `json:"consumed_at,omitempty"`
	CompletedAt        *time.Time    `json:"completed_at,omitempty"`
	ErrorClass         string        `json:"error_class,omitempty"`
	ConfirmationDigest string        `json:"-"`
}

type Store struct {
	dir string
	ttl time.Duration
	now func() time.Time
}

func NewStore(dir string, ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Store{dir: dir, ttl: ttl, now: time.Now}
}

// DefaultStoreDir returns the local directory used for pending approvals.
func DefaultStoreDir() (string, error) {
	if root := strings.TrimSpace(os.Getenv("FEDEX_DATA_DIR")); root != "" {
		return filepath.Join(root, "pending"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "fedex-pp-cli", "pending"), nil
}

func (s *Store) Create(mutation Mutation, summary ReviewSummary) (*Record, error) {
	if err := validateMutation(mutation); err != nil {
		return nil, err
	}
	requestHash, err := hashRequest(mutation)
	if err != nil {
		return nil, err
	}
	if err := s.ensureDir(); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 3; attempt++ {
		id, err := randomID()
		if err != nil {
			return nil, err
		}
		now := s.now().UTC()
		record := &Record{
			ID:                 id,
			Action:             mutation.Action,
			Environment:        mutation.Origin,
			AccountSuffix:      summary.AccountSuffix,
			RequestHash:        requestHash,
			Review:             summary,
			Status:             StatusPending,
			CreatedAt:          now,
			ExpiresAt:          now.Add(s.ttl),
			ConfirmationDigest: confirmationDigest(requestHash),
		}
		if err := s.writeNewRecord(record); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		return record, nil
	}
	return nil, fmt.Errorf("could not allocate a unique pending operation ID")
}

// Consume atomically validates and marks one pending operation as executing.
// The durable executing transition completes before a permit is returned, so
// no upstream request can start while the record remains reusable.
func (s *Store) Consume(id, digest string, mutation Mutation) (*Record, *Permit, error) {
	if err := validateID(id); err != nil {
		return nil, nil, err
	}
	if err := s.ensureDir(); err != nil {
		return nil, nil, err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return nil, nil, err
	}
	defer unlock()

	record, err := s.readRecord(id)
	if err != nil {
		return nil, nil, err
	}
	if record.Status != StatusPending {
		return nil, nil, ErrAlreadyConsumed
	}
	if digest != confirmationDigest(record.RequestHash) {
		return nil, nil, ErrDigestMismatch
	}
	if s.now().UTC().After(record.ExpiresAt) {
		record.Status = StatusExpired
		if writeErr := s.writeRecord(record); writeErr != nil {
			return nil, nil, writeErr
		}
		return nil, nil, ErrExpired
	}
	if err := validateMutation(mutation); err != nil {
		return nil, nil, err
	}
	requestHash, err := hashRequest(mutation)
	if err != nil {
		return nil, nil, err
	}
	if mutation.Action != record.Action || mutation.Origin != record.Environment || requestHash != record.RequestHash {
		return nil, nil, ErrRequestMismatch
	}

	now := s.now().UTC()
	record.Status = StatusExecuting
	record.ConsumedAt = &now
	if err := s.writeRecord(record); err != nil {
		return nil, nil, err
	}
	return record, &Permit{mutation: mutation, requestHash: requestHash}, nil
}

func (s *Store) Complete(id, status, errorClass string) error {
	if err := validateID(id); err != nil {
		return err
	}
	switch status {
	case StatusSucceeded, StatusRejected, StatusOutcomeUnknown:
	default:
		return fmt.Errorf("invalid completion status %q", status)
	}
	if err := s.ensureDir(); err != nil {
		return err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return err
	}
	defer unlock()

	record, err := s.readRecord(id)
	if err != nil {
		return err
	}
	if record.Status != StatusExecuting {
		return ErrAlreadyConsumed
	}
	now := s.now().UTC()
	record.Status = status
	record.CompletedAt = &now
	record.ErrorClass = errorClass
	return s.writeRecord(record)
}

func (s *Store) ensureDir() error {
	if strings.TrimSpace(s.dir) == "" {
		return fmt.Errorf("pending operation directory is required")
	}
	if filepath.Base(filepath.Clean(s.dir)) == "pending" {
		if err := secureio.EnsurePrivateDir(filepath.Dir(s.dir)); err != nil {
			return fmt.Errorf("securing pending operation root: %w", err)
		}
	}
	if err := secureio.EnsurePrivateDir(s.dir); err != nil {
		return fmt.Errorf("securing pending operation directory: %w", err)
	}
	return nil
}

func (s *Store) recordPath(id string) string { return filepath.Join(s.dir, id+".json") }

func (s *Store) readRecord(id string) (*Record, error) {
	data, err := secureio.ReadFile(s.recordPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading pending operation: %w", err)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decoding pending operation: %w", err)
	}
	record.ConfirmationDigest = confirmationDigest(record.RequestHash)
	return &record, nil
}

func (s *Store) encodeRecord(record *Record) ([]byte, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding pending operation: %w", err)
	}
	return append(data, '\n'), nil
}

func (s *Store) writeNewRecord(record *Record) error {
	data, err := s.encodeRecord(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.recordPath(record.ID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("writing pending operation: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("syncing pending operation: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing pending operation: %w", err)
	}
	return syncDir(s.dir)
}

func (s *Store) writeRecord(record *Record) error {
	data, err := s.encodeRecord(record)
	if err != nil {
		return err
	}
	if err := secureio.WriteFileAtomic(s.recordPath(record.ID), data); err != nil {
		return fmt.Errorf("committing pending operation: %w", err)
	}
	return nil
}

func (s *Store) lock(id string) (func(), error) {
	path := filepath.Join(s.dir, id+".lock")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrOperationBusy
	}
	if err != nil {
		return nil, fmt.Errorf("locking pending operation: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("closing pending operation lock: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func hashRequest(mutation Mutation) (string, error) {
	canonical, err := json.Marshal(struct {
		Version int    `json:"version"`
		Action  string `json:"action"`
		Origin  string `json:"origin"`
		Method  string `json:"method"`
		Path    string `json:"path"`
		Request any    `json:"request"`
	}{1, mutation.Action, mutation.Origin, mutation.Method, mutation.Path, mutation.Request})
	if err != nil {
		return "", fmt.Errorf("canonicalizing mutation request: %w", err)
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

func confirmationDigest(requestHash string) string { return "sha256:" + requestHash }

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generating pending operation ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func validateID(id string) error {
	if len(id) != 32 {
		return ErrInvalidID
	}
	if _, err := hex.DecodeString(id); err != nil {
		return ErrInvalidID
	}
	return nil
}

func validateMutation(mutation Mutation) error {
	if strings.TrimSpace(mutation.Action) == "" || strings.TrimSpace(mutation.Origin) == "" || strings.TrimSpace(mutation.Method) == "" || strings.TrimSpace(mutation.Path) == "" {
		return fmt.Errorf("action, origin, method, and path are required")
	}
	return nil
}

func NormalizeOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("invalid FedEx API origin")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return "", fmt.Errorf("invalid FedEx API origin scheme")
	}
	port := parsed.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return scheme + "://" + net.JoinHostPort(strings.ToLower(parsed.Hostname()), port), nil
}

// Summarize extracts only redacted, operation-critical review fields.
func Summarize(action string, request map[string]any) ReviewSummary {
	summary := ReviewSummary{}
	summary.AccountSuffix = suffix(nestedString(request, "accountNumber", "value"), 4)
	if summary.AccountSuffix == "" {
		summary.AccountSuffix = suffix(nestedString(request, "associatedAccountNumber", "value"), 4)
	}

	switch action {
	case "create_label":
		shipment := nestedMap(request, "requestedShipment")
		summary.ServiceType = directString(shipment, "serviceType")
		summary.ScheduledDate = directString(shipment, "shipDatestamp")
		packages := mapSlice(shipment, "requestedPackageLineItems")
		summary.PackageCount = directInt(shipment, "totalPackageCount")
		if summary.PackageCount == 0 {
			summary.PackageCount = len(packages)
		}
		summary.DestinationRegion = redactedRegion(firstRecipientAddress(shipment))
		summary.WeightSummary = packageWeightSummary(packages)
	case "cancel_shipment":
		summary.TrackingSuffix = suffix(directString(request, "trackingNumber"), 4)
		summary.DeletionControl = directString(request, "deletionControl")
	case "schedule_pickup":
		summary.CarrierCode = directString(request, "carrierCode")
		summary.PackageCount = directInt(request, "packageCount")
		origin := nestedMap(request, "originDetail")
		summary.DestinationRegion = redactedRegion(nestedMap(origin, "pickupAddress"))
		ready := directString(origin, "readyDateTimestamp")
		closeTime := directString(origin, "customerCloseTime")
		if ready != "" || closeTime != "" {
			summary.PickupWindow = strings.TrimSpace(ready + " — " + closeTime)
		}
		weight := nestedMap(request, "totalWeight")
		if value := directNumberString(weight, "value"); value != "" {
			summary.WeightSummary = strings.TrimSpace(value + " " + directString(weight, "units"))
		}
	case "cancel_pickup":
		summary.CarrierCode = directString(request, "carrierCode")
		summary.ConfirmationSuffix = suffix(directString(request, "pickupConfirmationCode"), 4)
		summary.ScheduledDate = directString(request, "scheduledDate")
		summary.LocationSuffix = suffix(directString(request, "location"), 4)
	default:
		// Unknown actions deliberately receive only the account suffix.
	}
	return summary
}

func mapSlice(value map[string]any, key string) []map[string]any {
	if typed, ok := value[key].([]map[string]any); ok {
		return typed
	}
	raw, _ := value[key].([]any)
	result := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if mapped, ok := item.(map[string]any); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func firstRecipientAddress(shipment map[string]any) map[string]any {
	recipients := mapSlice(shipment, "recipients")
	if len(recipients) == 0 {
		return map[string]any{}
	}
	return nestedMap(recipients[0], "address")
}

func packageWeightSummary(packages []map[string]any) string {
	var total float64
	units := ""
	for _, item := range packages {
		weight := nestedMap(item, "weight")
		valueText := directNumberString(weight, "value")
		unit := directString(weight, "units")
		value, err := strconv.ParseFloat(valueText, 64)
		if err != nil || value <= 0 || unit == "" || (units != "" && unit != units) {
			return ""
		}
		units = unit
		total += value
	}
	if total == 0 || units == "" {
		return ""
	}
	return strconv.FormatFloat(total, 'f', -1, 64) + " " + units
}

func nestedMap(value map[string]any, key string) map[string]any {
	result, _ := value[key].(map[string]any)
	if result == nil {
		return map[string]any{}
	}
	return result
}

func nestedString(value map[string]any, parent, key string) string {
	return directString(nestedMap(value, parent), key)
}

func directString(value map[string]any, key string) string {
	result, _ := value[key].(string)
	return strings.TrimSpace(result)
}

func directInt(value map[string]any, key string) int {
	switch number := value[key].(type) {
	case int:
		return number
	case float64:
		return int(number)
	case json.Number:
		parsed, _ := strconv.Atoi(number.String())
		return parsed
	default:
		return 0
	}
}

func directNumberString(value map[string]any, key string) string {
	switch number := value[key].(type) {
	case string:
		return number
	case float64:
		return strconv.FormatFloat(number, 'f', -1, 64)
	case json.Number:
		return number.String()
	default:
		return ""
	}
}

func redactedRegion(address map[string]any) string {
	city := directString(address, "city")
	state := directString(address, "stateOrProvinceCode")
	country := directString(address, "countryCode")
	postal := suffix(directString(address, "postalCode"), 3)
	parts := make([]string, 0, 4)
	for _, value := range []string{city, state, country} {
		if value != "" {
			parts = append(parts, value)
		}
	}
	if postal != "" {
		parts = append(parts, "***"+postal)
	}
	return strings.Join(parts, ", ")
}

func suffix(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[len(value)-length:]
}
