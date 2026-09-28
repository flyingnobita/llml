package models

import (
	"testing"
)

// A Runtime is detected when detection found its program or its server
// answered; either one is enough, and neither means not detected.
func TestRuntimeInfoDetected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		info RuntimeInfo
		b    ModelBackend
		want bool
	}{
		{"nothing found", RuntimeInfo{}, BackendLlama, false},
		{"llama-server found", RuntimeInfo{LlamaServerPath: "/bin/llama-server"}, BackendLlama, true},
		{"llama-server answers", RuntimeInfo{ServerRunning: true}, BackendLlama, true},
		{"koboldcpp found", RuntimeInfo{KoboldCppPath: "/bin/koboldcpp"}, BackendKobold, true},
		{"koboldcpp answers", RuntimeInfo{KoboldCppRunning: true}, BackendKobold, true},
		{"vllm found", RuntimeInfo{VLLMPath: "/bin/vllm"}, BackendVLLM, true},
		{"vllm missing", RuntimeInfo{LlamaServerPath: "/bin/llama-server"}, BackendVLLM, false},
		{"ollama found", RuntimeInfo{OllamaPath: "/bin/ollama"}, BackendOllama, true},
		{"ollama answers", RuntimeInfo{OllamaRunning: true}, BackendOllama, true},
		{"ninfer found", RuntimeInfo{NInferPath: "/opt/ninfer-serve"}, BackendNInfer, true},
		{"ninfer answers", RuntimeInfo{NInferRunning: true}, BackendNInfer, true},
		{"omlx found", RuntimeInfo{OMLXPath: "/bin/omlx"}, BackendOMLX, true},
		{"omlx answers", RuntimeInfo{OMLXRunning: true}, BackendOMLX, true},
		{"splash found", RuntimeInfo{SplashPath: "/bin/splash"}, BackendSplash, true},
		{"splash answers", RuntimeInfo{SplashRunning: true}, BackendSplash, true},
		{"another Runtime's server", RuntimeInfo{OllamaRunning: true}, BackendSplash, false},
		{"mlx-lm found", RuntimeInfo{MLXLMPath: "/venv/bin/mlx_lm.server"}, BackendMLXLM, true},
		{"mlx-lm answers", RuntimeInfo{MLXLMRunning: true}, BackendMLXLM, true},
		{"mlx-lm missing", RuntimeInfo{ServerRunning: true}, BackendMLXLM, false},
		{"mlx-vlm found", RuntimeInfo{MLXVLMPath: "/venv/bin/mlx_vlm.server"}, BackendMLXVLM, true},
		{"mlx-vlm answers", RuntimeInfo{MLXVLMRunning: true}, BackendMLXVLM, true},
		{"mlx-vlm missing", RuntimeInfo{MLXLMRunning: true, MLXLMPath: "/venv/bin/mlx_lm.server"}, BackendMLXVLM, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.info.Detected(tc.b); got != tc.want {
				t.Errorf("Detected(%v) = %t, want %t", tc.b, got, tc.want)
			}
		})
	}
}

// Status is the one mapping from a Runtime to its detection fields, and
// Detected follows it: a Runtime is detected exactly when Status says its
// program was found or its server answered.
func TestRuntimeInfoStatusDrivesDetected(t *testing.T) {
	t.Parallel()

	names := map[ModelBackend]string{
		BackendKobold: "koboldcpp",
		BackendNInfer: "ninfer",
		BackendOMLX:   "omlx",
		BackendSplash: "splash",
		BackendMLXLM:  "mlx-lm",
		BackendMLXVLM: "mlx-vlm",
		BackendOllama: "ollama",
	}
	infos := []RuntimeInfo{
		{},
		{KoboldCppPath: "/bin/koboldcpp", NInferRunning: true, OMLXPath: "/bin/omlx", OMLXRunning: true},
		{SplashRunning: true, OllamaPath: "/bin/ollama"},
		{MLXLMPath: "/venv/bin/mlx_lm.server"},
		{MLXLMRunning: true},
		{MLXVLMPath: "/venv/bin/mlx_vlm.server"},
		{MLXVLMRunning: true, MLXLMPath: "/venv/bin/mlx_lm.server"},
	}
	for i, info := range infos {
		for b, name := range names {
			st := info.Status(b)
			if info.Detected(b) != (st.Found() || st.Running) {
				t.Errorf("info %d: Detected(%s) disagrees with status %+v", i, name, st)
			}
		}
	}
}
