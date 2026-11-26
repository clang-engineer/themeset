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
	"github.com/clang-engineer/themeset/internal/model"
)

func TestPaletteFamilyResolveWithoutDownload(t *testing.T) {
	env := testutil.Env(t)
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

func TestPaletteFamilyCatppuccinFetchesLatestAndImportsOnlyPalette(t *testing.T) {
	env := testutil.Env(t)
	palette, err := os.ReadFile("../testdata/starship-catppuccin-mocha.toml")
	if err != nil {
		t.Fatal(err)
	}
	license, err := os.ReadFile("../testdata/starship-catppuccin-mocha.LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body []byte
		switch r.URL.String() {
		case filesPaletteCatSource:
			body = palette
		case filesPaletteCatLicenseSource:
			body = license
		default:
			t.Fatalf("unexpected palette request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	path := shared.Path("starship", env)
	testutil.Write(t, path, "format = '$directory'\nadd_newline = false\n[directory]\ntruncation_length = 2\n[palettes.keep]\nred = '#112233'\n")
	port, ok, err := filesResolveStarship("catppuccin-mocha", env)
	if err != nil || !ok || port.Source != filesPaletteCatSource || requests != 0 {
		t.Fatalf("resolve: %#v, ok=%v err=%v requests=%d", port, ok, err, requests)
	}
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("acquired %d assets, want palette and license", requests)
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
	if requests != 4 || config["palettes"].(map[string]any)["catppuccin_mocha"].(map[string]any)["mauve"] != "#123456" ||
		config["format"] != "$directory" {
		t.Fatalf("reapply did not import current palette or preserve prompt: %#v; requests=%d", config, requests)
	}
	previous := testutil.Read(t, path)
	// Even a valid palette must not apply if its license cannot be fetched.
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == filesPaletteCatLicenseSource {
			return nil, fmt.Errorf("offline")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(palette))), Header: make(http.Header)}, nil
	})
	if err := filesApplyStarship(port, env); err == nil || !bytes.Equal(previous, testutil.Read(t, path)) {
		t.Fatalf("license failure changed Starship settings: %v", err)
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
	port, ok, err := filesResolveStarship("gruvbox", env)
	if err != nil || !ok || port.Source != filesPaletteGruvSource {
		t.Fatalf("Gruvbox official port: %#v, %v", port, err)
	}
	color := "#458588"
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch r.URL.String() {
		case filesPaletteGruvSource:
			body = "format = 'DO NOT IMPORT'\npalette = 'gruvbox_dark'\n[palettes.gruvbox_dark]\ncolor_blue = '" + color + "'\n"
		case filesPaletteGruvLicenseSource:
			body = "Official license"
		default:
			t.Fatalf("unexpected Gruvbox download %s", r.URL)
		}
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
	if requests != 4 || config["format"] != "$directory" ||
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
	if _, err := filesAcquirePaletteFamily("lazygit", "gruvbox", testutil.Env(t)); err == nil {
		t.Fatal("unregistered lazygit theme acquired")
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
