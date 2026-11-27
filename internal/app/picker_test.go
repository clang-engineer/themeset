package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPickContext(t *testing.T) {
	tests := []struct {
		name  string
		words []string
		valid bool
	}{
		{"missing current word", nil, false},
		{"command completion", []string{"apply"}, false},
		{"root completion", []string{""}, false},
		{"first theme", []string{"apply", ""}, true},
		{"partial theme", []string{"apply", "dr"}, true},
		{"nonprefix query", []string{"apply", "not-a-prefix"}, true},
		{"flags before command", []string{"--config", "config.json", "--home=home", "apply", "dr"}, true},
		{"flags after command", []string{"apply", "--tools", "tmux", "--dry-run", "--json", ""}, true},
		{"single dash flags", []string{"-home", "home", "apply", "-tools=tmux", ""}, true},
		{"literal theme", []string{"apply", "--", "--theme"}, true},
		{"literal command", []string{"--", "apply", ""}, true},
		{"help disabled", []string{"apply", "--help=false", ""}, true},
		{"other command", []string{"list", ""}, false},
		{"preview command", []string{"__preview", ""}, false},
		{"theme already supplied", []string{"apply", "dracula", ""}, false},
		{"theme before flags", []string{"apply", "dracula", "--json", ""}, false},
		{"later positional", []string{"apply", "dracula", "nord", ""}, false},
		{"flag completion", []string{"apply", "--dry"}, false},
		{"flag value inline", []string{"apply", "--config=path"}, false},
		{"config path", []string{"apply", "--config", "path"}, false},
		{"home path", []string{"apply", "--home", ""}, false},
		{"tools value", []string{"apply", "--tools", "tm"}, false},
		{"single dash tools value", []string{"apply", "-tools", ""}, false},
		{"pending value equal prior theme", []string{"apply", "dracula", "--config", "dracula"}, false},
		{"invalid prior flag", []string{"apply", "--unknown", ""}, false},
		{"invalid boolean value", []string{"apply", "--json=invalid", ""}, false},
		{"help overrides apply", []string{"apply", "--help", ""}, false},
		{"short help overrides apply", []string{"-h", "apply", ""}, false},
		{"version overrides apply", []string{"--version", "apply", ""}, false},
		{"terminator used as a value", []string{"apply", "--config", "--", "--json"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, query, valid := pickContext(test.words)
			if valid != test.valid {
				t.Fatalf("valid=%v; want %v", valid, test.valid)
			}
			if valid && query != test.words[len(test.words)-1] {
				t.Fatalf("query=%q; want current word %q", query, test.words[len(test.words)-1])
			}
		})
	}
	opts, _, valid := pickContext([]string{"--config", "theme's config.json", "apply", "--home", "a home", "--tools=tmux,bat", "dr"})
	if !valid || opts.config != "theme's config.json" || opts.home != "a home" || !opts.toolsSet || opts.tools != "tmux,bat" {
		t.Fatalf("lost preview options: %+v; valid=%v", opts, valid)
	}
}

func TestPickRejectedContextIsSilentAndDoesNotReadConfig(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "invalid.json")
	fixtureFile(t, config, "not json")
	before := pickerSnapshot(t, home)
	for _, words := range [][]string{
		{""}, {"list", ""}, {"apply", "dracula", ""}, {"apply", "--tools", "tm"}, {"apply", "--config", ""}, {"apply", "--json"}, {"apply", "--help", ""},
	} {
		args := append([]string{"__pick", "--home", home, "--config", config}, words...)
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("words=%q code=%d stdout=%q stderr=%q", words, code, stdout.String(), stderr.String())
		}
	}
	if after := pickerSnapshot(t, home); !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected completion changed home: before=%v after=%v", before, after)
	}
}

func TestPickConfigurationAndToolErrors(t *testing.T) {
	home := t.TempDir()
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	fixtureFile(t, invalid, "not json")
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"missing config", []string{"--config", filepath.Join(home, "missing.json")}, "no such file"},
		{"invalid config", []string{"--config", invalid}, "decode config"},
		{"unknown tool", []string{"--tools", "not-a-tool"}, "unknown tool"},
		{"invalid tools list", []string{"--tools", "tmux,,bat"}, "empty tool"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"__pick", "--home", home}, test.args...)
			args = append(args, "apply", "")
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("picker error wrote files: %v, %v", entries, err)
	}
}

