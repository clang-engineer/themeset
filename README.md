# themeset

Apply existing application themes together. Use explicitly registered per-tool
overrides first, otherwise select installed native themes or fetch current
official ports, and skip unsupported tools without changing their settings.

**No palette generation, approximate substitutions, plugin installation, or
shell commands from configuration.**

## Install

Build locally with Go 1.23 or newer:

```sh
go build -o themeset ./cmd/themeset
install -m 755 themeset "$HOME/.local/bin/themeset"
```

Install the published release through Homebrew:

```sh
brew install clang-engineer/tap/themeset
```

The executable does not require Go on the user's machine. Installing it does not
apply a theme. Supported release targets are macOS/Linux on arm64/amd64; macOS
runtime behavior has been exercised locally, while Linux runtime validation
belongs to CI and application-specific testing.

## Use

```sh
themeset list
themeset list --all
themeset list --all --details
themeset list --sort rank
themeset list tools
themeset apply
themeset apply dracula --dry-run
themeset apply dracula
themeset current
```

- `apply --dry-run` inspects without writing app configuration or result state.
- `--tools ghostty,bat,fzf` restricts the selected tools.
- `--json` emits structured output, keeping diagnostics on stderr.
- `--home /path/to/scratch` selects that home and its `.config`, ignoring inherited
  XDG config paths. Explicit `paths` overrides still apply, including absolute
  paths outside that home: this is **not a security sandbox**. Installed binaries
  are still discovered through PATH. For smoke runs, use a scratch directory and
  check that any configured destinations also point into it.
- Flags are accepted before or after positional arguments.

`apply` without a theme first opens a group picker: All, Dark, Light, or Other
(configured themes). Enter opens the corresponding theme picker, with themes
on the left and a sourced visual preview and per-tool support on the right.
The picker ignores `FZF_DEFAULT_OPTS` and `FZF_DEFAULT_OPTS_FILE`;
Tab/Shift+Tab do not mark multiple themes. Enter reviews the selected theme's
plan; a separate `[y/N]` confirmation is required before any configuration/state
write.
Escape or a declined confirmation changes nothing. `apply --dry-run` uses the
picker and shows the plan without confirmation or writes. Interactive selection
needs a terminal and fzf (Homebrew installs it); `--json` requires an explicit
theme and remains suitable for scripts.

Visuals use an installed bat syntax theme or an actual native/custom Ghostty
asset, labeled with source app and exact variant. Ghostty swatches and ANSI
samples are not a promise of editor syntax colors. Missing source assets
explicitly show **Preview unavailable**. The preview, confirmation, and human
apply result group tools by requested theme, `default_theme` fallback, skips,
and errors; fallback rows name the theme actually used without repeating
warnings. The review makes no changes until `[y/N]` is confirmed. JSON and
recorded result metadata retain individual statuses and warnings.

Core commands are `apply`, `list`, and `current`. `list` shows curated
theme IDs with their associated GitHub project Stars (`-` for themes without a
Radar association); `list --all` also shows hidden bundled and configured IDs
that can be passed explicitly to `apply`. Neither option adds themes to the
picker or shell completion. `list --details` adds a `listed`/`hidden` marker,
appearance, declared tool IDs, and a known upstream support site (`-` when
absent). Only explicitly identified light themes show `light`: `ayu-light`,
`catppuccin-latte`, `flexoki-light`, `kanagawa-lotus`, `onehalf-light`,
`rose-pine-dawn`, `solarized-light`, `tokyo-night-day`, and `tomorrow`.
Unspecified appearance does not necessarily mean dark; custom theme
appearance is not guessed. The All picker view shows `` followed by the
GitHub Stars count, with an explicit sort label, and marks light themes with
`[light]`; Dark, Light, and Other remain available as separate groups. The
GitHub glyph requires a Nerd Font; Stars and sort direction remain readable
in text without it. Stars are display metadata: selection still returns the
exact theme ID and does not change which tools can apply it.
`--json` keeps the existing string array without `--details`; with it, each
entry has `id`, `listed`, `tools`, optional `appearance` and `support_site`,
and, when mapped, `radar_stars`, `radar_rank`, and `radar_updated_at`. Declared
tools are catalog/config metadata, not proof that an app or theme is installed.
`list tools` shows registered tools and installation status. The former
`check` and top-level `tools` commands have been removed: use
`apply THEME --dry-run` and `list tools`, respectively.

