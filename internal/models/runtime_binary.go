package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

var commonBinaryDirs = []string{
	"/usr/local/bin",
	"/opt/homebrew/bin",
}

// findBinaryInEnvAndCommonDirs resolves name as $envDir/name, then each of commonDirs/name,
// then [exec.LookPath]. envDir may be empty (skip that step).
func findBinaryInEnvAndCommonDirs(name, envDir string, commonDirs []string) string {
	if envDir != "" {
		clean := filepath.Clean(envDir)
		if isRegularFile(clean) && filepath.Base(clean) == name {
			return clean
		}
		candidate := filepath.Join(clean, name)
		if isRegularFile(candidate) {
			return candidate
		}
	}
	for _, dir := range commonDirs {
		candidate := filepath.Join(dir, name)
		if isRegularFile(candidate) {
			return candidate
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// findVLLMBinary resolves the vllm executable from vllmPath, then venvRoot, then
// common install directories, then PATH. Both parameters may be empty.
func findVLLMBinary(vllmPath, venvRoot string) string {
	return findVLLMBinaryIn(vllmPath, venvRoot, commonBinaryDirs)
}

// findVLLMBinaryIn is [findVLLMBinary] with the system install directories
// passed in, so a test can leave out a vllm the host has installed there.
func findVLLMBinaryIn(vllmPath, venvRoot string, systemDirs []string) string {
	if dir := vllmPath; dir != "" {
		clean := filepath.Clean(dir)
		if isRegularFile(clean) && filepath.Base(clean) == "vllm" {
			return clean
		}
		candidate := filepath.Join(clean, "vllm")
		if isRegularFile(candidate) {
			return candidate
		}
		// vllm often lives only at $VLLM_PATH/.venv/bin/vllm until the venv is activated.
		if p := vllmBinaryInProjectDotVenv(clean); p != "" {
			return p
		}
	}
	if d := strings.TrimSpace(venvRoot); d != "" {
		if p := vllmBinaryInVenvRoot(d); p != "" {
			return p
		}
	}
	common := slices.Clone(systemDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
		if runtime.GOOS == "darwin" {
			// Common local layout for Apple Silicon / Metal vLLM installs.
			common = append(common, filepath.Join(home, ".venv-vllm-metal", "bin"))
		}
	}
	return findBinaryInEnvAndCommonDirs("vllm", "", common)
}

// findLlamaBinary resolves name from llamaCppPath, then common install
// directories, then PATH. llamaCppPath may be empty.
func findLlamaBinary(name, llamaCppPath string) string {
	var envDir string
	if d := llamaCppPath; d != "" {
		envDir = filepath.Clean(d)
	}
	common := slices.Clone(commonBinaryDirs)
	common = append(common, "/opt/llama.cpp/build/bin")
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs(name, envDir, common)
}

// findOllamaBinary resolves the ollama executable from ollamaPath, then common
// install directories, then PATH. ollamaPath may be empty.
func findOllamaBinary(ollamaPath string) string {
	var envDir string
	if d := ollamaPath; d != "" {
		envDir = filepath.Clean(d)
	}
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs("ollama", envDir, common)
}

// koboldcppKnownNames lists every binary name published in upstream releases, in
// preference order for the current platform (primary CUDA variant first).
//
// Source: https://github.com/LostRuins/koboldcpp/releases
func koboldcppKnownNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"koboldcpp.exe", "koboldcpp-nocuda.exe", "koboldcpp-oldpc.exe"}
	case "darwin":
		return []string{"koboldcpp-mac-arm64"}
	default:
		return []string{"koboldcpp-linux-x64", "koboldcpp-linux-x64-nocuda", "koboldcpp-linux-x64-oldpc"}
	}
}

// pickFirstKoboldCppInDir returns the best koboldcpp binary from dir, preferring
// the primary platform variant. Falls back to any regular file with a "koboldcpp"
// prefix to cover future or renamed variants. Uses isRegularFile (os.Stat) so
// symlinks are followed, matching the other backends.
func pickFirstKoboldCppInDir(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, pref := range koboldcppKnownNames() {
		for _, e := range ents {
			p := filepath.Join(dir, e.Name())
			if isRegularFile(p) && e.Name() == pref {
				return p
			}
		}
	}
	for _, e := range ents {
		p := filepath.Join(dir, e.Name())
		if isRegularFile(p) && strings.HasPrefix(e.Name(), "koboldcpp") {
			return p
		}
	}
	return ""
}

// isExecutableFile reports whether path is a regular file with at least one
// execute bit (Unix) or is a regular file (Windows).
func isExecutableFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !fi.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return fi.Mode().Perm()&0o111 != 0
}

