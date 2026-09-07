package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flyingnobita/llml/internal/settings"
)

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name       string        `json:"name"`
	Model      string        `json:"model"`
	ModifiedAt time.Time     `json:"modified_at"`
	Size       int64         `json:"size"`
	Details    ollamaDetails `json:"details"`
}

type ollamaDetails struct {
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	Format            string   `json:"format"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

type ollamaPreloadRequest struct {
	Model     string `json:"model"`
	KeepAlive int    `json:"keep_alive"`
	Stream    bool   `json:"stream"`
}

// OllamaClient talks to one Ollama daemon. Every method takes a context: the
// deadline and cancellation come from the caller, so a scan the user abandons
// stops immediately instead of waiting out a fixed timeout.
//
// The zero value is usable and targets the default host.
type OllamaClient struct {
	// Host is the daemon's host:port. An empty Host means the built-in default.
	Host string
	// HTTP is the client to use. A nil HTTP means the package's shared client.
	HTTP *http.Client
}

// NewOllamaClient returns a client for host, using the package's shared HTTP client.
func NewOllamaClient(host string) OllamaClient {
	return OllamaClient{Host: host}
}

// baseURL returns the API base URL, falling back to the built-in default host.
func (c OllamaClient) baseURL() string {
	host := settings.NormalizeOllamaHost(c.Host)
	if host == "" {
		host = settings.DefaultOllamaHost
	}
	return "http://" + host + "/api"
}

func (c OllamaClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return sharedHTTPClient
}

func (c OllamaClient) doJSON(ctx context.Context, method, path string, reqBody any, out any) error {
	var body *bytes.Reader
	if reqBody == nil {
		body = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain before closing so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("ollama API %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Probe reports whether the daemon is reachable before ctx ends.
func (c OllamaClient) Probe(ctx context.Context) bool {
	var resp ollamaTagsResponse
	return c.doJSON(ctx, http.MethodGet, "/tags", nil, &resp) == nil
}

// Tags lists the models installed on the daemon.
func (c OllamaClient) Tags(ctx context.Context) ([]ModelFile, error) {
	var resp ollamaTagsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/tags", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]ModelFile, 0, len(resp.Models))
	for _, m := range resp.Models {
		id := strings.TrimSpace(m.Name)
		if id == "" {
			id = strings.TrimSpace(m.Model)
		}
		if id == "" {
			continue
		}
		out = append(out, ModelFile{
			Backend:    BackendOllama,
			ID:         id,
			Location:   "ollama://" + id,
			Name:       id,
			Size:       m.Size,
			ModTime:    m.ModifiedAt,
			Parameters: formatOllamaParams(m.Details),
		})
	}
	return out, nil
}

// Preload keeps modelID loaded in memory indefinitely.
func (c OllamaClient) Preload(ctx context.Context, modelID string) error {
	return c.doJSON(ctx, http.MethodPost, "/generate", ollamaPreloadRequest{
		Model:     modelID,
		KeepAlive: -1,
		Stream:    false,
	}, nil)
}

func formatOllamaParams(d ollamaDetails) string {
	parts := []string{"ollama"}
	if v := strings.TrimSpace(d.Family); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(d.ParameterSize); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(d.QuantizationLevel); v != "" {
		parts = append(parts, v)
	}
	return strings.Join(parts, " · ")
}
