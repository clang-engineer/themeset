package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
	"golang.org/x/term"
)

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func previewCommand(executable string, opts options) string {
	words := []string{executable, "__preview"}
	if opts.config != "" {
		words = append(words, "--config", opts.config)
	}
	if opts.home != "" {
		words = append(words, "--home", opts.home)
	}
	if opts.toolsSet {
		words = append(words, "--tools", opts.tools)
	}
	for i := range words {
		words[i] = shellQuote(words[i])
	}
	// fzf quotes its placeholder independently; never interpolate a selected name ourselves.
	return strings.Join(words, " ") + " {}"
}

// pickContext receives command words without the executable and with the current
// word last. Parsing only prior words rejects pending flag values and ensures
// that a theme already supplied earlier never opens another picker.
func pickContext(words []string) (options, string, bool) {
	if len(words) == 0 {
		return options{}, "", false
	}
	current := words[len(words)-1]
	opts, err := parse(words[:len(words)-1])
	if err != nil || opts.help || opts.version || len(opts.positionals) != 1 || opts.positionals[0] != "apply" {
		return options{}, "", false
	}
	if !opts.literal && strings.HasPrefix(current, "-") {
		return options{}, "", false
	}
	return opts, current, true
}

func pickerTerminal() (*os.File, error) {
	terminal, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open interactive terminal: %w", err)
	}
	return terminal, nil
}

// selectTheme only selects a known identifier. Planning, confirmation, and
// application belong to its caller, not to the shared fzf invocation.
func selectTheme(opts options, config model.Config, terminal *os.File, query, action string) (string, bool, error) {
	fzf, err := exec.LookPath("fzf")
	if err != nil {
		return "", false, errors.New("theme picker requires fzf; install fzf or use apply THEME")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	names := themeIDs(config)
	var input bytes.Buffer
	for _, name := range names {
		input.WriteString(name)
		input.WriteByte(0)
	}
	command := exec.Command(fzf, "--no-multi", "--read0", "--print0", "--layout=reverse", "--border", "--height=85%", "--prompt=Theme> ", "--query="+query, "--header=Enter: "+action+" | Esc: cancel | colors represent the named source app only", "--preview="+previewCommand(executable, opts), "--preview-window=right,65%,wrap")
	command.Stdin = &input
	command.Stderr = terminal
	// User picker defaults may bind Enter to other actions. Keep this picker isolated;
	// POSIX sh is also the contract used by previewCommand's argument quoting.
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if key != "FZF_DEFAULT_OPTS" && key != "FZF_DEFAULT_OPTS_FILE" && key != "SHELL" {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "SHELL=/bin/sh")
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 130 || exit.ExitCode() == 1) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("theme picker: %w", err)
	}
	if len(output) == 0 || output[len(output)-1] != 0 {
		return "", false, errors.New("theme picker returned an invalid selection")
	}
	theme := string(output[:len(output)-1])
	known := false
	for _, name := range names {
		if theme == name {
			known = true
			break
		}
	}
	if !known {
		return "", false, errors.New("theme picker returned an unknown theme")
	}
	return theme, true, nil
}

func printPickedTheme(opts options, config model.Config, query string, stdout, stderr io.Writer) int {
	terminal, err := pickerTerminal()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	defer terminal.Close()
	theme, selected, err := selectTheme(opts, config, terminal, query, "insert theme")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if !selected {
		return 130
	}
	if _, err := fmt.Fprintln(stdout, theme); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func chooseTheme(opts options, config model.Config, env model.Environment, selected []model.Adapter, stdout, stderr io.Writer) (string, bool, error) {
	if opts.json {
		return "", false, errors.New("--json requires an explicit theme: use apply THEME --dry-run --json")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", false, errors.New("interactive apply requires a terminal; use apply THEME instead")
	}
	terminal, err := pickerTerminal()
	if err != nil {
		return "", false, err
	}
	defer terminal.Close()
	theme, proceed, err := selectTheme(opts, config, terminal, "", "review plan")
	if err != nil || !proceed {
		return "", false, err
	}
	result := plan(theme, config, env, selected, true)
	if printResult(stdout, stderr, result) != 0 {
		return "", false, errors.New("could not display the application plan")
	}
	if opts.dryRun {
		if failed(result) {
			return "", false, errors.New("application plan contains errors")
		}
		return "", false, nil
	}
	if _, err := fmt.Fprintf(terminal, "Apply %q to the planned tools? [y/N] ", theme); err != nil {
		return "", false, err
	}
	answer, err := bufio.NewReader(terminal).ReadString('\n')
	if err != nil {
		return "", false, fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return theme, true, nil
	default:
		return "", false, nil
	}
}

func printPreview(theme string, config model.Config, env model.Environment, selected []model.Adapter, stdout, stderr io.Writer) int {
	if catalog.IsLight(theme) {
		fmt.Fprintln(stdout, "Appearance: light")
	}
	if err := renderVisualPreview(theme, config, env, stdout); err != nil {
		fmt.Fprintln(stderr, "preview:", err)
	}
	fmt.Fprintln(stdout, "\nApplication support (no settings changed):")
	result := plan(theme, config, env, selected, true)
	return printResult(stdout, stderr, result)
}