// findKoboldCppBinary resolves a koboldcpp executable from koboldCppPath (a
// directory or a direct file path), then common install directories, then PATH.
// koboldCppPath may be empty.
func findKoboldCppBinary(koboldCppPath string) string {
	var envDir string
	if d := koboldCppPath; d != "" {
		envDir = filepath.Clean(d)
	}
	// 1) $KOBOLDCPP_PATH points directly to a file — use it only when the
	//    base name matches a known koboldcpp pattern.
	if envDir != "" {
		clean := filepath.Clean(envDir)
		if isRegularFile(clean) && strings.HasPrefix(filepath.Base(clean), "koboldcpp") && isExecutableFile(clean) {
			return clean
		}
	}
	// 2) $KOBOLDCPP_PATH points to a directory — look for a koboldcpp binary.
	if envDir != "" {
		if p := pickFirstKoboldCppInDir(envDir); p != "" && isExecutableFile(p) {
			return p
		}
	}
	// 3) Common directories.
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	for _, dir := range common {
		if p := pickFirstKoboldCppInDir(dir); p != "" && isExecutableFile(p) {
			return p
		}
	}
	// 4) PATH fallback (already checks executability via LookPath).
	if p, err := exec.LookPath("koboldcpp"); err == nil {
		return p
	}
	return ""
}

// ninferServeName is the NInfer HTTP server executable.
const ninferServeName = "ninfer-serve"

// findNInferBinary resolves ninfer-serve from ninferPath, then common install
// directories, then PATH. ninferPath may be the binary itself, a directory
// containing it, or an NInfer checkout root: NInfer has no install target, so
// its build leaves the server at build/apps/ninfer-serve inside the checkout.
func findNInferBinary(ninferPath string) string {
	if d := ninferPath; d != "" {
		clean := filepath.Clean(d)
		if isRegularFile(clean) && filepath.Base(clean) == ninferServeName && isExecutableFile(clean) {
			return clean
		}
		for _, candidate := range []string{
			filepath.Join(clean, ninferServeName),
			filepath.Join(clean, "build", "apps", ninferServeName),
		} {
			if isExecutableFile(candidate) {
				return candidate
			}
		}
	}
	common := slices.Clone(commonBinaryDirs)
	if home, err := os.UserHomeDir(); err == nil {
		common = append(common, filepath.Join(home, ".local", "bin"))
	}
	return findBinaryInEnvAndCommonDirs(ninferServeName, "", common)
}

// findOMLXBinary resolves the omlx CLI from omlxPath, then the oMLX app's
// shim at ~/.omlx/bin/omlx, then common install directories, then PATH.
// omlxPath may be the CLI itself, a directory containing it, or the oMLX base
// directory, whose bin/ holds the shim the macOS app installs.
func findOMLXBinary(omlxPath string) string {
	const name = "omlx"
	var dirs []string
	if d := omlxPath; d != "" {
		clean := filepath.Clean(d)
		if filepath.Base(clean) == name && isExecutableFile(clean) {
			return clean
		}
		dirs = append(dirs, clean, filepath.Join(clean, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".omlx", "bin"))
	}
	for _, dir := range dirs {
		if p := filepath.Join(dir, name); isExecutableFile(p) {
			return p
		}
	}
	return findBinaryInEnvAndCommonDirs(name, "", commonBinaryDirs)
}

// findSplashBinary resolves the splash executable from splashPath (the binary
// or a directory containing it), then common install directories (Homebrew),
// then PATH.
func findSplashBinary(splashPath string) string {
	const name = "splash"
	if d := splashPath; d != "" {
		clean := filepath.Clean(d)
		if filepath.Base(clean) == name && isExecutableFile(clean) {
			return clean
		}
		if p := filepath.Join(clean, name); isExecutableFile(p) {
			return p
		}
	}
	return findBinaryInEnvAndCommonDirs(name, "", commonBinaryDirs)
}

// mlxLMServerName is mlx-lm's console script. `python -m mlx_lm.server` is
// deprecated upstream, and the script's shebang names its own interpreter, so
// it runs from a venv without activating it.
const mlxLMServerName = "mlx_lm.server"

// findMLXLMScript resolves mlx_lm.server; see [findConsoleScript].
func findMLXLMScript(configured string) string {
	return findConsoleScript(mlxLMServerName, configured)
}

// mlxVLMServerName is mlx-vlm's console script, run directly like
// [mlxLMServerName].
const mlxVLMServerName = "mlx_vlm.server"

