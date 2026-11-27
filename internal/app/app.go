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
	"text/tabwriter"

	"github.com/clang-engineer/themeset/internal/adapter"
	"github.com/clang-engineer/themeset/internal/model"
)

// Version can be set by the release build with -ldflags '-X .../internal/app.Version=...'.
var Version = "dev"

const usage = `Usage: themeset [flags] COMMAND [flags] [THEME]

Commands:
  apply [THEME] Apply a theme, or preview and select interactively when omitted
  current       Show the last apply result (including skips and failures)
  list          List themes with associated project Stars, creation and last-push dates
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
  --all            With list only: include hidden bundled and configured IDs
  --details        With list only: show discovery status; JSON includes full metadata
  --sort MODE      With list or interactive apply: listed, rank, or name
  --dry-run         With apply only: inspect a plan, change no files or state
  --help, -h        Show help

Without THEME, apply requires interactive input and fzf. It previews themes,
then shows the selected plan and asks for confirmation before writing.
Use an explicit THEME with --json or from scripts.

Config supports tools, paths, themes, a default_theme ID (catppuccin-mocha
when omitted), optional listed discovery IDs, and theme_sort (listed, rank,
or name). --sort overrides theme_sort for list or interactive apply without
changing which themes are listed or applied. A theme's radar_repo only
identifies its Radar ranking project; it is not a theme download source.
Omitted tools selects all adapters; tools: [] selects none. Relative paths
and custom file references resolve against the config directory; ~/ uses
--home. Explicit custom ports take precedence over native/official ports.
Invalid custom ports fail rather than silently falling back. If the requested
theme is unavailable for a tool, its
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
	sort        string
	toolsSet    bool
	json        bool
	all         bool
	details     bool
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
	flags.StringVar(&opts.sort, "sort", "", "")
	flags.BoolVar(&opts.json, "json", false, "")
	flags.BoolVar(&opts.all, "all", false, "")
	flags.BoolVar(&opts.details, "details", false, "")
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
	if (opts.all || opts.details) && (command != "list" || len(opts.positionals) != 1) {
		fmt.Fprintln(stderr, "error: --all and --details require list without a subcommand")
		return 2
	}
	if opts.sort != "" {
		if err := themeSort(opts.sort); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		if (command != "list" || len(opts.positionals) != 1) && (command != "apply" || len(opts.positionals) != 1) {
			fmt.Fprintln(stderr, "error: --sort requires list or interactive apply")
			return 2
		}
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
	sortMode := discoverySort(config, opts.sort)
	var ranks radarRanks
	if command == "__preview" || picking || command == "apply" && len(opts.positionals) == 1 ||
		command == "list" && len(opts.positionals) == 1 {
		ranks = ranksFor(loadRadar(env, command == "list"))
	}
	if command == "list" && len(opts.positionals) == 1 {
		names := themeIDs(config)
		if opts.all {
			names = allThemeIDs(config)
		}
		names = sortedThemes(names, sortMode, config, ranks)
		if opts.details {
			details := themeDetails(names, config, ranks)
			if opts.json {
				return emitJSON(stdout, stderr, details)
			}
			table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(table, "THEME\tSTARS\tCREATED\tLAST PUSH\tDISCOVERY"); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			for _, item := range details {
				status := "hidden"
				if item.Listed {
					status = "listed"
				}
				stars := "-"
				if item.RadarRank != 0 {
					stars = formatStars(item.RadarStars)
				}
				if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", item.ID, stars,
					projectDate(item.ProjectCreatedAt), projectDate(item.ProjectPushedAt), status); err != nil {
					fmt.Fprintln(stderr, "error:", err)
					return 1
				}
			}
			if err := table.Flush(); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			if _, err := fmt.Fprintf(stdout, "\nRadar GitHub projects (%s): Stars, repository creation and last code push; variants may share dates.\n'-' means unavailable. Use --all for hidden registered themes; --details --json for tools and support sites.\n", ranks.UpdatedAt[:10]); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			return 0
		}
		if opts.json {
			return emitJSON(stdout, stderr, names)
		}
		if _, err := fmt.Fprintf(stdout, "THEME\tGITHUB STARS (%s)\tPROJECT CREATED\tLAST PUSH\n", ranks.UpdatedAt[:10]); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		for _, name := range names {
			standing := ranks.standing(name, config)
			label := "-"
			if standing.Rank != 0 {
				label = formatStars(standing.Stars)
			}
			if _, err := fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", name, label,
				projectDate(standing.CreatedAt), projectDate(standing.PushedAt)); err != nil {
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
		return printPickedTheme(opts, config, ranks, query, stdout, stderr)
	}
	var theme string
	if command == "apply" && len(opts.positionals) == 1 {
		var proceed bool
		theme, proceed, err = chooseTheme(opts, config, ranks, env, selected, stdout, stderr)
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
		return printPreview(theme, config, ranks, env, selected, stdout, stderr)
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
	if err := printSelectionPlan(stdout, result); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	// Fallbacks and ordinary skips are already visible in the grouped result.
	// Keep unusual warnings and failures on stderr without repeating each row.
	for _, item := range result.Tools {
		for _, warning := range item.Warnings {
			normalSkip := item.Status == "unsupported" && item.Outcome == "skipped" &&
				(warning == "no verified native or official theme available" || warning == "no registered custom port; settings unchanged")
			normalFallback := item.FallbackTheme != "" &&
				warning == fmt.Sprintf("theme %s unavailable for %s; using default theme %s", result.Theme, item.Tool, item.FallbackTheme)
			if normalSkip || normalFallback {
				continue
			}
			fmt.Fprintf(stderr, "warning: %s: %s\n", item.Tool, warning)
		}
		if item.Error != "" {
			fmt.Fprintf(stderr, "error: %s: %s\n", item.Tool, item.Error)
		}
	}
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
