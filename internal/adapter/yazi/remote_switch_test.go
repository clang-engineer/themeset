package yazi

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

func TestRemoteYaziSwitchRemovesOnlyUnchangedManagedStyles(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\nuse = 'old'\ndark = 'old'\nlight = 'old'\n[mgr]\nratio = [1, 2, 3]\ncwd = { italic = true }\n")
	const firstURL = "https://example.test/first.toml"
	const secondURL = "https://example.test/second.toml"
	const syntaxURL = "https://example.test/syntax.tmTheme"
	testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
		var value string
		switch req.URL.String() {
		case firstURL:
			value = "[mgr]\ncwd = { fg = '#112233' }\n[status]\nperm_read = { fg = '#123456' }\n"
		case secondURL:
			value = "[mgr]\ncwd = { bg = '#445566' }\n"
		case syntaxURL:
			value = "<plist><dict/></plist>"
		default:
			t.Fatalf("unexpected source: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(value)), Header: make(http.Header)}, nil
	})
	first := model.Port{Kind: "custom", Name: "first", URL: firstURL, SyntaxURL: syntaxURL}
	if err := Adapter().Apply(first, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	flavor := config["flavor"].(map[string]any)
	if flavor["dark"] != "" || flavor["light"] != "" || flavor["use"] != nil {
		t.Fatalf("remote styles left the previous flavor selected: %#v", flavor)
	}
	cwd := mgr["cwd"].(map[string]any)
	cwd["fg"] = "#abcdef" // a user override must survive the next theme
	encoded, err := toml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := shared.Atomic(path, encoded); err != nil {
		t.Fatal(err)
	}
	second := model.Port{Kind: "custom", Name: "second", URL: secondURL, SyntaxURL: syntaxURL}
	if err := Adapter().Apply(second, env); err != nil {
		t.Fatal(err)
	}
	config, err = shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr = config["mgr"].(map[string]any)
	cwd = mgr["cwd"].(map[string]any)
	if cwd["fg"] != "#abcdef" || cwd["bg"] != "#445566" || cwd["italic"] != true || mgr["ratio"] == nil || config["status"] != nil {
		t.Fatalf("remote transition retained stale styles or lost user settings: %#v", config)
	}
	if mgr["syntect_theme"] != filepath.Join(filepath.Dir(path), "themeset-custom-second.tmTheme") {
		t.Fatalf("previous syntax still selected: %#v", mgr)
	}
}
