// Package app implements the CLI without owning the caller's process or streams.
package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter"
	"github.com/clang-engineer/themeset/internal/model"
)

// Version can be set by the release build with -ldflags '-X .../internal/app.Version=...'.
var Version = "dev"

const usage = `Usage: themeset [flags] COMMAND [flags] [THEME]

Commands:
  apply [THEME] Apply a theme, or preview and select interactively when omitted
  current       Show the last apply result (including skips and failures)
  list          List built-in theme identifiers and configured theme keys
  list tools    List registered tool IDs and whether each app is installed
  completion SHELL  Print Bash, Zsh, or Fish completion definitions
  help          Show this help
  version       Show the build version

Flags (accepted before or after the command or theme):
  --config PATH     JSON config; default $XDG_CONFIG_HOME/themeset/config.json
                    or ~/.config/themeset/config.json (missing default is okay)
  --home PATH       Use this home and PATH/.config, ignoring XDG_CONFIG_HOME
                    Explicit paths overrides still apply; not a security sandbox
  --tools IDS       Comma-separated IDs overriding config tools; empty means none
  --json            Emit JSON to stdout; diagnostics remain on stderr
  --dry-run         With apply only: inspect a plan, change no files or state
  --help, -h        Show help
  --version         Show version

Without THEME, apply requires interactive input and fzf. It previews themes,
then shows the selected plan and asks for confirmation before writing.
Use an explicit THEME with --json or from scripts.

Config supports tools, paths, themes, a default_theme ID (catppuccin-mocha
when omitted), and an optional listed array that replaces the discovery list
without restricting explicit apply. Omitted tools selects all adapters;
tools: [] selects none. Relative paths and custom file references resolve
against the config directory; ~/ uses --home. Explicit custom ports take
precedence over native/official ports. Invalid custom ports fail rather than
silently falling back. If the requested theme is unavailable for a tool, its
default theme is tried once; uninstalled apps and tools without an available
fallback are skipped unchanged.
Errors are visible and return a nonzero exit status.
Apply records only result metadata in the app's own state.json; current reports
that historical result, not a live assertion of current application settings.
`

type options struct {
	config      string
	home        string
	tools       string
	toolsSet    bool
	json        bool
	dryRun      bool
	help        bool
	version     bool
	literal     bool
	positionals []string
}

func parse(args []string) (options, error) {
	var opts options
	flags := flag.NewFlagSet("themeset", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.config, "config", "", "")
	flags.StringVar(&opts.home, "home", "", "")
	flags.StringVar(&opts.tools, "tools", "", "")
	flags.BoolVar(&opts.json, "json", false, "")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "")
	flags.BoolVar(&opts.help, "help", false, "")
	flags.BoolVar(&opts.help, "h", false, "")
	flags.BoolVar(&opts.version, "version", false, "")
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			opts.literal = true
			opts.positionals = append(opts.positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			opts.positionals = append(opts.positionals, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-"), "=")
		entry := flags.Lookup(name)
		if entry == nil {
			return opts, fmt.Errorf("unknown flag %q", arg)
		}
		boolean, isBool := entry.Value.(interface{ IsBoolFlag() bool })
		if !hasValue {
			if isBool && boolean.IsBoolFlag() {
				value = "true"
			} else {
				i++
				if i == len(args) {
					return opts, fmt.Errorf("flag --%s requires a value", name)
				}
				value = args[i]
			}
		}
		if err := flags.Set(name, value); err != nil {
			return opts, err
		}
		if name == "tools" {
			opts.toolsSet = true
		}
	}
	return opts, nil
}

// Run executes one command and returns an exit code without terminating the process.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, adapter.All())
}

