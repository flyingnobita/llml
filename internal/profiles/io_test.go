package profiles

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testTOML(profilesBlock string) string {
	return "schema_version = 2\n\n" + profilesBlock
}

func singleProfileTOML(name string) string {
	return testTOML(`[[profiles]]
name = "` + name + `"
backend = "llama"
model_hint = "test-model"
args = ["--ctx-size 4096"]
`)
}

func multiProfileTOML() string {
	return testTOML(`[[profiles]]
name = "profile-a"
backend = "llama"

[[profiles]]
name = "profile-b"
backend = "llama"

[[profiles]]
name = "profile-c"
backend = "llama"
`)
}

// useTestServerClient swaps the package-level httpClient with one that trusts
// the test TLS server's certificate. Returns a reset function.
func useTestServerClient(srv *httptest.Server) func() {
	orig := httpClient
	tsClient := srv.Client()
	httpClient = &http.Client{
		Timeout:   orig.Timeout,
		Transport: tsClient.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect from https to non-https")
			}
			return nil
		},
	}
	return func() { httpClient = orig }
}

// ---------------------------------------------------------------------------
// FetchPortable tests (cases 1–16 from test plan)
// ---------------------------------------------------------------------------

func TestFetchPortable(t *testing.T) {
	// Case 1: Happy path — valid single-profile TOML
	t.Run("happy path single profile", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(singleProfileTOML("fast")))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		f, err := FetchPortable(context.Background(), srv.URL+"/profile.toml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.SchemaVersion != SchemaVersion {
			t.Fatalf("version = %d (want %d)", f.SchemaVersion, SchemaVersion)
		}
		if len(f.Profiles) != 1 {
			t.Fatalf("got %d profiles", len(f.Profiles))
		}
		if f.Profiles[0].Name != "fast" {
			t.Fatalf("name = %q", f.Profiles[0].Name)
		}
	})

	// Case 2: Multi-profile happy path
	t.Run("happy path multi profile", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(multiProfileTOML()))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		f, err := FetchPortable(context.Background(), srv.URL+"/multi.toml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(f.Profiles) != 3 {
			t.Fatalf("got %d profiles", len(f.Profiles))
		}
	})

	// Case 3: Plain http:// rejected without network call
	t.Run("http rejected", func(t *testing.T) {
		_, err := FetchPortable(context.Background(), "http://example.com/foo.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "https://") {
			t.Fatalf("expected https-only error, got %v", err)
		}
	})

	// Case 4: HTTP 404
	t.Run("http 404", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/nope.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Fatalf("expected 404 in error, got %v", err)
		}
	})

	// Case 5: HTTP 500
	t.Run("http 500", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/error.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Fatalf("expected 500 in error, got %v", err)
		}
	})

	// Case 7: Connection refused (closed port)
	t.Run("connection refused", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		addr := srv.URL
		srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := FetchPortable(ctx, addr+"/profile.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "cannot reach") {
			t.Fatalf("expected connection refused error, got %v", err)
		}
	})

	// Case 8: Connect timeout — context already expired
	t.Run("connect timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(time.Millisecond)

		_, err := FetchPortable(ctx, "https://192.0.2.1:9999/profile.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "cannot reach") {
			t.Fatalf("expected timeout error, got %v", err)
		}
	})

	// Case 10: Redirect chain >= 5
	t.Run("too many redirects", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/loop.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "too many redirects") {
			t.Fatalf("expected redirect error, got %v", err)
		}
	})

	// Case 11: Redirect to non-HTTPS
	t.Run("redirect to non-https", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/evil.toml", http.StatusFound)
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/redirect.toml")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	// Case 12: Body > 256 KB
	t.Run("body exceeds cap", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := strings.Repeat("x", 300*1024)
			w.Write([]byte(body))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/huge.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "256KB") {
			t.Fatalf("expected size cap error, got %v", err)
		}
	})

	// Case 13: Non-TOML body (HTML)
	t.Run("non toml body", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<html><body>404 Not Found</body></html>"))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/page.html")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "not valid TOML") {
			t.Fatalf("expected 'not valid TOML' error, got %v", err)
		}
	})

	// Case 14: Valid TOML, missing schema_version
	t.Run("missing schema version", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[[profiles]]
name = "test"
backend = "llama"
`))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/noschema.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "schema_version") {
			t.Fatalf("expected schema_version error, got %v", err)
		}
	})

	// Case 15: schema_version = 1 (rejected)
	t.Run("schema version 1 rejected", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`schema_version = 1

[[profiles]]
name = "old"
backend = "llama"
`))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/v1.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "schema_version") {
			t.Fatalf("expected schema_version error, got %v", err)
		}
	})

	// Case 16: Valid TOML, missing name
	t.Run("missing name field", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`schema_version = 2

[[profiles]]
backend = "llama"
`))
		}))
		defer srv.Close()
		defer useTestServerClient(srv)()

		_, err := FetchPortable(context.Background(), srv.URL+"/noname.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "missing name") {
			t.Fatalf("expected missing name error, got %v", err)
		}
	})

	// Case 6: DNS failure — unresolvable host
	t.Run("dns failure", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := FetchPortable(ctx, "https://this-host-definitely-does-not-exist.invalid/profile.toml")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot reach") {
			t.Fatalf("expected 'cannot reach' error, got %v", err)
		}
	})
}
