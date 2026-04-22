// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetPackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/package/encoding/json" {
			t.Errorf("path = %q, want /v1/package/encoding/json", r.URL.Path)
		}
		if got := r.Header.Get("User-Agent"); got != "pkgsite-cli/v1" {
			t.Errorf("User-Agent = %q, want pkgsite-cli/v1", got)
		}
		if got := r.URL.Query().Get("version"); got != "go1.26.0" {
			t.Errorf("version = %q, want go1.26.0", got)
		}
		json.NewEncoder(w).Encode(PackageResponse{
			Path:              "encoding/json",
			ModulePath:        "std",
			ModuleVersion:     "go1.26.0",
			Synopsis:          "Package json implements encoding and decoding of JSON.",
			IsStandardLibrary: true,
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetPackage(context.Background(), "encoding/json", "go1.26.0", PackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Path != "encoding/json" {
		t.Errorf("Path = %q, want encoding/json", resp.Path)
	}
	if !resp.IsStandardLibrary {
		t.Error("IsStandardLibrary = false, want true")
	}
}

func TestGetPackageWithFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("doc"); got != "md" {
			t.Errorf("doc = %q, want md", got)
		}
		if got := q.Get("imports"); got != "true" {
			t.Errorf("imports = %q, want true", got)
		}
		if got := q.Get("licenses"); got != "true" {
			t.Errorf("licenses = %q, want true", got)
		}
		if got := q.Get("module"); got != "github.com/foo/bar" {
			t.Errorf("module = %q, want github.com/foo/bar", got)
		}
		json.NewEncoder(w).Encode(PackageResponse{
			Path:    "github.com/foo/bar/pkg",
			Docs:    "# package pkg",
			Imports: []string{"fmt", "strings"},
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetPackage(context.Background(), "github.com/foo/bar/pkg", "", PackageOptions{
		Doc:      "md",
		Imports:  true,
		Licenses: true,
		Module:   "github.com/foo/bar",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Docs != "# package pkg" {
		t.Errorf("Docs = %q, want # package pkg", resp.Docs)
	}
	if len(resp.Imports) != 2 {
		t.Errorf("len(Imports) = %d, want 2", len(resp.Imports))
	}
}

func TestGetModule(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/module/golang.org/x/text" {
			t.Errorf("path = %q, want /v1/module/golang.org/x/text", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ModuleResponse{
			Path:    "golang.org/x/text",
			Version: "v0.14.0",
			RepoURL: "https://github.com/golang/text",
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetModule(context.Background(), "golang.org/x/text", "v0.14.0", ModuleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Version != "v0.14.0" {
		t.Errorf("Version = %q, want v0.14.0", resp.Version)
	}
}

func TestGetVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/versions/golang.org/x/text" {
			t.Errorf("path = %q, want /v1/versions/golang.org/x/text", r.URL.Path)
		}
		json.NewEncoder(w).Encode(PaginatedResponse[VersionResponse]{
			Items: []VersionResponse{{Version: "v0.14.0"}, {Version: "v0.13.0"}},
			Total: 2,
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetVersions(context.Background(), "golang.org/x/text", PaginationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(resp.Items))
	}
}

func TestGetVulns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[VulnResponse]{
			Items: []VulnResponse{{ID: "GO-2023-0001", Details: "A vulnerability."}},
			Total: 1,
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetVulns(context.Background(), "golang.org/x/text", "v0.3.0", PaginationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].ID != "GO-2023-0001" {
		t.Errorf("ID = %q, want GO-2023-0001", resp.Items[0].ID)
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "json parser" {
			t.Errorf("q = %q, want %q", got, "json parser")
		}
		json.NewEncoder(w).Encode(PaginatedResponse[SearchResultResponse]{
			Items: []SearchResultResponse{{
				PackagePath: "encoding/json",
				ModulePath:  "std",
				Version:     "go1.26.0",
				Synopsis:    "Package json implements encoding and decoding of JSON.",
			}},
			Total: 1,
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Search(context.Background(), "json parser", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(resp.Items))
	}
}

func TestGetSymbols(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(PaginatedResponse[SymbolResponse]{
			Items: []SymbolResponse{{
				Name:     "Marshal",
				Kind:     "func",
				Synopsis: "func Marshal(v any) ([]byte, error)",
			}},
			Total: 1,
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetSymbols(context.Background(), "encoding/json", "", SymbolsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Items[0].Name != "Marshal" {
		t.Errorf("Name = %q, want Marshal", resp.Items[0].Name)
	}
}

func TestGetImportedBy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ImportedByResponse{
			ModulePath: "std",
			Version:    "go1.26.0",
			ImportedBy: PaginatedResponse[string]{
				Items: []string{"github.com/foo/bar", "github.com/baz/qux"},
				Total: 2,
			},
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.GetImportedBy(context.Background(), "encoding/json", "", ImportedByOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ImportedBy.Items) != 2 {
		t.Errorf("len(ImportedBy.Items) = %d, want 2", len(resp.ImportedBy.Items))
	}
}

func TestAmbiguousPackagePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIError{
			Code:    400,
			Message: "ambiguous package path",
			Candidates: []Candidate{
				{ModulePath: "github.com/foo/bar", PackagePath: "github.com/foo/bar/pkg"},
				{ModulePath: "github.com/foo/bar/pkg", PackagePath: "github.com/foo/bar/pkg"},
			},
		})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetPackage(context.Background(), "github.com/foo/bar/pkg", "", PackageOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--module=github.com/foo/bar") {
		t.Errorf("error missing candidate, got:\n%s", msg)
	}
	if !strings.Contains(msg, "--module=github.com/foo/bar/pkg") {
		t.Errorf("error missing candidate, got:\n%s", msg)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIError{Code: 404, Message: "not found"})
	}))
	defer srv.Close()

	c, err := New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetPackage(context.Background(), "nonexistent/pkg", "", PackageOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	aerr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if aerr.Code != 404 {
		t.Errorf("Code = %d, want 404", aerr.Code)
	}
}
