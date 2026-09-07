package models

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverOllamaModels(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{
					"name":        "qwen3.5:latest",
					"modified_at": "2026-04-21T00:00:00Z",
					"size":        int64(123),
					"details": map[string]any{
						"family":             "qwen",
						"parameter_size":     "32B",
						"quantization_level": "Q4_K_M",
					},
				},
			},
		})
	}))
	defer srv.Close()

	got, err := DiscoverOllamaModels(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d models", len(got))
	}
	if got[0].Backend != BackendOllama {
		t.Fatalf("backend %v", got[0].Backend)
	}
	if got[0].Identity() != "qwen3.5:latest" {
		t.Fatalf("identity %q", got[0].Identity())
	}
	if got[0].DisplayLocation() != "ollama://qwen3.5:latest" {
		t.Fatalf("location %q", got[0].DisplayLocation())
	}
	if !strings.Contains(got[0].Parameters, "ollama") {
		t.Fatalf("parameters %q", got[0].Parameters)
	}
}

func TestPreloadOllamaModel(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("path %q", r.URL.Path)
		}
		var req ollamaPreloadRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "qwen3.5:latest" || req.KeepAlive != -1 || req.Stream {
			t.Fatalf("request %+v", req)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := PreloadOllamaModel(strings.TrimPrefix(srv.URL, "http://"), "qwen3.5:latest"); err != nil {
		t.Fatal(err)
	}
}
