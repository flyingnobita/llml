package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/flyingnobita/llml/internal/fsutil"
	"github.com/flyingnobita/llml/internal/userdata"
)

// maxProfileBody is the maximum size of a profile TOML fetched over HTTP.
const maxProfileBody = 256 * 1024

// maxRedirects is the maximum number of HTTP redirects to follow.
const maxRedirects = 5

// httpClient is the HTTP client used by FetchPortable. Exposed as a package
// variable so tests can replace it with a client that trusts test server certs.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("too many redirects (max %d)", maxRedirects)
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing redirect from https to non-https")
		}
		return nil
	},
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	},
}

// FetchPortable fetches and parses a portable profile TOML from an HTTPS URL.
// Plain http:// URLs are rejected without making a network call.
func FetchPortable(ctx context.Context, rawURL string) (*PortableFile, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL %q: %w", rawURL, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("only https:// URLs are supported")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil, fmt.Errorf("timed out fetching %s", rawURL)
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if urlErr.Err != nil && strings.Contains(urlErr.Err.Error(), "connection refused") {
				return nil, fmt.Errorf("cannot reach %s: connection refused", u.Host)
			}
			if _, ok := urlErr.Err.(*net.DNSError); ok || strings.Contains(urlErr.Err.Error(), "no such host") {
				return nil, fmt.Errorf("cannot reach %s: %v", u.Host, urlErr.Err)
			}
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out fetching %s", rawURL)
		}
		return nil, fmt.Errorf("cannot reach %s: %v", u.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no profile at %s (404)", rawURL)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s returned %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxProfileBody+1))
	if err != nil {
		return nil, fmt.Errorf("error reading response from %s: %w", rawURL, err)
	}
	if len(body) > maxProfileBody {
		return nil, fmt.Errorf("profile body exceeds 256KB cap")
	}

	// Peek at schema_version before full parse.
	type versionOnly struct {
		SchemaVersion int `toml:"schema_version"`
	}
	var vo versionOnly
	if err := toml.Unmarshal(body, &vo); err != nil {
		preview := string(body)
		if len(preview) > 80 {
			preview = preview[:80]
		}
		return nil, fmt.Errorf("response is not valid TOML at %s (got %q)", rawURL, preview)
	}

	var f *PortableFile
	switch vo.SchemaVersion {
	case SchemaVersion:
		var pf PortableFile
		if err := toml.Unmarshal(body, &pf); err != nil {
			return nil, fmt.Errorf("response is not valid TOML at %s: %w", rawURL, err)
		}
		f = &pf
	case 2:
		// Legacy v2: primary was a single string; migrate to single-element array.
		var lf portableFileLegacyV2
		if err := toml.Unmarshal(body, &lf); err != nil {
			return nil, fmt.Errorf("response is not valid TOML at %s: %w", rawURL, err)
		}
		out := &PortableFile{
			SchemaVersion: SchemaVersion,
			Profiles:      make([]PortableProfile, len(lf.Profiles)),
		}
		for i, lp := range lf.Profiles {
			pp := PortableProfile{
				Name:      lp.Name,
				Backend:   lp.Backend,
				ModelHint: lp.ModelHint,
				Args:      lp.Args,
				Env:       lp.Env,
				Hardware:  lp.Hardware,
				UseCase:   PortableUseCase{Tags: lp.UseCase.Tags},
			}
			if lp.UseCase.Primary != "" {
				pp.UseCase.Primary = []string{lp.UseCase.Primary}
			}
			out.Profiles[i] = pp
		}
		f = out
	default:
		return nil, fmt.Errorf("invalid profile schema at %s: schema_version %d (expected %d)", rawURL, vo.SchemaVersion, SchemaVersion)
	}

	for i, p := range f.Profiles {
		if p.Name == "" {
			return nil, fmt.Errorf("invalid profile schema at %s: profile %d missing name", rawURL, i+1)
		}
	}

	return f, nil
}

// ConfigPath returns the path to model-params.json.
func ConfigPath() (string, error) {
	return userdata.ModelParamsPath()
}

// ReadFile reads the model-params.json root document.
func ReadFile() (file, error) {
	path, err := ConfigPath()
	if err != nil {
		return file{}, err
	}
	return readFile(path)
}

//nolint:gosec // G304: path from ConfigPath() using os.UserConfigDir — trusted source.
func readFile(path string) (file, error) {
	var f file
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f.Models = make(map[string]json.RawMessage)
			return f, nil
		}
		return f, err
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	return f, nil
}

