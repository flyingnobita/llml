package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultOllamaHost = "127.0.0.1:11434"

const (
	ollamaProbeTimeout   = 3 * time.Second
	ollamaPreloadTimeout = 5 * time.Minute
)

type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaPSResponse struct {
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

// OllamaHost returns the configured host:port for the Ollama API.
func OllamaHost() string {
	return normalizeOllamaHost(strings.TrimSpace(os.Getenv(EnvOllamaHost)))
}

// OllamaBaseURL returns the API base URL for the configured host.
func OllamaBaseURL() string {
	return "http://" + OllamaHost() + "/api"
}

func normalizeOllamaHost(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimSuffix(v, "/")
	if v == "" {
		return defaultOllamaHost
	}
	return v
}

func ollamaHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func doOllamaJSON(method, path string, reqBody any, out any, timeout time.Duration) error {
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
	req, err := http.NewRequest(method, OllamaBaseURL()+path, body)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := ollamaHTTPClient(timeout).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama API %s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ProbeOllama reports whether the configured Ollama daemon is reachable.
func ProbeOllama() bool {
	var resp ollamaTagsResponse
	return doOllamaJSON(http.MethodGet, "/tags", nil, &resp, ollamaProbeTimeout) == nil
}

// DiscoverOllamaModels lists installed Ollama models via the supported API.
func DiscoverOllamaModels() ([]ModelFile, error) {
	var resp ollamaTagsResponse
	if err := doOllamaJSON(http.MethodGet, "/tags", nil, &resp, ollamaProbeTimeout); err != nil {
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

// PreloadOllamaModel keeps the selected model loaded in memory indefinitely.
func PreloadOllamaModel(modelID string) error {
	return doOllamaJSON(http.MethodPost, "/generate", ollamaPreloadRequest{
		Model:     modelID,
		KeepAlive: -1,
		Stream:    false,
	}, nil, ollamaPreloadTimeout)
}

// ListRunningOllamaModels returns the currently loaded Ollama model IDs.
func ListRunningOllamaModels() ([]string, error) {
	var resp ollamaPSResponse
	if err := doOllamaJSON(http.MethodGet, "/ps", nil, &resp, ollamaProbeTimeout); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Models))
	for _, m := range resp.Models {
		id := strings.TrimSpace(m.Name)
		if id == "" {
			id = strings.TrimSpace(m.Model)
		}
		if id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}
