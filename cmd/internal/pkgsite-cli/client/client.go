// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client fetches data from the pkg.go.dev v1 API.
type Client struct {
	server     *url.URL
	httpClient *http.Client
}

// New creates a new Client.
func New(server string) (*Client, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	return &Client{
		server:     u,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// APIError is the error format returned by the v1 API.
type APIError struct {
	Code       int         `json:"code"`
	Message    string      `json:"message"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

type Candidate struct {
	ModulePath  string `json:"modulePath"`
	PackagePath string `json:"packagePath"`
}

func (e *APIError) Error() string {
	if len(e.Candidates) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s; specify module path:\n", e.Message)
		for _, c := range e.Candidates {
			fmt.Fprintf(&b, "  --module=%s\n", c.ModulePath)
		}
		return b.String()
	}
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.Code)
}

// get fetches url and decodes the JSON response into dst.
func (c *Client) get(ctx context.Context, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "pkgsite-cli/v1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // Limit to 1MB
		if err != nil {
			return fmt.Errorf("reading error response: %w", err)
		}
		var aerr APIError
		if json.Unmarshal(body, &aerr) == nil && aerr.Message != "" {
			return &aerr
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

// PackageResponse is the JSON response for /v1/package/.
type PackageResponse struct {
	Path              string            `json:"path"`
	ModulePath        string            `json:"modulePath"`
	ModuleVersion     string            `json:"moduleVersion"`
	Synopsis          string            `json:"synopsis"`
	IsStandardLibrary bool              `json:"isStandardLibrary"`
	IsLatest          bool              `json:"isLatest"`
	GOOS              string            `json:"goos"`
	GOARCH            string            `json:"goarch"`
	Docs              string            `json:"docs,omitempty"`
	Imports           []string          `json:"imports,omitempty"`
	Licenses          []LicenseResponse `json:"licenses,omitempty"`
}

type LicenseResponse struct {
	Types    []string `json:"types"`
	FilePath string   `json:"filePath"`
	Contents string   `json:"contents,omitempty"`
}

// PackageOptions contains options for GetPackage.
type PackageOptions struct {
	Module   string
	Doc      string
	Examples bool
	Imports  bool
	Licenses bool
	GOOS     string
	GOARCH   string
}

func (c *Client) GetPackage(ctx context.Context, path, version string, opts PackageOptions) (*PackageResponse, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	if opts.Doc != "" {
		q.Set("doc", opts.Doc)
	}
	if opts.Examples {
		q.Set("examples", "true")
	}
	if opts.Imports {
		q.Set("imports", "true")
	}
	if opts.Licenses {
		q.Set("licenses", "true")
	}
	if opts.GOOS != "" {
		q.Set("goos", opts.GOOS)
	}
	if opts.GOARCH != "" {
		q.Set("goarch", opts.GOARCH)
	}
	u := c.server.JoinPath("v1", "package", path)
	u.RawQuery = q.Encode()

	var resp PackageResponse
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PaginatedResponse is a generic paginated response.
type PaginatedResponse[T any] struct {
	Items         []T    `json:"items"`
	Total         int    `json:"total"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// SymbolResponse is a single symbol from /v1/symbols/.
type SymbolResponse struct {
	ModulePath string `json:"modulePath"`
	Version    string `json:"version"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Synopsis   string `json:"synopsis"`
	Parent     string `json:"parent,omitempty"`
}

// PaginationOptions contains common pagination options.
type PaginationOptions struct {
	Limit int
	Token string
}

// SymbolsOptions contains options for GetSymbols.
type SymbolsOptions struct {
	Module string
	GOOS   string
	GOARCH string
	PaginationOptions
}

func (c *Client) GetSymbols(ctx context.Context, path, version string, opts SymbolsOptions) (*PaginatedResponse[SymbolResponse], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	if opts.GOOS != "" {
		q.Set("goos", opts.GOOS)
	}
	if opts.GOARCH != "" {
		q.Set("goarch", opts.GOARCH)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "symbols", path)
	u.RawQuery = q.Encode()
	var resp PaginatedResponse[SymbolResponse]
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ImportedByResponse is the response for /v1/imported-by/.
type ImportedByResponse struct {
	ModulePath string                    `json:"modulePath"`
	Version    string                    `json:"version"`
	ImportedBy PaginatedResponse[string] `json:"importedBy"`
}

// ImportedByOptions contains options for GetImportedBy.
type ImportedByOptions struct {
	Module string
	PaginationOptions
}

func (c *Client) GetImportedBy(ctx context.Context, path, version string, opts ImportedByOptions) (*ImportedByResponse, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Module != "" {
		q.Set("module", opts.Module)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "imported-by", path)
	u.RawQuery = q.Encode()
	var resp ImportedByResponse
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ModuleResponse is the JSON response for /v1/module/.
type ModuleResponse struct {
	Path              string            `json:"path"`
	Version           string            `json:"version"`
	IsLatest          bool              `json:"isLatest"`
	IsRedistributable bool              `json:"isRedistributable"`
	IsStandardLibrary bool              `json:"isStandardLibrary"`
	HasGoMod          bool              `json:"hasGoMod"`
	RepoURL           string            `json:"repoUrl"`
	Readme            *ReadmeResponse   `json:"readme,omitempty"`
	Licenses          []LicenseResponse `json:"licenses,omitempty"`
}

type ReadmeResponse struct {
	Filepath string `json:"filepath"`
	Contents string `json:"contents"`
}

// ModuleOptions contains options for GetModule.
type ModuleOptions struct {
	Readme   bool
	Licenses bool
}

func (c *Client) GetModule(ctx context.Context, path, version string, opts ModuleOptions) (*ModuleResponse, error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Readme {
		q.Set("readme", "true")
	}
	if opts.Licenses {
		q.Set("licenses", "true")
	}
	u := c.server.JoinPath("v1", "module", path)
	u.RawQuery = q.Encode()
	var resp ModuleResponse
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// VersionResponse is a single version from /v1/versions/.
type VersionResponse struct {
	Version string `json:"version"`
}

func (c *Client) GetVersions(ctx context.Context, path string, opts PaginationOptions) (*PaginatedResponse[VersionResponse], error) {
	q := make(url.Values)
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "versions", path)
	u.RawQuery = q.Encode()
	var resp PaginatedResponse[VersionResponse]
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// VulnResponse is a single vulnerability from /v1/vulns/.
type VulnResponse struct {
	ID           string `json:"id"`
	Summary      string `json:"summary"`
	Details      string `json:"details"`
	FixedVersion string `json:"fixedVersion"`
}

func (c *Client) GetVulns(ctx context.Context, path, version string, opts PaginationOptions) (*PaginatedResponse[VulnResponse], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "vulns", path)
	u.RawQuery = q.Encode()
	var resp PaginatedResponse[VulnResponse]
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ModulePackageResponse is a single package from /v1/packages/.
type ModulePackageResponse struct {
	Path     string `json:"path"`
	Synopsis string `json:"synopsis"`
}

func (c *Client) GetPackages(ctx context.Context, modulePath, version string, opts PaginationOptions) (*PaginatedResponse[ModulePackageResponse], error) {
	q := make(url.Values)
	if version != "" {
		q.Set("version", version)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "packages", modulePath)
	u.RawQuery = q.Encode()
	var resp PaginatedResponse[ModulePackageResponse]
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SearchResultResponse is a single search result from /v1/search/.
type SearchResultResponse struct {
	PackagePath string `json:"packagePath"`
	ModulePath  string `json:"modulePath"`
	Version     string `json:"version"`
	Synopsis    string `json:"synopsis"`
}

// SearchOptions contains options for Search.
type SearchOptions struct {
	Symbol string
	PaginationOptions
}

func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) (*PaginatedResponse[SearchResultResponse], error) {
	q := make(url.Values)
	q.Set("q", query)
	if opts.Symbol != "" {
		q.Set("symbol", opts.Symbol)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Token != "" {
		q.Set("token", opts.Token)
	}
	u := c.server.JoinPath("v1", "search")
	u.RawQuery = q.Encode()
	var resp PaginatedResponse[SearchResultResponse]
	if err := c.get(ctx, u.String(), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
