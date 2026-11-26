package starship

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestPaletteFamilyResolveWithoutDownload(t *testing.T) {
	env := testutil.Env(t)
	for _, tt := range []struct{ name, source string }{
		{"catppuccin-mocha", shared.StarshipCatppuccinSource},
		{"gruvbox", shared.StarshipGruvboxSource},
	} {
		spec := catalog.Port(tt.name, "starship")
		port, ok, err := filesResolveStarship(spec, env)
		if err != nil || !ok || port.Kind != "official" || port.Name != tt.name || port.Source != tt.source {
			t.Fatalf("resolve %s: %#v, %v, %v", tt.name, port, ok, err)
		}
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created cache: %v", err)
	}
}

func TestPaletteFamilyCatppuccinFetchesLatestAndImportsOnlyPalette(t *testing.T) {
	env := testutil.Env(t)
	palette, err := os.ReadFile("../testdata/starship-catppuccin-mocha.toml")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != shared.StarshipCatppuccinSource {
			t.Fatalf("unexpected palette request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(palette)), Header: make(http.Header)}, nil
	})
	path := shared.Path("starship", env)
	testutil.Write(t, path, "format = '$directory'\nadd_newline = false\n[directory]\ntruncation_length = 2\n[palettes.keep]\nred = '#112233'\n")
	port, ok, err := filesResolveStarship(catalog.Port("catppuccin-mocha", "starship"), env)
	if err != nil || !ok || port.Source != shared.StarshipCatppuccinSource || requests != 0 {
		t.Fatalf("resolve: %#v, ok=%v err=%v requests=%d", port, ok, err, requests)
	}
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("acquired %d assets, want palette only", requests)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	palettes := config["palettes"].(map[string]any)
	mocha := palettes["catppuccin_mocha"].(map[string]any)
	if config["palette"] != "catppuccin_mocha" || config["format"] != "$directory" || config["add_newline"] != false ||
		config["directory"].(map[string]any)["truncation_length"] != int64(2) || palettes["keep"].(map[string]any)["red"] != "#112233" ||
		mocha["mauve"] != "#cba6f7" {
		t.Fatalf("palette import changed unrelated settings or colors: %#v", config)
	}
	// A changed upstream palette is fetched on reapply rather than read from a cache.
	palette = []byte("[palettes.catppuccin_mocha]\nmauve = '#123456'\n")
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}
	config, err = shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || config["palettes"].(map[string]any)["catppuccin_mocha"].(map[string]any)["mauve"] != "#123456" ||
		config["format"] != "$directory" {
		t.Fatalf("reapply did not import current palette or preserve prompt: %#v; requests=%d", config, requests)
	}
}

func TestPaletteFamilyOfflinePreservesConfiguration(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("starship", env)
	original := "format = '$directory'\n[palettes.keep]\nred = '#123456'\n"
	testutil.Write(t, path, original)
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != shared.StarshipCatppuccinSource {
			t.Fatalf("unexpected palette request %s", r.URL)
		}
		return nil, fmt.Errorf("offline")
	})
	port, ok, err := filesResolveStarship(catalog.Port("catppuccin-mocha", "starship"), env)
	if err != nil || !ok {
		t.Fatalf("resolve: %#v, ok=%v err=%v", port, ok, err)
	}
	if err := filesApplyStarship(port, env); err == nil || string(testutil.Read(t, path)) != original {
		t.Fatalf("offline palette changed Starship settings: %v", err)
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
	incoming, err := shared.TOML(data)
	if err != nil || len(incoming) != 2 || incoming["format"] != nil || incoming["directory"] != nil {
		t.Fatalf("preset leaked non-palette keys: %#v, %v", incoming, err)
	}
	env := testutil.Env(t)
	path := shared.Path("starship", env)
	testutil.Write(t, path, "format = '$character'\n[directory]\ntruncation_length = 2\n")
	portFile := filepath.Join(env.Home, "gruvbox.toml")
	testutil.Write(t, portFile, string(data))
	if err := filesApplyStarship(model.Port{Kind: "custom", File: portFile}, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if config["palette"] != "gruvbox_dark" || config["format"] != "$character" || config["directory"].(map[string]any)["truncation_length"] != int64(2) ||
		config["palettes"].(map[string]any)["gruvbox_dark"].(map[string]any)["color_blue"] != "#458588" {
		t.Fatalf("preset overwrote prompt or lost palette: %#v", config)
	}
}

func TestPaletteFamilyGruvboxFetchesCurrentPreset(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("starship", env)
	testutil.Write(t, path, "format = '$directory'\n[directory]\ntruncation_length = 2\n")
	port, ok, err := filesResolveStarship(catalog.Port("gruvbox", "starship"), env)
	if err != nil || !ok || port.Source != shared.StarshipGruvboxSource {
		t.Fatalf("Gruvbox official port: %#v, %v", port, err)
	}
	color := "#458588"
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != shared.StarshipGruvboxSource {
			t.Fatalf("unexpected Gruvbox download %s", r.URL)
		}
		body := "format = 'DO NOT IMPORT'\npalette = 'gruvbox_dark'\n[palettes.gruvbox_dark]\ncolor_blue = '" + color + "'\n"
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}
	color = "#123456"
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || config["format"] != "$directory" ||
		config["palettes"].(map[string]any)["gruvbox_dark"].(map[string]any)["color_blue"] != color {
		t.Fatalf("did not fetch current Gruvbox palette or preserve layout: %#v; requests=%d", config, requests)
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
}

func TestStarshipRejectsForgedOfficialMetadataBeforeFetching(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("starship", env)
	original := "palette = 'keep'\n[palettes.keep]\nred = '#123456'\n"
	testutil.Write(t, path, original)
	for _, tt := range []struct {
		name   string
		change func(*model.Port)
	}{
		{"source", func(p *model.Port) { p.Source = "https://example.test/unregistered.toml" }},
		{"variant", func(p *model.Port) { p.Variant = "wrong" }},
		{"license", func(p *model.Port) { p.LicenseURL = "https://example.test/unregistered-license" }},
		{"format", func(p *model.Port) { p.Format = "starship-preset" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			port := *catalog.Port("gruvbox", "starship").Official
			tt.change(&port)
			if err := filesApplyStarship(port, env); err == nil || string(testutil.Read(t, path)) != original {
				t.Fatalf("forged %s accepted or changed configuration: %v", tt.name, err)
			}
		})
	}
}

func TestRosePineStarshipImportsPaletteNotUpstreamPrompt(t *testing.T) {
	upstream := []byte("palette = 'rose-pine'\nformat = '$directory'\n[palettes.rose-pine]\npine = '#31748f'\nrose = '#ebbcba'\n[directory]\nformat = 'UPSTREAM PROMPT'\n")
	colors, err := filesPaletteFamilyTOML(upstream, "rose-pine")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := shared.TOML(colors)
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
