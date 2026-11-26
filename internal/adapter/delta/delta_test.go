package delta

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestRosePineDeltaRejectsUnrelatedFeature(t *testing.T) {
	if _, err := nativeRosePineDeltaMain([]byte("[delta \"other\"]\n\tdark = true\n")); err == nil {
		t.Fatal("accepted unrelated Delta feature")
	}
}

func TestResolveDeltaUsesDeclaredCandidatesAndOfficial(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "delta"), []byte("#!/bin/sh\nprintf 'Local Only\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	env := model.Environment{Home: t.TempDir()}
	spec := model.ThemePort{Native: []string{"missing", "Local Only"}, Official: &model.Port{Kind: "official", Name: "fallback", Source: "https://example.com/theme"}}
	port, ok, err := Adapter().Resolve(spec, env)
	if err != nil || !ok || port.Kind != "native" || port.Name != "Local Only" {
		t.Fatalf("declared native theme ignored: %#v, %v", port, err)
	}
	spec.Native = nil
	port, ok, err = Adapter().Resolve(spec, env)
	if err != nil || !ok || port != *spec.Official {
		t.Fatalf("declared official fallback ignored: %#v, %v", port, err)
	}
}
