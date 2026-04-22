// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"io"
	"time"

	"golang.org/x/pkgsite/cmd/internal/pkgsite-cli/client"
	"golang.org/x/sync/errgroup"
)

func runModule(fs *flag.FlagSet, m *moduleFlags, stdout, stderr io.Writer) int {
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	path, version := splitPathVersion(fs.Arg(0))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.New(m.server)
	if err != nil {
		handleErr(stdout, stderr, err, m.jsonOut)
		return 1
	}
	mod, err := c.GetModule(ctx, path, version, client.ModuleOptions{
		Readme:   m.readme,
		Licenses: m.licenses,
	})
	if err != nil {
		handleErr(stdout, stderr, err, m.jsonOut)
		return 1
	}
	result := moduleResult{Module: mod}

	var (
		vers  *client.PaginatedResponse[client.VersionResponse]
		vulns *client.PaginatedResponse[client.Vulnerability]
		pkgs  *client.PaginatedResponse[client.ModulePackageResponse]
	)

	g, gctx := errgroup.WithContext(ctx)

	if m.versions {
		g.Go(func() error {
			var err error
			vers, err = c.GetVersions(gctx, path, client.PaginationOptions{
				Limit: m.effectiveLimit(),
				Token: m.token,
			})
			return err
		})
	}
	if m.vulns {
		g.Go(func() error {
			var err error
			vulns, err = c.GetVulns(gctx, path, version, client.PaginationOptions{
				Limit: m.effectiveLimit(),
				Token: m.token,
			})
			return err
		})
	}
	if m.packages {
		g.Go(func() error {
			var err error
			pkgs, err = c.GetPackages(gctx, path, version, client.PaginationOptions{
				Limit: m.effectiveLimit(),
				Token: m.token,
			})
			return err
		})
	}

	if err := g.Wait(); err != nil {
		handleErr(stdout, stderr, err, m.jsonOut)
		return 1
	}

	result.Versions = vers
	result.Vulns = vulns
	result.Packages = pkgs

	if m.jsonOut {
		return writeJSON(stdout, stderr, result)
	}
	formatModule(stdout, result)
	return 0
}

// moduleFlags are flags for the module subcommand.
type moduleFlags struct {
	commonFlags
	readme   bool
	licenses bool
	versions bool
	vulns    bool
	packages bool
}

func (f *moduleFlags) register(fs *flag.FlagSet) {
	f.commonFlags.register(fs)
	fs.BoolVar(&f.readme, "readme", false, "include README")
	fs.BoolVar(&f.licenses, "licenses", false, "show license information")
	fs.BoolVar(&f.versions, "versions", false, "list versions")
	fs.BoolVar(&f.vulns, "vulns", false, "list vulnerabilities")
	fs.BoolVar(&f.packages, "packages", false, "list packages")
}
