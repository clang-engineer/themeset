package adapter

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestAssetAcquisitionReusesVerifiedCacheOffline(t *testing.T) {
	content := "filekinds:\n  directory: {foreground: '#8BE9FD'}\n"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; fmt.Fprint(w, content) }))
	defer server.Close()
	env := model.Environment{Home: t.TempDir()}
	first, err := filesAcquireAssetWithClient(env, server.URL, digest, server.Client())
	if err != nil || string(first) != content {
		t.Fatalf("first acquisition: %q %v", first, err)
	}
	server.Close()
	second, err := filesAcquireAssetWithClient(env, server.URL, digest, server.Client())
	if err != nil || string(second) != content || requests != 1 {
		t.Fatalf("offline reuse: %q %v requests=%d", second, err, requests)
	}
	cache := filepath.Join(env.Home, ".cache", "themeset", "assets", digest)
	if err := os.WriteFile(cache, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := filesAcquireAssetWithClient(env, server.URL, digest, server.Client()); err == nil || !strings.Contains(err.Error(), "cached theme asset checksum mismatch") {
		t.Fatalf("corrupt cache accepted: %v", err)
	}
}

func TestAssetAcquisitionRejectsFailedOrUnverifiedDownloads(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{"not-found", 404, "not found"},
		{"checksum", 200, "wrong bytes"},
		{"oversize", 200, strings.Repeat("x", filesAssetMaxBytes+1)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(scenario.status)
				fmt.Fprint(w, scenario.body)
			}))
			defer server.Close()
			env := model.Environment{Home: t.TempDir()}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte("expected")))
			if _, err := filesAcquireAssetWithClient(env, server.URL, digest, server.Client()); err == nil {
				t.Fatal("unverified download accepted")
			}
			if _, err := os.Stat(filepath.Join(env.Home, ".cache", "themeset", "assets", digest)); !os.IsNotExist(err) {
				t.Fatalf("failed download cached: %v", err)
			}
		})
	}
}
