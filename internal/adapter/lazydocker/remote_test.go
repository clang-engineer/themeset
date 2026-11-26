package lazydocker

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestLazydockerRemoteCustomTheme(t *testing.T) {
	if !Adapter().Remote {
		t.Fatal("lazydocker must advertise remote custom themes")
	}
	env := testutil.Env(t)
	path := shared.Path("lazydocker", env)
	const original = "gui:\n  scrollHeight: 9\n  theme:\n    optionsTextColor: [blue]\nlogs:\n  tail: '200'\n"
	testutil.Write(t, path, original)
	const url = "https://example.com/lazydocker.yml"
	port := model.Port{Kind: "custom", URL: url, Source: "theme author"}
	body := "gui:\n  theme:\n    activeBorderColor: [red]\n"
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != url {
			t.Fatalf("unexpected theme URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	body = "gui:\n  theme: {}\n  scrollHeight: 999\n"
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("accepted remote file without a nonempty gui.theme mapping")
	}
	if string(testutil.Read(t, path)) != original {
		t.Fatal("invalid theme modified lazydocker settings")
	}
	body = "gui:\n  theme:\n    activeBorderColor: [red]\n  scrollHeight: 999\n"
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.YAML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	gui := config["gui"].(map[string]any)
	theme := gui["theme"].(map[string]any)
	if gui["scrollHeight"] != 9 || config["logs"].(map[string]any)["tail"] != "200" || theme["optionsTextColor"] == nil || theme["activeBorderColor"] == nil {
		t.Fatalf("remote lazydocker theme did not preserve settings: %#v", config)
	}
}