func TestPickEnvironmentError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "relative")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"__pick", "apply", ""}, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "XDG_CONFIG_HOME must be an absolute path") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPickSelectionOnlyWithRedirectedStdin(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "theme's config.json")
	fixtureConfig(t, config, map[string]any{
		"tools":  []string{"tmux"},
		"themes": map[string]any{"custom theme": map[string]any{}},
	})
	fixtureFile(t, filepath.Join(home, ".config", "tmux", "tmux.conf"), "set -g status-style bg=blue\n")
	fixtureFile(t, filepath.Join(home, ".config", "themeset", "state.json"), "existing state\n")
	before := pickerSnapshot(t, home)
	args := []string{"__pick", "--config", config, "apply", "--home", home, "--tools", "tmux", "--json", "--dry-run", "dr"}
	result, directory := pickerProcess(t, args, map[string]string{"THEMESET_PICKER_GROUP": "other", "THEMESET_PICKER_THEME": "custom theme\t-    custom theme"}, true)
	if result.Code != 0 || result.Stdout != "custom theme\n" || result.Stderr != "" {
		t.Fatalf("selection result=%+v", result)
	}
	rows, err := os.ReadFile(filepath.Join(directory, "rows"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rows, []byte("custom theme\t-    custom theme\x00")) {
		t.Fatalf("other group contains unexpected themes: %q", rows)
	}
	if after := pickerSnapshot(t, home); !reflect.DeepEqual(before, after) {
		t.Fatalf("selection changed home: before=%v after=%v", before, after)
	}
}

