package models

import "testing"

func TestBackendString(t *testing.T) {
	tests := []struct {
		b    ModelBackend
		want string
	}{
		{BackendLlama, "llama"},
		{BackendVLLM, "vllm"},
		{BackendOllama, "ollama"},
		{BackendKobold, "koboldcpp"},
		{ModelBackend(99), "llama"}, // unknown → llama
	}
	for _, tt := range tests {
		if got := tt.b.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.b, got, tt.want)
		}
	}
}

func TestParseBackend(t *testing.T) {
	tests := []struct {
		input string
		want  ModelBackend
		err   bool
	}{
		{"llama", BackendLlama, false},
		{"", BackendLlama, false},
		{"  ", BackendLlama, false},
		{"LLAMA", BackendLlama, false},
		{"vllm", BackendVLLM, false},
		{"VLLM", BackendVLLM, false},
		{"ollama", BackendOllama, false},
		{"koboldcpp", BackendKobold, false},
		{"koboldCpp", BackendKobold, false},
		{"unknown", 0, true},
		{" llama ", BackendLlama, false},
	}
	for _, tt := range tests {
		got, err := ParseBackend(tt.input)
		if tt.err && err == nil {
			t.Errorf("ParseBackend(%q) expected error, got %d", tt.input, got)
		}
		if !tt.err && err != nil {
			t.Errorf("ParseBackend(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseBackend(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
