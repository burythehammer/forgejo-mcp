// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
)

func TestGetFileReadsASmallFileWhole(t *testing.T) {
	t.Parallel()
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello\n"))
	})
	got, err := rawAdapter(t, srv).GetFile(context.Background(), "o", "r", "main", "a.txt")
	if err != nil || string(got) != "hello\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGetFileRefusesASourceOverTheCap(t *testing.T) {
	t.Parallel()
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), forge.MaxFileSourceBytes+1))
	})
	_, err := rawAdapter(t, srv).GetFile(context.Background(), "o", "r", "main", "big.bin")
	if !errors.Is(err, forge.ErrPayloadTooLarge) {
		t.Fatalf("want ErrPayloadTooLarge, got %v", err)
	}
}

func TestGetFileReportsAServerFailureNotItsErrorBody(t *testing.T) {
	t.Parallel()
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"file does not exist"}`))
	})
	got, err := rawAdapter(t, srv).GetFile(context.Background(), "o", "r", "main", "missing.txt")
	if err == nil || got != nil {
		t.Fatalf("want an error and no bytes, got %q, %v", got, err)
	}
	var se *forge.StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusNotFound {
		t.Fatalf("want a 404 StatusError, got %v", err)
	}
}

func TestGetContentsRefusesABodyOverTheCap(t *testing.T) {
	t.Parallel()
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), contentsBodyCap+1))
	})
	_, err := rawAdapter(t, srv).GetContents(context.Background(), "o", "r", "main", "big.bin")
	if !errors.Is(err, forge.ErrPayloadTooLarge) {
		t.Fatalf("want ErrPayloadTooLarge, got %v", err)
	}
}

func TestGetContentsKeepsTheSlashesOfANestedPath(t *testing.T) {
	t.Parallel()
	var seen string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.EscapedPath() + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"name":"a b.txt","path":"docs/a b.txt","type":"file","size":1}`))
	})
	got, err := rawAdapter(t, srv).GetContents(context.Background(), "o", "r", "main", "docs/a b.txt")
	if err != nil || got.Path != "docs/a b.txt" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if want := "/api/v1/repos/o/r/contents/docs/a%20b.txt?ref=main"; seen != want {
		t.Fatalf("request %q, want %q", seen, want)
	}
}