func run(args []string, stdout, stderr io.Writer, adapters []model.Adapter) int {
	if len(args) > 0 && args[0] == "__complete" {
		for _, candidate := range completeWords(args[1:], adapters) {
			if _, err := fmt.Fprintln(stdout, candidate); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
		}
		return 0
	}
	var opts options
	var query string
	var err error
	picking := len(args) > 0 && args[0] == "__pick"
	if picking {
		var valid bool
		opts, query, valid = pickContext(args[1:])
		if !valid {
			return 2
		}
	} else if opts, err = parse(args); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	command := "help"
	if len(opts.positionals) > 0 {
		command = opts.positionals[0]
	}
	if opts.help {
		command = "help"
	} else if opts.version {
		command = "version"
	}
	if opts.dryRun && command != "apply" {
		fmt.Fprintln(stderr, "error: --dry-run is only valid with apply")
		return 2
	}
	if command == "help" {
		_, err := io.WriteString(stdout, usage)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	if command == "version" {
		if opts.json {
			return emitJSON(stdout, stderr, struct {
				Version string `json:"version"`
			}{Version})
		}
		if _, err := fmt.Fprintln(stdout, Version); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	if command == "completion" {
		if len(opts.positionals) != 2 {
			fmt.Fprintln(stderr, "error: completion expects bash, zsh, or fish")
			return 2
		}
		return printCompletion(opts.positionals[1], stdout, stderr)
	}
	switch command {
	case "apply":
		if len(opts.positionals) > 2 {
			fmt.Fprintln(stderr, "error: apply expects zero or one theme argument")
			return 2
		}
	case "list":
		if len(opts.positionals) != 1 && (len(opts.positionals) != 2 || opts.positionals[1] != "tools") {
			fmt.Fprintln(stderr, "error: list expects no arguments or tools")
			return 2
		}
	case "current":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "error: current expects no arguments")
			return 2
		}
	case "__preview":
		if len(opts.positionals) != 2 {
			fmt.Fprintln(stderr, "error: __preview expects one theme argument")
			return 2
		}
	default:
		fmt.Fprintf(stderr, "error: unknown command %q; use help\n", command)
		return 2
	}
	env, err := environment(opts.home)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if command == "current" {
		result, err := readState(statePath(env))
		if err != nil {
			fmt.Fprintln(stderr, "error: read state:", err)
			return 1
		}
		if opts.json {
			return emitJSON(stdout, stderr, result)
		}
		if result == nil {
			fmt.Fprintln(stdout, "No apply result recorded.")
			return 0
		}
		return printResult(stdout, stderr, *result)
	}
	config, _, err := loadConfig(opts.config, env)
	if err != nil && !(opts.config == "" && errors.Is(err, os.ErrNotExist)) {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	env.Paths = config.Paths
	if command == "list" && len(opts.positionals) == 1 {
		names := themeIDs(config)
		if opts.json {
			return emitJSON(stdout, stderr, names)
		}
		for _, name := range names {
			if _, err := fmt.Fprintln(stdout, name); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
		}
		return 0
	}
	if command == "list" {
		type tool struct {
			ID        string `json:"id"`
			Binary    string `json:"binary"`
			Installed bool   `json:"installed"`
		}
		tools := make([]tool, 0, len(adapters))
		for _, entry := range adapters {
			tools = append(tools, tool{entry.ID, entry.Binary, entry.Installed()})
		}
		sort.Slice(tools, func(i, j int) bool { return tools[i].ID < tools[j].ID })
		if opts.json {
			return emitJSON(stdout, stderr, tools)
		}
		for _, entry := range tools {
			status := "not-installed"
			if entry.Installed {
				status = "installed"
			}
			if _, err := fmt.Fprintf(stdout, "%s\t%s\n", entry.ID, status); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
		}
		return 0
	}
	ids := config.Tools
	if opts.toolsSet {
		ids, err = splitTools(opts.tools)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			if picking {
				return 1
			}
			return 2
		}
	}
	selected, err := selectTools(ids, adapters)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		if picking {
			return 1
		}
		return 2
	}
	if picking {
		return printPickedTheme(opts, config, query, stdout, stderr)
	}
	var theme string
	if command == "apply" && len(opts.positionals) == 1 {
		var proceed bool
		theme, proceed, err = chooseTheme(opts, config, env, selected, stdout, stderr)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		if !proceed {
			return 0
		}
	} else {
		theme = opts.positionals[1]
	}
	if !knownTheme(theme, config) {
		fmt.Fprintf(stderr, "error: unknown theme %q; use list or register it in config\n", theme)
		return 2
	}
	if command == "__preview" {
		return printPreview(theme, config, env, selected, stdout, stderr)
	}
	result := plan(theme, config, env, selected, opts.dryRun)
	stateError := false
	if !opts.dryRun {
		apply(&result, env, selected)
		if err := saveState(statePath(env), result); err != nil {
			fmt.Fprintln(stderr, "error: save state:", err)
			stateError = true
		}
	}
	code := 0
	if opts.json {
		printDiagnostics(stderr, result)
		code = emitJSON(stdout, stderr, result)
	} else {
		code = printResult(stdout, stderr, result)
	}
	if stateError || failed(result) {
		return 1
	}
	return code
}

func emitJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	return 0
}

func printResult(stdout, stderr io.Writer, result Result) int {
	if _, err := fmt.Fprintf(stdout, "Theme: %s\n", result.Theme); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	if result.AppliedAt != "" {
		if _, err := fmt.Fprintf(stdout, "Applied at: %s\n", result.AppliedAt); err != nil {
			fmt.Fprintln(stderr, "error: write output:", err)
			return 1
		}
	}
	applied, planned, skipped, failures := 0, 0, 0, 0
	for _, item := range result.Tools {
		switch item.Outcome {
		case "applied":
			applied++
		case "planned":
			planned++
		case "skipped":
			skipped++
		}
		if item.Outcome == "failed" || item.Error != "" {
			failures++
		}
	}
	if _, err := fmt.Fprintf(stdout, "Summary: %d applied, %d planned, %d skipped, %d failed\n", applied, planned, skipped, failures); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	for _, item := range result.Tools {
		detail := ""
		if item.Error != "" {
			detail = "\t" + item.Error
		} else if item.Reason != "" {
			detail = "\t" + item.Reason
		}
		if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s%s\n", item.Tool, item.Status, item.Outcome, detail); err != nil {
			fmt.Fprintln(stderr, "error: write output:", err)
			return 1
		}
	}
	printDiagnostics(stderr, result)
	return 0
}

// Human diagnostics are derived without changing stored or JSON warning metadata.
func printDiagnostics(stderr io.Writer, result Result) {
	unsupported := 0
	for _, item := range result.Tools {
		normalSkip := item.Status == "unsupported" && item.Outcome == "skipped" && item.Error == ""
		if normalSkip {
			unsupported++
		}
		for _, warning := range item.Warnings {
			if normalSkip && warning == "no verified native or official theme available" {
				continue
			}
			if normalSkip && warning == "no registered custom port; settings unchanged" {
				continue
			}
			fmt.Fprintf(stderr, "warning: %s: %s\n", item.Tool, warning)
		}
		if item.Error != "" {
			fmt.Fprintf(stderr, "error: %s: %s\n", item.Tool, item.Error)
		}
	}
	if unsupported > 0 {
		fmt.Fprintf(stderr, "warning: %d unsupported tools skipped (no native/official or registered custom port); settings unchanged\n", unsupported)
	}
}
