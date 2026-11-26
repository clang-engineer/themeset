package harlequin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestNativeHarlequinListReadsOnlyThemeOption(t *testing.T) {
	help := []byte("--theme [dracula|catppuccin-mocha|nord]\n  Theme to use\n--adapter TEXT\n gruvbox adapter\n")
	if !nativeHarlequinHasTheme(help, "catppuccin-mocha") {
		t.Fatal("theme was not detected")
	}
	if nativeHarlequinHasTheme(help, "gruvbox") {
		t.Fatal("unrelated option text was interpreted as a theme")
	}
	if nativeHarlequinHasTheme(help, "catppuccin") {
		t.Fatal("partial theme name matched")
	}
	if nativeHarlequinHasTheme([]byte("--theme [rose-pine-moon|dracula]\n"), "rose-pine") {
		t.Fatal("substituted a different Harlequin variant")
	}
	if !nativeHarlequinHasTheme([]byte("--theme [rose-pine|dracula]\n"), "rose-pine") {
		t.Fatal("did not detect listed native theme")
	}
}

func TestResolveHarlequinUsesDeclaredCandidates(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "harlequin"), []byte("#!/bin/sh\nprintf '%s\\n' '--theme [local-only|dracula]' '--adapter TEXT' 'missing option'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	env := model.Environment{Home: t.TempDir()}
	spec := model.ThemePort{Native: []string{"missing", "local-only"}, Official: &model.Port{Kind: "official", Name: "fallback"}}
	port, ok, err := Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "local-only" {
		t.Fatalf("declared native theme ignored: %#v, %v", port, err)
	}
	spec.Native = nil
	port, ok, err = Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "official" || port.Name != "fallback" {
		t.Fatalf("declared official fallback ignored: %#v, %v", port, err)
	}
}
