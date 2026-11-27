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
	// fzf quotes its placeholder independently; {1} is the theme ID before the tab label.
	return strings.Join(words, " ") + " {1}"
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

func pickerCategory(theme string, bundled map[string]bool) string {
	if catalog.IsLight(theme) {
		return "light"
	}
	if bundled[theme] {
		return "dark"
	}
	return "other"
}

func pickerRow(name, group string, stars int, showGroup bool) string {
	icon, value := "  ", "-"
	if stars > 0 {
		icon, value = " ", formatStars(stars)
	}
	row := name + "\t" + fmt.Sprintf("%s%7s  %s", icon, value, name)
	if showGroup && group == "light" {
		row += "  [light]"
	}
	return row
}

// runPicker isolates user fzf defaults for both selection stages.
func runPicker(fzf string, terminal *os.File, input []byte, args ...string) (string, bool, error) {
	command := exec.Command(fzf, args...)
	command.Stdin = bytes.NewReader(input)
	command.Stderr = terminal
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
	return string(output[:len(output)-1]), true, nil
}

// selectTheme only selects a known identifier. Planning, confirmation, and
// application belong to its caller, not to the shared fzf invocation.
func selectTheme(opts options, config model.Config, ranks radarRanks, terminal *os.File, query, action string) (string, bool, error) {
	fzf, err := exec.LookPath("fzf")
	if err != nil {
		return "", false, errors.New("theme picker requires fzf; install fzf or use apply THEME")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	category, selected, err := runPicker(fzf, terminal, []byte("all\x00dark\x00light\x00other\x00"),
		"--no-multi", "--no-sort", "--read0", "--print0", "--layout=reverse", "--border",
		"--height=10", "--prompt=Group> ", "--header=Choose a group | Enter: continue | Esc: cancel")
	if err != nil || !selected {
		return "", false, err
	}
	switch category {
	case "all", "dark", "light", "other":
	default:
		return "", false, errors.New("theme picker returned an unknown group")
	}
	mode := discoverySort(config, opts.sort)
	names := sortedThemes(themeIDs(config), mode, config, ranks)
	bundled := make(map[string]bool)
	for _, name := range catalog.Listed() {
		bundled[name] = true
	}
	var input bytes.Buffer
	groups := []string{"dark", "light", "other"}
	if category == "all" && mode != "" && mode != "listed" {
		groups = []string{"all"}
	}
	for _, group := range groups {
		if category != "all" && category != group {
			continue
		}
		for _, name := range names {
			appearance := pickerCategory(name, bundled)
			if group != "all" && appearance != group {
				continue
			}
			input.WriteString(pickerRow(name, appearance, ranks.stars(name, config), category == "all"))
			input.WriteByte(0)
		}
	}
	order := "GitHub Stars ↓"
	switch mode {
	case "name":
		order = "Name A–Z"
	case "listed":
		order = "Listed order"
	}
	row, selected, err := runPicker(fzf, terminal, input.Bytes(),
		"--no-multi", "--no-sort", "--read0", "--print0", "--delimiter=\t", "--with-nth=2",
		"--layout=reverse", "--border", "--height=85%", "--prompt=Theme> ", "--query="+query,
		"--header=Group: "+category+" | Sort: "+order+" | Enter: "+action+" | Esc: cancel",
		"--preview="+previewCommand(executable, opts), "--preview-window=right,60%,wrap")
	if err != nil || !selected {
		return "", false, err
	}
	theme, _, _ := strings.Cut(row, "\t")
	for _, name := range names {
		if theme != name {
			continue
		}
		if category != "all" && pickerCategory(name, bundled) != category {
			break
		}
		expected := pickerRow(name, pickerCategory(name, bundled), ranks.stars(name, config), category == "all")
		if row == expected {
			return theme, true, nil
		}
		break
	}
	return "", false, errors.New("theme picker returned an unknown theme")
}

func printPickedTheme(opts options, config model.Config, ranks radarRanks, query string, stdout, stderr io.Writer) int {
	terminal, err := pickerTerminal()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	defer terminal.Close()
	theme, selected, err := selectTheme(opts, config, ranks, terminal, query, "insert theme")
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

func chooseTheme(opts options, config model.Config, ranks radarRanks, env model.Environment, selected []model.Adapter, stdout, stderr io.Writer) (string, bool, error) {
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
	theme, proceed, err := selectTheme(opts, config, ranks, terminal, "", "review plan")
	if err != nil || !proceed {
		return "", false, err
	}
	result := plan(theme, config, env, selected, true)
	if err := printSelectionPlan(stdout, result); err != nil {
		return "", false, fmt.Errorf("display application plan: %w", err)
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

func printPreview(theme string, config model.Config, ranks radarRanks, env model.Environment, selected []model.Adapter, stdout, stderr io.Writer) int {
	if catalog.IsLight(theme) {
		fmt.Fprintln(stdout, "Appearance: light")
	}
	if stars := ranks.stars(theme, config); stars > 0 {
		fmt.Fprintf(stdout, "GitHub Stars: %s (Radar as of %s)\n", formatStars(stars), ranks.UpdatedAt[:10])
	}
	if err := renderVisualPreview(theme, config, env, stdout); err != nil {
		fmt.Fprintln(stderr, "preview:", err)
	}
	result := plan(theme, config, env, selected, true)
	fmt.Fprintln(stdout)
	if err := printSelectionPlan(stdout, result); err != nil {
		fmt.Fprintln(stderr, "error: write preview:", err)
		return 1
	}
	return 0
}

// Group the selected theme's actual ports so the same explanation works in the
// picker, before confirmation, and after application. JSON keeps full metadata.
func printSelectionPlan(w io.Writer, result Result) error {
	direct, fallback, skipped, failures := 0, 0, 0, 0
	for _, item := range result.Tools {
		switch {
		case item.Error != "" || item.Outcome == "failed":
			failures++
		case item.Outcome == "skipped":
			skipped++
		case item.FallbackTheme != "":
			fallback++
		default:
			direct++
		}
	}
	if _, err := fmt.Fprintf(w, "Theme: %s\n", result.Theme); err != nil {
		return err
	}
	applied := result.AppliedAt != ""
	if applied {
		if _, err := fmt.Fprintf(w, "Applied at: %s\nResult: %d requested, %d default fallback, %d skipped, %d errors\n", result.AppliedAt, direct, fallback, skipped, failures); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(w, "Plan: %d requested, %d default fallback, %d skipped, %d errors (no settings changed)\n", direct, fallback, skipped, failures); err != nil {
		return err
	}
	counts := [...]int{direct, fallback, skipped, failures}
	labels := [...]string{"Requested theme", "Default theme instead", "Skipped", "Errors"}
	for group, count := range counts {
		if count == 0 {
			continue
		}
		label := labels[group]
		if _, err := fmt.Fprintf(w, "\n%s (%d):\n", label, count); err != nil {
			return err
		}
		for _, item := range result.Tools {
			category := 0
			switch {
			case item.Error != "" || item.Outcome == "failed":
				category = 3
			case item.Outcome == "skipped":
				category = 2
			case item.FallbackTheme != "":
				category = 1
			}
			if category != group {
				continue
			}
			var err error
			switch group {
			case 0:
				_, err = fmt.Fprintf(w, "  %s (%s)\n", item.Tool, item.Status)
			case 1:
				_, err = fmt.Fprintf(w, "  %s -> %s (%s)\n", item.Tool, item.FallbackTheme, item.Status)
			case 2:
				_, err = fmt.Fprintf(w, "  %s: %s\n", item.Tool, item.Reason)
			case 3:
				_, err = fmt.Fprintf(w, "  %s: %s\n", item.Tool, item.Error)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
