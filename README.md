# themeset

Apply existing application themes together. Inspect which installed tools support
a theme, prefer their native or official ports, use explicitly registered custom
ports when necessary, and skip unsupported tools without changing their settings.

**No palette generation, approximate substitutions, plugin installation, or
shell commands from configuration.**

## Install

Build locally with Go 1.23 or newer:

```sh
go build -o themeset ./cmd/themeset
install -m 755 themeset "$HOME/.local/bin/themeset"
```

Homebrew release tooling is included, but no public repository or formula has
been published yet. After publication the tap installation will be:

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
themeset tools
themeset check dracula
themeset apply dracula --dry-run
themeset apply dracula
themeset current
```

- `check` and `--dry-run` inspect without writing app configuration or result state.
- `--tools ghostty,bat,fzf` restricts the selected tools.
- `--json` emits structured output, keeping diagnostics on stderr.
- `--home /path/to/scratch` selects that home and its `.config`, ignoring inherited
  XDG config paths. Explicit `paths` overrides still apply, including absolute
  paths outside that home: this is **not a security sandbox**. Installed binaries
  are still discovered through PATH. For smoke runs, use a scratch directory and
  check that any configured destinations also point into it.
- Flags are accepted before or after positional arguments.

## Resolution and results

1. Resolve the installed app's native theme/provider or a bundled official port.
2. If unavailable, warn and select the registered custom port for the exact
   **theme × tool** combination.
3. If neither exists, warn and skip that tool. Its previous settings remain.
4. Skip missing executables. Actual inspection/apply failures remain errors;
   other selected tools are still processed and the command returns nonzero.

Support is reported as `native`, `official`, `custom`, `unsupported`,
`not-installed`, or `error`. Application outcomes are separate: `planned`,
`applied`, `skipped`, or `failed`. `current` shows the last recorded application
result, including partial failures; it does not assert that every app currently
matches the last requested theme.

Theme identifiers in `list` are not a promise of support by every tool. Existing
providers must already be installed. A custom registration is never relabeled
as official.

## Integrations

| Tool | Existing support discovery | Custom input |
|---|---|---|
| Ghostty | Installed `+list-themes` names | Native Ghostty theme asset |
| Neovim | Installed colorscheme runtime files | Existing Lua/Vim theme script |
| tmux | No general native theme registry | Trusted tmux theme config |
| bat | Installed syntax-theme list | bat config containing `--theme` |
| delta | Installed syntax-theme list | Gitconfig containing delta theme settings |
| Harlequin | Installed CLI's supported theme names | Native TOML theme/profile settings |
| Starship | Existing named palettes; pinned official Dracula | Selected palette in Starship TOML |
| fzf | Pinned official Dracula options | Color-only native options file |
| Yazi | Complete installed flavors | Native `theme.toml` fragment |
| Atuin | Shipped or installed Atuin themes | Native theme TOML asset |
| lazygit/lazydocker | Custom registration required | `gui.theme` YAML fragment |
| btop | Installed user/system theme files | Native `.theme` file |
| eza | Custom registration required | Native `theme.yml` fragment |

Atuin does not ship all of Harlequin's similarly named Textual themes. tmux does
not have a standard built-in family-name selector. These cases are reported
honestly rather than replaced by newly synthesized colors.

Changes take effect on the next application launch or prompt/invocation; running
apps are not forcibly restarted. Neovim's generated selection runs on VimEnter.
Existing CLI flags, environment overrides, project-local profiles, and user
startup code can still override an application's normal config.

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
- Repository secret `HOMEBREW_TAP_TOKEN`: a token with write access to that tap.

Do not put credentials in source. A manual tap update uses the same command:

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
