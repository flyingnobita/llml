package models

// RuntimeStatus is what detection learned about one Runtime: the program it
// found, if any, and whether the Runtime's server answered its probe.
//
// [RuntimeInfo.Status] is the one place that maps a Runtime to its detection
// fields. The first-seen check ([RuntimeInfo.Detected]), the runtime panel's
// status marks, and [RuntimeInfo.Summary] all read it, so they cannot disagree.
type RuntimeStatus struct {
	// Path is the program detection found (configured path, common install
	// locations, or PATH), or "" when none was found.
	Path string
	// Running reports that the Runtime's server answered its health or
	// model-list probe.
	Running bool
}

// Found reports whether detection found the Runtime's program.
func (s RuntimeStatus) Found() bool { return s.Path != "" }

// Status reports what detection recorded for Runtime b. It reads only the
// detection result and never looks anything up again, so it describes the
// same run every caller saw. A Runtime in [RuntimeInfo.Skipped] got no probe,
// so it is never Running.
//
// For llama.cpp the program is llama-server, the one llml launches.
func (r RuntimeInfo) Status(b ModelBackend) RuntimeStatus {
	switch b {
	case BackendLlama:
		return RuntimeStatus{Path: r.LlamaServerPath, Running: r.ServerRunning}
	case BackendKobold:
		return RuntimeStatus{Path: r.KoboldCppPath, Running: r.KoboldCppRunning}
	case BackendVLLM:
		return RuntimeStatus{Path: r.VLLMPath, Running: r.VLLMRunning}
	case BackendOllama:
		return RuntimeStatus{Path: r.OllamaPath, Running: r.OllamaRunning}
	case BackendNInfer:
		return RuntimeStatus{Path: r.NInferPath, Running: r.NInferRunning}
	case BackendOMLX:
		return RuntimeStatus{Path: r.OMLXPath, Running: r.OMLXRunning}
	case BackendSplash:
		return RuntimeStatus{Path: r.SplashPath, Running: r.SplashRunning}
	default:
		return RuntimeStatus{}
	}
}

// Detected reports whether b is a Detected Runtime: detection found its
// program (configured path, common install locations, or PATH) or its server
// answered its health or model-list probe.
//
// A Runtime in [RuntimeInfo.Skipped] counts as detected only when its program
// was found: it got no probe.
func (r RuntimeInfo) Detected(b ModelBackend) bool {
	s := r.Status(b)
	return s.Found() || s.Running
}
