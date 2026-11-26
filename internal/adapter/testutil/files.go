package testutil

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Env(t *testing.T) model.Environment {
	t.Helper()
	home := t.TempDir()
	return model.Environment{Home: home, ConfigHome: filepath.Join(home, ".config"), OS: "linux", Paths: make(map[string]string)}
}

func Write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func Read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func AssetClient(t *testing.T, serve func(*http.Request) (*http.Response, error)) {
	t.Helper()
	previous := shared.AssetHTTPClient
	shared.AssetHTTPClient = &http.Client{Transport: transport(serve)}
	t.Cleanup(func() { shared.AssetHTTPClient = previous })
}
