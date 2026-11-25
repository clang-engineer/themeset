package adapter

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestPaletteFamilyResolveWithoutDownload(t *testing.T) {
	env := filesTestEnv(t)
	for _, tt := range []struct {
		id, name, source string
		ok               bool
	}{
		{"starship", "catppuccin-mocha", filesPaletteCatSource, true},
		{"starship", "gruvbox", filesPaletteGruvSource, true},
		{"starship", "nord", "", false},
		{"lazygit", "gruvbox", "", false},
		{"lazygit", "nord", "", false},
		{"lazygit", "moonfly", "", false},
	} {
		port, ok, err := filesResolvePaletteFamily(tt.id, tt.name, env)
		if err != nil || ok != tt.ok || (ok && (port.Kind != "official" || port.Name != tt.name || port.Source != tt.source)) {
			t.Fatalf("resolve %s/%s: %#v, %v, %v", tt.id, tt.name, port, ok, err)
		}
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created cache: %v", err)
	}
}

func TestPaletteFamilyCatppuccinOfflineAcquisitionAndImport(t *testing.T) {
	env := filesTestEnv(t)
	palette, err := os.ReadFile("testdata/starship-catppuccin-mocha.toml")
	if err != nil {
		t.Fatal(err)
	}
	license, err := os.ReadFile("testdata/starship-catppuccin-mocha.LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range []struct {
		data   []byte
		digest string
	}{
		{palette, filesPaletteCatSHA256},
		{license, filesPaletteCatLicenseSHA256},
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256(asset.data)); got != asset.digest {
			t.Fatalf("fixture digest changed: %s != %s", got, asset.digest)
		}
		filesTestWrite(t, filepath.Join(env.Home, ".cache", "themeset", "assets", asset.digest), string(asset.data))
	}
	path := filesPath("starship", env)
	filesTestWrite(t, path, "format = '$directory'\nadd_newline = false\n[directory]\ntruncation_length = 2\n[palettes.keep]\nred = '#112233'\n")
	var first []byte
	for i := range 2 {
		data, err := filesAcquirePaletteFamily("starship", "catppuccin-mocha", env)
		if err != nil {
			t.Fatal(err)
		}
		incoming, err := filesTOML(data)
		if err != nil || incoming["palette"] != "catppuccin_mocha" {
			t.Fatalf("native selector: %#v, %v", incoming, err)
		}
		if len(incoming) != 2 {
			t.Fatalf("unexpected imported application keys: %#v", incoming)
		}
		// Exercise the existing Starship import path without requiring network:
		// this is exactly the normalized data parent wires into official apply.
		portFile := filepath.Join(env.Home, "normalized.toml")
		filesTestWrite(t, portFile, string(data))
		if err := filesApplyStarship(model.Port{Kind: "custom", File: portFile}, env); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = filesTestRead(t, path)
		} else if !bytes.Equal(first, filesTestRead(t, path)) {
			t.Fatal("reapplying palette changed Starship config")
		}
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	palettes := config["palettes"].(map[string]any)
	mocha := palettes["catppuccin_mocha"].(map[string]any)
	if config["palette"] != "catppuccin_mocha" || config["format"] != "$directory" || config["add_newline"] != false ||
		config["directory"].(map[string]any)["truncation_length"] != int64(2) || palettes["keep"].(map[string]any)["red"] != "#112233" ||
		len(mocha) != 26 || mocha["mauve"] != "#cba6f7" || mocha["crust"] != "#11111b" {
		t.Fatalf("palette import changed unrelated settings or colors: %#v", config)
	}
	// The verified license is mandatory even when the palette itself is cached.
	// Acquisition must fail without touching the active config.
	filesTestWrite(t, filepath.Join(env.Home, ".cache", "themeset", "assets", filesPaletteCatLicenseSHA256), "invalid")
	if _, err := filesAcquirePaletteFamily("starship", "catppuccin-mocha", env); err == nil {
		t.Fatal("corrupted cached license accepted")
	}
	if !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatal("failed acquisition changed Starship config")
	}
}

func TestPaletteFamilyGruvboxImportsOnlyPresetPalette(t *testing.T) {
	// The official Gruvbox Rainbow source is a complete prompt preset. Its
	// format/modules must not appear in the normalized data or active config.
	preset := []byte("format = 'DO NOT IMPORT'\npalette = 'gruvbox_dark'\n[palettes.gruvbox_dark]\ncolor_fg0 = '#fbf1c7'\ncolor_blue = '#458588'\n[directory]\ntruncation_length = 99\n")
	data, err := filesPaletteFamilyTOML(preset, "gruvbox_dark")
	if err != nil {
		t.Fatal(err)
	}
	incoming, err := filesTOML(data)
	if err != nil || len(incoming) != 2 || incoming["format"] != nil || incoming["directory"] != nil {
		t.Fatalf("preset leaked non-palette keys: %#v, %v", incoming, err)
	}
	env := filesTestEnv(t)
	path := filesPath("starship", env)
	filesTestWrite(t, path, "format = '$character'\n[directory]\ntruncation_length = 2\n")
	portFile := filepath.Join(env.Home, "gruvbox.toml")
	filesTestWrite(t, portFile, string(data))
	if err := filesApplyStarship(model.Port{Kind: "custom", File: portFile}, env); err != nil {
		t.Fatal(err)
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if config["palette"] != "gruvbox_dark" || config["format"] != "$character" || config["directory"].(map[string]any)["truncation_length"] != int64(2) ||
		config["palettes"].(map[string]any)["gruvbox_dark"].(map[string]any)["color_blue"] != "#458588" {
		t.Fatalf("preset overwrote prompt or lost palette: %#v", config)
	}
}

func TestPaletteFamilyRejectsMissingAndMismatchedPalettes(t *testing.T) {
	for _, source := range []string{
		"palette = 'not_gruvbox'\n[palettes.gruvbox_dark]\ncolor_blue = '#458588'\n",
		"palette = 'gruvbox_dark'\n[palettes.other]\ncolor_blue = '#458588'\n",
		"[palettes.gruvbox_dark]\n",
		"this is not TOML",
	} {
		if _, err := filesPaletteFamilyTOML([]byte(source), "gruvbox_dark"); err == nil {
			t.Fatalf("invalid palette accepted: %q", source)
		}
	}
	if _, err := filesAcquirePaletteFamily("lazygit", "gruvbox", filesTestEnv(t)); err == nil {
		t.Fatal("unregistered lazygit theme acquired")
	}
}