Stars come from the [Radar Developer Color Themes ranking](https://clang-engineer.github.io/data/rankings.json)
(GitHub Stars, all-time). They are counts for the associated GitHub project,
not separate counts for each theme variant. The bundled offline snapshot
contains the full category, not only the top 30; older registered themes
such as Rosé Pine, Moonfly, and Ayu remain usable by explicit ID and appear
under `list --all`, but are hidden from the default shortlist. When a valid
cache is younger than 24 hours, `list` does not fetch again; otherwise it
checks for updated data (three-second timeout). The last valid category is
cached under `~/.cache/themeset/radar-ranks.json`. Interactive `apply` only
reads that cache; neither previews nor `--home` targeting another home fetch
it. Without a usable network/cache, the bundled snapshot is used. The
displayed date is the source data's update date, not the time themeset checked
it. Counts and order can change when Radar updates; some projects are ports
of already listed families and others target unsupported applications.
Bundled project associations live alongside the entries in
`internal/catalog/defaults.json`'s `listed` array. Hidden bundled IDs are
not mapped and show `-` under `list --all` unless the user supplies
`themes.<id>.radar_repo`. A name alone does not establish a shared source
(e.g. `vscode-dark` is not assigned an unrelated port's Stars).

## Shell completion

Homebrew installs Bash, Zsh, and Fish completion definitions in its standard
completion directories. Restart your shell after installing or upgrading.
The default completion menu shows only `apply`, `current`, and `list`. Options
appear after `-`; auxiliary `completion`, `help`, and `version` appear when
their names are started. Completing `apply` offers theme IDs (including custom
config entries); `list` offers `tools`. Comma-separated `--tools` values and
paths are also completed. Completion never applies themes.

For a source build, or to load completion into an existing session:

```sh
# Bash
source <(themeset completion bash)
# Zsh, after compinit
source <(themeset completion zsh)
# Fish
themeset completion fish | source
```

Use `themeset <Tab>` for command names. Tab always retains the shell's existing
completion behavior. In interactive Fish, only Shift+Tab while completing the
first theme in `themeset apply ` opens the sourced preview picker.
Enter fills the selected theme into the command line only; it never
executes `apply`. Escape leaves the line unchanged. Already supplied themes,
flags, file paths, and other commands retain their previous key handlers.

Homebrew installs an interactive Fish startup hook so this works on the first
Shift+Tab of a fresh shell, not just after lazy-loading completions. Existing
sessions can load it with the Fish `source` command above. When upgrading from
v0.2.2, start a fresh Fish session (`exec fish`) to discard its old Tab wrapper.
The Shift+Tab wrapper preserves previous bindings outside the theme position;
Bash/Zsh completion remains unchanged.
In Zsh, Homebrew's completion directory must be in `fpath` before `compinit`.

## Resolution and results

1. Skip missing executables.
2. Use an explicitly registered custom port for the exact **theme × tool**
   combination. Invalid overrides fail; they never silently fall back.
3. Otherwise resolve the installed app's native theme/provider or a registered
   current official port.
4. If unavailable, try `default_theme` for that tool (built-in default:
   `catppuccin-mocha`). If the fallback is also unavailable, warn and skip,
   leaving its settings unchanged. Inspection/apply errors never trigger a
   fallback; other selected tools continue and the command returns nonzero.

Support is reported as `native`, `official`, `custom`, `unsupported`,
`not-installed`, or `error`. Application outcomes are separate: `planned`,
`applied`, `skipped`, or `failed`. `current` shows the last recorded application
result, including partial failures; it does not assert that every app currently
matches the last requested theme.

Human-readable results include a per-tool table and a summary of applied, planned,
skipped, and failed tools. A fallback is labeled with its actual theme ID in
`fallback_theme` and a warning; `current` preserves that metadata. A skipped
row explains why no compatible port was found. Inspection/apply errors remain
visible and make the command fail.

Ghostty saves its theme selector and, on macOS, requests `reload_config`
through Ghostty's native AppleScript API when the app is running for the
current account. `--home` pointing at a scratch home never reloads the user's
real app. Automation permission may be required; a denied request reports an
error after saving the file. `applied` confirms the action acknowledged the
request, not that every window visually refreshed. On Linux, press
`Ctrl+Shift+,` or open a new window. Automatic `SIGUSR2` hard reload is not
used: on Ghostty 1.4.0 it can desynchronize attached tmux clients.

The bundled registry in `internal/catalog/defaults.json` includes Dracula,
Catppuccin, Gruvbox, Tokyo Night, Nord, Kanagawa, Everforest, Rose Pine,
Solarized Light, Nightfox, and One Dark, alongside other supported IDs.
The curated default shortlist consists of supported themes associated with
Radar's top 30 projects; outside it, Rose Pine, Moonfly, Ayu Light, and
VS Code Dark remain registered for explicit use and `list --all`. Among the
top 30, Tomorrow (day/night), Molokai, Night Owl, PaperColor, Iceberg, OneHalf
(dark/light), Jellybeans, and Seoul256 have registered installed-native
candidates. Ghostty supplies the matching Tomorrow, Molokai, Night Owl,
Iceberg, and Jellybeans themes; bat supplies OneHalf Dark/Light. Neovim
candidates require an independently installed colorscheme; themeset does
not install executable editor plugins. Unsupported tools still follow the
configured fallback. Additional ranked repositories such as Catppuccin
Neovim/tmux, Gruvbox Neovim, Nord Vim, and Vim One are ports of already listed
families, not new selectable themes. Islands Dark targets VS Code-compatible
editors, Spacegray targets Sublime Text UI, and Material Design targets
iTerm2; none has a verified themeset tool port. These projects do not get
an unrelated same-named theme ID.
`github-dark` is in the default list. Nightfox
declares upstream Ghostty, Starship palette-only, and tmux color-style-only
ports, plus the installed Neovim `nightfox` colorscheme. Other tools use the
configured fallback where no Nightfox port exists. Catppuccin Latte, Frappé,
and Macchiato select their matching Neovim colorscheme when the plugin is
installed; themeset does not install it. Each bundled `listed` entry has an
`id` and, where available, a `support_site` pointing to the project's port
directory and `radar_repo` identifying its family in Radar. The separate
`light` metadata includes both curated and hidden bundled IDs.
These links are reference metadata, not verified themeset ports or download
sources. The catalog also declares each tool's installed-native candidate names
and, where available, a verified upstream port. Listed IDs feed `list`, shell
completion, and the interactive picker; the shortlist is not a promise that
every tool supports each theme.
Without a `listed` override, IDs registered under `themes` in the user config
are added to the built-in shortlist. The default `list` and All picker view
sort by Radar rank, with unranked IDs last; tied ranks keep the original
discovery order. A user `listed` array replaces the displayed shortlist exactly
and preserves its order by default. `theme_sort` can explicitly choose `rank`,
`listed`, or `name` in either case; `--sort` overrides it for one `list` or
interactive `apply`. The `listed` mode groups dark, light, and other entries
in the All picker view, preserving order within each group. The `rank` and
`name` modes order that view globally; group-specific views still show only
their group. Sorting never changes visibility or application. Completion
sorts suggestions independently. `[]` hides every discovery entry.
Hidden IDs remain usable with `apply THEME` and may still serve as
`default_theme`; find their exact names with `list --all`. `listed` changes
default discovery only, not bundled ports or sources.
For example, `cyberdream` is hidden from the default picker but has installed
native candidates for bat and Neovim and verified upstream ports for btop,
Ghostty, lazydocker, lazygit, and Yazi. Its Delta upstream feature requires a
separately installed Bat syntax theme and is not registered as an automatic
port; unregistered tools retain the documented fallback behavior.

tmux supports upstream plugins for
[Catppuccin](https://github.com/catppuccin/tmux) (all four flavors),
[Rosé Pine](https://github.com/rose-pine/tmux) (main, Moon, Dawn),
[Dracula](https://github.com/dracula/tmux),
[Nord](https://github.com/nordtheme/tmux),
[Gruvbox](https://github.com/egel/tmux-gruvbox) (dark256), and
[Everforest](https://github.com/TanglingTreats/tmux-everforest) (dark-medium).
On `apply`, themeset reuses a complete installed plugin or clones its registered
upstream repository to `$XDG_CONFIG_HOME/tmux/plugins/OWNER/REPO/` (normally
`~/.config/tmux/plugins/OWNER/REPO/`). It checks required files in a temporary
directory before moving the clone into place. An incomplete existing directory
is never overwritten: repair it manually if installation fails. No plugin is
downloaded by `list`, preview, or `apply --dry-run`. Git and network access are
required only when a plugin is missing. Catppuccin is installed at tag v2.3.2
and requires tmux 3.2+; other plugins use their upstream default branch at
installation time and are not automatically updated. An existing TPM plugin at
`~/.tmux/plugins/REPO` is reused when complete; plugins named `tmux` share that
TPM directory, while themeset's separate XDG paths allow multiple families.
These plugins run upstream shell code on the next tmux config load and may
execute additional status commands. Themeset does not copy or generate their
colors. To avoid executing third-party code, use `--dry-run` or choose a
style-only theme or an explicitly trusted `custom.file` instead.

Applying a plugin writes a flavor selector and an upstream `run-shell` include
to the selected tmux config. When invoked inside tmux for the current user's
home, themeset automatically reloads **that tmux server** after saving. If the
top-level config directly includes the managed file, it reloads that config
first, restoring the base layout (including Oh my tmux!) before applying the
new theme. Otherwise it reloads only the changed file. A plugin can replace
the existing status-line layout; load themeset after other status owners
(and TPM) to give it precedence. `--home` targeting a scratch home, commands
outside tmux, and `--dry-run` do not reload a server. A failed reload reports
an error after saving the theme file.

Switching to Moonfly, Tokyo Night, or Nightfox replaces the managed include
with upstream styles only. If the top-level config does not directly include
the managed file, those styles cannot undo a full plugin's status layout on
the running server; reload the base layout or restart the server. Other tmux
themes can use `custom.file`.

`kanagawa` selects the Wave variant with registered btop, eza, Ghostty, and
Yazi ports; Bat and Delta use a previously installed `Kanagawa` syntax theme
only. `everforest` selects dark-medium ports for btop, fzf, and Yazi.
The `github-dark` and hidden `modus-vivendi` fzf presets require fzf 0.63.0 or
newer for their footer colors; older fzf versions fail before changing options.
The hidden `gruvbox-dark` and `tokyonight` declarations contain native selectors
only; use the listed `gruvbox` and `tokyo-night` IDs for their wider registered
port coverage. Atuin-only built-in names do not imply themes for other tools.

Use an empty theme object to register an extra ID without custom ports.
User custom entries replace the bundled declaration for that theme and tool;
other tools remain unchanged. Custom URLs are never labeled official.

Bundled `listed` entries are objects; the user config's `listed` override
remains an array of theme ID strings. Bundled ports are grouped by
`themes.<theme>.<tool>` so each theme's supported tools are together.
Official ports infer `kind: official` and, unless an upstream filename
differs, their name from the theme key. Keep `native` candidates, `format`,
`variant`, and companion URLs only where the adapter actually uses them.
This does not change user config: overrides still use
`themes.<theme>.custom.<tool>`.

## Integrations

For each tool's source, installation/placement, activation, reload, preservation
rules, and current implementation gaps, see the
[adapter lifecycle reference](theme-adapters.md).
Bundled official assets are fetched over HTTPS on each apply; adapters validate
their native formats before changing settings. Installed native themes and custom
ports remain local. Rosé Pine uses Main-variant sources; Tokyo Night uses Night.
The bundled catalog owns theme IDs, per-tool installed names, and official
source URLs. Each tool package checks availability and applies the declared port
using its native format; `internal/adapter/shared/` supplies common I/O and
download guards.

| Tool | Existing support discovery | Custom input |
|---|---|---|
| Ghostty | Installed `+list-themes` names or verified theme-author color files | Native Ghostty theme asset |
| Neovim | Installed colorscheme runtime files | Existing Lua/Vim theme script |
| tmux | Fetch current Moonfly/Tokyo Night styles; no general native registry | Trusted tmux theme config |
| bat | Installed syntax-theme list, including Rosé Pine when present | bat config containing `--theme` |
| delta | Installed syntax-theme list or registered Dracula, Rosé Pine, and Catppuccin features | Gitconfig containing delta theme settings |
| Harlequin | Installed CLI's supported theme names | Native TOML theme/profile settings |
| Starship | Existing named palettes or registered Dracula, Moonfly, Catppuccin flavors, Gruvbox Dark, and Rosé Pine palettes | Selected palette in Starship TOML |
| fzf | Registered Dracula, Moonfly, Catppuccin flavors, Gruvbox Dark, Nord, Rosé Pine, and Tokyo Night colors | Color-only native options file |
| Yazi | Installed complete flavors or registered Dracula, Rosé Pine, Tokyo Night, and Catppuccin files | Native `theme.toml` fragment |
| Atuin | Shipped/installed themes or registered Catppuccin flavors and Rosé Pine files | Native theme TOML asset |
| lazygit | Registered Dracula, Catppuccin flavors, Rosé Pine, and Tokyo Night settings | `gui.theme` YAML fragment |
| lazydocker | Registered Dracula settings | `gui.theme` YAML fragment |
| btop | Installed user/system themes or registered Catppuccin flavors, Dracula, Gruvbox Dark, Nord, Rosé Pine, and Tokyo Night files | Native `.theme` file |
| eza | Registered Dracula, Gruvbox Dark, Catppuccin flavors, Rosé Pine, and Tokyo Night YAML | Native `theme.yml` fragment |

Atuin does not ship all of Harlequin's similarly named Textual themes. tmux does
not have a standard built-in family-name selector. These cases are reported
honestly rather than replaced by newly synthesized colors.

Official ports are applied through the application's native settings/assets;
no separate theme installer script is needed. Downloads fetch current upstream
files on every apply and require network access.
Native discovery inspects fixed app commands or local theme files. `native`
means available through the app; it does not certify
theme-author provenance. `official` includes application-maintained recipes
for Gruvbox eza/Starship and btop's shipped colors; it is not a claim that
Gruvbox's theme author maintains those repos. Starship's palette selection
does not recolor modules that hardcode colors instead of using palette names.

Catppuccin fzf requires fzf 0.52.0 or newer; resolution and apply report an
error for older versions before writing its options. Gruvbox/Nord use stable
color option names, not the newer layout options shown in the fzf web generator.
Catppuccin Atuin installs and selects each declared `catppuccin-<flavor>-mauve`
name. Yazi imports only UI style tables and installs the corresponding
Catppuccin syntax theme; separators, icons, file rules, layouts, and keymaps
remain intact.

### Acquiring registered assets

`apply` fetches registered files from the official project's current branch
over HTTPS each time. Downloads are size-bounded and parsed as data, never
executed. A failed download or malformed file leaves the previous selection
unchanged. This trades reproducibility and offline reapplication for current
upstream colors. Dry-run does not download.

- **Yazi:** installs `flavor.toml`, `tmtheme.xml`, and upstream license files from
  the application-maintained `yazi-rs/flavors` Dracula package beneath the effective
  config directory's `flavors/dracula.yazi/`. Selects `[flavor] dark/light =
  "dracula"` in `theme.toml`; honors `YAZI_CONFIG_HOME` or `paths.yazi`.
  Compatible independently installed flavors are reused, not replaced.
  Switching from a themeset-managed Catppuccin import removes only matching
  previous style values, preserving user edits and layout. New applies record
  the imported values for precise future cleanup; legacy installs without that
  record compare current upstream values and may retain stale historical styles.
- **eza:** fetches `eza-community/eza-themes`' native Dracula or Gruvbox Dark
  YAML, validates the theme structure, and merges styles into `theme.yml`,
  preserving unrelated filename/icon rules. Honors `EZA_CONFIG_DIR` or
  `paths.eza`. Existing `LS_COLORS`/`EZA_COLORS` overrides can affect rendering.
- **btop:** reuses installed user/system themes; otherwise fetches registered
  Catppuccin flavors or btop's Dracula/Gruvbox Dark/Nord files, installs the
  native `.theme` under the effective user theme directory, then selects it in
  `btop.conf`. Relaunch btop to see the change.
- **Starship:** fetches a registered Catppuccin or Rosé Pine palette or Starship's
  Gruvbox Rainbow preset, imports only the selected palette, then changes
  `palette`. It never replaces a custom prompt layout or module configuration.

Fetch/validation/install failures remain errors and do not change the prior theme
selection. A conflicting independent Yazi flavor is not silently overwritten;
themeset-owned flavors carry a source marker and refresh from upstream.
Yazi Dracula and eza's files are application-maintained, not necessarily
theme-author ports. Relaunch Yazi; eza and Starship use their new settings
on the next invocation/prompt.

Dracula has no registered Atuin source; tmux uses its installed upstream
plugin rather than a downloaded color file.

Changes take effect on the next application launch or prompt/invocation; running
apps are not forcibly restarted. Neovim's generated selection runs on VimEnter.
Existing CLI flags, environment overrides, project-local profiles, and user
startup code can still override an application's normal config.

### Moonfly provenance and scope

The [official Moonfly extras](https://github.com/bluz71/vim-moonfly-colors/tree/master/extras)
are read from the author's current branch on apply:

- **fzf:** upstream color options are extracted, with shell export syntax removed
  and `--color VALUE` expressed as `--color=VALUE`. Ordinary picker options and
  shell startup files are not changed.
- **Starship:** only the selected upstream `[palettes.moonfly]` is imported and
  selected, just as for Dracula. Prompt/module formats, symbols, truncation, and
  other palettes remain intact. Only the colors defined by the upstream palette
  are used; no missing colors are invented. Existing styles use this palette
  only when they reference those names.
- **tmux:** only upstream style commands (message, pane borders,
  status, window activity, and current-window styles) are imported. Upstream's
  status-left/right, window formats, and shell conditional are deliberately
  omitted to preserve layouts and keymaps. A dedicated
  `$XDG_CONFIG_HOME/themeset/tmux-moonfly.conf` (default
  `~/.config/themeset/tmux-moonfly.conf`) is included after the existing config.
  A `paths.tmux` override can target an existing managed theme include, as in an
  Oh my tmux! wrapper. Oh my tmux!'s own layout and embedded segment colors are
  not remapped; inline colors can still override the imported style fields.
  The permitted and required style keys for each official tmux port live in
  `internal/catalog/defaults.json`; the tmux adapter applies one shared parser.

Ghostty and Neovim continue to use installed native Moonfly providers. Other
applications remain unsupported unless a real installed/native or explicitly
registered custom port exists. The matching but distinct `fly16` bat theme is
**not** treated as Moonfly. The current upstream sources are listed in
[third-party notices](THIRD_PARTY_NOTICES.md).


### fzf activation

The default output is `~/.config/themeset/fzf.opts` (under `XDG_CONFIG_HOME` when
set). It contains only color options. Point fzf at it in your shell:

```sh
export FZF_DEFAULT_OPTS_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/themeset/fzf.opts"
```

Do this after first applying an fzf-supported theme, so the file exists. Keep
ordinary picker options in `FZF_DEFAULT_OPTS`/`FZF_CTRL_T_OPTS` as before. If you
already use a different options file, do not silently replace its non-theme
options: move those options into the existing environment configuration first,
or explicitly configure a dedicated theme-only target file. Color imports reject
`--bind`, `--preview`, shell code, and arbitrary non-theme options.

## Configuration

Default: `$XDG_CONFIG_HOME/themeset/config.json`, otherwise
`~/.config/themeset/config.json`. Override with `--config /path/to/config.json`.

```json
{
  "tools": ["ghostty", "bat", "fzf", "lazygit"],
  "listed": ["dracula", "catppuccin-mocha", "my-mocha"],
  "theme_sort": "rank",
  "default_theme": "catppuccin-mocha",
  "paths": {
    "lazygit": "~/Library/Application Support/lazygit/config.yml"
  },
  "themes": {
    "dracula": {
      "custom": {
        "lazygit": {
          "file": "ports/dracula/lazygit.yml",
          "source": "My registered lazygit Dracula settings"
        }
      }
    },
    "my-mocha": {
      "radar_repo": "catppuccin/catppuccin",
      "custom": {
        "fzf": {
          "url": "https://raw.githubusercontent.com/catppuccin/fzf/main/themes/catppuccin-fzf-mocha.rc"
        }
      }
    }
  }
}
```

You can design your own colors and override a bundled theme or register a new
theme ID. Write a separate theme/settings file in each application's native
format, then register it under `themes.<id>.custom.<tool>`. For example, to use
your own fzf colors for Dracula, set
`"themes": {"dracula": {"custom": {"fzf": {"file": "ports/dracula/fzf.rc"}}}}`.
The file must already exist; themeset does not generate app themes from a single
shared palette. Custom `file` entries reference app-native theme/settings files.
Alternatively, `url` fetches an HTTPS file at each apply and uses the same
tool-native parser; `file` and `url` cannot both be set. This works
for fzf, Starship, Yazi, Atuin, lazygit, lazydocker, btop, and eza. Yazi's remote
styles additionally require a `syntax_url` pointing at a complete `.tmTheme`.
Remote Lua/Vim/tmux or other executable app configs are rejected. A URL configured
by the user is reported as **custom**, not verified as an official theme.
`source` remains optional human-readable provenance, not a download URL.
Relative `file` paths resolve against the JSON config directory; `~/`
uses the selected home. For remote ports, `name` defaults to the theme ID;
set it explicitly if a Starship file declares a differently named palette.
For local ports, `name` may select an existing app theme. Omit `tools` for all
adapters; `"tools": []` means none. Canonical Neovim ID is `neovim`. Unknown
JSON fields and unknown selected tool IDs are errors.

Check the chosen source with `themeset apply dracula --dry-run` before applying;
this inspects local files but does not change app settings. A missing or invalid
custom file fails rather than falling back to the bundled theme.

`listed` optionally replaces the built-in discovery list for `themeset list`,
completion, and the picker. It is **not** a source declaration or a restriction
on explicit `apply THEME`. If omitted, the built-in shortlist plus IDs in
`themes` is used. If present, include custom IDs explicitly to display them;
`"listed": []` displays none. `list tools` is unchanged.

`themes.<id>.radar_repo` optionally identifies the exact `owner/repo` in Radar
for a user-defined theme. It only supplies rank metadata: it never downloads
ports or grants tool support. Use it only for the same theme family, not a
similar-looking palette. Bundled `listed` entries already map to their
upstream projects; an explicitly configured `radar_repo` overrides that
association.

`default_theme` sets the theme used for a tool only when the requested theme
has no available native, official, or custom port for that tool. It defaults to
`catppuccin-mocha`; set it to a bundled or user-defined theme ID to override.
It is tried once, never after an inspection or apply error. If that theme is
also unavailable for the tool, the tool is skipped unchanged.

Explicit `themes.<id>.custom.<tool>` entries override even available native or
official ports. An invalid custom file is an error, not permission to use other
colors. Use an empty theme object to register an extra ID without custom ports.
Remote files must use HTTPS without URL credentials; downloads are size-bounded,
checked as tool-native data, and never executed. Dry-run does not download.
Malformed downloads fail without falling back to another theme. Review third-party
URLs yourself: registering one does not make its maintainer trustworthy.

Explicit `paths` override native destination files. macOS lazygit/lazydocker and
eza follow their native Library paths; existing legacy lazy-tool vendor paths
are recognized. Atuin preserves an existing legacy XDG config, otherwise uses
its native `~/.atuin` location. Harlequin uses an existing native config or seeds
`~/.harlequin.toml`; point its override at an explicit generated catalog when
using `HARLEQUIN_CONFIG_PATH`.

Only relevant theme sections/keys are imported. Unrelated active values and file
modes are preserved; structured TOML/YAML rewrites normalize comments/formatting.
Existing symlinks are retained while their target config is updated atomically.
Dedicated managed theme assets can be replaced. Custom Lua/Vim/tmux files are
explicitly trusted executable application configuration: do not register files
from untrusted sources.

A dotfile manager must not also render over the same selected theme keys. Use
native includes or dedicated runtime-owned assets, and let the dotfile manager
distribute this JSON configuration instead. The sibling dotfiles repository has
an integration using those ownership boundaries.

## Homebrew releases

```sh
sh scripts/package.sh v1.0.0 clang-engineer/themeset
```

This produces macOS/Linux arm64/amd64 archives, `dist/checksums.txt`, and
`dist/themeset.rb`. Formula hashes are computed from those actual archives, not
placeholders. Archives include this README, the license, and third-party
notices. Binary version is set from the supplied tag.

Pushing a version tag runs `.github/workflows/release.yml`. It tests the Go code,
builds archives, generates the formula, and uploads release assets. To also
update an existing Homebrew tap, set:

- Repository variable `HOMEBREW_TAP_REPOSITORY`: `OWNER/homebrew-tap`.
- Repository secret `HOMEBREW_TAP_SSH_KEY`: a private SSH deploy key whose
  matching public key has write access **only to that tap**.

Do not store a personal access token or credentials in source. The workflow loads
the deploy key into a temporary SSH agent and obtains GitHub's host keys over
HTTPS. A manual tap update uses your existing SSH authentication:

```sh
ruby scripts/publish-tap.rb OWNER/homebrew-tap dist/themeset.rb
```

Users install through `brew install OWNER/tap/themeset` and update with
`brew upgrade`. Publishing scripts are explicit release operations; ordinary
builds, checks and applies do not create repositories, upload files, or access
GitHub credentials.

## Development

```sh
go test ./...
go vet ./...
```

CI runs the suite on macOS/Linux. Runtime smoke checks must also execute installed
applications with an isolated home, never a user's real configuration.

## License

MIT. Upstream theme files are downloaded rather than redistributed; source and
license references are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
