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

// Cyberdream's upstream Yazi file includes a relative bat syntax path; the
// separate TextMate extra, not that example path, must become Yazi's syntax.
func TestRemoteCyberdreamYaziUsesTextmateExtraAndPreservesSettings(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\nuse = 'previous'\ndark = 'previous'\nlight = 'previous'\n[mgr]\nratio = [1, 3, 4]\ncwd = { italic = true }\n[status]\nperm_read = { bg = '#123456' }\n[filetype]\nrules = [{ mime = 'text/*', fg = '#abcdef' }]\n")
	neighbor := filepath.Join(filepath.Dir(path), "yazi.toml")
	testutil.Write(t, neighbor, "[mgr]\nshow_hidden = true\n")
	const themeURL = "https://raw.githubusercontent.com/scottmckendry/cyberdream.nvim/main/extras/yazi/cyberdream.toml"
	const syntaxURL = "https://raw.githubusercontent.com/scottmckendry/cyberdream.nvim/main/extras/textmate/cyberdream.tmTheme"
	// Representative upstream sections and values, including settings that
	// must not be imported from a remote theme.
	const theme = "[mgr]\nsyntect_theme = '../bat/themes/cyberdream.tmTheme'\nborder_style = { fg = '#3c4048' }\ncwd = { fg = '#5ef1ff' }\n[cmp]\nactive = { bg = '#7b8496', fg = '#bd5eff' }\n[tabs]\nactive = { bg = '#5ea1ff', fg = '#16181a' }\n[status]\nperm_read = { fg = '#f1ff5e' }\n[filetype]\nrules = [{ fg = '#5ef1ff', mime = 'image/*' }]\n"
	const syntax = "<?xml version=\"1.0\"?><plist version=\"1.0\"><dict><key>name</key><string>Cyberdream</string><key>settings</key><array><dict><key>settings</key><dict><key>background</key><string>#16181a</string></dict></dict></array></dict></plist>"
	requests := map[string]int{}
	testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
		requests[req.URL.String()]++
		var data string
		switch req.URL.String() {
		case themeURL:
			data = theme
		case syntaxURL:
			data = syntax
		default:
			t.Fatalf("unexpected Yazi source (including upstream relative syntax path): %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(data)), Header: make(http.Header)}, nil
	})
	port := model.Port{Kind: "custom", Name: "cyberdream", URL: themeURL, SyntaxURL: syntaxURL}
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	flavor := config["flavor"].(map[string]any)
	status := config["status"].(map[string]any)["perm_read"].(map[string]any)
	tabs := config["tabs"].(map[string]any)["active"].(map[string]any)
	syntaxPath := filepath.Join(filepath.Dir(path), "themeset-custom-cyberdream.tmTheme")
	if mgr["syntect_theme"] != syntaxPath || mgr["border_style"].(map[string]any)["fg"] != "#3c4048" ||
		mgr["cwd"].(map[string]any)["fg"] != "#5ef1ff" || mgr["cwd"].(map[string]any)["italic"] != true ||
		tabs["bg"] != "#5ea1ff" || status["fg"] != "#f1ff5e" || status["bg"] != "#123456" {
		t.Fatalf("Cyberdream UI styles or TextMate activation missing: %#v", config)
	}
	if flavor["use"] != nil || flavor["dark"] != "" || flavor["light"] != "" ||
		mgr["ratio"] == nil || config["cmp"] != nil ||
		config["filetype"].(map[string]any)["rules"].([]any)[0].(map[string]any)["fg"] != "#abcdef" {
		t.Fatalf("remote theme imported unrelated settings or discarded user configuration: %#v", config)
	}
	if string(testutil.Read(t, syntaxPath)) != syntax || string(testutil.Read(t, neighbor)) != "[mgr]\nshow_hidden = true\n" ||
		requests[themeURL] != 1 || requests[syntaxURL] != 1 || len(requests) != 2 {
		t.Fatalf("TextMate asset or neighboring settings were changed: requests=%v", requests)
	}
}
