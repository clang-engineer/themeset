package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func fixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixtureConfig(t *testing.T, path string, config any) {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, path, string(data))
}

// Only executable discovery is simulated. All resolution and application use
// the production adapters and real config/theme files; no tool is launched.
func installedFixtures(t *testing.T, binaries ...string) {
	t.Helper()
	bin := t.TempDir()
	for _, name := range binaries {
		path := filepath.Join(bin, name)
		fixtureFile(t, path, "#!/bin/sh\nexit 99\n")
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

func invoke(t *testing.T, args ...string) (int, Result, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append(args, "--json"), &stdout, &stderr)
	var result Result
	if stdout.Len() != 0 {
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode output: %v: %s", err, stdout.String())
		}
	}
	return code, result, stderr.String()
}

func TestConfiguredFallbackCustomPortThroughCLI(t *testing.T) {
	installedFixtures(t, "tmux")
	home, directory := t.TempDir(), t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
	fixtureFile(t, filepath.Join(home, ".tmux.conf"), "set -g mouse on\n")
	fixtureConfig(t, configPath, map[string]any{
		"tools": []string{"tmux"}, "default_theme": "personal",
		"themes": map[string]any{"personal": map[string]any{"custom": map[string]any{
			"tmux": map[string]string{"file": "theme.conf"},
		}}},
	})
	code, result, diagnostics := invoke(t, "apply", "nord", "--home", home, "--config", configPath)
	if code != 0 || len(result.Tools) != 1 || result.Tools[0].FallbackTheme != "personal" ||
		result.Tools[0].Outcome != "applied" || !strings.Contains(diagnostics, "personal") {
		t.Fatalf("configured fallback: code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
	data, err := os.ReadFile(filepath.Join(home, ".tmux.conf"))
	if err != nil || !strings.Contains(string(data), filepath.Join(directory, "theme.conf")) {
		t.Fatalf("fallback did not update tmux: %q, %v", data, err)
	}
}

func TestDefaultThemeFallbackSelectionAndApplication(t *testing.T) {
	installedFixtures(t, "fixture")
	destination := filepath.Join(t.TempDir(), "applied")
	custom := filepath.Join(t.TempDir(), "fallback.theme")
	fixtureFile(t, custom, "theme")
	env := model.Environment{}
	resolveCalls := 0
	adapter := model.Adapter{
		ID: "fixture", Binary: "fixture",
		Resolve: func(declaration model.ThemePort, _ model.Environment) (model.Port, bool, error) {
			resolveCalls++
			if len(declaration.Native) != 0 {
				return model.Port{Kind: "native", Name: declaration.Native[0]}, true, nil
			}
			return model.Port{}, false, nil
		},
		Apply: func(port model.Port, _ model.Environment) error {
			return os.WriteFile(destination, []byte(port.File), 0600)
		},
	}
	config := model.Config{DefaultTheme: "personal", Themes: map[string]model.Theme{
		"personal": {Custom: map[string]model.Port{"fixture": {File: custom}}},
	}}
	result := plan("requested", config, env, []model.Adapter{adapter}, false)
	item := result.Tools[0]
	if item.Status != "custom" || item.FallbackTheme != "personal" || item.Port == nil || item.Port.File != custom ||
		len(item.Warnings) == 0 || !strings.Contains(item.Warnings[0], "personal") || resolveCalls != 1 {
		t.Fatalf("fallback selection: %+v; resolver calls=%d", item, resolveCalls)
	}
	apply(&result, env, []model.Adapter{adapter})
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != custom || result.Tools[0].Outcome != "applied" {
		t.Fatalf("fallback not applied: %+v, %q, %v", result.Tools[0], data, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(encoded), `"fallback_theme":"personal"`) {
		t.Fatalf("fallback missing from JSON: %s, %v", encoded, err)
	}
	config.Themes["requested"] = model.Theme{Custom: map[string]model.Port{"fixture": {Name: "requested-override"}}}
	result = plan("requested", config, env, []model.Adapter{adapter}, false)
	if result.Tools[0].Status != "custom" || result.Tools[0].Port.Name != "requested-override" ||
		result.Tools[0].FallbackTheme != "" || resolveCalls != 1 {
		t.Fatalf("requested override lost precedence: %+v", result.Tools[0])
	}
}

func TestDefaultThemeFallbackOnlyOnUnavailablePort(t *testing.T) {
	installedFixtures(t, "fixture")
	inspectionError := errors.New("native inspection failed")
	calls := 0
	adapter := model.Adapter{ID: "fixture", Binary: "fixture", Resolve: func(model.ThemePort, model.Environment) (model.Port, bool, error) {
		calls++
		return model.Port{}, false, inspectionError
	}}
	config := model.Config{DefaultTheme: "personal", Themes: map[string]model.Theme{
		"personal": {Custom: map[string]model.Port{"fixture": {Name: "fallback"}}},
	}}
	result := plan("requested", config, model.Environment{}, []model.Adapter{adapter}, false)
	if item := result.Tools[0]; item.Status != "error" || item.FallbackTheme != "" || calls != 1 {
		t.Fatalf("inspection error incorrectly fell back: %+v; calls=%d", item, calls)
	}
	adapter.Resolve = func(model.ThemePort, model.Environment) (model.Port, bool, error) {
		calls++
		return model.Port{}, false, nil
	}
	config.DefaultTheme = "missing"
	result = plan("requested", config, model.Environment{}, []model.Adapter{adapter}, false)
	if item := result.Tools[0]; item.Status != "unsupported" || item.Outcome != "skipped" ||
		item.FallbackTheme != "" || item.Port != nil || calls != 3 {
		t.Fatalf("missing fallback incorrectly selected: %+v; calls=%d", item, calls)
	}
	config.DefaultTheme = "personal"
	config.Themes["personal"] = model.Theme{Custom: map[string]model.Port{"fixture": {File: filepath.Join(t.TempDir(), "missing.theme")}}}
	result = plan("requested", config, model.Environment{}, []model.Adapter{adapter}, false)
	if item := result.Tools[0]; item.Status != "error" || item.FallbackTheme != "personal" ||
		!strings.Contains(item.Error, "default theme personal") || len(item.Warnings) != 0 {
		t.Fatalf("invalid fallback should report an error without claiming it was applied: %+v", item)
	}
}

func TestCustomPrecedesNativeAndInvalidOverrideDoesNotFallBack(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(map[bool]string{true: "valid", false: "missing"}[valid], func(t *testing.T) {
			installedFixtures(t, "nvim")
			home, directory := t.TempDir(), t.TempDir()
			config := filepath.Join(directory, "config.json")
			init := filepath.Join(home, ".config", "nvim", "init.lua")
			fixtureFile(t, init, "vim.opt.number = true\n")
			fixtureFile(t, filepath.Join(home, ".config", "nvim", "colors", "dracula.vim"), "hi Normal ctermfg=7\n")
			if valid {
				fixtureFile(t, filepath.Join(directory, "override.lua"), "vim.g.custom_theme = true\n")
			}
			fixtureConfig(t, config, map[string]any{
				"tools": []string{"neovim"},
				"themes": map[string]any{"dracula": map[string]any{"custom": map[string]any{
					"neovim": map[string]string{"file": "override.lua"},
				}}},
			})
			code, result, diagnostics := invoke(t, "--home", home, "apply", "dracula", "--config", config)
			destination := filepath.Join(home, ".config", "nvim", "plugin", "themeset.lua")
			if valid {
				if code != 0 || result.Tools[0].Status != "custom" || result.Tools[0].Outcome != "applied" {
					t.Fatalf("override not applied: code=%d result=%+v diagnostics=%s", code, result, diagnostics)
				}
				data, err := os.ReadFile(destination)
				if err != nil || !strings.Contains(string(data), filepath.Join(directory, "override.lua")) || strings.Contains(string(data), "runtimepath:prepend") {
					t.Fatalf("native theme replaced custom selection: %q, %v", data, err)
				}
			} else {
				if code != 1 || result.Tools[0].Outcome != "failed" || result.Tools[0].Port != nil {
					t.Fatalf("invalid override fell back: code=%d result=%+v", code, result)
				}
				if _, err := os.Stat(destination); !os.IsNotExist(err) {
					t.Fatalf("invalid override changed native selection: %v", err)
				}
			}
			data, err := os.ReadFile(init)
			if err != nil || string(data) != "vim.opt.number = true\n" {
				t.Fatalf("init config changed: %q, %v", data, err)
			}
		})
	}
}

func TestPartialFailureRecordsActualOutcomes(t *testing.T) {
	installedFixtures(t, "bat", "tmux")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "bad-bat.conf"), "--style=numbers\n")
	fixtureFile(t, filepath.Join(directory, "tmux-theme.conf"), "set -g status-style bg=blue\n")
	fixtureFile(t, filepath.Join(home, ".tmux.conf"), "set -g mouse on\n")
	fixtureFile(t, filepath.Join(home, ".config", "bat", "config"), "--style=plain\n")
	fixtureConfig(t, config, map[string]any{
		"tools": []string{"tmux", "bat"},
		"themes": map[string]any{"fixture": map[string]any{"custom": map[string]any{
			"bat":  map[string]string{"kind": "native", "file": "bad-bat.conf"},
			"tmux": map[string]string{"file": "tmux-theme.conf"},
		}}},
	})
	code, result, diagnostics := invoke(t, "apply", "fixture", "--config", config, "--home", home)
	if code != 1 || len(result.Tools) != 2 || result.Tools[0].Tool != "bat" || result.Tools[0].Outcome != "failed" || result.Tools[1].Outcome != "applied" {
		t.Fatalf("code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
	if result.Tools[0].Port.Kind != "custom" || result.Tools[1].Status != "custom" || !strings.Contains(diagnostics, "error: bat") {
		t.Fatalf("custom kinds/diagnostics: %+v, %s", result, diagnostics)
	}
	data, err := os.ReadFile(filepath.Join(home, ".tmux.conf"))
	if err != nil || !strings.Contains(string(data), "set -g mouse on") || !strings.Contains(string(data), filepath.Join(directory, "tmux-theme.conf")) {
		t.Fatalf("other tool did not progress preserving settings: %q, %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(home, ".config", "bat", "config"))
	if err != nil || string(data) != "--style=plain\n" {
		t.Fatalf("failed tool changed: %q, %v", data, err)
	}
	currentCode, current, _ := invoke(t, "current", "--home", home)
	if currentCode != 0 || !reflect.DeepEqual(current, result) {
		t.Fatalf("current lost actual result: %+v vs %+v", current, result)
	}
}

func TestDryRunNeverWrites(t *testing.T) {
	installedFixtures(t, "tmux")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
	fixtureConfig(t, config, map[string]any{"tools": []string{"tmux"}, "themes": map[string]any{
		"fixture": map[string]any{"custom": map[string]any{"tmux": map[string]string{"file": "theme.conf"}}},
	}})
	code, result, diagnostics := invoke(t, "apply", "fixture", "--dry-run", "--home", home, "--config", config)
	if code != 0 || !result.DryRun || len(result.Tools) != 1 || result.Tools[0].Outcome != "planned" {
		t.Fatalf("code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("inspection created home files: %v, %v", entries, err)
	}
}

func TestEmptyToolsAndOverrideDeduplication(t *testing.T) {
	installedFixtures(t, "tmux")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
	fixtureConfig(t, config, map[string]any{"tools": []string{}, "themes": map[string]any{
		"fixture": map[string]any{"custom": map[string]any{"tmux": map[string]string{"file": "theme.conf"}}},
	}})
	code, result, diagnostics := invoke(t, "apply", "fixture", "--home", home, "--config", config)
	if code != 0 || result.Tools == nil || len(result.Tools) != 0 {
		t.Fatalf("explicit [] not preserved: %d %+v %s", code, result, diagnostics)
	}
	if _, err := os.Stat(filepath.Join(home, ".tmux.conf")); !os.IsNotExist(err) {
		t.Fatalf("empty selection wrote app config: %v", err)
	}
	code, result, diagnostics = invoke(t, "apply", "fixture", "--tools", "tmux,tmux", "--home", home, "--config", config)
	if code != 0 || len(result.Tools) != 1 || result.Tools[0].Outcome != "applied" {
		t.Fatalf("override/deduplication failed: %d %+v %s", code, result, diagnostics)
	}
}

func TestOmittedToolsSelectAllAndMissingAppsSkip(t *testing.T) {
	installedFixtures(t)
	home := t.TempDir()
	code, result, diagnostics := invoke(t, "apply", "dracula", "--home", home)
	if code != 0 || len(result.Tools) != len(adapter.All()) {
		t.Fatalf("omitted tools did not select all: %d %+v %s", code, result, diagnostics)
	}
	for _, item := range result.Tools {
		if item.Status != "not-installed" || item.Outcome != "skipped" {
			t.Fatalf("missing app not skipped: %+v", item)
		}
	}
}

func TestSkippedToolsExplainUnchangedSettings(t *testing.T) {
	installedFixtures(t, "tmux")
	home := t.TempDir()
	code, result, diagnostics := invoke(t, "apply", "nord", "--dry-run", "--home", home, "--tools", "tmux,lazydocker")
	if code != 0 || len(result.Tools) != 2 {
		t.Fatalf("code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
	if result.Tools[0].Status != "not-installed" || !strings.Contains(result.Tools[0].Reason, "lazydocker") {
		t.Fatalf("missing executable not explained: %+v", result.Tools[0])
	}
	if result.Tools[1].Status != "unsupported" || !strings.Contains(result.Tools[1].Reason, "status layout") {
		t.Fatalf("unsafe plugin not distinguished from absent binary: %+v", result.Tools[1])
	}
	var stdout, stderr bytes.Buffer
	if printResult(&stdout, &stderr, result) != 0 || !strings.Contains(stdout.String(), result.Tools[1].Reason) {
		t.Fatalf("user-facing skip reason missing: %s / %s", stdout.String(), stderr.String())
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("dry run changed settings: %v, %v", entries, err)
	}
}

func TestInvalidSelectedToolRejectsBeforeWrites(t *testing.T) {
	installedFixtures(t, "tmux")
	home := t.TempDir()
	code, _, diagnostics := invoke(t, "apply", "dracula", "--home", home, "--tools", "tmux,nvim")
	if code == 0 || !strings.Contains(diagnostics, "unknown tool") {
		t.Fatalf("invalid tool accepted: %d %s", code, diagnostics)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid selection wrote files: %v, %v", entries, err)
	}
}

func TestStrictConfigErrors(t *testing.T) {
	for _, contents := range []string{
		`{"unknown":true}`,
		`{"themes":{"fixture":{"custom":{"tmux":{"command":"unsafe"}}}}}`,
		`{"tools":"tmux"}`,
		`{"tools":[]} {}`,
		`null`,
		`{`,
	} {
		t.Run(contents, func(t *testing.T) {
			home, directory := t.TempDir(), t.TempDir()
			config := filepath.Join(directory, "config.json")
			fixtureFile(t, config, contents)
			code, _, diagnostics := invoke(t, "apply", "dracula", "--home", home, "--config", config)
			if code == 0 || !strings.Contains(diagnostics, "config") {
				t.Fatalf("config error missing: %d %s", code, diagnostics)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatalf("bad config wrote files: %v, %v", entries, err)
			}
		})
	}
}

func TestHomeIgnoresInheritedXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	home := t.TempDir()
	env, err := environment(home)
	if err != nil || env.ConfigHome != filepath.Join(home, ".config") || env.ConfigHomeExplicit {
		t.Fatalf("sandbox inherited external config: %+v, %v", env, err)
	}
}

func TestListThemes(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{"themes": map[string]any{"fixture": map[string]any{}, "dracula": map[string]any{}}})
	want := append(append([]string(nil), builtinThemes...), "fixture")
	sort.Strings(want)
	for _, jsonOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[jsonOutput], func(t *testing.T) {
			args := []string{"list", "--home", home, "--config", config}
			if jsonOutput {
				args = append(args, "--json")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
				t.Fatalf("code=%d diagnostics=%s", code, stderr.String())
			}
			if !jsonOutput {
				if got := stdout.String(); got != strings.Join(want, "\n")+"\n" {
					t.Fatalf("got %q; want themes %q", got, want)
				}
				return
			}
			var got []string
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q; want %q; decode error=%v", got, want, err)
			}
		})
	}
}

func TestListAllDiscoversHiddenBundledAndCustomThemes(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{
		"listed": []string{"dracula", "my-theme"},
		"themes": map[string]any{"my-theme": map[string]any{}, "private": map[string]any{}},
	})
	var stdout, stderr bytes.Buffer
	args := []string{"list", "--all", "--json", "--home", home, "--config", config}
	if code := Run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("list --all: code=%d stderr=%s", code, stderr.String())
	}
	var got []string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := append(catalog.ThemeIDs(), "my-theme", "private")
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("full discovery: got %q, want %q", got, want)
	}
	stdout.Reset()
	if code := Run([]string{"list", "--json", "--home", home, "--config", config}, &stdout, &stderr); code != 0 || stdout.String() != "[\n  \"dracula\",\n  \"my-theme\"\n]\n" {
		t.Fatalf("default discovery changed: code=%d output=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	fixtureConfig(t, config, map[string]any{"listed": []string{}})
	stdout.Reset()
	if code := Run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("list --all with hidden discovery failed: %d %s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || len(got) != len(catalog.ThemeIDs()) {
		t.Fatalf("empty listed override hid registered IDs: %v, %q", err, got)
	}
}

func TestListDetailsDescribeDeclaredPortsNotRuntimeSupport(t *testing.T) {
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "mine.opts"), "--color=fg:#112233\n")
	fixtureConfig(t, config, map[string]any{
		"listed": []string{"dracula", "my-theme"},
		"themes": map[string]any{
			"my-theme": map[string]any{"custom": map[string]any{"fzf": map[string]string{"file": "mine.opts"}}},
		},
	})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list", "--all", "--details", "--json", "--home", home, "--config", config}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("list details: code=%d diagnostics=%s", code, stderr.String())
	}
	var got []themeDetail
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]themeDetail, len(got))
	for _, entry := range got {
		byID[entry.ID] = entry
	}
	if !byID["dracula"].Listed || byID["dracula"].SupportSite != "https://draculatheme.com/" ||
		!reflect.DeepEqual(byID["my-theme"].Tools, []string{"fzf"}) || !byID["my-theme"].Listed ||
		byID["kanagawa"].Listed || byID["kanagawa"].SupportSite != "" ||
		!slices.Contains(byID["kanagawa"].Tools, "eza") || !slices.Contains(byID["kanagawa"].Tools, "yazi") {
		t.Fatalf("incorrect listed state, metadata, or declared ports: %#v", byID)
	}
	if byID["catppuccin-latte"].Appearance != "light" || byID["rose-pine-dawn"].Appearance != "light" ||
		byID["solarized-light"].Appearance != "light" || byID["ayu-light"].Appearance != "light" ||
		byID["flexoki-light"].Appearance != "light" || byID["kanagawa-lotus"].Appearance != "light" ||
		byID["tokyo-night-day"].Appearance != "light" || byID["tokyo-night-day"].Listed ||
		!slices.Contains(byID["tokyo-night-day"].Tools, "btop") ||
		!slices.Contains(byID["kanagawa-lotus"].Tools, "ghostty") ||
		byID["dracula"].Appearance != "" || byID["rose-pine"].Appearance != "" || byID["my-theme"].Appearance != "" {
		t.Fatalf("incorrect light appearance labels: %#v", byID)
	}
	stdout.Reset()
	if code := Run([]string{"list", "--all", "--details", "--home", home, "--config", config}, &stdout, &stderr); code != 0 ||
		!strings.HasPrefix(stdout.String(), "THEME\tDISCOVERY\tAPPEARANCE\tDECLARED TOOLS\tSUPPORT SITE\n") ||
		!strings.Contains(stdout.String(), "solarized-light\thidden\tlight\t") ||
		!strings.Contains(stdout.String(), "kanagawa\thidden\t-\t") || !strings.Contains(stdout.String(), "dracula\tlisted\t-\t") {
		t.Fatalf("details text omitted heading or visibility: code=%d output=%q diagnostics=%s", code, stdout.String(), stderr.String())
	}
}

