package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
)

func TestConfiguredRemoteThemeAppliesWithoutAdapterRegistration(t *testing.T) {
	installedFixtures(t, "fzf")
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/safe" {
			_, _ = w.Write([]byte("--color=fg:#112233,bg:#000000\n"))
			return
		}
		_, _ = w.Write([]byte("--bind=enter:execute(echo unsafe)\n"))
	}))
	defer server.Close()
	previousClient := shared.AssetHTTPClient
	shared.AssetHTTPClient = server.Client()
	t.Cleanup(func() { shared.AssetHTTPClient = previousClient })

	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureConfig(t, config, map[string]any{"tools": []string{"fzf"}, "themes": map[string]any{
		"personal-theme": map[string]any{"custom": map[string]any{"fzf": map[string]string{"url": server.URL + "/safe"}}},
	}})
	args := []string{"--home", home, "--config", config, "apply", "personal-theme"}
	if code, result, diagnostics := invoke(t, append(args, "--dry-run")...); code != 0 || result.Tools[0].Outcome != "planned" || requests != 0 {
		t.Fatalf("dry-run downloaded a source: code=%d result=%+v requests=%d diagnostics=%s", code, result, requests, diagnostics)
	}
	code, result, diagnostics := invoke(t, args...)
	path := filepath.Join(home, ".config", "themeset", "fzf.opts")
	data, err := os.ReadFile(path)
	if code != 0 || result.Tools[0].Status != "custom" || result.Tools[0].Outcome != "applied" || err != nil || string(data) != "--color=fg:#112233,bg:#000000\n" || requests != 1 {
		t.Fatalf("remote colors not applied: code=%d result=%+v file=%q err=%v requests=%d diagnostics=%s", code, result, data, err, requests, diagnostics)
	}

	fixtureConfig(t, config, map[string]any{"tools": []string{"fzf"}, "themes": map[string]any{
		"personal-theme": map[string]any{"custom": map[string]any{"fzf": map[string]string{"url": server.URL + "/unsafe"}}},
	}})
	code, result, diagnostics = invoke(t, args...)
	unchanged, readErr := os.ReadFile(path)
	if code != 1 || result.Tools[0].Outcome != "failed" || !strings.Contains(result.Tools[0].Error, "color") || readErr != nil || string(unchanged) != string(data) || requests != 2 {
		t.Fatalf("unsafe remote data changed settings: code=%d result=%+v file=%q err=%v requests=%d diagnostics=%s", code, result, unchanged, readErr, requests, diagnostics)
	}
}

func TestRemoteThemeConfigurationRejectsUnsafeURLsAndScriptAdapters(t *testing.T) {
	for _, port := range []map[string]string{
		{"url": "http://example.com/theme"},
		{"url": "https://user:password@example.com/theme"},
		{"url": "https://example.com/theme", "file": "theme.opts"},
		{"url": "https://example.com/theme"},
		{"url": "https://example.com/theme", "syntax_url": "http://example.com/syntax"},
	} {
		installedFixtures(t, "yazi")
		config := filepath.Join(t.TempDir(), "config.json")
		fixtureConfig(t, config, map[string]any{"tools": []string{"yazi"}, "themes": map[string]any{
			"fixture": map[string]any{"custom": map[string]any{"yazi": port}},
		}})
		code, result, _ := invoke(t, "apply", "fixture", "--config", config, "--home", t.TempDir())
		if code != 1 || len(result.Tools) != 0 {
			t.Fatalf("invalid URL accepted: port=%v code=%d result=%+v", port, code, result)
		}
	}
	installedFixtures(t, "nvim")
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{"tools": []string{"neovim"}, "themes": map[string]any{
		"fixture": map[string]any{"custom": map[string]any{"neovim": map[string]string{"url": "https://example.com/plugin.lua"}}},
	}})
	code, result, _ := invoke(t, "apply", "fixture", "--config", config, "--home", home)
	if code != 1 || len(result.Tools) != 1 || result.Tools[0].Outcome != "failed" || !strings.Contains(result.Tools[0].Error, "does not accept remote") {
		t.Fatalf("remote script adapter accepted: code=%d result=%+v", code, result)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "nvim", "plugin", "themeset.lua")); !os.IsNotExist(err) {
		t.Fatalf("rejected script source changed settings: %v", err)
	}
}
