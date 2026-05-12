package profiles

import (
	"strings"
	"testing"
)

func validPortableFile(profiles ...PortableProfile) *PortableFile {
	return &PortableFile{
		SchemaVersion: SchemaVersion,
		Profiles:      profiles,
	}
}

func TestFormatPortablePreview(t *testing.T) {
	// Case 17: Single profile, no target
	t.Run("single profile no target", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name:    "test-profile",
			Backend: "llama",
			Args:    []string{"--ctx-size 4096"},
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if !strings.Contains(out, "Profile: test-profile") {
			t.Fatalf("missing profile name, got: %s", out)
		}
		if !strings.Contains(out, "Backend: llama") {
			t.Fatalf("missing backend, got: %s", out)
		}
		if strings.Contains(out, "Target model:") {
			t.Fatal("should not contain Target model line")
		}
	})

	// Case 18: Single profile with target
	t.Run("single profile with target", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name:    "test-profile",
			Backend: "llama",
		})
		out := FormatPortablePreview(f, PreviewOpts{TargetModel: "/path/to/model.gguf"})
		if !strings.Contains(out, "Target model: /path/to/model.gguf") {
			t.Fatalf("missing target model line, got: %s", out)
		}
	})

	// Case 19: Multi-profile file
	t.Run("multi profile", func(t *testing.T) {
		f := validPortableFile(
			PortableProfile{Name: "profile-a", Backend: "llama"},
			PortableProfile{Name: "profile-b", Backend: "ollama"},
		)
		out := FormatPortablePreview(f, PreviewOpts{})
		if !strings.Contains(out, "profile-a") {
			t.Fatal("missing first profile")
		}
		if !strings.Contains(out, "profile-b") {
			t.Fatal("missing second profile")
		}
		if !strings.Contains(out, "---") {
			t.Fatal("missing separator between profiles")
		}
	})

	// Case 20: Args > 6 entries
	t.Run("args more than 6", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name: "many-args",
			Args: []string{
				"--arg1", "--arg2", "--arg3", "--arg4",
				"--arg5", "--arg6", "--arg7", "--arg8",
			},
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if !strings.Contains(out, "(+2 more)") {
			t.Fatalf("expected (+2 more), got: %s", out)
		}
		if !strings.Contains(out, "Args (8):") {
			t.Fatalf("expected Args (8):, got: %s", out)
		}
	})

	// Case 21: Env > 6 entries
	t.Run("env more than 6", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name: "many-env",
			Env: []PortableEnvVar{
				{Key: "A", Value: "1"},
				{Key: "B", Value: "2"},
				{Key: "C", Value: "3"},
				{Key: "D", Value: "4"},
				{Key: "E", Value: "5"},
				{Key: "F", Value: "6"},
				{Key: "G", Value: "7"},
			},
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if !strings.Contains(out, "(+1 more)") {
			t.Fatalf("expected (+1 more), got: %s", out)
		}
		if !strings.Contains(out, "Env (7):") {
			t.Fatalf("expected Env (7):, got: %s", out)
		}
	})

	// Case 22: Env value > 40 chars is truncated
	t.Run("env value truncated", func(t *testing.T) {
		longValue := strings.Repeat("x", 50)
		f := validPortableFile(PortableProfile{
			Name: "long-env",
			Env:  []PortableEnvVar{{Key: "LONG_VAR", Value: longValue}},
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if len(out) > len(longValue)+100 {
			t.Fatalf("expected truncated value, got full length: %s", out)
		}
		if !strings.Contains(out, "…") {
			t.Fatalf("expected truncation ellipsis, got: %s", out)
		}
	})

	// Case 23: Empty args / empty env — lines are omitted
	t.Run("empty args and env", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name:    "bare",
			Backend: "vllm",
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if strings.Contains(out, "Args") {
			t.Fatal("should not contain Args line")
		}
		if strings.Contains(out, "Env") {
			t.Fatal("should not contain Env line")
		}
	})

	// Model hint line
	t.Run("model hint displayed", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name:      "with-hint",
			Backend:   "llama",
			ModelHint: "Qwen3-14B-Q4_K_M.gguf",
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if !strings.Contains(out, "Model hint: Qwen3-14B-Q4_K_M.gguf") {
			t.Fatalf("missing model hint, got: %s", out)
		}
	})

	t.Run("empty model hint omitted", func(t *testing.T) {
		f := validPortableFile(PortableProfile{
			Name:    "no-hint",
			Backend: "llama",
		})
		out := FormatPortablePreview(f, PreviewOpts{})
		if strings.Contains(out, "Model hint:") {
			t.Fatal("should not contain Model hint line")
		}
	})
}
