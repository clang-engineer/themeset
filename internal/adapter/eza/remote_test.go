package eza

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestEzaRemoteCustomTheme(t *testing.T) {
	if !Adapter().Remote {
		t.Fatal("eza must advertise remote custom themes")
	}
	env := testutil.Env(t)
	path := shared.Path("eza", env)
	const original = "filekinds:\n  directory: {is_bold: true}\nfilenames:\n  Makefile: {icon: {glyph: KEEP}}\n"
	testutil.Write(t, path, original)
	const url = "https://example.com/theme.yml"
	port := model.Port{Kind: "custom", URL: url, Source: "the author's page"}
	body := "filekinds:\n  directory: {foreground: '#112233'}\n"
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != url {
			t.Fatalf("unexpected theme URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	for _, invalid := range []string{"gui: {theme: {activeBorderColor: [red]}}\n", "filekinds: {directory: {foreground: false}}\n", "colourful: true\n", "filekinds: {directory: {foreground: red, shell: 'touch /tmp/untrusted'}}\n", "filekinds: {directory: {foreground: not-a-color}}\n", "filenames:\n  Makefile: {icon: {glyph: \"\\x1b[31m\"}}\n"} {
		body = invalid
		if err := Adapter().Apply(port, env); err == nil {
			t.Fatalf("accepted non-native eza theme: %s", invalid)
		}
		if string(testutil.Read(t, path)) != original {
			t.Fatal("invalid remote theme modified eza settings")
		}
	}
	body = "filekinds:\n  directory: {foreground: '#112233'}\n"
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.YAML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	style := config["filekinds"].(map[string]any)["directory"].(map[string]any)
	if style["foreground"] != "#112233" || style["is_bold"] != true || !strings.Contains(string(testutil.Read(t, path)), "KEEP") {
		t.Fatalf("native eza theme did not preserve existing settings: %#v", config)
	}
}
