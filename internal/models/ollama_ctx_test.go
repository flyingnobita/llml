package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flyingnobita/llml/internal/settings"
)

// A handler that never answers must not outlive the caller's deadline: the
// context is what bounds the call, since the shared client sets no Timeout.
func TestOllamaClient_ProbeRespectsContextDeadline(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	c := NewOllamaClient(strings.TrimPrefix(srv.URL, "http://"))
	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	if c.Probe(ctx) {
		t.Fatal("Probe should fail against a handler that never responds")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Probe took %v; it should end with the context", elapsed)
	}
}

// Cancelling mid-flight must return promptly rather than waiting for the server.
func TestOllamaClient_TagsStopsOnCancel(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})

	c := NewOllamaClient(strings.TrimPrefix(srv.URL, "http://"))
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan error, 1)
	go func() {
		_, err := c.Tags(ctx)
		done <- err
	}()

	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error after cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Tags did not return after its context was cancelled")
	}
}

// Two sequential probes must share one TCP connection; a fresh http.Client per
// call would open two.
func TestOllamaClient_ReusesConnections(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	var dials atomic.Int64
	ctx := httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
		ConnectStart: func(string, string) { dials.Add(1) },
	})

	c := NewOllamaClient(strings.TrimPrefix(srv.URL, "http://"))
	for i := range 2 {
		if !c.Probe(ctx) {
			t.Fatalf("probe %d should succeed", i+1)
		}
	}
	if got := dials.Load(); got != 1 {
		t.Errorf("opened %d connections across two probes, want 1", got)
	}
}

// Discover walks the filesystem and must not touch the network, so a hung or
// missing Ollama daemon cannot stall or alter a scan.
func TestDiscover_MakesNoHTTPRequests(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"models":[{"name":"should-not-appear:latest"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(t.Context(), Options{
		Settings:         settingsWithOllamaHost(strings.TrimPrefix(srv.URL, "http://"), dir),
		MaxDepth:         4,
		SkipDefaultRoots: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Errorf("Discover made %d HTTP requests; it must be filesystem-only", requests.Load())
	}
	for _, f := range got {
		if f.Backend == BackendOllama {
			t.Errorf("Discover returned an Ollama row: %+v", f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("expected the one gguf on disk, got %+v", got)
	}
}

// A cancelled context stops the walk instead of scanning the whole tree.
func TestDiscover_StopsOnCancelledContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.gguf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Discover(ctx, Options{
		Settings:         settingsWithOllamaHost("", dir),
		MaxDepth:         4,
		SkipDefaultRoots: true,
	})
	if err == nil {
		t.Fatal("expected the cancelled context to be reported")
	}
}

// settingsWithOllamaHost builds settings pointing at one extra model root and
// the given Ollama host.
func settingsWithOllamaHost(host, root string) settings.Settings {
	return settings.Settings{OllamaHost: host, ExtraModelPaths: []string{root}}
}
