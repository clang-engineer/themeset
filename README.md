# themeset

**One command to coordinate themes across your terminal tools.**

Switch between existing themes across Ghostty, Neovim, tmux, fzf, bat,
Starship, lazygit, Yazi, and more. Preview the plan before changing settings.

```sh
brew install clang-engineer/tap/themeset
themeset apply dracula --dry-run
```


Apply existing application themes together. Use explicitly registered per-tool
overrides first, otherwise select installed native themes or fetch current
official ports, and skip unsupported tools without changing their settings.

**No palette generation, approximate substitutions, or shell commands from
configuration.** Neovim plugins are not installed; registered tmux plugins may
be cloned when a theme is applied.

![Interactive theme selection, native preview, and dry-run plan](docs/demo.gif)

The [recording script](docs/demo.tape) contrasts Tomorrow's light and dark
**native previews**, then shows a Dracula dry-run plan. The terminal itself
does not switch themes: the script uses a disposable home and changes no app
settings. To regenerate: `vhs docs/demo.tape` (requires VHS, fzf, bat, and
Ghostty for the light/dark previews).

## Install

Homebrew (macOS/Linux, arm64/amd64):

```sh
brew install clang-engineer/tap/themeset
themeset --version
```

The formula installs `fzf` for the interactive picker. To upgrade an existing
installation, run `brew upgrade clang-engineer/tap/themeset`.

Build locally with Go 1.23 or newer instead:

```sh
go build -o themeset ./cmd/themeset
install -m 755 themeset "$HOME/.local/bin/themeset"
```

Installing does not apply a theme. Homebrew binaries do not require Go on the
user's machine. macOS behavior has been exercised locally; Linux release builds
and tests run in CI, while application-specific Linux behavior may vary.

## Use

### Quick reference

```sh
themeset list                    # curated themes, project Stars and dates
themeset list --sort name        # alphabetical instead of default Stars order
themeset list --all --details    # hidden themes in an aligned comparison table
themeset list tools              # installed tool status
themeset apply                   # select a theme and review the plan
themeset apply dracula --dry-run # inspect without applying
themeset apply dracula           # review, then confirm changes
themeset current                 # last recorded result
themeset rollback                # restore pre-Themeset files
```

| Option | Behavior |
|---|---|
| `apply --dry-run` | Inspects and shows the plan without writing app configuration or result state. |
| `--tools ghostty,bat,fzf` | Restricts the selected tools. |
| `--json` | Emits structured output; diagnostics stay on stderr. Requires an explicit theme for `apply` (suitable for scripts). |
| `--home /path/to/scratch` | Selects that home and its `.config`, ignoring inherited XDG config paths; installed binaries are still found through PATH. |

Flags work before or after positional arguments. **`--home` is not a security
sandbox:** explicit `paths` overrides still apply, even absolute destinations
outside the selected home. For smoke runs, use a scratch directory and check
that configured destinations also point into it.

### Interactive selection and dry-run

- `apply` without a theme opens an All / Dark / Light / Other (configured
  themes) group picker. Enter opens a theme picker: themes on the left, sourced
  visual preview and per-tool support on the right.
- The picker needs a terminal and fzf (installed by Homebrew). It ignores
  `FZF_DEFAULT_OPTS` and `FZF_DEFAULT_OPTS_FILE`; Tab/Shift+Tab do not mark
  multiple themes.
- Enter reviews the selected theme's plan. Only a separate `[y/N]`
  confirmation writes configuration or result state. Escape or declining
  changes nothing. `apply --dry-run` also uses the picker, but shows the plan
  without confirmation or writes.
- Visuals come from an installed bat syntax theme or actual native/custom
  Ghostty asset, labeled with source app and exact variant. Ghostty swatches
  and ANSI samples do not promise editor syntax colors; absent source assets
  show **Preview unavailable**.
- Preview, confirmation, and human apply result group tools by requested theme,
  `default_theme` fallback, skips, and errors. Fallback rows identify the theme
  actually used without repeating warnings. JSON and recorded results retain
  per-tool statuses and warnings.

