package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter"
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

func TestNativePrecedesCustomAndPreservesConfig(t *testing.T) {
	installedFixtures(t, "nvim")
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	init := filepath.Join(home, ".config", "nvim", "init.lua")
	fixtureFile(t, init, "vim.opt.number = true\n")
	fixtureFile(t, filepath.Join(home, ".config", "nvim", "colors", "dracula.vim"), "hi Normal ctermfg=7\n")
	fixtureConfig(t, config, map[string]any{
		"tools": []string{"neovim"},
		"themes": map[string]any{"dracula": map[string]any{"custom": map[string]any{
			"neovim": map[string]string{"kind": "official", "file": "absent.lua"},
		}}},
	})
	code, result, diagnostics := invoke(t, "--home", home, "apply", "dracula", "--config", config)
	if code != 0 || len(result.Tools) != 1 || result.Tools[0].Status != "native" || result.Tools[0].Outcome != "applied" {
		t.Fatalf("code=%d result=%+v diagnostics=%s", code, result, diagnostics)
	}
	data, err := os.ReadFile(init)
	if err != nil || string(data) != "vim.opt.number = true\n" {
		t.Fatalf("init config changed: %q, %v", data, err)
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
	if result.Tools[0].Port.Kind != "custom" || result.Tools[1].Status != "custom" || !strings.Contains(diagnostics, "warning: tmux") || !strings.Contains(diagnostics, "error: bat") {
		t.Fatalf("fallback kinds/diagnostics: %+v, %s", result, diagnostics)
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

func TestDryRunAndCheckNeverWrite(t *testing.T) {
	installedFixtures(t, "tmux")
	for _, command := range []string{"check", "apply"} {
		t.Run(command, func(t *testing.T) {
			home, directory := t.TempDir(), t.TempDir()
			config := filepath.Join(directory, "config.json")
			fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
			fixtureConfig(t, config, map[string]any{"tools": []string{"tmux"}, "themes": map[string]any{
				"fixture": map[string]any{"custom": map[string]any{"tmux": map[string]string{"file": "theme.conf"}}},
			}})
			args := []string{command, "fixture", "--home", home, "--config", config}
			if command == "apply" {
				args = append(args, "--dry-run")
			}
			code, result, diagnostics := invoke(t, args...)
			if code != 0 || len(result.Tools) != 1 || result.Tools[0].Outcome != "planned" || !strings.Contains(diagnostics, "no verified native") {
				t.Fatalf("code=%d result=%+v diagnostics=%s", code, result, diagnostics)
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatalf("inspection created home files: %v, %v", entries, err)
			}
		})
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
