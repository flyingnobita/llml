package models

// Detected reports whether b is a Detected Runtime: detection found its
// program (configured path, common install locations, or PATH) or its server
// answered its health or model-list probe.
//
// It reads only what detection recorded, so a Runtime in [RuntimeInfo.Skipped]
// counts as detected only when its program was found: it got no probe.
func (r RuntimeInfo) Detected(b ModelBackend) bool {
	switch b {
	case BackendLlama:
		return r.LlamaServerPath != "" || r.ServerRunning
	case BackendKobold:
		return r.KoboldCppPath != "" || r.KoboldCppRunning
	case BackendVLLM:
		// vLLM has no server probe.
		return r.VLLMPath != ""
	case BackendOllama:
		return r.OllamaPath != "" || r.OllamaRunning
	case BackendNInfer:
		return r.NInferPath != "" || r.NInferRunning
	case BackendOMLX:
		return r.OMLXPath != "" || r.OMLXRunning
	case BackendSplash:
		return r.SplashPath != "" || r.SplashRunning
	default:
		return false
	}
}