### Discovering themes

| Command | What it shows |
|---|---|
| `list` | Curated IDs, associated GitHub project Stars, project creation date, and last code-push date (`-` when unavailable). |
| `list --all` | Also includes hidden bundled/configured IDs usable by exact `apply THEME`; it does **not** add them to the picker or shell completion. |
| `list --details` | Aligned comparison table with `listed`/`hidden` discovery status. For appearance, declared tools and support sites, use `list --details --json`; declared tools do not prove installation. |
| `list tools` | Registered tools and installation status. |
| `current` | Last recorded application result, not a live audit. |

- `--json` without `--details` keeps the theme-ID string array. With
  `--details`, entries have `id`, `listed`, `tools`, optional `appearance` and
  `support_site`, and, when mapped, `radar_stars`, `radar_rank`,
  `radar_updated_at`, `project_created_at`, and `project_pushed_at`. Project
  dates are UTC RFC 3339 timestamps; unavailable dates are omitted.
- Only explicitly identified light themes show `light`: `ayu-light`,
  `catppuccin-latte`, `flexoki-light`, `kanagawa-lotus`, `onehalf-light`,
  `rose-pine-dawn`, `solarized-light`, `tokyo-night-day`, and `tomorrow`.
  Unspecified appearance need not mean dark; custom appearance is not guessed.
- All shows `` and the GitHub Stars count, an explicit sort label, and `[light]`
  on light themes. Dark, Light, and Other have separate group views. The glyph
  needs a Nerd Font, but Stars and sort direction are readable without it.
  Stars only affect display/order, not the selected exact ID or tool support.
- Core commands are `apply`, `list`, and `current`: the former `check` and
  top-level `tools` commands are gone. Use `apply THEME --dry-run` and
  `list tools`, respectively.

### Ranking and offline behavior

