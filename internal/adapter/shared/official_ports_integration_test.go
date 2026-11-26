package shared_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/bat"
	"github.com/clang-engineer/themeset/internal/adapter/delta"
	"github.com/clang-engineer/themeset/internal/adapter/fzf"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/app"
	"github.com/clang-engineer/themeset/internal/model"
)

func rosePineFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRosePineOfficialRecipesAreReadOnly(t *testing.T) {
	env := filesTestEnv(t)
	calls := 0
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("resolve cannot download")
	})
	for _, id := range []string{"starship", "fzf", "lazygit", "atuin", "eza", "btop"} {
		source := app.DefaultPort("rose-pine", id).Official.Source
		if !strings.Contains(source, "https://raw.githubusercontent.com/rose-pine/"+id+"/main/") {
			t.Fatalf("missing current %s source: %q", id, source)
		}
		port, ok, err := filesTestAdapter(t, id).Resolve(app.DefaultPort("rose-pine", id), env)
		if err != nil || !ok || port.Kind != "official" || port.Source != source {
			t.Fatalf("resolve %s: %#v, ok=%v, err=%v", id, port, ok, err)
		}
		if _, ok, err := filesTestAdapter(t, id).Resolve(model.ThemePort{}, env); ok || err != nil {
			t.Fatalf("unregistered variant available for %s: %v", id, err)
		}
	}
	if calls != 0 {
		t.Fatalf("resolve/download rejection made %d requests", calls)
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolve created asset cache: %v", err)
	}
}

func TestRosePineFZFOfficialColorsPreserveUserOptionsAndRefresh(t *testing.T) {
	env := filesTestEnv(t)
	port, ok, err := fzf.Adapter().Resolve(app.DefaultPort("rose-pine", "fzf"), env)
	if err != nil || !ok {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	source := port.Source
	script := rosePineFixture(t, "rose-pine-fzf.txt")
	shell := filepath.Join(env.Home, ".zshrc")
	filesTestWrite(t, shell, "export FZF_DEFAULT_OPTS='--height=40% --bind=ctrl-j:down'\n")
	path := shared.Path("fzf", env)
	filesTestWrite(t, path, "--color=fg:#000000\n")
	calls := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != source {
			t.Fatalf("unexpected asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(script)), Header: make(http.Header)}, nil
	})
	if err := fzf.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	first := filesTestRead(t, path)
	if calls != 1 || !strings.Contains(string(first), "--color=fg:#908caa,bg:#191724,hl:#ebbcba\n") || strings.Contains(string(first), "export") || strings.Contains(string(first), "#000000") {
		t.Fatalf("fzf official color extraction: %q, requests=%d", first, calls)
	}
	if string(filesTestRead(t, shell)) != "export FZF_DEFAULT_OPTS='--height=40% --bind=ctrl-j:down'\n" {
		t.Fatal("overwrote ordinary shell options")
	}
	updated := []byte("export FZF_DEFAULT_OPTS=\"\n--color=fg:#ffffff,bg:#000000\"\n")
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(updated)), Header: make(http.Header)}, nil
	})
	if err := fzf.Adapter().Apply(port, env); err != nil || bytes.Equal(first, filesTestRead(t, path)) || !strings.Contains(string(filesTestRead(t, path)), "fg:#ffffff") {
		t.Fatalf("latest upstream colors not applied: %v", err)
	}
}

func TestRosePineFailedAssetsDoNotChangeTheme(t *testing.T) {
	for _, failure := range []string{"offline", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := shared.Path("fzf", env)
			filesTestWrite(t, path, "--color=fg:#000000\n")
			testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "offline" {
					return nil, errors.New("offline")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("not fzf options"))), Header: make(http.Header)}, nil
			})
			port, _, _ := fzf.Adapter().Resolve(app.DefaultPort("rose-pine", "fzf"), env)
			if err := fzf.Adapter().Apply(port, env); err == nil {
				t.Fatal("accepted unavailable or malformed official asset")
			}
			if string(filesTestRead(t, path)) != "--color=fg:#000000\n" {
				t.Fatal("failed acquisition replaced active theme")
			}
		})
	}
}

