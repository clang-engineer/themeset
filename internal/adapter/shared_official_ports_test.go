package adapter

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

	"github.com/clang-engineer/themeset/internal/model"
)

func rosePineFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRosePineOfficialRecipesAreReadOnly(t *testing.T) {
	env := filesTestEnv(t)
	calls := 0
	ezaDraculaClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("resolve cannot download")
	})
	for _, id := range []string{"starship", "fzf", "lazygit", "atuin", "eza", "btop"} {
		recipe, ok := rosePineRecipe(id)
		if !ok || !strings.Contains(recipe.url, "https://raw.githubusercontent.com/rose-pine/"+id+"/main/") {
			t.Fatalf("missing current %s source: %#v", id, recipe)
		}
		port, ok, err := filesResolveRosePinePort(id, "rose-pine", env)
		if err != nil || !ok || port.Kind != "official" || port.Source != recipe.url {
			t.Fatalf("resolve %s: %#v, ok=%v, err=%v", id, port, ok, err)
		}
		if _, err := filesAcquireRosePinePort(id, model.Port{Kind: "official", Name: "rose-pine", Source: "https://example.org/unverified"}, env); err == nil {
			t.Fatalf("accepted unregistered %s asset", id)
		}
		if _, ok, err := filesResolveRosePinePort(id, "rose-pine-moon", env); ok || err != nil {
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
	port, ok, err := filesResolveFZF("rose-pine", env)
	if err != nil || !ok {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	recipe, _ := rosePineRecipe("fzf")
	script := rosePineFixture(t, "rose-pine-fzf.txt")
	shell := filepath.Join(env.Home, ".zshrc")
	filesTestWrite(t, shell, "export FZF_DEFAULT_OPTS='--height=40% --bind=ctrl-j:down'\n")
	path := filesPath("fzf", env)
	filesTestWrite(t, path, "--color=fg:#000000\n")
	calls := 0
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != recipe.url {
			t.Fatalf("unexpected asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(script)), Header: make(http.Header)}, nil
	})
	if err := filesApplyFZF(port, env); err != nil {
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
	ezaDraculaClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(updated)), Header: make(http.Header)}, nil
	})
	if err := filesApplyFZF(port, env); err != nil || bytes.Equal(first, filesTestRead(t, path)) || !strings.Contains(string(filesTestRead(t, path)), "fg:#ffffff") {
		t.Fatalf("latest upstream colors not applied: %v", err)
	}
}

func TestRosePineFailedAssetsDoNotChangeTheme(t *testing.T) {
	for _, failure := range []string{"offline", "malformed"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath("fzf", env)
			filesTestWrite(t, path, "--color=fg:#000000\n")
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "offline" {
					return nil, errors.New("offline")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("not fzf options"))), Header: make(http.Header)}, nil
			})
			port, _, _ := filesResolveRosePinePort("fzf", "rose-pine", env)
			if err := filesApplyFZF(port, env); err == nil {
				t.Fatal("accepted unavailable or malformed official asset")
			}
			if string(filesTestRead(t, path)) != "--color=fg:#000000\n" {
				t.Fatal("failed acquisition replaced active theme")
			}
		})
	}
	for _, script := range []string{"--color=fg:#ffffff\n", "export FZF_DEFAULT_OPTS=\"\n --bind=execute(evil)\n\"", "export FZF_DEFAULT_OPTS=\"\n--color=fg:#ffffff\n--preview=evil\""} {
		if _, err := rosePineFZFOptions([]byte(script)); err == nil {
			t.Fatalf("accepted untrusted fzf script %q", script)
		}
	}
}

func TestRosePineYaziNativeFlavorAndFailedOfficialInstall(t *testing.T) {
	env := filesTestEnv(t)
	port, ok, err := filesResolveRosePineYazi("rose-pine", env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("missing official flavor: %#v, %v", port, err)
	}
	path := filesPath("yazi", env)
	original := "[flavor]\ndark = 'old'\n[mgr]\nratio = [1, 2, 3]\n"
	filesTestWrite(t, path, original)
	ezaDraculaClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if err := filesApplyRosePineYazi(port, env); err == nil || string(filesTestRead(t, path)) != original {
		t.Fatalf("failed Yazi install modified theme: %v", err)
	}
	root := filesYaziFlavorRoot("rose-pine", env)
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#9ccfd8' }\n")
	filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), "<plist><dict/></plist>")
	port, ok, err = filesResolveRosePineYazi("rose-pine", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") {
		t.Fatalf("installed flavor not native: %#v, %v", port, err)
	}
	if err := filesApplyRosePineYazi(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	flavor := config["flavor"].(map[string]any)
	if flavor["dark"] != "rose-pine" || flavor["light"] != "rose-pine" || config["mgr"].(map[string]any)["ratio"] == nil {
		t.Fatalf("lost Yazi settings: %#v", config)
	}
	if _, ok, err := filesResolveRosePineYazi("rose-pine-dawn", env); ok || err != nil {
		t.Fatalf("unregistered Yazi variant accepted: %v", err)
	}
}

