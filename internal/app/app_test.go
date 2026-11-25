package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter"
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

func TestDryRunNeverWrites(t *testing.T) {
	installedFixtures(t, "tmux")
	home, directory := t.TempDir(), t.TempDir()
	config := filepath.Join(directory, "config.json")
	fixtureFile(t, filepath.Join(directory, "theme.conf"), "set -g status-style bg=blue\n")
	fixtureConfig(t, config, map[string]any{"tools": []string{"tmux"}, "themes": map[string]any{
		"fixture": map[string]any{"custom": map[string]any{"tmux": map[string]string{"file": "theme.conf"}}},
	}})
	code, result, diagnostics := invoke(t, "apply", "fixture", "--dry-run", "--home", home, "--config", config)
	if code != 0 || !result.DryRun || len(result.Tools) != 1 || result.Tools[0].Outcome != "planned" || !strings.Contains(diagnostics, "no verified native") {
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

func TestHistoricalCustomResultAlwaysWarns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := Result{Theme: "fixture", Tools: []ToolResult{
		{Tool: "fallback", Status: "custom", Outcome: "applied"},
	}}
	if printResult(&stdout, &stderr, result) != 0 || strings.Count(stderr.String(), "warning:") != 1 {
		t.Fatalf("custom fallback warning missing: %s", stderr.String())
	}
}
