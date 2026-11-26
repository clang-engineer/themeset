package neovim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestResolveNeovimChecksDeclaredInstalledProviders(t *testing.T) {
	home := t.TempDir()
	env := model.Environment{Home: home}
	root := filepath.Join(home, ".config", "nvim")
	if err := os.MkdirAll(filepath.Join(root, "colors"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "colors", "local-only.lua"), []byte("return {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := model.ThemePort{Native: []string{"missing", "local-only"}, Official: &model.Port{Kind: "official", Name: "fallback"}}
	port, ok, err := Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "local-only" || port.Source != root {
		t.Fatalf("installed provider ignored: %#v, %v", port, err)
	}
	spec.Native = []string{"missing"}
	port, ok, err = Adapter().Resolve(spec, env)
	if err != nil || !ok || port != *spec.Official {
		t.Fatalf("declared official fallback ignored: %#v, %v", port, err)
	}
}
