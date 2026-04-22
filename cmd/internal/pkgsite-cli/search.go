// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"io"
	"strings"
	"time"

	"golang.org/x/pkgsite/cmd/internal/pkgsite-cli/client"
)

func runSearch(fs *flag.FlagSet, s *searchFlags, stdout, stderr io.Writer) int {
	if fs.NArg() < 1 {
		fs.Usage()
		return 2
	}
	query := strings.Join(fs.Args(), " ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.New(s.server)
	if err != nil {
		handleErr(stdout, stderr, err, s.jsonOut)
		return 1
	}
	results, err := c.Search(ctx, query, client.SearchOptions{
		Symbol: s.symbol,
		PaginationOptions: client.PaginationOptions{
			Limit: s.effectiveLimit(),
			Token: s.token,
		},
	})
	if err != nil {
		handleErr(stdout, stderr, err, s.jsonOut)
		return 1
	}

	if s.jsonOut {
		return writeJSON(stdout, stderr, results)
	}
	formatSearch(stdout, results)
	return 0
}

// searchFlags are flags for the search subcommand.
type searchFlags struct {
	commonFlags
	symbol string
}

func (f *searchFlags) register(fs *flag.FlagSet) {
	f.commonFlags.register(fs)
	fs.StringVar(&f.symbol, "symbol", "", "search for a symbol")
}