func TestPreviewMarksLightAppearanceWithoutChangingThemeID(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	for _, test := range []struct {
		theme string
		light bool
	}{
		{theme: "catppuccin-latte", light: true},
		{theme: "rose-pine-dawn", light: true},
		{theme: "solarized-light", light: true},
		{theme: "ayu-light", light: true},
		{theme: "flexoki-light", light: true},
		{theme: "kanagawa-lotus", light: true},
		{theme: "tokyo-night-day", light: true},
		{theme: "catppuccin-mocha"},
	} {
		t.Run(test.theme, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"__preview", test.theme, "--home", home}, &stdout, &stderr); code != 0 {
				t.Fatalf("preview failed: code=%d diagnostics=%q", code, stderr.String())
			}
			output := stdout.String()
			if strings.Contains(output, "Appearance: light\n") != test.light ||
				!strings.Contains(output, "Theme: "+test.theme+"\n") {
				t.Fatalf("wrong appearance or theme in preview: %q", output)
			}
		})
	}
}

func TestListOnlyFlagsRejectOtherCommands(t *testing.T) {
	for _, args := range [][]string{
		{"list", "tools", "--all"},
		{"list", "tools", "--details"},
		{"apply", "dracula", "--all"},
		{"current", "--details"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
			!strings.Contains(stderr.String(), "require list without a subcommand") {
			t.Fatalf("accepted list-only flag for %q: code=%d out=%q err=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestListedOverrideControlsDiscoveryWithoutRestrictingApply(t *testing.T) {
	installedFixtures(t, "fzf")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "hidden.opts"), "--color=fg:#112233,bg:#000000\n")
	fixtureConfig(t, config, map[string]any{
		"listed": []string{"rose-pine", "my-theme"},
		"themes": map[string]any{"hidden": map[string]any{"custom": map[string]any{
			"fzf": map[string]string{"file": "hidden.opts"},
		}}},
	})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list", "--config", config, "--home", home}, &stdout, &stderr); code != 0 ||
		stdout.String() != "rose-pine\nmy-theme\n" || stderr.Len() != 0 {
		t.Fatalf("listed override did not replace the displayed themes: %d %q %q", code, stdout.String(), stderr.String())
	}
	if got := completeWords([]string{"--config", config, "--home", home, "apply", ""}, adapter.All()); !reflect.DeepEqual(got, []string{"my-theme", "rose-pine"}) {
		t.Fatalf("completion did not follow the visible themes: %q", got)
	}
	code, result, diagnostics := invoke(t, "--config", config, "--home", home, "--tools", "fzf", "apply", "hidden")
	data, err := os.ReadFile(filepath.Join(home, ".config", "themeset", "fzf.opts"))
	if code != 0 || len(result.Tools) != 1 || result.Tools[0].Outcome != "applied" ||
		err != nil || string(data) != "--color=fg:#112233,bg:#000000\n" {
		t.Fatalf("hidden custom theme cannot be applied directly: %d %+v %q %v %s", code, result, data, err, diagnostics)
	}
}

func TestBundledHiddenThemeAppliesExplicitlyWithoutEnteringDiscovery(t *testing.T) {
	installedFixtures(t, "lazygit")
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list", "--home", home, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("list: %d %s", code, stderr.String())
	}
	var ids []string
	if err := json.Unmarshal(stdout.Bytes(), &ids); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == "cyberdream" {
			t.Fatal("hidden bundled theme appeared in discovery")
		}
	}
	code, result, diagnostics := invoke(t, "apply", "cyberdream", "--tools", "lazygit", "--dry-run", "--home", home)
	if code != 0 || result.Theme != "cyberdream" || len(result.Tools) != 1 || result.Tools[0].Status != "official" ||
		result.Tools[0].Port == nil || result.Tools[0].Port.Name != "cyberdream" ||
		result.Tools[0].FallbackTheme != "" {
		t.Fatalf("bundled hidden theme cannot be selected directly: code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
}

func TestEmptyListedOverrideHidesPickerEntriesWithoutDisablingDirectApply(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{"listed": []string{}})
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"list", "--config", config, "--home", home, "--json"}, &stdout, &stderr); code != 0 ||
		stdout.String() != "[]\n" || stderr.Len() != 0 {
		t.Fatalf("empty listed override did not hide themes: %d %q %q", code, stdout.String(), stderr.String())
	}
	if code := Run([]string{"apply", "dracula", "--tools", "", "--config", config, "--home", home}, &stdout, &stderr); code != 0 {
		t.Fatalf("explicit apply was blocked by an empty discovery list: %d %s", code, stderr.String())
	}
}

