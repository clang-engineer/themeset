# themeset

Apply existing application themes together. Use explicitly registered per-tool
overrides first, otherwise select installed native themes or bundled official
ports, and skip unsupported tools without changing their settings.

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

`apply` without a theme opens a single-selection fzf picker: themes on the left,
a sourced visual preview and per-tool support on the right. The picker ignores
`FZF_DEFAULT_OPTS` and `FZF_DEFAULT_OPTS_FILE`; Tab/Shift+Tab do not mark multiple
themes. Enter reviews the current theme's plan; a separate `[y/N]` confirmation
is required before any configuration/state write.
Escape or a declined confirmation changes nothing. `apply --dry-run` uses the
picker and shows the plan without confirmation or writes. Interactive selection
needs a terminal and fzf (Homebrew installs it); `--json` requires an explicit
theme and remains suitable for scripts.

Visuals use an installed bat syntax theme or an actual native/custom Ghostty
asset, labeled with source app and exact variant. Ghostty swatches and ANSI
samples are not a promise of editor syntax colors. No palette is generated and
apps can differ. Missing source assets explicitly show **Preview unavailable**.
Browsing only inspects support/assets; it never applies a theme.

Core commands are `apply`, `list`, and `current`. `list` shows theme IDs;
`list tools` shows registered tools and installation status. The former `check`
and top-level `tools` commands have been removed: use `apply THEME --dry-run`
and `list tools`, respectively.

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
3. Otherwise resolve the installed app's native theme/provider or a bundled
   official port.
4. If neither exists, warn and skip that tool. Its previous settings remain.
   Actual inspection/apply failures remain errors; other selected tools are
   still processed and the command returns nonzero.

Support is reported as `native`, `official`, `custom`, `unsupported`,
`not-installed`, or `error`. Application outcomes are separate: `planned`,
`applied`, `skipped`, or `failed`. `current` shows the last recorded application
result, including partial failures; it does not assert that every app currently
matches the last requested theme.

Human-readable results include a per-tool table and a summary of applied, planned,
skipped, and failed tools. Normal unsupported tools produce one aggregate warning
and keep their settings unchanged. Selecting a custom override is not a warning.
Inspection/apply errors remain visible and make the command fail.
JSON output retains each tool's structured warnings and errors.

The default catalog is curated to five themes with broader native/official
coverage: `catppuccin-mocha`, `dracula`, `gruvbox`, `moonfly`, and `nord`.
The same catalog feeds `list`, shell completion, and the interactive picker.
This is a fixed shortlist, not a dynamic filter or a promise that every tool
supports each theme; providers must already be installed.
Other theme IDs can be explicitly registered under `themes` in the JSON config,
including an empty entry when only installed native providers are needed.
A custom registration is never relabeled as official.

## Integrations

For each tool's source, installation/placement, activation, reload, preservation
rules, and current implementation gaps, see the
[adapter lifecycle reference](theme-adapters.md).
That reference describes the requested acquire-if-missing design; the current CLI
still uses installed providers and bundled files, without runtime downloads.

| Tool | Existing support discovery | Custom input |
|---|---|---|
| Ghostty | Installed `+list-themes` names | Native Ghostty theme asset |
| Neovim | Installed colorscheme runtime files | Existing Lua/Vim theme script |
| tmux | Pinned official Moonfly style commands; no general native registry | Trusted tmux theme config |
| bat | Installed syntax-theme list | bat config containing `--theme` |
| delta | Installed syntax-theme list | Gitconfig containing delta theme settings |
| Harlequin | Installed CLI's supported theme names | Native TOML theme/profile settings |
| Starship | Existing named palettes; pinned official Dracula and Moonfly | Selected palette in Starship TOML |
| fzf | Pinned Dracula, Moonfly, Catppuccin Mocha ports; fzf-author Gruvbox Dark and Nord presets | Color-only native options file |
| Yazi | Complete installed flavors; bundled Catppuccin Mocha/Mauve styles and syntax theme | Native `theme.toml` fragment |
| Atuin | Shipped/installed themes; bundled Catppuccin Mocha/Mauve | Native theme TOML asset |
| lazygit | Bundled Dracula and Catppuccin Mocha/Mauve | `gui.theme` YAML fragment |
| lazydocker | Bundled Dracula | `gui.theme` YAML fragment |
| btop | Installed user/system theme files | Native `.theme` file |
| eza | Bundled Catppuccin Mocha/Mauve | Native `theme.yml` fragment |

