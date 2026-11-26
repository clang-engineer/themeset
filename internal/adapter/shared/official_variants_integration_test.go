package shared_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/btop"
	"github.com/clang-engineer/themeset/internal/adapter/eza"
	"github.com/clang-engineer/themeset/internal/adapter/fzf"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/adapter/tmux"
	"github.com/clang-engineer/themeset/internal/adapter/yazi"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestTokyoNightOfficialRoutesReadOnly(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"btop", "eza", "fzf", "lazygit", "yazi"} {
		port, ok, err := shared.ResolveTokyoNightPort(id, "tokyo-night", env)
		source, _ := shared.TokyoNightSource(id)
		if err != nil || !ok || port.Kind != "official" || port.Source != source {
			t.Fatalf("%s: port=%#v ok=%v err=%v", id, port, ok, err)
		}
		if id == "btop" && port.Name != "tokyonight_night" {
			t.Fatalf("btop must select upstream filename: %#v", port)
		}
		if id != "btop" && port.Name != "tokyo-night" {
			t.Fatalf("%s unexpected theme name: %#v", id, port)
		}
		if _, ok, err := shared.ResolveTokyoNightPort(id, "tokyonight", env); ok || err != nil {
			t.Fatalf("%s unexpectedly registered an additional catalog ID", id)
		}
	}
	for _, id := range []string{"atuin", "starship", "lazydocker", "syntax"} {
		if _, ok, err := shared.ResolveTokyoNightPort(id, "tokyo-night", env); ok || err != nil {
			t.Fatalf("unsupported %s port resolved", id)
		}
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolution touched cache: %v", err)
	}
}

func TestTokyoNightAcquisitionFailurePreservesActiveThemes(t *testing.T) {
	for _, id := range []string{"eza", "lazygit", "fzf", "btop", "yazi"} {
		t.Run(id, func(t *testing.T) {
			env := filesTestEnv(t)
			path := shared.Path(id, env)
			const original = "# untouched\n"
			filesTestWrite(t, path, original)
			testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline: %s", r.URL) })
			port, ok, err := shared.ResolveTokyoNightPort(id, "tokyo-night", env)
			if err != nil || !ok {
				t.Fatal("official port missing")
			}
			switch id {
			case "eza":
				err = eza.Adapter().Apply(port, env)
			case "lazygit":
				err = filesTestAdapter(t, id).Apply(port, env)
			case "fzf":
				err = fzf.Adapter().Apply(port, env)
			case "btop":
				err = btop.Adapter().Apply(port, env)
			case "yazi":
				err = yazi.Adapter().Apply(port, env)
			}
			if err == nil || !strings.Contains(err.Error(), "offline") {
				t.Fatalf("expected acquisition failure, got %v", err)
			}
			if string(filesTestRead(t, path)) != original {
				t.Fatal("failed acquisition changed existing config")
			}
		})
	}
}

func TestTokyoNightRejectsUnregisteredOfficialSources(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"eza", "fzf", "btop", "lazygit", "yazi"} {
		port, _, _ := shared.ResolveTokyoNightPort(id, "tokyo-night", env)
		port.Source += "?unverified=true"
		if _, err := shared.AcquireTokyoNightPort(id, port, env); err == nil {
			t.Fatalf("accepted unregistered %s source", id)
		}
	}
	btop, _, _ := shared.ResolveTokyoNightPort("btop", "tokyo-night", env)
	btop.Name = "tokyo-night"
	if _, err := shared.AcquireTokyoNightPort("btop", btop, env); err == nil {
		t.Fatal("accepted wrong native btop filename")
	}
	if _, err := shared.AcquireTokyoNightPort("starship", model.Port{Kind: "official", Name: "tokyo-night"}, env); err == nil {
		t.Fatal("accepted unsupported Starship source")
	}
}

func TestTokyoNightTmuxAcquisitionFailureKeepsConfig(t *testing.T) {
	env := filesTestEnv(t)
	path := filepath.Join(env.Home, ".tmux.conf")
	filesTestWrite(t, path, "set -g status-left 'my layout'\n")
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline: %s", r.URL)
	})
	port, ok, err := tmux.Adapter().Resolve("tokyo-night", env)
	source, _ := shared.TokyoNightSource("tmux")
	if err != nil || !ok || port.Kind != "official" || port.Source != source {
		t.Fatalf("Tokyo Night tmux not registered: %#v, %v", port, err)
	}
	if err := tmux.Adapter().Apply(port, env); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("expected safe failed acquisition, got %v", err)
	}
	if got := string(filesTestRead(t, path)); got != "set -g status-left 'my layout'\n" {
		t.Fatalf("acquisition rewrote tmux layout: %q", got)
	}
}

func TestTokyoNightNativeFlavorsTakePrecedenceOverDownloads(t *testing.T) {
	env := filesTestEnv(t)
	const flavor = "[mgr]\ncwd = { fg = '#abcdef' }\n"
	const syntax = "<plist><dict/></plist>"
	root := filepath.Join(filepath.Dir(shared.Path("yazi", env)), "flavors", "tokyo-night.yazi")
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), flavor)
	filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), syntax)
	port, ok, err := yazi.Adapter().Resolve("tokyo-night", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") {
		t.Fatalf("installed native Tokyo Night Yazi port ignored: %#v, %v", port, err)
	}
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("native port must not download: %s", r.URL)
		return nil, fmt.Errorf("unexpected download")
	})
	if err := yazi.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	if string(filesTestRead(t, filepath.Join(root, "flavor.toml"))) != flavor || string(filesTestRead(t, filepath.Join(root, "tmtheme.xml"))) != syntax {
		t.Fatal("replaced an independently installed flavor")
	}
	config, err := shared.TOML(filesTestRead(t, shared.Path("yazi", env)))
	if err != nil || config["flavor"].(map[string]any)["dark"] != "tokyo-night" {
		t.Fatalf("native Yazi flavor not selected: %#v %v", config, err)
	}

	btopPath := filepath.Join(filepath.Dir(shared.Path("btop", env)), "themes", "tokyonight_night.theme")
	filesTestWrite(t, btopPath, "theme[main_bg]=\"#1a1b26\"\n")
	btopPort, ok, err := btop.Adapter().Resolve("tokyo-night", env)
	if err != nil || !ok || btopPort.Kind != "native" || btopPort.File != btopPath || btopPort.Name != "tokyonight_night" {
		t.Fatalf("installed native btop port ignored: %#v, %v", btopPort, err)
	}
	if err := btop.Adapter().Apply(btopPort, env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(filesTestRead(t, shared.Path("btop", env))), "tokyonight_night") {
		t.Fatal("native btop theme was not selected")
	}
}