func TestListToolsIncludesInstalledAndMissingAdapters(t *testing.T) {
	installedFixtures(t, "alpha-bin")
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{"tools": []string{}})
	adapters := []model.Adapter{
		{ID: "zeta", Binary: "zeta-bin"},
		{ID: "alpha", Binary: "alpha-bin"},
	}
	for _, jsonOutput := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[jsonOutput], func(t *testing.T) {
			args := []string{"--config", config, "list", "--home", home, "tools"}
			if jsonOutput {
				args = append(args, "--json")
			}
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr, adapters); code != 0 || stderr.Len() != 0 {
				t.Fatalf("code=%d diagnostics=%s", code, stderr.String())
			}
			if !jsonOutput {
				if got := stdout.String(); got != "alpha\tinstalled\nzeta\tnot-installed\n" {
					t.Fatalf("unexpected tool listing: %q", got)
				}
				return
			}
			type tool struct {
				ID        string `json:"id"`
				Binary    string `json:"binary"`
				Installed bool   `json:"installed"`
			}
			var got []tool
			want := []tool{{"alpha", "alpha-bin", true}, {"zeta", "zeta-bin", false}}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v; want %+v; decode error=%v", got, want, err)
			}
		})
	}
}

func TestCommandArgumentRejections(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"removed check", []string{"check", "dracula"}, "unknown command"},
		{"removed tools", []string{"tools"}, "unknown command"},
		{"unknown list argument", []string{"list", "themes"}, "list expects no arguments or tools"},
		{"extra list argument", []string{"list", "tools", "extra"}, "list expects no arguments or tools"},
		{"extra apply theme", []string{"apply", "dracula", "nord"}, "apply expects zero or one theme argument"},
		{"current argument", []string{"current", "dracula"}, "current expects no arguments"},
		{"missing preview theme", []string{"__preview"}, "__preview expects one theme argument"},
		{"extra preview theme", []string{"__preview", "dracula", "nord"}, "__preview expects one theme argument"},
		{"missing completion shell", []string{"completion"}, "completion expects bash, zsh, or fish"},
		{"extra completion shell", []string{"completion", "bash", "fish"}, "completion expects bash, zsh, or fish"},
		{"invalid completion shell", []string{"completion", "sh"}, "completion expects bash, zsh, or fish"},
		{"unknown apply theme", []string{"apply", "not-a-theme"}, "unknown theme"},
		{"unknown preview theme", []string{"__preview", "not-a-theme"}, "unknown theme"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := Run(append(append([]string(nil), test.args...), "--home", home), &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid command wrote files: %v, %v", entries, err)
			}
		})
	}
}

