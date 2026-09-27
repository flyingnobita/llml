package models

import "testing"

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