// findMLXVLMScript resolves mlx_vlm.server; see [findConsoleScript].
func findMLXVLMScript(configured string) string {
	return findConsoleScript(mlxVLMServerName, configured)
}

// findConsoleScript resolves the Python console script name from configured
// (the script itself, or a directory containing it such as a venv's bin/),
// then PATH. It does not search common install directories or look for
// venvs: a script installed with pip, uv, or mise is on PATH, and one in a
// venv is found through the configured path.
func findConsoleScript(name, configured string) string {
	if configured != "" {
		clean := filepath.Clean(configured)
		if filepath.Base(clean) == name && isExecutableFile(clean) {
			return clean
		}
		if p := filepath.Join(clean, name); isExecutableFile(p) {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// probeHealthEndpoint reports whether the server on host:port answers GET
// /health with 200, bounded by ctx. KoboldCpp is detected by this alone.
func probeHealthEndpoint(ctx context.Context, host string, port int) bool {
	return getHealth(ctx, host, port).ok
}

// healthAnswer is what a server said to GET /health.
type healthAnswer struct {
	// ok is true when it answered 200.
	ok bool
	// server is the Server response header.
	server string
	// status is the "status" field of a JSON body, or "" without one.
	status string
}

// fromMLXLM reports whether the answer came from mlx_lm.server: status "ok",
// as llama-server also sends, from Python's stdlib HTTP server, which names
// itself "BaseHTTP/<version> Python/<version>".
func (h healthAnswer) fromMLXLM() bool {
	return h.ok && h.status == "ok" && strings.HasPrefix(h.server, "BaseHTTP/")
}

// fromMLXVLM reports whether the answer came from mlx_vlm.server, which
// answers /health with status "healthy" where llama-server, ninfer-serve, and
// mlx_lm.server, the other servers on its default port, say "ok". Its Server
// header changed from "uvicorn" to "mlx_vlm/<version>" in 0.5.0, so the header
// is not checked.
func (h healthAnswer) fromMLXVLM() bool {
	return h.ok && h.status == "healthy"
}

// llamaCppServerHeader begins the Server header of current llama-server builds.
const llamaCppServerHeader = "llama.cpp"

// fromLlamaCpp reports whether the answer came from a llama-server that names
// itself. Older builds send no Server header, so false does not rule it out.
func (h healthAnswer) fromLlamaCpp() bool {
	return h.ok && strings.HasPrefix(h.server, llamaCppServerHeader)
}

// getHealth GETs /health on host:port, bounded by ctx. It shares the package
// HTTP client so repeated probes reuse connections.
func getHealth(ctx context.Context, host string, port int) healthAnswer {
	resp, err := probeGet(ctx, host, port, "/health")
	if err != nil {
		return healthAnswer{}
	}
	defer func() { _ = resp.Body.Close() }()
	h := healthAnswer{ok: resp.StatusCode == http.StatusOK, server: resp.Header.Get("Server")}
	body := io.LimitReader(resp.Body, 4<<10)
	var parsed struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(body).Decode(&parsed) == nil {
		h.status = parsed.Status
	}
	// Drain so the connection returns to the pool.
	_, _ = io.Copy(io.Discard, body)
	return h
}

// probeGet sends a detection GET for path to host:port.
func probeGet(ctx context.Context, host string, port int, path string) (*http.Response, error) {
	url := fmt.Sprintf("http://%s%s", net.JoinHostPort(host, strconv.Itoa(port)), path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return sharedHTTPClient.Do(req)
}

// Model owners that name a Runtime in /v1/models.
const (
	ownerNInfer   = "ninfer"
	ownerOMLX     = "omlx"
	ownerSplash   = "splash"
	ownerVLLM     = "vllm"
	ownerLlamaCpp = "llamacpp"
)

// probeModelsOwner reports whether the OpenAI-compatible server on host:port
// lists a model owned by any of owners. Servers that share a port and answer
// /health alike (oMLX and Splash on 8000, ninfer-serve and llama-server on
// 8080) each report themselves as the owner in /v1/models. A server that does
// not answer 200 with a model list owns nothing.
func probeModelsOwner(ctx context.Context, host string, port int, owners ...string) bool {
	resp, err := probeGet(ctx, host, port, "/v1/models")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return false
	}
	var list struct {
		Data []struct {
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list); err != nil {
		return false
	}
	for _, m := range list.Data {
		if slices.Contains(owners, m.OwnedBy) {
			return true
		}
	}
	return false
}

// probeLlamaServer reports whether llama-server answers on host:port.
// llama-server, ninfer-serve, mlx_lm.server, and mlx_vlm.server all default
// to port 8080 and all answer /health with 200, so a 200 there is
// llama-server unless the answer is mlx-lm's or mlx-vlm's, or the model list
// names NInfer as the owner. vLLM, moved onto the same port, names itself the
// owner too.
// Current llama-server builds send "Server: llama.cpp", which settles it
// without the second request; older builds may not, so the header is not
// required.
func probeLlamaServer(ctx context.Context, host string, port int) bool {
	h := getHealth(ctx, host, port)
	if !h.ok || h.fromMLXLM() || h.fromMLXVLM() {
		return false
	}
	return h.fromLlamaCpp() || !probeModelsOwner(ctx, host, port, ownerNInfer, ownerVLLM)
}

// probeMLXLM reports whether mlx_lm.server answers on host:port. Its model
// list names no owner and is often empty, so the /health answer alone tells
// it apart from llama-server and ninfer-serve on their shared port.
func probeMLXLM(ctx context.Context, host string, port int) bool {
	return getHealth(ctx, host, port).fromMLXLM()
}

// probeMLXVLM reports whether mlx_vlm.server answers on host:port. Its model
// list names no owner, so the /health status tells it apart from the other
// servers on port 8080.
func probeMLXVLM(ctx context.Context, host string, port int) bool {
	return getHealth(ctx, host, port).fromMLXVLM()
}

// probeVLLM reports whether vLLM answers on host:port. vLLM shares port 8000
// with oMLX and Splash, which both answer /health too, so a 200 counts as vLLM
// unless the model list names one of them as the owner. An MLX server or a
// llama-server moved onto the port is not vLLM either; a llama-server without
// a Server header still names llamacpp as the owner.
func probeVLLM(ctx context.Context, host string, port int) bool {
	h := getHealth(ctx, host, port)
	if !h.ok || h.fromMLXLM() || h.fromMLXVLM() || h.fromLlamaCpp() {
		return false
	}
	return !probeModelsOwner(ctx, host, port, ownerOMLX, ownerSplash, ownerLlamaCpp)
}

// defaultProbeHost is the loopback address used for health probes that have no
// configurable host of their own (KoboldCpp).
const defaultProbeHost = "127.0.0.1"

// probeHost returns the address to probe a server configured to listen on
// host. A wildcard listen address accepts loopback connections but is not a
// portable destination, so it is probed on loopback.
func probeHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		return defaultProbeHost
	default:
		return strings.TrimSpace(host)
	}
}

// resolvePath returns existing if non-empty, otherwise the first match for cmdName on PATH.
func resolvePath(existing, cmdName string) string {
	if existing != "" {
		return existing
	}
	if p, err := exec.LookPath(cmdName); err == nil {
		return p
	}
	return ""
}

// ResolveLlamaServerPath returns the detected llama-server binary path, or the first match on PATH.
func ResolveLlamaServerPath(r RuntimeInfo) string {
	return resolvePath(r.LlamaServerPath, "llama-server")
}

// ResolveVLLMPath returns the detected vllm binary path, or the first match on PATH.
func ResolveVLLMPath(r RuntimeInfo) string {
	return resolvePath(r.VLLMPath, "vllm")
}

// ResolveOllamaPath returns the detected ollama binary path, or the first match on PATH.
func ResolveOllamaPath(r RuntimeInfo) string {
	return resolvePath(r.OllamaPath, "ollama")
}

// ResolveNInferPath returns the detected ninfer-serve binary path, or the first match on PATH.
func ResolveNInferPath(r RuntimeInfo) string {
	return resolvePath(r.NInferPath, ninferServeName)
}

// ResolveOMLXPath returns the detected omlx CLI path, or the first match on PATH.
func ResolveOMLXPath(r RuntimeInfo) string {
	return resolvePath(r.OMLXPath, "omlx")
}

// ResolveSplashPath returns the detected splash path, or the first match on PATH.
func ResolveSplashPath(r RuntimeInfo) string {
	return resolvePath(r.SplashPath, "splash")
}

// ResolveMLXLMPath returns the detected mlx_lm.server path, or the first match on PATH.
func ResolveMLXLMPath(r RuntimeInfo) string {
	return resolvePath(r.MLXLMPath, mlxLMServerName)
}

// ResolveMLXVLMPath returns the detected mlx_vlm.server path, or the first match on PATH.
func ResolveMLXVLMPath(r RuntimeInfo) string {
	return resolvePath(r.MLXVLMPath, mlxVLMServerName)
}

// ResolveKoboldCppPath returns the detected koboldcpp binary path, or the first match on PATH.
func ResolveKoboldCppPath(r RuntimeInfo) string {
	return resolvePath(r.KoboldCppPath, "koboldcpp")
}
