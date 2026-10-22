// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// shutdownRequest is a request the door admits: the listen config built by
// defaultListenConfig refuses anonymous requests.
func shutdownRequest(t *testing.T, addr, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://"+addr+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "token decoy")
	return req
}

// startDrainable serves handler on a loopback listener through the real
// policy stack and the shutdown path. Cancelling ctx stands in for the signal.
func startDrainable(t *testing.T, ctx context.Context, handler http.Handler, grace time.Duration) (addr string, done <-chan error) {
	t.Helper()
	ln := mustListenLoopback(t)
	srv := newMCPHTTPServer(handler, resolveTransportConfig(defaultListenConfig()))
	errc := make(chan error, 1)
	go func() { errc <- serveUntilDone(ctx, srv, []net.Listener{ln}, grace) }()
	return ln.Addr().String(), errc
}

func TestShutdownLetsInFlightRequestFinish(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() { close(entered) })
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, done := startDrainable(t, ctx, handler, 5*time.Second)

	bodyc := make(chan string, 1)
	go func() {
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(shutdownRequest(t, addr, "/"))
		if err != nil {
			bodyc <- "error: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		bodyc <- string(b)
	}()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the request never reached the handler")
	}
	cancel()

	select {
	case body := <-bodyc:
		if body != "done" {
			t.Fatalf("in-flight request was cut off: got %q", body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request did not finish within the grace period")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a requested shutdown must exit cleanly, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = c.Close()
		t.Fatal("a new connection was accepted after shutdown began")
	}
}

func TestStreamOpenAtDeadlineIsClosedAndShutdownStillExits(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		once.Do(func() { close(entered) })
		<-r.Context().Done()
	})

	const grace = 200 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr, done := startDrainable(t, ctx, handler, grace)

	readc := make(chan error, 1)
	go func() {
		resp, err := (&http.Client{}).Do(shutdownRequest(t, addr, "/"))
		if err != nil {
			readc <- err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, err = io.ReadAll(resp.Body)
		readc <- err
	}()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the stream never opened")
	}
	cancel()
	start := time.Now()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a deadline-forced close must still exit cleanly, got %v", err)
		}
		if elapsed := time.Since(start); elapsed < grace/2 {
			t.Fatalf("serve returned after %v, before the grace period was spent", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the grace period")
	}
	select {
	case <-readc:
		// The stream ended: the client sees a closed body, which it can
		// reconnect from.
	case <-time.After(3 * time.Second):
		t.Fatal("the open stream was not closed at the deadline")
	}
}

func TestStdioReturnsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	errc := make(chan error, 1)
	go func() {
		errc <- serveStdio(ctx, server.NewMCPServer("test", "0.0.0"), pr, io.Discard)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("a cancelled stdio session must return nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stdio did not return after its context was cancelled")
	}
}

func TestStdioReturnsWhenInputEnds(t *testing.T) {
	err := serveStdio(context.Background(), server.NewMCPServer("test", "0.0.0"),
		strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatalf("stdio input ending must return nil, got %v", err)
	}
}