// LoadEntry returns stored profiles for modelPath, or one empty default profile if none.
func LoadEntry(modelPath string) (Entry, error) {
	cfgPath, err := ConfigPath()
	if err != nil {
		return Entry{}, err
	}
	key := ModelParamsKey(modelPath)
	f, err := readFile(cfgPath)
	if err != nil {
		return Entry{}, err
	}
	raw, ok := f.Models[key]
	if !ok {
		return Entry{Profiles: []Profile{DefaultProfile()}, ActiveIndex: 0}, nil
	}
	return ParseEntry(raw, f.Version)
}

// SaveEntry writes the entry for modelPath and preserves other models in the file.
func SaveEntry(modelPath string, ent Entry) error {
	cfgPath, err := ConfigPath()
	if err != nil {
		return err
	}
	key := ModelParamsKey(modelPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		return err
	}
	f, err := readFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if f.Models == nil {
		f.Models = make(map[string]json.RawMessage)
	}
	f.Version = FileVersion
	ent = NormalizeEntry(ent)
	if len(ent.Profiles) == 0 {
		delete(f.Models, key)
	} else {
		raw, err := json.Marshal(ent)
		if err != nil {
			return err
		}
		f.Models[key] = raw
	}
	out, err := json.MarshalIndent(&f, "", "  ")
	if err != nil {
		return err
	}
	_ = userdata.BackupFileIfExists(cfgPath)
	return fsutil.WriteFileAtomic(cfgPath, out, 0o644)
}

// LoadParamsForRun returns the active profile's env/args for modelPath.
func LoadParamsForRun(modelPath string) (ModelParams, error) {
	ent, err := LoadEntry(modelPath)
	if err != nil {
		return ModelParams{}, err
	}
	if len(ent.Profiles) == 0 {
		return ModelParams{}, nil
	}
	idx := clampInt(ent.ActiveIndex, 0, len(ent.Profiles)-1)
	p := ent.Profiles[idx]
	return NormalizeModelParams(ModelParams{Env: p.Env, Args: p.Args, UseCase: p.UseCase}), nil
}

// ParseEntry decodes one model entry according to the file version.
func ParseEntry(raw json.RawMessage, version int) (Entry, error) {
	switch version {
	case 0, 1:
		var v1 modelParamsV1
		if err := json.Unmarshal(raw, &v1); err != nil {
			return Entry{}, err
		}
		return Entry{
			Profiles: []Profile{
				NormalizeProfile(Profile{Name: "default", Env: v1.Env, Args: v1.Args}),
			},
			ActiveIndex: 0,
		}, nil
	case 2:
		var v2 entryV2
		if err := json.Unmarshal(raw, &v2); err != nil {
			return Entry{}, err
		}
		ent := Entry{ActiveIndex: v2.ActiveIndex}
		for _, p := range v2.Profiles {
			ent.Profiles = append(ent.Profiles, Profile{
				Name: p.Name,
				Env:  p.Env,
				Args: p.Args,
			})
		}
		return applyMigrationDefaults(NormalizeEntry(ent)), nil
	case 3:
		var v3 Entry
		if err := json.Unmarshal(raw, &v3); err != nil {
			return Entry{}, err
		}
		return NormalizeEntry(v3), nil
	default:
		return Entry{}, fmt.Errorf("unsupported model params version %d", version)
	}
}

// ModelParamsKey canonicalizes the per-model storage key.
func ModelParamsKey(modelPath string) string {
	key := strings.TrimSpace(modelPath)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "ollama://") {
		return key
	}
	if strings.Contains(key, "://") {
		return key
	}
	return filepath.Clean(key)
}

// SetActiveProfile marks the profile with the given name as active for
// targetKey. Returns an error if no profile with that name is found.
func SetActiveProfile(targetKey, profileName string) error {
	if targetKey == "" {
		return fmt.Errorf("target model key is required")
	}
	ent, err := LoadEntry(targetKey)
	if err != nil {
		return fmt.Errorf("loading entry for %s: %w", targetKey, err)
	}
	for i, p := range ent.Profiles {
		if p.Name == profileName {
			ent.ActiveIndex = i
			return SaveEntry(targetKey, ent)
		}
	}
	return fmt.Errorf("profile %q not found for model %s", profileName, targetKey)
}

func applyMigrationDefaults(ent Entry) Entry {
	for i := range ent.Profiles {
		ent.Profiles[i].Backend = normalizeBackend(ent.Profiles[i].Backend)
	}
	return ent
}
