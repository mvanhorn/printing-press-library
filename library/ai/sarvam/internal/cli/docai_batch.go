// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelDocaiBatchCmd(flags *rootFlags) *cobra.Command {
	var flagSchema string
	var flagDir string
	var flagOut string
	var flagLanguage string
	var flagWait bool

	cmd := &cobra.Command{
		Use:         "batch",
		Short:       "Run a saved extraction schema over a folder of documents with job pacing",
		Example:     "  sarvam-pp-cli docai batch --schema invoice-v1 --dir ./docs/ --out ./results/",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--schema=invoice-v1;--dir=./docs/", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docai batch")
			}
			if flagSchema == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--schema is required"))
			}
			if flagDir == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dir is required"))
			}
			if flagOut == "" {
				flagOut = "./results"
			}
			if flagLanguage == "" {
				flagLanguage = "en-IN"
			}

			// Load the saved schema from local store.
			schemaJSON, err := loadDocaiSchema(cmd, flagSchema)
			if err != nil {
				return err
			}

			// Enumerate documents in the folder.
			entries, err := os.ReadDir(flagDir)
			if err != nil {
				return fmt.Errorf("reading dir %s: %w", flagDir, err)
			}
			docs := make([]string, 0)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(e.Name()))
				switch ext {
				case ".pdf", ".png", ".jpg", ".jpeg", ".tiff", ".tif", ".webp", ".bmp":
					docs = append(docs, filepath.Join(flagDir, e.Name()))
				}
			}
			if len(docs) == 0 {
				return notFoundErr(fmt.Errorf("no document files (pdf/png/jpg/tiff) found in %s", flagDir))
			}
			if err := os.MkdirAll(flagOut, 0o750); err != nil {
				return fmt.Errorf("creating out dir: %w", err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type docResult struct {
				File     string          `json:"file"`
				JobID    string          `json:"job_id,omitempty"`
				Status   string          `json:"status"`
				Result   json.RawMessage `json:"result,omitempty"`
				Error    string          `json:"error,omitempty"`
			}
			results := make([]docResult, 0, len(docs))
			for _, doc := range docs {
				res := docResult{File: filepath.Base(doc), Status: "started"}
				// 1. Presign upload URL.
				uploadData, _, err := c.Post(ctx, "/doc-ai/v1/job/upload", map[string]any{
					"content_type": mimeTypeFor(doc),
					"file_name":    filepath.Base(doc),
				})
				if err != nil {
					res.Error = err.Error()
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				var uploadResp struct {
					UploadID  string `json:"upload_id"`
					UploadURL string `json:"upload_url"`
				}
				if err := json.Unmarshal(uploadData, &uploadResp); err != nil || uploadResp.UploadURL == "" {
					res.Error = "no upload URL returned"
					res.Status = "failed"
					results = append(results, res)
					continue
				}

				// 2. Upload the file to the presigned URL.
				// #nosec G304 -- doc is a path enumerated from the user-supplied --dir.
				fileBytes, err := os.ReadFile(doc)
				if err != nil {
					res.Error = err.Error()
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadResp.UploadURL, strings.NewReader(string(fileBytes)))
				if err != nil {
					res.Error = err.Error()
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				req.Header.Set("Content-Type", mimeTypeFor(doc))
				uploadClient := c.HTTPClient
				if uploadClient == nil {
					uploadClient = http.DefaultClient
				}
				upResp, err := uploadClient.Do(req)
				if err != nil {
					res.Error = err.Error()
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				_, _ = io.Copy(io.Discard, upResp.Body)
				_ = upResp.Body.Close()
				if upResp.StatusCode >= 400 {
					res.Error = fmt.Sprintf("upload HTTP %d", upResp.StatusCode)
					res.Status = "failed"
					results = append(results, res)
					continue
				}

				// 3. Create the extract job with the saved schema.
				extractData, _, err := c.PostMultipart(ctx, "/doc-ai/v1/job/extract", map[string]string{
					"upload_ids":    uploadResp.UploadID,
					"schema":        string(schemaJSON),
					"language":      flagLanguage,
					"output_format": "json",
				}, nil)
				if err != nil {
					res.Error = err.Error()
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				var extractResp struct {
					JobID  string `json:"job_id"`
					Status string `json:"status"`
				}
				if err := json.Unmarshal(extractData, &extractResp); err != nil || extractResp.JobID == "" {
					res.Error = "no job id returned"
					res.Status = "failed"
					results = append(results, res)
					continue
				}
				res.JobID = extractResp.JobID
				res.Status = extractResp.Status

				// 4. Poll until terminal (if --wait) or record the job id.
				if flagWait {
					terminal := false
					for i := 0; i < 60 && !terminal; i++ {
						statusData, err := c.GetNoCache(ctx, "/doc-ai/v1/job/"+extractResp.JobID+"/status", nil)
						if err != nil {
							res.Error = err.Error()
							res.Status = "failed"
							terminal = true
							break
						}
						var jobStatus struct {
							Status string `json:"status"`
						}
						_ = json.Unmarshal(statusData, &jobStatus)
						res.Status = jobStatus.Status
						switch jobStatus.Status {
						case "completed", "partially_completed", "failed", "rejected":
							terminal = true
							// Fetch results for completed jobs.
							if jobStatus.Status == "completed" || jobStatus.Status == "partially_completed" {
								resultsData, err := c.GetNoCache(ctx, "/doc-ai/v1/job/"+extractResp.JobID+"/results", nil)
								if err == nil {
									res.Result = resultsData
									outFile := filepath.Join(flagOut, strings.TrimSuffix(filepath.Base(doc), filepath.Ext(doc))+".json")
									// #nosec G306 -- user-facing extraction result the caller explicitly requested; 0644 keeps it readable by downstream tooling.
									if werr := os.WriteFile(outFile, resultsData, 0o644); werr != nil {
										res.Error = "writing result: " + werr.Error()
										res.Status = "failed"
									}
								} else {
									res.Error = "fetching results: " + err.Error()
									res.Status = "failed"
								}
							}
						default:
							select {
							case <-ctx.Done():
								terminal = true
							case <-time.After(3 * time.Second):
							}
						}
					}
					// Anything short of a genuine completed/partially_completed
					// status produced no saved result — rejected, failed,
					// poll-exhausted, or the ctx deadline firing mid-poll.
					// Surface that as an error so it counts as a failure
					// below instead of silently reporting success.
					if res.Error == "" {
						if reason := docaiBatchFailureReason(res.Status); reason != "" {
							res.Error = reason
						}
					}
				}
				results = append(results, res)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"schema":  flagSchema,
					"dir":     flagDir,
					"results": results,
				}, flags); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					if r.Error != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%-30s FAIL: %s\n", r.File, r.Error)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%-30s %s job=%s\n", r.File, r.Status, r.JobID)
					}
				}
			}

			// Surface persistence failures through the exit code so scripts
			// and agents can detect failed result writes, not just read about
			// them in the output. Any failed doc (write error, API error, or
			// non-terminal status) makes the run exit non-zero.
			failed := 0
			for _, r := range results {
				if r.Error != "" || r.Status == "failed" {
					failed++
				}
			}
			if failed > 0 {
				return partialFailureErr(fmt.Errorf("%d of %d document(s) failed to extract or save results", failed, len(results)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSchema, "schema", "", "Name of the saved extraction schema")
	cmd.Flags().StringVar(&flagDir, "dir", "", "Directory containing documents to process")
	cmd.Flags().StringVar(&flagOut, "out", "./results", "Directory to write extraction results")
	cmd.Flags().StringVar(&flagLanguage, "language", "en-IN", "Document language code (BCP-47)")
	cmd.Flags().BoolVar(&flagWait, "wait", true, "Poll each job to completion before moving on")
	return cmd
}

func loadDocaiSchema(cmd *cobra.Command, name string) (json.RawMessage, error) {
	db, err := openDocaiSchemaDB(cmd)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, apiErr(fmt.Errorf("no local database. Run 'sarvam-pp-cli sync' first"))
	}
	defer db.Close()
	raw, err := db.Get(docaiSchemaResourceType, name)
	if err != nil {
		return nil, notFoundErr(fmt.Errorf("schema %q not found; save it first with 'docai schema save %s --file schema.json'", name, name))
	}
	var entry docaiSchemaEntry
	if err := json.Unmarshal(raw, &entry); err != nil || len(entry.Schema) == 0 {
		return nil, apiErr(fmt.Errorf("schema %q is corrupt", name))
	}
	return entry.Schema, nil
}

// docaiBatchFailureReason reports why a batch item never produced a saved
// result. Only "completed" and "partially_completed" are genuine successes;
// any other final status — failed, rejected, still non-terminal because the
// poll loop exhausted, or still non-terminal because the command's context
// deadline fired mid-poll (ctx.Done()) — means no result was saved and is
// surfaced as an error. Checking the status alone (rather than tracking why
// the poll loop stopped) covers all of these exit paths uniformly.
func docaiBatchFailureReason(status string) string {
	switch status {
	case "completed", "partially_completed":
		return ""
	default:
		return fmt.Sprintf("job did not complete (status: %s)", status)
	}
}

func mimeTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