func TestDryRunOnlyAcceptedWithApply(t *testing.T) {
	for _, args := range [][]string{
		{"list"}, {"list", "tools"}, {"current"}, {"completion", "bash"},
		{"help"}, {"version"}, {"__preview", "dracula"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(append(append([]string(nil), args...), "--dry-run"), &stdout, &stderr)
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--dry-run is only valid with apply") {
				t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestExplicitApplyDoesNotRequirePicker(t *testing.T) {
	installedFixtures(t)
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"apply", "dracula", "--tools", "", "--home", home}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Theme: dracula") {
		t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "themeset", "state.json")); err != nil {
		t.Fatalf("explicit apply did not record state: %v", err)
	}
}

func TestInteractiveApplyRejectsJSONWithoutWrites(t *testing.T) {
	installedFixtures(t)
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"apply", "--json", "--tools", "", "--home", home}, &stdout, &stderr)
	if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--json") {
		t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("interactive JSON rejection wrote files: %v, %v", entries, err)
	}
}

func TestInteractiveApplyValidatesSelectionBeforePicker(t *testing.T) {
	installedFixtures(t)
	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"apply", "--json", "--tools", "not-a-tool", "--home", home}, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "use 'list tools'") {
		t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
	}
}

func TestHiddenPreviewNeverWrites(t *testing.T) {
	installedFixtures(t, "tmux")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
	fixtureConfig(t, config, map[string]any{"tools": []string{"tmux"}, "themes": map[string]any{
		"fixture": map[string]any{"custom": map[string]any{"tmux": map[string]string{"file": "theme.conf"}}},
	}})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"__preview", "fixture", "--home", home, "--config", config}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "fixture") || !strings.Contains(stdout.String(), "tmux") {
		t.Fatalf("code=%d stdout=%s diagnostics=%s", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview wrote files or state: %v, %v", entries, err)
	}
}

