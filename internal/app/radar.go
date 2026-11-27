package app

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

const radarURL = "https://clang-engineer.github.io/data/rankings.json"
const radarCategory = "developer-color-themes"

//go:embed radar_snapshot.json
var radarSnapshot []byte

type radarItem struct {
	ID        string `json:"id"`
	Stars     int    `json:"stars"`
	CreatedAt string `json:"createdAt,omitempty"`
	PushedAt  string `json:"pushedAt,omitempty"`
}

type radarData struct {
	UpdatedAt string      `json:"updatedAt"`
	Items     []radarItem `json:"items"`
}

type radarFeed struct {
	UpdatedAt     string `json:"updatedAt"`
	DefaultPeriod int    `json:"defaultPeriod"`
	Categories    []struct {
		ID     string      `json:"id"`
		Metric string      `json:"metric"`
		Items  []radarItem `json:"items"`
	} `json:"categories"`
}

func validRadar(data radarData) bool {
	if _, err := time.Parse(time.RFC3339Nano, data.UpdatedAt); err != nil || len(data.Items) == 0 {
		return false
	}
	seen := make(map[string]bool, len(data.Items))
	for _, item := range data.Items {
		if !validRadarRepo(item.ID) || item.Stars <= 0 || seen[strings.ToLower(item.ID)] ||
			!validRadarDate(item.CreatedAt) || !validRadarDate(item.PushedAt) {
			return false
		}
		seen[strings.ToLower(item.ID)] = true
	}
	return true
}

// Old rank caches contain no project dates; absent dates remain unknown.
func validRadarDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func validRadarRepo(id string) bool {
	owner, repo, ok := strings.Cut(id, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return false
	}
	for _, r := range id {
		if r != '/' && r != '-' && r != '_' && r != '.' && (r < '0' || r > '9') && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func decodeRadarFeed(reader io.Reader) (radarData, error) {
	var feed radarFeed
	if err := json.NewDecoder(io.LimitReader(reader, 4<<20)).Decode(&feed); err != nil {
		return radarData{}, err
	}
	if feed.DefaultPeriod != 0 {
		return radarData{}, errors.New("Radar ranking no longer defaults to all-time")
	}
	for _, category := range feed.Categories {
		if category.ID != radarCategory || category.Metric != "stars" || len(category.Items) == 0 {
			continue
		}
		data := radarData{UpdatedAt: feed.UpdatedAt, Items: category.Items}
		if validRadar(data) {
			return data, nil
		}
	}
	return radarData{}, errors.New("Radar color-theme ranking is unavailable or invalid")
}

var radarClient = &http.Client{Timeout: 3 * time.Second}
var radarSource = radarURL

func loadRadar(env model.Environment, refresh bool) radarData {
	var bundled radarData
	if err := json.Unmarshal(radarSnapshot, &bundled); err != nil || !validRadar(bundled) {
		panic("invalid bundled Radar snapshot")
	}
	path := filepath.Join(env.Home, ".cache", "themeset", "radar-ranks.json")
	current := bundled
	cacheCurrent := false
	if content, err := os.ReadFile(path); err == nil {
		var cached radarData
		if json.Unmarshal(content, &cached) == nil && validRadar(cached) {
			bundledTime, _ := time.Parse(time.RFC3339Nano, bundled.UpdatedAt)
			cachedTime, _ := time.Parse(time.RFC3339Nano, cached.UpdatedAt)
			if !cachedTime.Before(bundledTime) && (!cachedTime.Equal(bundledTime) ||
				len(cached.Items) > len(bundled.Items) ||
				len(cached.Items) == len(bundled.Items) && (bundled.Items[0].CreatedAt == "" || cached.Items[0].CreatedAt != "")) {
				current, cacheCurrent = cached, true
			}
		}
	}
	if !refresh {
		return current
	}
	account, err := user.Current()
	if err != nil || filepath.Clean(env.Home) != filepath.Clean(account.HomeDir) {
		return current
	}
	return refreshRadar(path, current, cacheCurrent)
}

func refreshRadar(path string, current radarData, cacheCurrent bool) radarData {
	if stat, err := os.Stat(path); cacheCurrent && err == nil && time.Since(stat.ModTime()) < 24*time.Hour {
		return current
	}
	response, err := radarClient.Get(radarSource)
	if err != nil {
		return current
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return current
	}
	fresh, err := decodeRadarFeed(response.Body)
	if err != nil {
		return current
	}
	oldTime, _ := time.Parse(time.RFC3339Nano, current.UpdatedAt)
	newTime, _ := time.Parse(time.RFC3339Nano, fresh.UpdatedAt)
	if newTime.Before(oldTime) || (newTime.Equal(oldTime) && len(fresh.Items) < len(current.Items)) {
		return current
	}
	if data, err := json.Marshal(fresh); err == nil {
		_ = shared.Atomic(path, data)
	}
	return fresh
}

type radarStanding struct {
	Rank      int
	Stars     int
	CreatedAt string
	PushedAt  string
}

type radarRanks struct {
	UpdatedAt string
	byRepo    map[string]radarStanding
}

func ranksFor(data radarData) radarRanks {
	ranks := radarRanks{UpdatedAt: data.UpdatedAt, byRepo: make(map[string]radarStanding, len(data.Items))}
	for i, item := range data.Items {
		ranks.byRepo[strings.ToLower(item.ID)] = radarStanding{
			Rank: i + 1, Stars: item.Stars, CreatedAt: item.CreatedAt, PushedAt: item.PushedAt,
		}
	}
	return ranks
}

func (r radarRanks) standing(theme string, config model.Config) radarStanding {
	repo := config.Themes[theme].RadarRepo
	if repo == "" {
		repo = catalog.RadarRepo(theme)
	}
	return r.byRepo[strings.ToLower(repo)]
}

func (r radarRanks) rank(theme string, config model.Config) int {
	return r.standing(theme, config).Rank
}

func (r radarRanks) stars(theme string, config model.Config) int {
	return r.standing(theme, config).Stars
}
func projectDate(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	return timestamp[:10]
}

func formatStars(value int) string {
	var digits [32]byte
	index := len(digits)
	for count := 0; ; count++ {
		if count > 0 && count%3 == 0 {
			index--
			digits[index] = ','
		}
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
		if value == 0 {
			return string(digits[index:])
		}
	}
}

func discoverySort(config model.Config, override string) string {
	if override != "" {
		return override
	}
	if config.ThemeSort != "" {
		return config.ThemeSort
	}
	if config.Listed != nil {
		return "listed"
	}
	return "rank"
}

func sortedThemes(ids []string, mode string, config model.Config, ranks radarRanks) []string {
	if mode == "listed" {
		return ids
	}
	ordered := append([]string(nil), ids...)
	if mode == "name" {
		sort.Strings(ordered)
		return ordered
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ranks.rank(ordered[i], config), ranks.rank(ordered[j], config)
		if a == 0 {
			a = len(ranks.byRepo) + 1
		}
		if b == 0 {
			b = len(ranks.byRepo) + 1
		}
		return a < b
	})
	return ordered
}

func themeSort(mode string) error {
	switch mode {
	case "", "listed", "name", "rank":
		return nil
	}
	return fmt.Errorf("theme_sort/--sort must be listed, name, or rank (got %q)", mode)
}
