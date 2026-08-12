// SPDX-License-Identifier: 0BSD

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"example.com/eql-shaped/views"
	"gamertan.com/sandwich-hime/sando"
)

func main() {
	address := os.Getenv("HIMESAN_LISTEN_ADDR")
	if address != "" {
		serve(address)
		return
	}

	output, err := renderPage(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = output.WriteTo(os.Stdout)
}

func serve(address string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /", func(response http.ResponseWriter, request *http.Request) {
		output, err := renderPage(request.Context())
		if err != nil {
			http.Error(response, "render failed", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'")
		response.WriteHeader(http.StatusOK)
		_, _ = output.WriteTo(response)
	})

	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func renderPage(ctx context.Context) (*bytes.Buffer, error) {
	body := views.Home(views.HomeView{
		Heading: "EQL-shaped records",
		Intro:   "Typed markup without making the template compiler your web framework.",
		Browse: views.BrowseView{
			Query: "pioneer & archivist",
			Records: []views.RecordView{
				{URL: "/items/1?from=home&kind=book", Title: "A <field> guide", Kind: "book", Featured: true},
				{URL: "/items/2", Title: "Community memory", Kind: "archive"},
			},
		},
	})

	page := views.Layout(views.LayoutView{
		SiteName: "EQL Wiki Fixture",
		Title:    "Home",
		Body:     body,
	})

	var output bytes.Buffer
	if err := sando.Render(ctx, &output, page); err != nil {
		return nil, err
	}
	return &output, nil
}