func TestHumanReportingPreservesHistoricalMetadata(t *testing.T) {
	result := Result{Theme: "fixture", AppliedAt: "2026-01-01T00:00:00Z", Tools: []ToolResult{
		{Tool: "unsupported-a", Status: "unsupported", Outcome: "skipped", Warnings: []string{
			"no verified native or official theme available", "no registered custom port; settings unchanged",
		}},
		{Tool: "unsupported-b", Status: "unsupported", Outcome: "skipped", Warnings: []string{
			"no verified native or official theme available", "no registered custom port; settings unchanged",
		}},
		{Tool: "fallback", Status: "custom", Outcome: "applied", Port: &model.Port{Kind: "custom"},
			Warnings: []string{"no verified native or official theme available"}, Error: "inspection failure"},
		{Tool: "failed", Status: "error", Outcome: "failed", Error: "application failure"},
	}}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := printResult(&stdout, &stderr, result); code != 0 {
		t.Fatalf("report failed: %d", code)
	}
	after, err := json.Marshal(result)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("report changed historical metadata: %s -> %s (%v)", before, after, err)
	}
	// One aggregate skip diagnostic, one custom warning, and both real errors.
	if strings.Count(stderr.String(), "warning:") != 2 || strings.Count(stderr.String(), "error:") != 2 {
		t.Fatalf("diagnostics not consolidated or errors lost: %s", stderr.String())
	}
	for _, message := range []string{"inspection failure", "application failure"} {
		if !strings.Contains(stderr.String(), message) {
			t.Fatalf("lost error %q: %s", message, stderr.String())
		}
	}
	for _, item := range result.Tools {
		if !strings.Contains(stdout.String(), item.Tool+"\t"+item.Status+"\t"+item.Outcome) {
			t.Fatalf("missing tool metadata for %s: %s", item.Tool, stdout.String())
		}
	}
}
