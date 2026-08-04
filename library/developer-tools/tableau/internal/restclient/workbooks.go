package client

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// singlePublishMaxBytes is the Tableau single-request publish limit (64 MiB).
// Above this we use multi-part file upload.
const singlePublishMaxBytes = 64 * 1024 * 1024

// appendChunkSize is the size of each multi-part upload block (under 64 MiB).
const appendChunkSize = 50 * 1024 * 1024

// ListWorkbooks returns workbooks on the signed-in site.
// If projectID is non-empty, results are filtered to that project.
func (c *Client) ListWorkbooks(projectID string) ([]Workbook, error) {
	if err := c.EnsureSignedIn(); err != nil {
		return nil, err
	}
	return getAllPages(func(page int) ([]Workbook, int, error) {
		base := c.siteURL("workbooks")
		u, err := url.Parse(base)
		if err != nil {
			return nil, 0, err
		}
		q := u.Query()
		q.Set("pageSize", fmt.Sprintf("%d", pageSize))
		q.Set("pageNumber", fmt.Sprintf("%d", page))
		if projectID != "" {
			q.Set("filter", "projectId:eq:"+projectID)
		}
		u.RawQuery = q.Encode()

		status, data, err := c.doAuth(http.MethodGet, u.String(), nil, "")
		if err != nil {
			return nil, 0, fmt.Errorf("list workbooks: %w", err)
		}
		if status != http.StatusOK {
			if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
				return nil, 0, fmt.Errorf("list workbooks (HTTP %d): %w", status, apiErr)
			}
			return nil, 0, fmt.Errorf("list workbooks failed (HTTP %d): %s", status, truncate(string(data), 300))
		}
		workbooks, pag, err := ParseWorkbooksResponse(bytes.NewReader(data))
		if err != nil {
			return nil, 0, err
		}
		return workbooks, paginationTotal(pag), nil
	})
}

// DownloadWorkbook downloads a workbook (.twb or .twbx) to outputPath.
func (c *Client) DownloadWorkbook(workbookID, outputPath string) error {
	if workbookID == "" {
		return fmt.Errorf("workbook id is required")
	}
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if err := c.EnsureSignedIn(); err != nil {
		return err
	}

	u := c.siteURL(fmt.Sprintf("workbooks/%s/content", url.PathEscape(workbookID)))
	// includeExtract=true packages extracts into .twbx when present.
	u += "?includeExtract=true"

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Tableau-Auth", c.cred.Token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download workbook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return fmt.Errorf("download workbook (HTTP %d): %w", resp.StatusCode, apiErr)
		}
		return fmt.Errorf("download workbook failed (HTTP %d): %s", resp.StatusCode, truncate(string(data), 300))
	}

	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write workbook: %w", err)
	}
	return nil
}

// PublishWorkbook uploads a local .twb/.twbx to the given project.
// Files larger than 64 MiB use multi-part upload automatically.
func (c *Client) PublishWorkbook(filePath, projectID, name string, overwrite bool) (*PublishResult, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	if name == "" {
		return nil, fmt.Errorf("workbook name is required")
	}
	if err := c.EnsureSignedIn(); err != nil {
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat publish file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("publish file is a directory: %s", filePath)
	}

	workbookType, err := workbookTypeFromPath(filePath)
	if err != nil {
		return nil, err
	}

	if info.Size() > singlePublishMaxBytes {
		return c.publishMultipart(filePath, projectID, name, workbookType, overwrite)
	}
	return c.publishSingle(filePath, projectID, name, workbookType, overwrite)
}

func workbookTypeFromPath(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".twb":
		return "twb", nil
	case ".twbx":
		return "twbx", nil
	default:
		return "", fmt.Errorf("unsupported workbook extension %q (want .twb or .twbx)", ext)
	}
}

func (c *Client) publishSingle(filePath, projectID, name, workbookType string, overwrite bool) (*PublishResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open publish file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	boundary := "tableau-rest-pp-cli-boundary"
	w := multipart.NewWriter(&body)
	if err := w.SetBoundary(boundary); err != nil {
		return nil, err
	}

	// Tableau requires multipart/mixed with specific Content-Disposition names.
	payloadHeader := make(textproto.MIMEHeader)
	payloadHeader.Set("Content-Disposition", `name="request_payload"`)
	payloadHeader.Set("Content-Type", "text/xml")
	pw, err := w.CreatePart(payloadHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(pw, BuildPublishWorkbookPayload(name, projectID)); err != nil {
		return nil, err
	}

	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`name="tableau_workbook"; filename=%q`, filepath.Base(filePath)))
	fileHeader.Set("Content-Type", "application/octet-stream")
	fw, err := w.CreatePart(fileHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("read publish file: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	u := c.siteURL("workbooks")
	q := url.Values{}
	q.Set("workbookType", workbookType)
	if overwrite {
		q.Set("overwrite", "true")
	}
	u += "?" + q.Encode()

	status, data, err := c.doAuth(http.MethodPost, u, &body, "multipart/mixed; boundary="+boundary)
	if err != nil {
		return nil, fmt.Errorf("publish workbook: %w", err)
	}
	if status != http.StatusCreated && status != http.StatusOK {
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return nil, fmt.Errorf("publish workbook (HTTP %d): %w", status, apiErr)
		}
		return nil, fmt.Errorf("publish workbook failed (HTTP %d): %s", status, truncate(string(data), 300))
	}
	return ParsePublishResponse(bytes.NewReader(data))
}

