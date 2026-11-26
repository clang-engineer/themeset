package delta

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestDeltaFeatureIsolatesSelectedSection(t *testing.T) {
	asset := []byte("[delta \"rose-pine\"]\n\tdark = true\n\tfile-style = \"#123456\" bold\n" +
		"[delta \"rose-pine-moon\"]\n\tfile-style = \"#abcdef\"\n" +
		"[delta \"rose-pine-dawn\"]\n\tfile-style = \"#ffffff\"\n" +
		"[delta \"dark\"]\n\tfeatures = rose-pine-moon\n")
	main, err := nativeDeltaFeature(asset, "rose-pine")
	if err != nil {
		t.Fatal(err)
	}
	if want := "[delta \"rose-pine\"]\n\tdark = true\n\tfile-style = \"#123456\" bold\n"; string(main) != want {
		t.Fatalf("imported unintended sections: %q", main)
	}
	moon, err := nativeDeltaFeature(asset, "rose-pine-moon")
	if err != nil || string(moon) != "[delta \"rose-pine-moon\"]\n\tfile-style = \"#abcdef\"\n" {
		t.Fatalf("failed to select middle feature without importing neighbors: %q, %v", moon, err)
	}
}

func TestDeltaFeatureAcceptsCommentedSourceAndSpaceIndentedStyle(t *testing.T) {
	asset := []byte("# Upstream install instructions\n# [include]\n" +
		"[delta \"dracula\"]\n    dark = true\n    file-style = \"#f8f8f2\"\n" +
		"[delta \"other\"]\n    file-style = red\n")
	main, err := nativeDeltaFeature(asset, "dracula")
	if err != nil || string(main) != "[delta \"dracula\"]\n    dark = true\n    file-style = \"#f8f8f2\"\n" {
		t.Fatalf("failed to isolate Dracula feature from instructions and other themes: %q, %v", main, err)
	}
}

func TestDeltaFeatureRejectsMalformedOrOtherSource(t *testing.T) {
	for _, tc := range []struct {
		feature, source string
	}{
		{"rose-pine", "[delta \"other\"]\n\tfile-style = blue\n"},
		{"rose-pine", "[delta \"rose-pine\"]\n\tdark = true\n[delta \"other\"]\n\tfile-style = blue\n"},
		{"other", "[delta \"rose-pine\"]\n\tfile-style = blue\n[delta \"other\"]\n\tdark = true\n"},
		{"rose-pine", "[delta \"rose-pine\"]\n\tdark = true\n"},
		{"rose-pine", "garbage\n[delta \"rose-pine\"]\n\tdark = true\n"},
	} {
		if _, err := nativeDeltaFeature([]byte(tc.source), tc.feature); err == nil {
			t.Fatalf("accepted malformed or unrelated source for %q: %q", tc.feature, tc.source)
		}
	}
}

func TestDeltaOfficialPortsRejectTampering(t *testing.T) {
	port := *catalog.Port("rose-pine", "delta").Official
	for _, mutate := range []func(*model.Port){
		func(p *model.Port) { p.Name = "other" },
		func(p *model.Port) { p.Source = "https://example.test/other" },
	} {
		bad := port
		mutate(&bad)
		if err := nativeApplyDelta(bad, model.Environment{Home: t.TempDir()}); err == nil {
			t.Fatalf("accepted unregistered official Delta port: %#v", bad)
		}
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
