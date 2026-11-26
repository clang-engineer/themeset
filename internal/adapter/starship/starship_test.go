package starship

import (
	"bytes"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"io"
	"net/http"
	"strings"
	"testing"
)

const filesStarshipTestMoonfly = `palette = "moonfly"
format = "UPSTREAM FORMAT MUST NOT BE IMPORTED"
[directory]
style = "UPSTREAM STYLE MUST NOT BE IMPORTED"
[palettes.moonfly]
fern = "#87d787"
lavender = "#adadf3"
orange = "#f09479"
red = "#f07496"
`

func filesStarshipTestSource(t *testing.T, sources map[string]*string) {
	t.Helper()
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		source, ok := sources[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected Starship source %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(*source)), Header: make(http.Header)}, nil
	})
}

func TestFilesStarshipOfficialFetchesLatestPaletteOnly(t *testing.T) {
	for _, tt := range []struct{ name, url, source string }{
		{"dracula", shared.StarshipDraculaSource, "palette = 'dracula'\nformat = 'DO NOT COPY'\n[palettes.dracula]\nforeground = '#f8f8f2'\n"},
		{"moonfly", shared.MoonflySource + "moonfly-starship.toml", filesStarshipTestMoonfly},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("starship", env)
			testutil.Write(t, path, "format = '$directory'\n[directory]\nstyle = 'keep'\n[palettes.keep]\nred = '#123456'\n")
			source := tt.source
			filesStarshipTestSource(t, map[string]*string{tt.url: &source})
			port, ok, err := filesResolveStarship(tt.name, env)
			if !ok || err != nil || port.Source != tt.url {
				t.Fatalf("resolve: %#v, %v", port, err)
			}
			if err := filesApplyStarship(port, env); err != nil {
				t.Fatal(err)
			}
			config, err := shared.TOML(testutil.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			if config["format"] != "$directory" || config["directory"].(map[string]any)["style"] != "keep" || config["palettes"].(map[string]any)["keep"] == nil {
				t.Fatalf("upstream replaced unrelated settings: %#v", config)
			}
			previous := testutil.Read(t, path)
			source = strings.ReplaceAll(source, "#f8f8f2", "#aabbcc")
			source = strings.ReplaceAll(source, "#87d787", "#aabbcc")
			if err := filesApplyStarship(port, env); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(previous, testutil.Read(t, path)) || !strings.Contains(string(testutil.Read(t, path)), "#aabbcc") {
				t.Fatal("reapply did not import latest upstream palette")
			}
			previous = testutil.Read(t, path)
			for _, invalid := range []string{
				"palette = 'wrong'\n[palettes.wrong]\nred = '#123456'\n",
				"palette = '" + tt.name + "'\n[palettes." + tt.name + "]\nred = '$(echo unsafe)'\n",
				"broken = [\n",
			} {
				source = invalid
				if err := filesApplyStarship(port, env); err == nil || !bytes.Equal(previous, testutil.Read(t, path)) {
					t.Fatalf("invalid upstream source changed Starship: %q, %v", invalid, err)
				}
			}
		})
	}
}

func TestFilesStarshipOfficialFailedDownloadPreservesConfiguration(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("starship", env)
	original := "format = '$directory'\n[palettes.keep]\nred = '#123456'\n"
	testutil.Write(t, path, original)
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != shared.StarshipDraculaSource {
			t.Fatalf("unexpected source: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveStarship("dracula", env)
	if !ok || err != nil {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	if err := filesApplyStarship(port, env); err == nil || string(testutil.Read(t, path)) != original {
		t.Fatalf("failed download changed Starship configuration: %v", err)
	}
}