Atuin does not ship all of Harlequin's similarly named Textual themes. tmux does
not have a standard built-in family-name selector. These cases are reported
honestly rather than replaced by newly synthesized colors.

Bundled ports are applied by writing the application's native settings/assets;
no separate theme installation, runtime network fetch, or installer script is
needed. Native discovery inspects fixed app commands or local theme files.
`native` means available through the app; it does not certify theme-author
provenance. Bundled `official` ports record whether their source is the theme
author or, for fzf Gruvbox/Nord, the application's author.

Catppuccin Mocha fzf requires fzf 0.52.0 or newer; resolution and apply report an
error for older versions before writing its options. Gruvbox/Nord use stable
color option names, not the newer layout options shown in the fzf web generator.
Catppuccin Atuin installs and selects its declared `catppuccin-mocha-mauve`
name. Yazi imports only UI style tables and installs the official Catppuccin
syntax theme; separators, icons, file rules, layouts, and keymaps remain intact.

Dracula remains unregistered for Atuin, eza, tmux, and current Yazi through these
adapters. No verified Atuin port was found; the currently examined theme-author
eza port uses shell exports; tmux's full Dracula plugin executes scripts and
changes status layouts; the Dracula-author Yazi example is incomplete/outdated.
Application-maintained alternatives do exist: Yazi's flavor collection has a
complete Dracula package, and eza's theme collection has native Dracula YAML.
These are identified in the lifecycle reference but are not yet acquired or
selected automatically. Current skips describe registry/implementation gaps,
not proof that those applications cannot use Dracula.

Changes take effect on the next application launch or prompt/invocation; running
apps are not forcibly restarted. Neovim's generated selection runs on VimEnter.
Existing CLI flags, environment overrides, project-local profiles, and user
startup code can still override an application's normal config.

### Moonfly provenance and scope

The [official Moonfly extras](https://github.com/bluz71/vim-moonfly-colors/tree/2c883835f5b7b380a0a9e80695c420f7efebdfe4/extras)
are pinned at `2c883835f5b7b380a0a9e80695c420f7efebdfe4`:

- **fzf:** the exact 14 upstream color options, with shell export syntax removed
  and `--color VALUE` expressed as `--color=VALUE`. Ordinary picker options and
  shell startup files are not changed.
- **Starship:** only the selected upstream `[palettes.moonfly]` is imported and
  selected, just as for Dracula. Prompt/module formats, symbols, truncation, and
  other palettes remain intact. The upstream palette defines only `fern`,
  `lavender`, `orange`, and `red`; no missing colors are invented. Existing styles
  use this palette only when they reference those names.
- **tmux:** only the six exact upstream style commands (message, pane borders,
  status, window activity, and current-window styles) are imported. Upstream's
  status-left/right, window formats, and shell conditional are deliberately
  omitted to preserve layouts and keymaps. A dedicated
  `$XDG_CONFIG_HOME/themeset/tmux-moonfly.conf` (default
  `~/.config/themeset/tmux-moonfly.conf`) is included after the existing config.
  A `paths.tmux` override can target an existing managed theme include, as in an
  Oh my tmux! wrapper. Oh my tmux!'s own layout and embedded segment colors are
  not remapped; inline colors can still override the imported style fields.

Ghostty and Neovim continue to use installed native Moonfly providers. Other
applications remain unsupported unless a real installed/native or explicitly
registered custom port exists. The matching but distinct `fly16` bat theme is
**not** treated as Moonfly. Source hashes, adaptation details, and the full
upstream MIT license are retained in
[`internal/adapter/assets/moonfly.LICENSE`](internal/adapter/assets/moonfly.LICENSE);
see also [third-party notices](THIRD_PARTY_NOTICES.md).


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
    }
  }
}
```

Custom `file` entries reference existing **app-native theme/settings files**, not
shared palettes. Relative paths resolve against the JSON config directory; `~/`
uses the selected home. `name` may select an existing app theme for adapters
supporting native name selection. Omit `tools` for all adapters; `"tools": []`
means none. Canonical Neovim ID is `neovim`. Unknown JSON fields and unknown
selected tool IDs are errors.

Explicit `themes.<id>.custom.<tool>` entries override even available native or
official ports. An invalid custom file is an error, not permission to use other
colors. Use an empty theme object to register an extra ID without custom ports.

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

MIT. Official port assets retain their licenses and pinned provenance; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
