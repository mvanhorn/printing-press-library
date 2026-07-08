package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ContentVersionFetcher streams Salesforce ContentVersion.VersionData bytes.
type ContentVersionFetcher interface {
	FetchContentVersion(ctx context.Context, contentVersionID string) (io.ReadCloser, error)
}

// FileAttestation is the manifest file-byte proof.
type FileAttestation struct {
	Name             string `json:"name"`
	SHA256           string `json:"sha256"`
	ContentVersionID string `json:"sf_content_version_id"`
	SizeBytes        int    `json:"size_bytes"`
}

// HashContentVersion streams a ContentVersion and returns its sha256 and byte
// count without buffering the full file in memory.
func HashContentVersion(ctx context.Context, fetcher ContentVersionFetcher, contentVersionID string) (sha string, size int, err error) {
	if fetcher == nil {
		return "", 0, fmt.Errorf("content version fetcher is required")
	}
	if contentVersionID == "" {
		return "", 0, fmt.Errorf("content version id is required")
	}
	body, err := fetcher.FetchContentVersion(ctx, contentVersionID)
	if err != nil {
		return "", 0, err
	}
	defer body.Close()

	hash := sha256.New()
	n, err := io.Copy(hash, body)
	if err != nil {
		return "", 0, fmt.Errorf("hash ContentVersion %s: %w", contentVersionID, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), int(n), nil
}

// AttestContentVersion builds a FileRef suitable for Manifest.Files.
func AttestContentVersion(ctx context.Context, fetcher ContentVersionFetcher, name, contentVersionID string) (FileRef, error) {
	sha, size, err := HashContentVersion(ctx, fetcher, contentVersionID)
	if err != nil {
		return FileRef{}, err
	}
	return FileRef{Name: name, SHA256: sha, ContentVersionID: contentVersionID, SizeBytes: size}, nil
}

// VerifyFileAttestations re-fetches every manifest file and rejects byte drift.
func VerifyFileAttestations(ctx context.Context, files []FileRef, fetcher ContentVersionFetcher) error {
	for _, file := range files {
		sha, size, err := HashContentVersion(ctx, fetcher, file.ContentVersionID)
		if err != nil {
			return err
		}
		if sha != file.SHA256 || size != file.SizeBytes {
			return fmt.Errorf("FILE_BYTES_TAMPERED: ContentVersion %s expected sha256=%s size=%d got sha256=%s size=%d", file.ContentVersionID, file.SHA256, file.SizeBytes, sha, size)
		}
	}
	return nil
}
