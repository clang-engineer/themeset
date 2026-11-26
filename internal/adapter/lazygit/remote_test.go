package lazygit

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestLazygitRemoteCustomTheme(t *testing.T) {
	if !Adapter().Remote {
		t.Fatal("lazygit must advertise remote custom themes")
	}
	env := testutil.Env(t)
	path := shared.Path("lazygit", env)
	const original = "gui:\n  mouseEvents: false\n  theme:\n    inactiveBorderColor: [blue]\ngit:\n  autoFetch: false\n"
	testutil.Write(t, path, original)
	const url = "https://example.com/lazygit.yml"
	port := model.Port{Kind: "custom", URL: url, Source: "theme's creator"}
	body := "gui:\n  theme:\n    activeBorderColor: [red, bold]\n"
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != url {
			t.Fatalf("unexpected theme URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	body = "gui:\n  mouseEvents: true\ngit:\n  autoFetch: true\n"
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("accepted remote file without a gui.theme mapping")
	}
	if string(testutil.Read(t, path)) != original {
		t.Fatal("invalid theme modified lazygit settings")
	}
	body = "gui:\n  theme:\n    activeBorderColor: [red, bold]\n  mouseEvents: true\n"
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.YAML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	gui := config["gui"].(map[string]any)
	theme := gui["theme"].(map[string]any)
	if gui["mouseEvents"] != false || config["git"].(map[string]any)["autoFetch"] != false || theme["inactiveBorderColor"] == nil || theme["activeBorderColor"] == nil {
		t.Fatalf("remote lazygit theme did not preserve settings: %#v", config)
	}
}