func (c *Client) publishMultipart(filePath, projectID, name, workbookType string, overwrite bool) (*PublishResult, error) {
	// 1. Initiate file upload
	status, data, err := c.doAuth(http.MethodPost, c.siteURL("fileUploads"), nil, "")
	if err != nil {
		return nil, fmt.Errorf("initiate file upload: %w", err)
	}
	if status != http.StatusCreated && status != http.StatusOK {
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return nil, fmt.Errorf("initiate file upload (HTTP %d): %w", status, apiErr)
		}
		return nil, fmt.Errorf("initiate file upload failed (HTTP %d)", status)
	}
	sessionID, err := ParseFileUploadResponse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// 2. Append chunks
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open publish file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, appendChunkSize)
	seq := 0
	for {
		n, readErr := io.ReadFull(file, buf)
		if n > 0 {
			seq++
			if err := c.appendUploadChunk(sessionID, filepath.Base(filePath), buf[:n], seq); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read publish file: %w", readErr)
		}
	}

	// 3. Commit publish
	var body bytes.Buffer
	boundary := "tableau-rest-pp-cli-commit"
	w := multipart.NewWriter(&body)
	if err := w.SetBoundary(boundary); err != nil {
		return nil, err
	}
	payloadHeader := make(textproto.MIMEHeader)
	payloadHeader.Set("Content-Disposition", `name="request_payload"`)
	payloadHeader.Set("Content-Type", "text/xml")
	pw, err := w.CreatePart(payloadHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(pw, BuildPublishWorkbookPayload(name, projectID)); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("uploadSessionId", sessionID)
	q.Set("workbookType", workbookType)
	if overwrite {
		q.Set("overwrite", "true")
	}
	u := c.siteURL("workbooks") + "?" + q.Encode()

	status, data, err = c.doAuth(http.MethodPost, u, &body, "multipart/mixed; boundary="+boundary)
	if err != nil {
		return nil, fmt.Errorf("commit publish: %w", err)
	}
	if status != http.StatusCreated && status != http.StatusOK {
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return nil, fmt.Errorf("commit publish (HTTP %d): %w", status, apiErr)
		}
		return nil, fmt.Errorf("commit publish failed (HTTP %d): %s", status, truncate(string(data), 300))
	}
	return ParsePublishResponse(bytes.NewReader(data))
}

func (c *Client) appendUploadChunk(sessionID, filename string, chunk []byte, sequenceID int) error {
	var body bytes.Buffer
	boundary := "tableau-rest-pp-cli-append"
	w := multipart.NewWriter(&body)
	if err := w.SetBoundary(boundary); err != nil {
		return err
	}

	// Blank request_payload part (required by Tableau for append).
	payloadHeader := make(textproto.MIMEHeader)
	payloadHeader.Set("Content-Disposition", `name="request_payload"`)
	payloadHeader.Set("Content-Type", "text/xml")
	pw, err := w.CreatePart(payloadHeader)
	if err != nil {
		return err
	}
	// Empty body for request_payload on append.
	_, _ = pw.Write(nil)

	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`name="tableau_file"; filename=%q`, filename))
	fileHeader.Set("Content-Type", "application/octet-stream")
	fw, err := w.CreatePart(fileHeader)
	if err != nil {
		return err
	}
	if _, err := fw.Write(chunk); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	u := c.siteURL(fmt.Sprintf("fileUploads/%s", url.PathEscape(sessionID)))
	// sequenceID helps parallel assembly on API 3.27+; harmless on older if ignored via query.
	u += fmt.Sprintf("?sequenceID=%d", sequenceID)

	req, err := http.NewRequest(http.MethodPut, u, &body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Tableau-Auth", c.cred.Token)
	req.Header.Set("Content-Type", "multipart/mixed; boundary="+boundary)
	req.Header.Set("Accept", "application/xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("append file upload: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return fmt.Errorf("append file upload (HTTP %d): %w", resp.StatusCode, apiErr)
		}
		return fmt.Errorf("append file upload failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}
