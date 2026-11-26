package app

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
)

func TestUserOverrideReplacesOnlyOneBundledTool(t *testing.T) {
	installedFixtures(t, "fzf", "lazydocker")
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	const override = "https://example.test/custom.rc"
	fixtureConfig(t, config, map[string]any{"tools": []string{"fzf", "lazydocker"}, "themes": map[string]any{
		"dracula": map[string]any{"custom": map[string]any{"fzf": map[string]string{"url": override}}},
	}})
	requests := map[string]int{}
	testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
		requests[request.URL.String()]++
		var body string
		switch request.URL.String() {
		case override:
			body = "--color=fg:#112233,bg:#000000\n"
		case DefaultPort("dracula", "lazydocker").Official.Source:
			body = "gui:\n  theme:\n    activeBorderColor: [red]\n"
		default:
			t.Fatalf("unexpected theme fetch: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	code, result, diagnostics := invoke(t, "--home", home, "--config", config, "apply", "dracula")
	fzfOptions, fzfErr := os.ReadFile(filepath.Join(home, ".config", "themeset", "fzf.opts"))
	env, envErr := environment(home)
	if envErr != nil {
		t.Fatal(envErr)
	}
	lazyConfig, lazyErr := os.ReadFile(shared.Path("lazydocker", env))
	if code != 0 || len(result.Tools) != 2 || result.Tools[0].Status != "custom" || result.Tools[1].Status != "official" ||
		fzfErr != nil || string(fzfOptions) != "--color=fg:#112233,bg:#000000\n" || lazyErr != nil ||
		!strings.Contains(string(lazyConfig), "activeBorderColor") || requests[override] != 1 || requests[DefaultPort("dracula", "lazydocker").Official.Source] != 1 {
		t.Fatalf("tool override did not merge with bundled default: code=%d result=%+v fzf=%q lazy=%q errs=%v,%v requests=%v diagnostics=%s", code, result, fzfOptions, lazyConfig, fzfErr, lazyErr, requests, diagnostics)
	}
}
