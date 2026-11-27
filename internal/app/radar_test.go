package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestRadarRefreshKeepsFullCategoryAndOfflineCache(t *testing.T) {
	var bundled radarData
	if err := json.Unmarshal(radarSnapshot, &bundled); err != nil || !validRadar(bundled) {
		t.Fatalf("bundled snapshot invalid: %v", err)
	}
	if got := ranksFor(bundled).rank("catppuccin-latte", model.Config{}); got != 2 {
		t.Fatalf("family rank overwritten by a port repository: %d", got)
	}
	ranks := ranksFor(bundled)
	if ranks.rank("rose-pine", model.Config{}) != 32 ||
		ranks.rank("moonfly", model.Config{}) != 38 || ranks.rank("ayu-light", model.Config{}) != 61 {
		t.Fatalf("bundled ranking missing supported themes beyond top 30")
	}
	for _, id := range catalog.Listed() {
		if id != "vscode-dark" && ranks.rank(id, model.Config{}) == 0 {
			t.Fatalf("listed theme %q lacks a Radar project association", id)
		}
	}
	items := append([]string(nil), bundled.Items...)
	items[0], items[1] = items[1], items[0]
	const updated = "2027-01-01T00:00:00Z"
	feed := struct {
		UpdatedAt  string `json:"updatedAt"`
		Categories []struct {
			ID     string `json:"id"`
			Metric string `json:"metric"`
			Items  []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"categories"`
	}{UpdatedAt: updated}
	feed.Categories = append(feed.Categories, struct {
		ID     string `json:"id"`
		Metric string `json:"metric"`
		Items  []struct {
			ID string `json:"id"`
		} `json:"items"`
	}{ID: radarCategory, Metric: "stars"})
	for _, id := range items {
		feed.Categories[0].Items = append(feed.Categories[0].Items, struct {
			ID string `json:"id"`
		}{id})
	}
	body, err := json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write(body)
	}))
	oldSource, oldClient := radarSource, radarClient
	radarSource, radarClient = server.URL, server.Client()
	t.Cleanup(func() { radarSource, radarClient = oldSource, oldClient; server.Close() })
	path := filepath.Join(t.TempDir(), "cache", "ranks.json")
	fresh := refreshRadar(path, bundled, false)
	if fresh.UpdatedAt != updated || requests != 1 || ranksFor(fresh).rank("catppuccin-mocha", model.Config{}) != 1 {
		t.Fatalf("new ranking not adopted: %+v requests=%d", fresh, requests)
	}
	if got := refreshRadar(path, fresh, true); got.UpdatedAt != updated || requests != 1 {
		t.Fatalf("fresh cache caused another fetch: %+v requests=%d", got, requests)
	}
	if err := os.Chtimes(path, time.Now().Add(-48*time.Hour), time.Now().Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if got := refreshRadar(path, fresh, true); got.UpdatedAt != updated {
		t.Fatalf("offline refresh discarded last known ranking: %+v", got)
	}
	if data, err := decodeRadarFeed(bytes.NewReader([]byte(`{"updatedAt":"2027-01-02T00:00:00Z","categories":[]}`))); err == nil || len(data.Items) != 0 {
		t.Fatalf("missing category accepted: %+v, %v", data, err)
	}
	wrongPeriod := append([]byte(`{"defaultPeriod":7,`), body[1:]...)
	if _, err := decodeRadarFeed(bytes.NewReader(wrongPeriod)); err == nil {
		t.Fatal("non-all-time ranking accepted as cumulative ranks")
	}
}

func TestBundledRadarSupersedesSameDateShortCache(t *testing.T) {
	var bundled radarData
	if err := json.Unmarshal(radarSnapshot, &bundled); err != nil {
		t.Fatal(err)
	}
	old := radarData{UpdatedAt: bundled.UpdatedAt, Items: bundled.Items[:30]}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	fixtureFile(t, filepath.Join(home, ".cache", "themeset", "radar-ranks.json"), string(data))
	if got := loadRadar(model.Environment{Home: home}, false); len(got.Items) != len(bundled.Items) {
		t.Fatalf("short cache masked full bundled category: %d entries", len(got.Items))
	}
}

func TestRadarSortingPreservesGapsAndCustomOrder(t *testing.T) {
	var bundled radarData
	if err := json.Unmarshal(radarSnapshot, &bundled); err != nil {
		t.Fatal(err)
	}
	ranks := ranksFor(bundled)
	config := model.Config{Themes: map[string]model.Theme{
		"tomorrow": {RadarRepo: "chriskempson/tomorrow-theme"},
	}}
	ids := []string{"unranked-a", "tokyo-night", "tomorrow", "catppuccin-latte", "unranked-b", "dracula", "catppuccin-mocha"}
	if got := sortedThemes(ids, "rank", config, ranks); !reflect.DeepEqual(got, []string{"dracula", "catppuccin-latte", "catppuccin-mocha", "tomorrow", "tokyo-night", "unranked-a", "unranked-b"}) {
		t.Fatalf("rank sorting lost gaps, ties or custom metadata: %q", got)
	}
	if got := sortedThemes(ids, "listed", config, ranks); !reflect.DeepEqual(got, ids) {
		t.Fatalf("listed order changed: %q", got)
	}
	if got := sortedThemes(ids, "name", config, ranks); !reflect.DeepEqual(got, []string{"catppuccin-latte", "catppuccin-mocha", "dracula", "tokyo-night", "tomorrow", "unranked-a", "unranked-b"}) {
		t.Fatalf("name sorting failed: %q", got)
	}
	if ranks.rank("rose-pine", config) != 32 || ranks.rank("rose-pine-dawn", config) != 32 ||
		ranks.rank("moonfly", config) != 38 || ranks.rank("ayu-light", config) != 61 ||
		ranks.rank("cyberdream", config) != 0 || ranks.rank("gruvbox-dark", config) != 0 ||
		ranks.rank("vscode-dark", config) != 0 {
		t.Fatal("listed rank, hidden theme or unrelated VS Code port received an incorrect rank")
	}
	config.Themes["dracula"] = model.Theme{RadarRepo: "rose-pine/rose-pine-theme"}
	if got := ranks.rank("dracula", config); got != 32 {
		t.Fatalf("custom project association did not override bundled rank: %d", got)
	}
	if !strings.Contains(bundled.UpdatedAt, "2026-11-27") {
		t.Fatal("bundled provenance absent")
	}
}