func TestRosePineDeltaFeatureImportsOnlyMainAndPreservesGitConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for native Git config parsing")
	}
	env := nativeTestEnvironment(t)
	port, ok, err := nativeResolveDelta("rose-pine", env)
	if err != nil || !ok || port.Kind != "official" || port.Source != rosePineDeltaSource {
		t.Fatalf("resolve Delta feature: %#v, %v", port, err)
	}
	theme := rosePineFixture(t, "rose-pine-delta.gitconfig")
	path := filepath.Join(env.Home, ".gitconfig")
	nativeTestPut(t, path, "[user]\n name = User\n[delta]\n line-numbers = true\n syntax-theme = Old\n[include]\n path = private.gitconfig\n")
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != rosePineDeltaSource {
			t.Fatalf("unexpected Delta request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(theme)), Header: make(http.Header)}, nil
	})
	if err := nativeApplyDelta(port, env); err != nil {
		t.Fatal(err)
	}
	first := nativeTestGet(t, path)
	for _, keep := range []string{"name = User", "line-numbers = true", "syntax-theme = Old", "path = private.gitconfig", "features = rose-pine"} {
		if !strings.Contains(first, keep) {
			t.Errorf("missing user or official Delta setting %q in %s", keep, first)
		}
	}
	style, err := nativeCommand(env, "git", "config", "--file", path, "--get", "delta.rose-pine.file-style")
	if err != nil || strings.TrimSpace(string(style)) != `#9ccfd8 bold ul` {
		t.Fatalf("official Delta feature style not imported: %q, %v", style, err)
	}
	if strings.Contains(first, "rose-pine-moon") || strings.Contains(first, "rose-pine-dawn") || strings.Contains(first, "[delta \"dark\"]") {
		t.Fatalf("imported unselected Delta variants: %s", first)
	}
	ezaDraculaClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if err := nativeApplyDelta(port, env); err == nil || nativeTestGet(t, path) != first {
		t.Fatalf("offline Delta reapply changed config: %v", err)
	}
}

func TestRosePineDeltaRejectedAssetLeavesGitConfigUnchanged(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.Home, ".gitconfig")
	nativeTestPut(t, path, "[user]\n name = Keep\n")
	for _, port := range []model.Port{
		{Kind: "official", Name: "rose-pine", Source: "https://example.com/unverified"},
		{Kind: "official", Name: "rose-pine", Source: rosePineDeltaSource, File: "override"},
	} {
		if err := nativeApplyDelta(port, env); err == nil {
			t.Fatal("unregistered Delta feature accepted")
		}
	}
	ezaDraculaClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	port, _, _ := nativeResolveDelta("rose-pine", env)
	if err := nativeApplyDelta(port, env); err == nil || nativeTestGet(t, path) != "[user]\n name = Keep\n" {
		t.Fatalf("failed Delta download changed Git config: %v", err)
	}
	if _, err := nativeRosePineDeltaMain([]byte("[delta \"other\"]\n\tdark = true\n")); err == nil {
		t.Fatal("accepted unrelated Delta feature")
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
	resolve := nativeListedResolver("bat", "--list-themes", "--color=never")
	port, ok, err := resolve("rose-pine", env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "Rosé Pine" || port.Source != "https://github.com/rose-pine/tm-theme" {
		t.Fatalf("installed Rosé Pine syntax: %#v, %v", port, err)
	}
	path := env.Paths["bat"]
	nativeTestPut(t, path, "--plain\n--theme=\"Old\"\n")
	if err := nativeApplyBat(port, env); err != nil {
		t.Fatal(err)
	}
	if data := nativeTestGet(t, path); !strings.Contains(data, "--plain\n") || !strings.Contains(data, `--theme="Rosé Pine"`) {
		t.Fatalf("bat options lost: %s", data)
	}
	nativeTestPut(t, binary, "#!/bin/sh\nprintf '%s\\n' 'Rosé Pine Moon'\n")
	if _, ok, err := resolve("rose-pine", env); err != nil || ok {
		t.Fatalf("substituted Moon variant: %v", err)
	}
}

func TestRosePineStarshipImportsPaletteNotUpstreamPrompt(t *testing.T) {
	upstream := []byte("palette = 'rose-pine'\nformat = '$directory'\n[palettes.rose-pine]\npine = '#31748f'\nrose = '#ebbcba'\n[directory]\nformat = 'UPSTREAM PROMPT'\n")
	colors, err := filesPaletteFamilyTOML(upstream, "rose-pine")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := filesTOML(colors)
	if err != nil {
		t.Fatal(err)
	}
	if selected["format"] != nil || selected["directory"] != nil || selected["palette"] != "rose-pine" {
		t.Fatalf("prompt settings leaked into theme palette: %#v", selected)
	}
	palette := selected["palettes"].(map[string]any)["rose-pine"].(map[string]any)
	if palette["pine"] != "#31748f" || palette["rose"] != "#ebbcba" {
		t.Fatalf("lost official Starship colors: %#v", palette)
	}
	if _, err := filesPaletteFamilyTOML(upstream, "rose-pine-moon"); err == nil {
		t.Fatal("accepted an unselected upstream palette")
	}
}

func TestRosePineHarlequinRequiresListedNativeTheme(t *testing.T) {
	if names := nativeNames["harlequin"]["rose-pine"]; len(names) != 1 || names[0] != "rose-pine" {
		t.Fatalf("native Harlequin theme mapping missing: %q", names)
	}
	if nativeHarlequinHasTheme([]byte("--theme [rose-pine-moon|dracula]\n"), "rose-pine") {
		t.Fatal("substituted a different Harlequin variant")
	}
	if !nativeHarlequinHasTheme([]byte("--theme [rose-pine|dracula]\n"), "rose-pine") {
		t.Fatal("did not detect listed native theme")
	}
}