func TestRosePineDeltaFeatureImportsOnlyMainAndPreservesGitConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for native Git config parsing")
	}
	env := nativeTestEnvironment(t)
	port, ok, err := delta.Adapter().Resolve(app.DefaultPort("rose-pine", "delta"), env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("resolve Delta feature: %#v, %v", port, err)
	}
	theme := rosePineFixture(t, "rose-pine-delta.gitconfig")
	path := filepath.Join(env.Home, ".gitconfig")
	nativeTestPut(t, path, "[user]\n name = User\n[delta]\n line-numbers = true\n syntax-theme = Old\n[include]\n path = private.gitconfig\n")
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != port.Source {
			t.Fatalf("unexpected Delta request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(theme)), Header: make(http.Header)}, nil
	})
	if err := delta.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	first := nativeTestGet(t, path)
	for _, keep := range []string{"name = User", "line-numbers = true", "syntax-theme = Old", "path = private.gitconfig", "features = rose-pine"} {
		if !strings.Contains(first, keep) {
			t.Errorf("missing user or official Delta setting %q in %s", keep, first)
		}
	}
	style, err := shared.Command(env, "git", "config", "--file", path, "--get", "delta.rose-pine.file-style")
	if err != nil || strings.TrimSpace(string(style)) != `#9ccfd8 bold ul` {
		t.Fatalf("official Delta feature style not imported: %q, %v", style, err)
	}
	if strings.Contains(first, "rose-pine-moon") || strings.Contains(first, "rose-pine-dawn") || strings.Contains(first, "[delta \"dark\"]") {
		t.Fatalf("imported unselected Delta variants: %s", first)
	}
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if err := delta.Adapter().Apply(port, env); err == nil || nativeTestGet(t, path) != first {
		t.Fatalf("offline Delta reapply changed config: %v", err)
	}
}

func TestRosePineDeltaRejectedAssetLeavesGitConfigUnchanged(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.Home, ".gitconfig")
	nativeTestPut(t, path, "[user]\n name = Keep\n")
	registered, _, _ := delta.Adapter().Resolve(app.DefaultPort("rose-pine", "delta"), env)
	for _, port := range []model.Port{
		{Kind: "official", Name: "rose-pine", Source: "https://example.com/unverified"},
		{Kind: "official", Name: "rose-pine", Source: registered.Source, File: "override"},
	} {
		if err := delta.Adapter().Apply(port, env); err == nil {
			t.Fatal("unregistered Delta feature accepted")
		}
	}
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	port, _, _ := delta.Adapter().Resolve(app.DefaultPort("rose-pine", "delta"), env)
	if err := delta.Adapter().Apply(port, env); err == nil || nativeTestGet(t, path) != "[user]\n name = Keep\n" {
		t.Fatalf("failed Delta download changed Git config: %v", err)
	}
}

func TestRosePineBatNeedsInstalledSyntaxTheme(t *testing.T) {
	env := nativeTestEnvironment(t)
	bin := filepath.Join(env.Home, "bin")
	binary := filepath.Join(bin, "bat")
	nativeTestPut(t, binary, "#!/bin/sh\nprintf '%s\\n' 'Rosé Pine' 'Rosé Pine Moon'\n")
	if err := os.Chmod(binary, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	resolve := bat.Adapter().Resolve
	port, ok, err := resolve(app.DefaultPort("rose-pine", "bat"), env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "Rosé Pine" {
		t.Fatalf("installed Rosé Pine syntax: %#v, %v", port, err)
	}
	path := env.Paths["bat"]
	nativeTestPut(t, path, "--plain\n--theme=\"Old\"\n")
	if err := bat.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	if data := nativeTestGet(t, path); !strings.Contains(data, "--plain\n") || !strings.Contains(data, `--theme="Rosé Pine"`) {
		t.Fatalf("bat options lost: %s", data)
	}
	nativeTestPut(t, binary, "#!/bin/sh\nprintf '%s\\n' 'Rosé Pine Moon'\n")
	if _, ok, err := resolve(app.DefaultPort("rose-pine", "bat"), env); err != nil || ok {
		t.Fatalf("substituted Moon variant: %v", err)
	}
}
