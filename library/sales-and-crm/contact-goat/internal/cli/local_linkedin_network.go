// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	localLinkedInSourceTag = "local_linkedin_export"
	localLinkedInRootEnv   = "CONTACT_GOAT_LINKEDIN_NETWORK_ROOT"
	localLinkedInHostEnv   = "CONTACT_GOAT_LINKEDIN_NETWORK_HOST"
	localLinkedInCmdEnv    = "CONTACT_GOAT_LINKEDIN_NETWORK_CMD"
)

const defaultLocalLinkedInRoot = "/Users/gonkv2/Documents/Agentic Engineering/Playmaker/LinkedIn-Network"

type localLinkedInEnvelope struct {
	Count   int                   `json:"count"`
	Results []localLinkedInPerson `json:"results"`
}

type localLinkedInPerson struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	LinkedInURL       string   `json:"linkedin_url"`
	Email             string   `json:"email"`
	Company           string   `json:"company"`
	Title             string   `json:"title"`
	Owners            []string `json:"owners"`
	Relationship      string   `json:"relationship"`
	Sources           []string `json:"sources"`
	Rationale         string   `json:"rationale"`
	Score             float64  `json:"score"`
	ConnectedOnFirst  string   `json:"connected_on_first"`
	ConnectedOnLatest string   `json:"connected_on_latest"`
}

func fetchLocalLinkedInCompany(ctx context.Context, company string, limit int) ([]flagshipPerson, error) {
	if strings.TrimSpace(company) == "" {
		return nil, errors.New("local LinkedIn company search requires a company")
	}
	return fetchLocalLinkedIn(ctx, []string{"company", company, "--limit", strconv.Itoa(normalizedLimit(limit)), "--format", "json"})
}

func fetchLocalLinkedInSearch(ctx context.Context, query string, limit int) ([]flagshipPerson, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("local LinkedIn search requires a query")
	}
	return fetchLocalLinkedIn(ctx, []string{"search", query, "--limit", strconv.Itoa(normalizedLimit(limit)), "--format", "json"})
}

func fetchLocalLinkedIn(ctx context.Context, args []string) ([]flagshipPerson, error) {
	raw, err := runLocalLinkedInNetwork(ctx, args...)
	if err != nil {
		return nil, err
	}
	var envelope localLinkedInEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse local LinkedIn Network response: %w", err)
	}
	out := make([]flagshipPerson, 0, len(envelope.Results))
	for _, p := range envelope.Results {
		row := localLinkedInToFlagship(p)
		if row.Name == "" && row.LinkedInURL == "" {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func localLinkedInToFlagship(p localLinkedInPerson) flagshipPerson {
	sources := p.Sources
	if len(sources) == 0 {
		sources = []string{localLinkedInSourceTag}
	}
	return flagshipPerson{
		Name:         p.Name,
		LinkedInURL:  p.LinkedInURL,
		Title:        p.Title,
		Company:      p.Company,
		Sources:      sources,
		Rationale:    p.Rationale,
		Relationship: p.Relationship,
		MutualCount:  len(p.Owners),
		Owners:       p.Owners,
		Score:        p.Score,
		Raw:          p,
	}
}

func runLocalLinkedInNetwork(ctx context.Context, args ...string) ([]byte, error) {
	if override := strings.TrimSpace(os.Getenv(localLinkedInCmdEnv)); override != "" {
		return runCommand(ctx, "", override, args...)
	}

	root := strings.TrimSpace(os.Getenv(localLinkedInRootEnv))
	if root == "" {
		root = defaultLocalLinkedInRoot
	}
	if localBin := filepath.Join(root, "linkedin-network"); fileExists(localBin) {
		return runCommand(ctx, root, localBin, args...)
	}

	host := strings.TrimSpace(os.Getenv(localLinkedInHostEnv))
	if host == "" {
		host = "macmini"
	}
	remote := "cd " + shellQuote(root) + " && exec ./linkedin-network " + shellJoin(args)
	return runCommand(ctx, "", "ssh", host, remote)
}

func runCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s failed: %s", name, msg)
	}
	return out, nil
}

func normalizedLimit(limit int) int {
	if limit <= 0 {
		return 25
	}
	return limit
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
