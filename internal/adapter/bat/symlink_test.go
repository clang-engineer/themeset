package bat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestNativeThemeUpdatesKeepConfigAndParentSymlinks(t *testing.T) {
	env := model.Environment{Home: t.TempDir(), OS: "linux", Paths: map[string]string{}}
	directory := filepath.Join(env.Home, "config-target")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(directory, "bat-settings")
	if err := os.WriteFile(target, []byte("--style=numbers\n--theme=Dracula\n"), 0640); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(directory, "config")
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(env.Home, "alias")
	if err := os.Symlink(directory, parent); err != nil {
		t.Fatal(err)
	}
	env.Paths["bat"] = filepath.Join(parent, "config")
	if err := Adapter().Apply(model.Port{Name: "Nord"}, env); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{leaf: target, parent: directory} {
		got, err := os.Readlink(path)
		if err != nil || got != want {
			t.Fatalf("link %s changed: %q %v", path, got, err)
		}
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	theme, err := nativeParseBat(data)
	if err != nil || theme != "Nord" || !strings.Contains(string(data), "--style=numbers") {
		t.Fatalf("theme/config not preserved: %q, %v", data, err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("config permissions changed: %v %v", info, err)
	}
}

func TestNativeBatRequiresThemeSelection(t *testing.T) {
	if _, err := nativeParseBat([]byte("--style=plain\n")); err == nil {
		t.Fatal("missing theme accepted")
	}
}

func TestResolveBatUsesDeclaredCandidatesInOrder(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "bat"), []byte("#!/bin/sh\nprintf 'Second\\nFirst\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	env := model.Environment{Home: t.TempDir()}
	spec := model.ThemePort{Native: []string{"First", "Second"}, Official: &model.Port{Kind: "official", Name: "fallback"}}
	port, ok, err := Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "First" {
		t.Fatalf("candidate order ignored: %#v, %v", port, err)
	}
	spec.Native = nil
	port, ok, err = Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "official" || port.Name != "fallback" {
		t.Fatalf("declared official fallback ignored: %#v, %v", port, err)
	}
}