func TestPickerGroupsAppearanceAndReturnsExactThemeID(t *testing.T) {
	result, directory := pickerProcess(t, []string{"__pick", "apply", ""}, map[string]string{
		"THEMESET_PICKER_THEME": "catppuccin-latte\t#02  catppuccin-latte  [light]",
	}, true)
	if result.Code != 0 || result.Stdout != "catppuccin-latte\n" || result.Stderr != "" {
		t.Fatalf("picker result=%+v", result)
	}
	rows, err := os.ReadFile(filepath.Join(directory, "rows"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(rows, []byte("catppuccin-latte\t#02  catppuccin-latte  [light]\x00")) ||
		!bytes.Contains(rows, []byte("rose-pine-dawn\t#32  rose-pine-dawn  [light]\x00")) ||
		!bytes.Contains(rows, []byte("catppuccin-mocha\t#02  catppuccin-mocha\x00")) ||
		bytes.Index(rows, []byte("catppuccin-mocha\t#02  catppuccin-mocha\x00")) > bytes.Index(rows, []byte("catppuccin-latte\t#02  catppuccin-latte  [light]\x00")) {
		t.Fatalf("incorrect picker labels: %q", rows)
	}
}

func TestPickerRankSortAcrossAppearanceKeepsRankedIDAndLightBadge(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{
		"listed":     []string{"custom", "tokyo-night", "catppuccin-latte", "dracula"},
		"theme_sort": "rank",
		"themes":     map[string]any{"custom": map[string]any{}},
	})
	result, directory := pickerProcess(t, []string{"__pick", "--home", home, "--config", config, "apply", ""}, map[string]string{
		"THEMESET_PICKER_THEME": "catppuccin-latte\t#02  catppuccin-latte  [light]",
	}, true)
	if result.Code != 0 || result.Stdout != "catppuccin-latte\n" || result.Stderr != "" {
		t.Fatalf("picker selection failed: %+v", result)
	}
	rows, err := os.ReadFile(filepath.Join(directory, "rows"))
	if err != nil {
		t.Fatal(err)
	}
	first, second, third, unranked := []byte("dracula\t#01  dracula\x00"), []byte("catppuccin-latte\t#02  catppuccin-latte  [light]\x00"),
		[]byte("tokyo-night\t#07  tokyo-night\x00"), []byte("custom\t-    custom\x00")
	if a, b, c, d := bytes.Index(rows, first), bytes.Index(rows, second), bytes.Index(rows, third), bytes.Index(rows, unranked); a < 0 || b < a || c < b || d < c {
		t.Fatalf("rank sort lost global order or appearance labels: %q", rows)
	}
}

func TestPickerSortFlagOverridesConfiguredRank(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.json")
	fixtureConfig(t, config, map[string]any{
		"listed":     []string{"z-custom", "dracula", "ayu-light"},
		"theme_sort": "rank",
		"themes":     map[string]any{"z-custom": map[string]any{}},
	})
	result, directory := pickerProcess(t, []string{"__pick", "--sort", "name", "--home", home, "--config", config, "apply", ""}, map[string]string{
		"THEMESET_PICKER_THEME": "ayu-light\t#61  ayu-light  [light]",
	}, true)
	if result.Code != 0 || result.Stdout != "ayu-light\n" || result.Stderr != "" {
		t.Fatalf("sort override selection failed: %+v", result)
	}
	rows, err := os.ReadFile(filepath.Join(directory, "rows"))
	if err != nil {
		t.Fatal(err)
	}
	a, d, z := bytes.Index(rows, []byte("ayu-light\t#61  ayu-light  [light]\x00")),
		bytes.Index(rows, []byte("dracula\t#01  dracula\x00")),
		bytes.Index(rows, []byte("z-custom\t-    z-custom\x00"))
	if a < 0 || d < a || z < d {
		t.Fatalf("name override did not sort all appearances: %q", rows)
	}
}

func TestPickerCategorySelection(t *testing.T) {
	for _, test := range []struct {
		group, theme, want, absent string
	}{
		{"dark", "dracula\t#01  dracula", "dracula\t#01  dracula\x00", "ayu-light\t#61  ayu-light\x00"},
		{"light", "ayu-light\t#61  ayu-light", "ayu-light\t#61  ayu-light\x00", "dracula\t#01  dracula\x00"},
	} {
		t.Run(test.group, func(t *testing.T) {
			result, directory := pickerProcess(t, []string{"__pick", "apply", ""}, map[string]string{
				"THEMESET_PICKER_GROUP": test.group, "THEMESET_PICKER_THEME": test.theme,
			}, true)
			if result.Code != 0 || result.Stdout != strings.Split(test.theme, "\t")[0]+"\n" || result.Stderr != "" {
				t.Fatalf("picker result=%+v", result)
			}
			rows, err := os.ReadFile(filepath.Join(directory, "rows"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(rows, []byte(test.want)) || bytes.Contains(rows, []byte(test.absent)) ||
				bytes.Contains(rows, []byte("[light]")) || bytes.Contains(rows, []byte("[dark]")) {
				t.Fatalf("incorrect category rows: %q", rows)
			}
		})
	}
}

func TestPickCancellationAndErrorsDoNotWrite(t *testing.T) {
	for _, test := range []struct {
		name string
		env  map[string]string
		code int
		want string
	}{
		{"escape", map[string]string{"THEMESET_PICKER_EXIT": "130"}, 130, ""},
		{"no match", map[string]string{"THEMESET_PICKER_EXIT": "1"}, 130, ""},
		{"fzf error", map[string]string{"THEMESET_PICKER_EXIT": "2"}, 1, "theme picker"},
		{"invalid output", map[string]string{"THEMESET_PICKER_MODE": "invalid"}, 1, "invalid selection"},
		{"empty output", map[string]string{"THEMESET_PICKER_MODE": "empty"}, 1, "invalid selection"},
		{"multiple selections", map[string]string{"THEMESET_PICKER_MODE": "multiple"}, 1, "unknown theme"},
		{"unknown theme", map[string]string{"THEMESET_PICKER_THEME": "unknown"}, 1, "unknown theme"},
		{"missing fzf", map[string]string{"THEMESET_PICKER_MISSING_FZF": "1"}, 1, "requires fzf"},
		{"output failure", map[string]string{"THEMESET_PICKER_FAIL_OUTPUT": "1"}, 1, "fixture output failure"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			result, _ := pickerProcess(t, []string{"__pick", "--home", home, "apply", ""}, test.env, true)
			if result.Code != test.code || result.Stdout != "" || (test.want == "" && result.Stderr != "") || (test.want != "" && !strings.Contains(result.Stderr, test.want)) {
				t.Fatalf("result=%+v; want code=%d diagnostic=%q", result, test.code, test.want)
			}
			if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
				t.Fatalf("picker cancellation/error wrote files: %v, %v", entries, err)
			}
		})
	}
}

func TestInteractiveApplyStillReviewsDryRunWithoutWrites(t *testing.T) {
	home := t.TempDir()
	result, _ := pickerProcess(t, []string{"apply", "--home", home, "--tools", "", "--dry-run"}, nil, false)
	if result.Code != 0 || result.Stderr != "" {
		t.Fatalf("interactive dry-run result=%+v", result)
	}
	if entries, err := os.ReadDir(home); err != nil || len(entries) != 0 {
		t.Fatalf("interactive dry-run wrote files: %v, %v", entries, err)
	}
}

type pickerProcessResult struct {
	Code   int
	Stdout string
	Stderr string
}

type pickerFailWriter struct{}

func (pickerFailWriter) Write([]byte) (int, error) {
	return 0, errors.New("fixture output failure")
}

// The helper runs the actual dispatcher in a controlling terminal, while its
// stdin can independently be redirected as Fish command substitutions require.
func TestPickerSubprocess(t *testing.T) {
	if os.Getenv("THEMESET_PICKER_HELPER") != "1" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("THEMESET_PICKER_WORDS")), &args); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("THEMESET_PICKER_REDIRECT_STDIN") == "1" {
		input, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer input.Close()
		os.Stdin = input
	}
	var stdout, stderr bytes.Buffer
	var destination io.Writer = &stdout
	if os.Getenv("THEMESET_PICKER_FAIL_OUTPUT") == "1" {
		destination = pickerFailWriter{}
	}
	t.Setenv("SHELL", "/nonexistent/shell")
	code := Run(args, destination, &stderr)
	data, err := json.Marshal(pickerProcessResult{Code: code, Stdout: stdout.String(), Stderr: stderr.String()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("THEMESET_PICKER_RESULT"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func pickerProcess(t *testing.T, words []string, values map[string]string, redirectStdin bool) (pickerProcessResult, string) {
	t.Helper()
	script, err := exec.LookPath("script")
	if err != nil || (runtime.GOOS != "darwin" && runtime.GOOS != "linux") {
		t.Skip("picker integration requires the Darwin or Linux script terminal utility")
	}
	directory := t.TempDir()
	bin := filepath.Join(directory, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	if values["THEMESET_PICKER_MISSING_FZF"] != "1" {
		fixtureFile(t, filepath.Join(bin, "fzf"), `#!/bin/sh
for arg do
    if [ "$arg" = "--prompt=Group> " ]; then
        /bin/cat > "$THEMESET_PICKER_CAPTURE/groups"
        if [ -n "$THEMESET_PICKER_EXIT" ]; then exit "$THEMESET_PICKER_EXIT"; fi
        printf '%s\000' "$THEMESET_PICKER_GROUP"
        exit
    fi
done
/bin/cat > "$THEMESET_PICKER_CAPTURE/rows"
if [ -n "$THEMESET_PICKER_EXIT" ]; then exit "$THEMESET_PICKER_EXIT"; fi
case "$THEMESET_PICKER_MODE" in
    invalid) printf '%s' "$THEMESET_PICKER_THEME" ;;
    empty) ;;
    multiple) printf '%s\000%s\000' "$THEMESET_PICKER_THEME" "$THEMESET_PICKER_THEME" ;;
    *) printf '%s\000' "$THEMESET_PICKER_THEME" ;;
esac
`)
		if err := os.Chmod(filepath.Join(bin, "fzf"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.CommandContext(ctx, script, "-q", os.DevNull, executable, "-test.run=^TestPickerSubprocess$")
	} else {
		command = exec.CommandContext(ctx, script, "-q", "-e", "-c", shellQuote(executable)+" -test.run=^TestPickerSubprocess$", os.DevNull)
	}
	// Keep script's input open until its child exits; no keystrokes are needed,
	// and an accidental confirmation prompt must time out rather than get EOF.
	input, keepalive, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer keepalive.Close()
	command.Stdin = input
	encoded, err := json.Marshal(words)
	if err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{
		"PATH": bin, "THEMESET_PICKER_HELPER": "1", "THEMESET_PICKER_WORDS": string(encoded),
		"THEMESET_PICKER_RESULT": filepath.Join(directory, "result.json"), "THEMESET_PICKER_CAPTURE": directory,
		"THEMESET_PICKER_THEME": "dracula\t#01  dracula", "THEMESET_PICKER_EXIT": "", "THEMESET_PICKER_MODE": "",
		"THEMESET_PICKER_GROUP": "all", "THEMESET_PICKER_FAIL_OUTPUT": "", "THEMESET_PICKER_REDIRECT_STDIN": "0",
		"FZF_DEFAULT_OPTS": "--bind=enter:abort", "FZF_DEFAULT_OPTS_FILE": "/nonexistent/options", "SHELL": "/bin/sh",
	}
	if redirectStdin {
		environment["THEMESET_PICKER_REDIRECT_STDIN"] = "1"
	}
	for key, value := range values {
		environment[key] = value
	}
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, replaced := environment[key]; !replaced {
			command.Env = append(command.Env, value)
		}
	}
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("terminal subprocess: %v: %s", err, output)
	}
	data, err := os.ReadFile(environment["THEMESET_PICKER_RESULT"])
	if err != nil {
		t.Fatal(err)
	}
	var result pickerProcessResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result, directory
}

func pickerSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(data)
		}
		result[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
