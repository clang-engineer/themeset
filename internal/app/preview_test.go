package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func previewPaletteFixture() string {
	var data strings.Builder
	data.WriteString("# local fixture\nbackground = #012345\nforeground = fedcba\n")
	for index := range 16 {
		fmt.Fprintf(&data, "palette = %d=#%02x%02x%02x\n", index, index, index+16, index+32)
	}
	return data.String()
}

func TestPreviewGhosttyParsesExactValuesAndLastAssignment(t *testing.T) {
	data := previewPaletteFixture() + "background = abcdef\npalette = 15=#ffffff\npalette = 255=#000000\ncursor-color = #123456\n"
	palette, err := parsePreviewGhostty(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if palette.background != (previewRGB{0xab, 0xcd, 0xef}) || palette.foreground != (previewRGB{0xfe, 0xdc, 0xba}) {
		t.Fatalf("wrong base colors: %+v", palette)
	}
	if palette.ansi[0] != (previewRGB{0, 16, 32}) || palette.ansi[15] != (previewRGB{255, 255, 255}) {
		t.Fatalf("wrong palette values: %+v", palette.ansi)
	}
}

func TestPreviewGhosttyRejectsMissingOrAmbiguousColors(t *testing.T) {
	base := previewPaletteFixture()
	for name, data := range map[string]string{
		"missing background":     strings.ReplaceAll(base, "background = #012345\n", ""),
		"missing foreground":     strings.ReplaceAll(base, "foreground = fedcba\n", ""),
		"missing slot":           strings.ReplaceAll(base, "palette = 15=#0f1f2f\n", ""),
		"negative slot":          base + "palette = -1=#000000\n",
		"out of range slot":      base + "palette = 256=#000000\n",
		"noninteger slot":        base + "palette = 1.5=#000000\n",
		"named color":            base + "foreground = red\n",
		"truncated RGB":          base + "foreground = #fff\n",
		"invalid RGB":            base + "foreground = #zzzzzz\n",
		"color control sequence": base + "foreground = #ffffff\x1b]0;unsafe\a\n",
		"inherited theme":        base + "theme = other\n",
		"included file":          base + "config-file = /tmp/other\n",
		"nonassignment":          base + "arbitrary script text\n",
		"oversized line":         base + "#" + strings.Repeat("x", 70*1024),
		"oversized asset":        base + strings.Repeat("# fixture\n", 110000),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parsePreviewGhostty(strings.NewReader(data)); err == nil {
				t.Fatal("accepted an incomplete or ambiguous theme")
			}
		})
	}
}

func TestPreviewGhosttyDiscoveryUsesEnvironmentAndUserPrecedence(t *testing.T) {
	home, resources, configHome := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GHOSTTY_RESOURCES_DIR", resources)
	env := model.Environment{Home: home, ConfigHome: configHome, OS: "linux"}
	name := "themeset-preview-discovery-fixture"
	resourceAsset := filepath.Join(resources, "themes", name)
	fixtureFile(t, resourceAsset, previewPaletteFixture())
	path, err := previewGhosttyAsset(name, env)
	if err != nil || path != resourceAsset {
		t.Fatalf("resource lookup: path=%q err=%v", path, err)
	}
	userAsset := filepath.Join(configHome, "ghostty", "themes", name)
	fixtureFile(t, userAsset, previewPaletteFixture())
	path, err = previewGhosttyAsset(name, env)
	if err != nil || path != userAsset {
		t.Fatalf("user lookup: path=%q err=%v", path, err)
	}
	env.Paths = map[string]string{"ghostty": filepath.Join(home, "configured", "config")}
	configuredAsset := filepath.Join(home, "configured", "themes", name)
	fixtureFile(t, configuredAsset, previewPaletteFixture())
	path, err = previewGhosttyAsset(name, env)
	if err != nil || path != configuredAsset {
		t.Fatalf("configured lookup: path=%q err=%v", path, err)
	}
	if err := os.Remove(configuredAsset); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configuredAsset, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := previewGhosttyAsset(name, env); err == nil {
		t.Fatal("nonregular shadowing asset must be an inspection error")
	}
}

func TestPreviewGhosttyDiscoveryRejectsPathTraversal(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../outside", "nested/theme", "nested\\theme", "name\x00", "name\n", "name\x1b"} {
		if _, err := previewGhosttyAsset(name, model.Environment{Home: t.TempDir()}); err == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
}

func TestVisualPreviewCustomAssetDoesNotEmitAssetControlSequences(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	path := filepath.Join(home, "fixture.theme")
	fixtureFile(t, path, previewPaletteFixture()+"ignored = \x1b]0;asset-injection\a\n")
	config := model.Config{Themes: map[string]model.Theme{
		"fixture": {Custom: map[string]model.Port{"ghostty": {File: path, Name: "variant\x1b]0;name-injection\a", Source: "source\x1b]0;source-injection\a"}}},
	}}
	var out bytes.Buffer
	if err := renderVisualPreview("fixture", config, model.Environment{Home: home, OS: "linux"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "\x1b]") || strings.Contains(text, "\a") || strings.Contains(text, "asset-injection") {
		t.Fatalf("asset text escaped into the terminal: %q", text)
	}
	if !strings.Contains(text, "\x1b[48;2;1;35;69m") {
		t.Fatalf("missing source or actual palette output: %q", text)
	}
}

func TestVisualPreviewMissingAssetIsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	config := model.Config{Themes: map[string]model.Theme{
		"fixture": {Custom: map[string]model.Port{"ghostty": {File: filepath.Join(home, "absent.theme")}}},
	}}
	var out bytes.Buffer
	if err := renderVisualPreview("fixture", config, model.Environment{Home: home, OS: "linux"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("missing asset fabricated a preview: %q", out.String())
	}
}

func TestPreviewGhosttyInspectionErrorsQuoteAssetPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent\x1b]0;path-injection\a")
	var out bytes.Buffer
	err := previewGhosttyFile(path, "", "registered custom", "", &out)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing asset classification lost: %v", err)
	}
	if strings.ContainsAny(err.Error(), "\x1b\a") {
		t.Fatalf("raw asset path in diagnostic: %q", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("failed inspection produced a visual: %q", out.String())
	}
}

func TestVisualPreviewDoesNotSubstituteCustomForUnreadableNative(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	t.Setenv("PATH", bin)
	ghostty := filepath.Join(bin, "ghostty")
	fixtureFile(t, ghostty, "#!/bin/sh\nprintf 'Nord (resources)\\n'\n")
	if err := os.Chmod(ghostty, 0700); err != nil {
		t.Fatal(err)
	}
	configHome := filepath.Join(home, ".config")
	fixtureFile(t, filepath.Join(configHome, "ghostty", "themes", "Nord"), "background = #000000\n")
	custom := filepath.Join(home, "custom.theme")
	fixtureFile(t, custom, previewPaletteFixture())
	config := model.Config{Themes: map[string]model.Theme{
		"nord": {Custom: map[string]model.Port{"ghostty": {File: custom}}},
	}}
	var out bytes.Buffer
	err := renderVisualPreview("nord", config, model.Environment{Home: home, ConfigHome: configHome, OS: "linux"}, &out)
	if err == nil || strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("native inspection error must not preview an unapplied custom palette: err=%v output=%q", err, out.String())
	}
}