- Stars and dates come from the [Radar Developer Color Themes ranking](https://clang-engineer.github.io/data/rankings.json)
  for the associated **GitHub project**, not the individual theme variant.
  `PROJECT CREATED` is the repository creation date; `LAST PUSH` is GitHub's
  last code-push date, not metadata activity or the date a variant changed.
  Multiple variants may share both dates. The bundled offline snapshot
  contains the full category, not just the curated shortlist. Counts and
  dates may change as Radar updates; the displayed ranking date is the
  **source data's** update date, not themeset's check time.
- A valid cache younger than 24 hours prevents `list` from fetching. Otherwise
  `list` checks for updates with a three-second timeout, caching the last valid
  category at `~/.cache/themeset/radar-ranks.json`. Interactive `apply` only
  reads that cache; previews and `--home` targeting another home do not fetch.
  Without a usable network/cache, themeset uses the bundled snapshot.
- Rosé Pine and Moonfly are deliberately hidden by default despite top-50
  ranks; Ayu Light and VS Code Dark are also available by exact ID under
  `list --all`. A user `listed` override can display them.
- Bundled associations are in `internal/catalog/defaults.json`'s `listed`
  array. Hidden bundled IDs show `-` under `list --all` unless
  `themes.<id>.radar_repo` maps them. Similar names are not proof of common
  provenance: `vscode-dark` is not assigned an unrelated port's Stars. Some
  ranked projects are ports of existing families or target unsupported apps.

## Shell completion

- Homebrew installs Bash, Zsh, and Fish definitions in standard completion
  directories; restart your shell after installation/upgrade. For Zsh, put
  Homebrew's directory in `fpath` before `compinit`.
- The default command menu lists `apply`, `current`, `list`; options appear
  after `-`, and `completion`, `help`, `version` after typing their names.
  Completing `apply` offers theme IDs including custom config entries;
  `list` offers `tools`. Comma-separated `--tools` values and paths are
  completed. Completion never applies themes.

For source builds or an existing shell session:

```sh
# Bash
source <(themeset completion bash)
# Zsh, after compinit
source <(themeset completion zsh)
# Fish
themeset completion fish | source
```

- **Tab:** `themeset <Tab>` completes commands and keeps existing shell Tab
  behavior.
- **Fish Shift+Tab:** only at the first theme position in `themeset apply `,
  opens the sourced preview picker. Enter inserts the theme into the command
  line, never executes `apply`; Escape leaves it unchanged. Supplied themes,
  flags, paths, and other commands retain previous key handlers.
- **Fresh Fish:** Homebrew installs a startup hook so the first Shift+Tab works
  before completions are lazy-loaded; the Fish `source` command above works in
  existing sessions. Upgrading from v0.2.2? Run `exec fish` to discard the old
  Tab wrapper. Shift+Tab preserves previous bindings outside the theme
  position; Bash/Zsh completion is unchanged.

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

### Status and recorded results

| Field | Values / meaning |
|---|---|
| Support | `native`, `official`, `custom`, `unsupported`, `not-installed`, or `error`. |
| Outcome | `planned`, `applied`, `skipped`, or `failed` (separate from support). |
| `fallback_theme` | Actual fallback theme ID, with a warning; retained by `current`. |

Human-readable results show per-tool rows and totals for applied, planned,
skipped, and failed. Skipped rows explain the missing compatible port;
inspection/apply errors stay visible and make the command fail. `current` records
the last application result, including partial failures—not whether all apps
currently match the last requested theme.

### Ghostty reload

- Ghostty saves its theme selector. On macOS, if running for the current
  account, themeset asks Ghostty's native AppleScript API to `reload_config`.
  Scratch `--home` never reloads the real app.
- Automation permission may be needed. Denial is reported **after** the file
  is saved; `applied` means the action was acknowledged, not that every window
  visually refreshed.
- On Linux, press `Ctrl+Shift+,` or open a new window. themeset avoids automatic
  `SIGUSR2` hard reload: Ghostty 1.4.0 can desynchronize attached tmux clients.

### Bundled catalog and discovery order

The [catalog](internal/catalog/defaults.json) registers verified native
candidates and official ports for a curated selection of Radar's top 50, not
every ranked repository or every tool. Ranked app-specific ports of an existing
family are not separate theme IDs. `support_site` and `radar_repo` are reference
metadata, never download sources or proof of tool support. Neovim colorschemes
must already be installed; other missing ports use `default_theme`.

| Discovery/sort setting | Default `list` and All picker view |
|---|---|
| No user `listed` | Built-in shortlist plus IDs in user `themes`; Radar rank by default, unranked last, ties in original discovery order. |
| User `listed` array | Replaces the displayed shortlist exactly; preserves array order by default. `[]` hides all discovery entries. |
| `theme_sort` / `--sort` | Choose `rank`, `listed`, or `name`; `--sort` overrides config for one `list` or interactive `apply`. |
| `listed` sort | All groups dark, light, then other, preserving order within each group. |
| `rank` / `name` sort | All orders globally; group-specific views still show only their group. |

Sorting changes neither visibility nor application; completion sorts its own
suggestions. Hidden IDs still work via `apply THEME` and as `default_theme`;
find exact IDs with `list --all`. `listed` only changes default discovery,
not bundled ports or sources.

### tmux plugins and reload

| Plugin | Variants |
|---|---|
| [Catppuccin](https://github.com/catppuccin/tmux) | All four flavors; installed at tag v2.3.2, requires tmux 3.2+. |
| [Rosé Pine](https://github.com/rose-pine/tmux) | Main, Moon, Dawn. |
| [Dracula](https://github.com/dracula/tmux) | Upstream plugin. |
| [Nord](https://github.com/nordtheme/tmux) | Upstream plugin. |
| [Gruvbox](https://github.com/egel/tmux-gruvbox) | dark256. |
| [Everforest](https://github.com/TanglingTreats/tmux-everforest) | dark-medium. |

- **Installation:** `apply` reuses complete installed plugins or clones the
  registered repository to `$XDG_CONFIG_HOME/tmux/plugins/OWNER/REPO/`
  (normally `~/.config/tmux/plugins/OWNER/REPO/`). It checks required files in
  a temporary directory before moving the clone into place. Repair an
  incomplete existing directory manually: themeset never overwrites it.
  `list`, preview, and `apply --dry-run` never download plugins. Git/network
  are needed only for missing plugins. Non-Catppuccin plugins use the upstream
  default branch at installation time and are not automatically updated.
- **TPM:** a complete `~/.tmux/plugins/REPO` is reused; plugins named `tmux`
  share that TPM path, while separate XDG paths can hold multiple families.
- **Trust:** plugins run upstream shell code on the next tmux config load and
  may execute additional status commands. themeset neither copies nor
  generates their colors. To avoid third-party execution, use `--dry-run`, a
  style-only theme, or an explicitly trusted `custom.file`.
- **Activation:** applying a plugin writes a flavor selector and upstream
  `run-shell` include to the selected tmux config. Inside tmux for the current
  user's home, themeset reloads **that tmux server** after saving. A direct
  top-level include of the managed file reloads the top-level config first,
  restoring base layout (including Oh my tmux!); otherwise only the changed
  file reloads. Plugins can replace the status-line layout: load themeset
  after other status owners and TPM for precedence.
- **No reload:** scratch `--home`, commands outside tmux, and `--dry-run` do not
  reload a server. A failed reload reports an error after saving the file.
  Switching to Moonfly, Tokyo Night, or Nightfox replaces the managed include
  with upstream styles only; without a direct top-level include, these styles
  cannot undo a plugin's running status layout. Reload the base layout or
  restart the server. Other tmux themes may use `custom.file`.

## Integrations

For each tool's source, installation/placement, activation, reload,
preservation rules, and implementation gaps, see the
[adapter lifecycle reference](theme-adapters.md).

- Bundled official assets come over HTTPS on each apply; adapters validate
  native formats before changing settings. Installed native themes and custom
  local files stay local. Rosé Pine sources use Main; Tokyo Night uses Night.
- The catalog owns theme IDs, installed names per tool, and official source
  URLs. Tool packages check availability and apply native formats;
  `internal/adapter/shared/` supplies common I/O and download guards.

### Supported applications

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
| Television | Built-in Catppuccin, Dracula, Gruvbox Dark, Nord Dark, and Tokyo Night | Native `ui.theme` in `config.toml` |

- `native` means installed and selectable, not necessarily maintained by the
  theme author. Registered `official` assets may come from an app-maintained
  collection. An app without a verified port falls back rather than synthesizing
  colors.
- Version and config precedence matter: Catppuccin fzf needs 0.52.0+;
  GitHub Dark and Modus Vivendi need 0.63.0+. Hardcoded Starship module styles,
  `LS_COLORS`, and `EZA_COLORS` can override selected theme colors.

`themeset rollback` restores the first pre-Themeset version of each changed
file and removes files first created by Themeset. It searches the selected
home, XDG config directory, and directories named by `paths`. Backups remain
available until rollback succeeds.

### Acquiring registered assets

`apply` fetches registered official files from their current upstream branches
over HTTPS, checks size and native format, and leaves previous selections intact
on failure. Dry-run and native/local ports do not download. This favors current
upstream colors over offline reproducibility. Running apps generally pick up
changes on their next launch or invocation; existing flags and environment
settings can take precedence. The [per-tool reference](theme-adapters.md)
documents files, ownership, version requirements, and activation details.

For Moonfly, upstream [extras](https://github.com/bluz71/vim-moonfly-colors/tree/master/extras)
provide fzf colors, a Starship palette, and tmux **styles only** (not
status-line layout). Ghostty and Neovim use installed native providers;
unregistered applications fall back. The similarly named bat `fly16` syntax
theme is not Moonfly.

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
  "fallbacks": {"television": "dracula"},
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

### Configuration keys

| Key | Meaning |
|---|---|
| `tools` | Omit to select all adapters; `[]` selects none. Use `neovim` for Neovim. Unknown selected tool IDs are errors. |
| `listed` | Optional exact replacement for `list`, completion, and picker discovery. Omit for built-in shortlist plus IDs in `themes`; include custom IDs explicitly if supplied. `[]` hides all. It neither declares sources nor restricts `apply THEME`; `list tools` is unaffected. |
| `theme_sort` | `rank`, `listed`, or `name` for discovery order; `--sort` takes precedence for one command (see [discovery order](#bundled-catalog-and-discovery-order)). |
| `default_theme` | Per-tool fallback if the requested theme has no available native, official, or custom port. Defaults to `catppuccin-mocha`; may name a bundled or user-defined ID. Tried once, never after inspection/apply errors; if unavailable, that tool is skipped unchanged. |
| `fallbacks.<tool>` | Optional tool-specific fallback ID, taking precedence over `default_theme` for that tool. |
| `paths.<tool>` | Explicit native destination override; an absolute destination can be outside `--home`. |
| `themes.<id>.radar_repo` | Exact Radar `owner/repo` for rank metadata only: never downloads ports or grants support. Use the same family, not a similar-looking palette; overrides a bundled association. |
| `themes.<id>.custom.<tool>` | Registered app-native port for that exact theme/tool, taking precedence over native and official ports. An invalid custom port is an error, not permission to substitute colors. `{}` registers an ID without custom ports. |

Unknown JSON fields are errors. You may design your own colors and register an
ID or override a bundled theme; themeset does not generate app themes from a
shared palette. For example, Dracula fzf colors can be registered as
`"themes": {"dracula": {"custom": {"fzf": {"file": "ports/dracula/fzf.rc"}}}}`.
Inspect with `themeset apply dracula --dry-run` first: it reads local files but
does not change app settings. A missing/invalid file fails instead of falling
back to the bundled theme.

### Custom port inputs and trust

| Field | Contract |
|---|---|
| `file` | Existing app-native theme/settings file; relative paths use the JSON config directory, while `~/` uses the selected home. Local `name` can select an existing app theme. |
| `url` | HTTPS asset fetched on each apply using the tool-native parser, **custom** rather than official. Mutually exclusive with `file`; allowed for fzf, Starship, Yazi, Atuin, lazygit, lazydocker, btop, and eza. Yazi remote styles also need `syntax_url` for a complete `.tmTheme`. |
| `name` | For remote ports defaults to the theme ID; set it when, for example, a Starship file declares a differently named palette. |
| `source` | Optional human-readable provenance, never the download URL. |

Remote Lua/Vim/tmux and other executable app configurations are rejected.
Remote URLs require HTTPS without credentials; downloads are size-bounded,
parsed as native data, and never executed. Dry-run does not download.
Malformed downloads fail without fallback. Review third-party URLs yourself:
registration does not establish trust. Custom **local** Lua/Vim/tmux files are
trusted executable application configuration; never register untrusted files.

### Destination and ownership

- Native defaults: macOS lazygit/lazydocker and eza follow their Library paths;
  existing legacy lazy-tool vendor paths are recognized. Atuin preserves an
  existing legacy XDG config or uses native `~/.atuin`. Harlequin uses its
  existing native config or seeds `~/.harlequin.toml`; with
  `HARLEQUIN_CONFIG_PATH`, point the override at an explicit generated catalog.
- Only relevant theme keys/sections are imported. Unrelated active values and
  file modes remain; structured TOML/YAML rewrites may normalize comments and
  formatting. Existing symlinks are retained while their target configs are
  updated atomically. Dedicated managed theme assets can be replaced.
- A dotfile manager must not also render the same selected theme keys. Use
  native includes or dedicated runtime-owned assets and distribute this JSON
  through the manager. The sibling dotfiles repository follows these ownership
  boundaries.

## Homebrew releases

```sh
sh scripts/package.sh v1.0.0 clang-engineer/themeset
```

- **Package locally:** the command creates macOS/Linux arm64/amd64 archives,
  `dist/checksums.txt`, and `dist/themeset.rb`. Formula hashes come from actual
  archives, not placeholders. Archives contain this README, license, and
  third-party notices; the supplied tag sets the binary version.
- **Release via tag:** pushing a version tag runs
  `.github/workflows/release.yml`: Go tests, archives, formula, and uploaded
  release assets. To update an existing Homebrew tap too, configure:

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
