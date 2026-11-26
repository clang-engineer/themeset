package adapter

import (
	"fmt"
	"github.com/clang-engineer/themeset/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetAcquisitionFetchesCurrentContents(t *testing.T) {
	content := "first"
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, content)
	}))
	defer server.Close()
	first, err := filesAcquireAssetWithClient(model.Environment{}, server.URL, server.Client())
	if err != nil || string(first) != "first" {
		t.Fatalf("first acquisition: %q %v", first, err)
	}
	content = "second"
	second, err := filesAcquireAssetWithClient(model.Environment{}, server.URL, server.Client())
	if err != nil || string(second) != "second" || requests != 2 {
		t.Fatalf("latest source not fetched: %q %v requests=%d", second, err, requests)
	}
}

func TestAssetAcquisitionRejectsUnsafeOrFailedDownloads(t *testing.T) {
	if _, err := filesAcquireAssetWithClient(model.Environment{}, "http://example.com/theme", http.DefaultClient); err == nil {
		t.Fatal("non-HTTPS source accepted")
	}
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{"not-found", 404, "not found"},
		{"oversize", 200, strings.Repeat("x", filesAssetMaxBytes+1)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(scenario.status)
				fmt.Fprint(w, scenario.body)
			}))
			defer server.Close()
			if _, err := filesAcquireAssetWithClient(model.Environment{}, server.URL, server.Client()); err == nil {
				t.Fatal("failed download accepted")
			}
		})
	}
}
