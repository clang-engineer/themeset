# Theme adapter installation and activation

This is the per-tool implementation reference for themeset's
**registered source → fetch current asset → activate with the tool's own mechanism**
lifecycle. Official sources provide Catppuccin/btop themes, btop's own
Dracula/Gruvbox Dark/Nord files, eza's Dracula/Gruvbox Dark YAML,
Starship's Catppuccin Mocha/Gruvbox Dark palettes, and Yazi's Dracula flavor.
Complete independently installed Yazi Gruvbox/Nord/Moonfly flavors are selected
without downloading. Other adapters inspect installed providers, fetch current
official files, or apply explicitly registered native files. No plugin manager
or arbitrary installer script is invoked.
See [current integrations](README.md#integrations) and
[official source notices](THIRD_PARTY_NOTICES.md).

## Shared recipe contract

Each enabled `(theme, tool)` recipe must record:

| Field | Meaning |
| --- | --- |
| Provenance | Theme-author port, application-maintained preset, or explicit user override; these are different claims |
| Source | Official repository/current-branch URL and required artifact paths |
| Compatibility | Native selector/variant, format, minimum tool version, and required companion files |
| Ownership and placement | Effective config destination, separate owned asset location, and any package-manager ownership |
| Detection | Evidence that the exact usable asset/provider is installed, not merely that a directory exists |
| Acquisition | Fixed adapter operation to retrieve missing assets or invoke a supported package manager |
| Activation | Native command with fixed argument structure, config-key edit, file placement, or include/symlink |
| Effect boundary | Reload operation or next launch/prompt, including the specific live instance affected |
| Preservation | Theme-owned fields versus unrelated user settings; known higher-priority overrides |
| Verification | Asset validation plus observation that the actual application consumes the selected theme |

Documentation links below describe upstream procedures, not executable registry
entries. Automatic downloads use registered official HTTPS URLs and are validated
as data by the tool adapter.
No unimplemented JSON fields or new CLI commands are introduced by this document.

Requested application rules:

- The selected `tools` list determines targets. Installing an application itself
  is outside theme installation; missing applications remain skipped.
- An explicit per-tool custom override wins. An invalid override fails; do not
  replace it silently with an official theme.
- Without an override, reuse a compatible installed provider or fetch the
  current registered official source on apply. Ordinary switching does not run
  `git pull`, execute downloaded scripts, or upgrade unrelated packages.
- Resolve each application's actual config path and startup overrides. A
  themeset `paths` override must match the file the application really reads.
- Acquire and validate required files before changing active selection. Reapply
  must not duplicate includes, package entries, or YAML/TOML keys.
- A registered source's download/install failure is an error, not an unsupported
  skip. Skip only when there is no compatible registered or installed port.
- Do not evaluate web-page instructions or shell snippets as arbitrary commands.
  Plugin code is executable even when installed through a trusted manager;
  identify its source, pin, and configuration effects before activation.
- Preserve unrelated settings and user-owned repositories. Replace only owned
  theme state. Do not put licenses/README files in directories whose every file
  is interpreted as a theme. Removal of previous theme-owned overrides must not
  erase unrelated user customization.
- Dry-run remains read-only: no downloads, cache builds, installs, or reloads.
  A config write is not proof that an already running app has changed.

Paths below are Unix defaults unless stated otherwise. Respect XDG/app-specific
roots and existing package managers; do not hardcode a developer's home path.

## ghostty

- **Sources:** [Ghostty theme reference](https://ghostty.org/docs/config/reference#theme)
  and the [Ghostty-format collection](https://github.com/mbadolato/iTerm2-Color-Schemes/tree/master/ghostty)
  referenced by the existing adapter. This collection is not a promise that each
  theme's original author maintains its port.
- **Detect / acquire:** inspect `ghostty +list-themes` for the exact native name.
  Shipped themes need no download. Otherwise retrieve a registered Ghostty-format
  file, not the entire collection.
- **Placement:** `$XDG_CONFIG_HOME/ghostty/themes/<name>` or
  `~/.config/ghostty/themes/<name>`; an absolute theme asset path is also valid.
- **Activate:** edit `theme = <native-name-or-absolute-path>` in the effective
  Ghostty config or an already included theme selector file. On macOS, when
  Ghostty is running and `--home` refers to the current user's home, use its
  [native AppleScript `perform action`](https://ghostty.org/docs/features/applescript)
  to invoke `reload_config` on the front window's focused terminal.
- **Effect / preserve:** the action's acknowledgement confirms a reload
  request, not visual verification of every window. macOS Automation permission
  may be required; failure leaves the theme file saved and reports an error.
  If Ghostty is not running or the target is a scratch home, next launch reads
  the selection. On Linux, reload manually with `Ctrl+Shift+,` or open a fresh
  window. Retain fonts, keybindings and terminal behavior. Explicit
  `background`, `foreground` and `palette` settings can override theme colors.
  Never automate `SIGUSR2`: on Ghostty 1.4.0 its hard reload can desynchronize
  attached tmux mouse/paste modes
  ([upstream report](https://github.com/ghostty-org/ghostty/discussions/15101)).
- **Current / target gap:** installed names are selected but missing themes
  are not retrieved. No safe noninteractive GUI reload has been verified on
  Linux, and some options require a fresh window.

## neovim

- **Sources:** [Dracula installation](https://draculatheme.com/vim),
  [dracula/vim](https://github.com/dracula/vim), and
  [Neovim package loading](https://neovim.io/doc/user/pack/).
  Other themes require their own registered provider and native colorscheme name.
- **Detect / acquire:** find the provider's `colors/<native-name>.lua` or `.vim`
  plus its runtime dependencies. Reuse an existing plugin-manager installation.
  Otherwise a dedicated owned package under
  `stdpath("data")/site/pack/themeset/opt/<provider>` is a possible native
  placement; register that ownership instead of cloning into Lazy's directory
  without updating Lazy's plugin specification.
- **Activate:** make the full provider runtime available through package loading
  or `runtimepath`, then execute `:colorscheme <native-name>`. Persist the choice
  in the managed selector loaded after the user's plugin setup. Copying only a
  `colors` file can break `autoload`/Lua dependencies.
- **Effect / preserve:** new Neovim instances load the persistent selection.
  Changing another running instance requires addressing that instance explicitly;
  do not assume a new headless process changes an existing editor. Preserve init,
  plugin declarations, keymaps, and provider options.
- **Current / target gap:** current code detects supported runtime locations and
  writes a startup selector. It does not install missing providers or manage a
  running editor. Registry addresses alone do not install plugins.

## tmux

- **Sources:** [Dracula's TPM route](https://draculatheme.com/tmux),
  [dracula/tmux](https://github.com/dracula/tmux),
  [TPM scripted operations](https://github.com/tmux-plugins/tpm/blob/master/docs/managing_plugins_via_cmd_line.md),
  and [Moonfly extras](https://github.com/bluz71/vim-moonfly-colors/tree/2c883835f5b7b380a0a9e80695c420f7efebdfe4/extras).
- **Detect / acquire:** a full Dracula plugin needs its entrypoint and scripts,
  not just a color file. With TPM, register `set -g @plugin 'dracula/tmux'` and
  use TPM installation (`prefix + I`, or its documented `bin/install_plugins`).
  That installer can install other declared missing plugins too; it is not an
  isolated single-theme operation. Respect an existing TPM installation root.
- **Activate:** keep TPM's loader after plugin declarations/options. A simple
  official style-only asset, such as Moonfly's styles, can instead be selected
  through a managed `source-file` include. These are distinct recipe kinds.
- **Effect / preserve:** config loading affects the addressed tmux server; a
  documented `tmux source-file <config>` reload is not a machine-wide broadcast.
  Existing plugin options and keybindings must survive. Dracula's full plugin
  executes shell code and changes status layouts; it cannot be described as a
  colors-only operation that preserves an Oh my tmux! status layout.
- **Current / target gap:** current code writes Moonfly's six style commands or a
  trusted custom include; it neither runs TPM nor installs/activates Dracula.
  Full-plugin recipes must explicitly declare their layout impact.

## bat

- **Sources:** [bat custom theme installation](https://github.com/sharkdp/bat#adding-new-themes)
  and [Catppuccin's official bat assets](https://github.com/catppuccin/bat).
- **Detect / acquire:** use `bat --list-themes --color=never`. For a missing
  registered theme, retrieve a `.tmTheme` file; `.sublime-color-scheme` is not
  the supported custom-theme format in the documented workflow.
- **Placement / preparation:** put the asset under the effective
  `bat --config-dir` result's `themes/` directory, then run `bat cache --build`
  after installation/change. Recheck the list: the `.tmTheme` filename determines
  the theme name. Do not rebuild the cache on every unchanged switch.
- **Activate:** edit only `--theme="<native-name>"` in `bat --config-file`, or the
  explicitly configured equivalent. A `--theme` invocation flag is temporary.
- **Effect / preserve:** next bat invocation. Retain pager, style, mapping and
  syntax settings. Ambient `BAT_THEME` or per-invocation flags can override the
  persistent selection and must not be silently rewritten.
- **Current / target gap:** current code selects themes already visible to bat;
  missing `.tmTheme` retrieval and cache rebuild are not implemented.

## delta

- **Sources:** [delta custom themes](https://dandavison.github.io/delta/custom-themes.html),
  [Dracula installation](https://draculatheme.com/delta), and
  [dracula/delta](https://github.com/dracula/delta).
- **Detect / acquire:** distinguish a syntax theme from a whole delta feature.
  `delta --list-syntax-themes` probes syntax themes. A named feature is a separate
  Gitconfig settings group. Dracula's official repository supplies
  `dracula.gitconfig`; acquire it only when the registered feature file is absent.
- **Placement / activate:** keep the retrieved feature file at an owned stable
  path, include it using Git's `[include] path`, and select its name through
  `[delta] features`. Syntax-only selection uses `[delta] syntax-theme` instead.
  Installing a bat syntax theme may also supply delta's syntax highlighting;
  verify delta's own list rather than assuming cache/environment parity.
- **Effect / preserve:** next delta/Git invocation. Retain Git identity, aliases,
  pager setup, navigation, layout, non-theme features and existing includes.
  Replace a previous managed theme feature, not the entire user feature list.
  Project Git config, command options and environment can override global values.
- **Current / target gap:** current native resolution checks syntax themes and
  edits `syntax-theme`; custom Gitconfig imports supported theme keys. It does
  not acquire official feature repositories/files at runtime or automatically
  install missing syntax assets. Syntax-only selection is not full-port activation.

## starship

- **Sources:** [Starship palette configuration](https://starship.rs/config/#color-palette),
  [Dracula palette instructions](https://github.com/dracula/starship/blob/master/INSTALL.md),
  [Moonfly extras](https://github.com/bluz71/vim-moonfly-colors/tree/2c883835f5b7b380a0a9e80695c420f7efebdfe4/extras),
  [Catppuccin Starship palette](https://github.com/catppuccin/starship),
  and [Starship's Gruvbox Rainbow preset](https://github.com/starship/starship/blob/1b5c2f257ef276c3020899623a1942b3ecd457f7/docs/public/presets/toml/gruvbox-rainbow.toml).
- **Detect / acquire:** inspect a nonempty `[palettes.<name>]` in the effective
  config. Otherwise fetch the registered palette TOML. Dracula's `starship.toml`
  palette and its full `starship.theme.toml` prompt preset are different artifacts.
- **Placement / activate:** merge only the selected palette into the config
  normally at `~/.config/starship.toml` (honor `STARSHIP_CONFIG`), then set the
  top-level `palette = "<name>"`. No plugin install or named-theme command is needed.
- **Effect / preserve:** next prompt render, with shell integration already
  present. Keep prompt formats, modules, symbols and unrelated palettes. Modules
  with hardcoded colors do not become themed merely by selecting a palette;
  styles must reference its semantic color names.
- **Current / target gap:** current code fetches official Dracula/Moonfly and
  Catppuccin Mocha/Gruvbox Dark palettes or selects an existing one. It imports
  only a palette, never a preset's prompt modules/layout.
  A palette selector alone cannot recolor modules with hardcoded styles;
  `paths.starship` is supported but `STARSHIP_CONFIG` is not detected automatically.

## fzf

- **Sources:** [Dracula options](https://draculatheme.com/fzf),
  [Catppuccin fzf](https://github.com/catppuccin/fzf),
  [fzf-author presets](https://junegunn.github.io/fzf/color-themes/), and
  [fzf manual](https://github.com/junegunn/fzf/blob/master/man/man1/fzf.1).
- **Detect / acquire:** no installed-theme registry or `fzf theme set` command.
  Retrieve/export the registered exact color-option data when the owned artifact
  is missing. Not every source needs a repository clone.
- **Placement / activate:** write a colors-only options file and arrange for the
  shell to load it through `FZF_DEFAULT_OPTS_FILE`, or integrate the color options
  with the existing fzf environment. `~/.config/themeset/fzf.opts` is a themeset
  convention, not an automatically discovered fzf theme path.
- **Effect / preserve:** next fzf invocation. A child process cannot modify its
  parent shell's environment; shell integration must already exist or be explicitly
  set up. Preserve layout, binds, previews, widget options, existing options
  files, and `NO_COLOR` policy. Do not overwrite all of `FZF_DEFAULT_OPTS` with a
  theme author's example export.
- **Compatibility:** the current Catppuccin artifact needs fzf 0.52.0+; some
  options on the author's web generator require 0.73.0+. Record requirements per
  artifact. Author-site presets are not automatically licensed by the fzf
  repository; current Gruvbox/Nord provenance limitations are in the notices.
- **Current / target gap:** current code fetches color data for the seven
  catalog themes and writes only color options; shell integration is external.

## harlequin

- **Sources:** [Harlequin themes](https://harlequin.sh/docs/themes),
  [config discovery](https://harlequin.sh/docs/config-file/discovery), and
  [profiles](https://harlequin.sh/docs/config-file/profiles).
- **Detect / acquire:** ask the installed `harlequin --help` for supported theme
  names. Supported catalog themes are built in; no separate download is needed.
  Moonfly is not in the verified list and has no registered compatible port.
- **Activate:** persist `theme = "<native-name>"` under the intended
  `[profiles.<name>]` tables (or `[tool.harlequin.profiles.<name>]` in
  `pyproject.toml`). `harlequin --theme <name>` is a real launch override, not a
  persistent theme-setting command. `--config` opens a wizard; `--config-path`
  selects a file.
- **Effective path:** explicit `--config-path` / `HARLEQUIN_CONFIG_PATH`, project
  files, platform user-config files and home files participate in discovery and
  merging. Do not change a shadowed global file and claim the active profile changed.
- **Effect / preserve:** relaunch with the same connection/profile arguments.
  Preserve connections, secrets references, adapters, keymaps and profile names.
- **Current / target gap:** current code probes builtins and changes all profiles
  in one selected file. It does not reproduce full merged/project discovery;
  explicit `paths.harlequin` must point at the file the user actually uses.

## yazi

- **Sources:** [Yazi flavors](https://yazi-rs.github.io/docs/flavors/overview/),
  [Yazi CLI/package manager](https://yazi-rs.github.io/docs/cli/),
  [application-maintained complete flavors](https://github.com/yazi-rs/flavors),
  and [Catppuccin's direct theme](https://github.com/catppuccin/yazi).
- **Detect / acquire:** a complete flavor needs `flavor.toml` and `tmtheme.xml`,
  associated licenses and package files. Check package registration plus deployed
  files, not only the presence of a directory. Match the `ya` and `yazi` versions.
- **Native install route:** `ya pkg add yazi-rs/flavors:dracula` or
  `ya pkg add yazi-rs/flavors:catppuccin-mocha`. The grammar is `owner/repo:child`,
  without a `flavor/` prefix. Files deploy to
  `<Yazi config>/flavors/<name>.yazi/`; package registration is in `package.toml`.
  Already registered packages require detection, not another blind `add`;
  `ya pkg install` restores registered packages and can affect other dependencies.
- **Activate:** set `[flavor] dark = "<name>"` and `light = "<name>"` in the
  effective `theme.toml`, respecting `YAZI_CONFIG_HOME`. Alternatively, a direct
  official TOML port needs its associated syntax theme and `mgr.syntect_theme`;
  this is not the same recipe as installing a flavor.
- **Effect / preserve:** relaunch Yazi. Preserve `yazi.toml`, `keymap.toml`, plugins,
  package entries, icons and file rules. Existing direct styles override flavor
  styles; remove only previous adapter-owned overrides when switching strategies.
- **Source caveat:** `dracula/yazi`'s old `flavor.toml` and `[flavor] use` example
  are not a complete current package. `yazi-rs/flavors:dracula` is a verified
  application-maintained alternative, not the same source/provenance claim.
- **Current / target gap:** Dracula fetches current native runtime files and
  both licenses, stages installation, and selects the flavor. Independently
  installed compatible flavors remain native; themeset-owned flavors refresh.
  Independently installed complete Gruvbox, Nord and Moonfly flavors can be
  selected without network access; the application-maintained collection does
  not ship these three. Its Gruvbox/Nord README links lead to independent
  community ports, not registered automatic recipes. Previously managed
  Catppuccin styles are removed only when the managed syntax path establishes
  ownership and individual values still match. No `ya` command or package
  manager registration is performed.

## atuin

- **Sources:** [Atuin theming](https://docs.atuin.sh/latest/guide/theming/),
  [config discovery](https://docs.atuin.sh/main/configuration/config/),
  [actual theme loader](https://github.com/atuinsh/atuin/blob/main/crates/atuin-client/src/theme.rs),
  and [Catppuccin Atuin](https://github.com/catppuccin/atuin).
- **Detect / acquire:** builtins need no download. Other themes need their named
  TOML file, valid `[theme]` / `[colors]`, and any declared parent theme. Fetch the
  registered asset if missing. Catppuccin Mocha/Mauve's native name is
  `catppuccin-mocha-mauve`, not the catalog alias `catppuccin-mocha`.
- **Placement:** use the loader's effective themes directory. Current upstream
  prioritizes absolute `ATUIN_HOME`, existing `~/.atuin/themes`, deprecated
  `ATUIN_THEME_DIR`, existing `ATUIN_CONFIG_DIR/themes`, then legacy XDG paths;
  the fresh default is `~/.atuin/themes`. The config file has separate discovery;
  `dirname(config.toml)/themes` is not a universal rule.
- **Activate / effect:** set `[theme] name = "<native-name>"` in the effective
  config; the next Atuin search/process loads it. No theme-install subcommand or
  daemon restart is established by these docs. A missing theme can fall back to
  unstyled output, so successful process exit alone does not verify application.
- **Preserve:** history/sync/account settings, database paths, search/keybindings
  and other theme options.
- **Current / target gap:** current code fetches official Catppuccin and selects
  its native name, or selects installed/builtin themes. Its path heuristics do not
  implement the full upstream environment resolver.

## lazygit

- **Sources:** [lazygit configuration](https://github.com/jesseduffield/lazygit/blob/master/docs/Config.md),
  [Dracula lazygit](https://github.com/dracula/lazygit), and
  [Catppuccin lazygit](https://github.com/catppuccin/lazygit).
- **Detect / acquire:** there is no native theme registry. Fetch the registered
  YAML artifact on apply. Dracula/Catppuccin theme fragments start with `theme:`,
  intended to sit beneath `gui:`.
- **Placement / activate:** resolve the effective directory using
  `lazygit --print-config-dir` and launch overrides. Merge the fragment into
  `gui.theme` without replacing the whole file. Defaults are XDG-style on Linux
  and `~/Library/Application Support/lazygit/config.yml` on macOS, with legacy
  vendor-qualified locations also possible.
- **Alternative native load:** Catppuccin's `themes-mergable/` full presets work
  through `--use-config-file=<base>,<theme>` or `LG_CONFIG_FILE`. Those launch
  options load files; they do not download a named theme.
- **Effect / preserve:** reopen lazygit. Preserve non-theme GUI fields, Git/pager
  settings, custom commands and keybindings. `gui.darkTheme`/`gui.lightTheme`,
  `gui.colorScheme`, repository and ancestor config may override `gui.theme`.
- **Current / target gap:** official Dracula/Catppuccin fragments are fetched
  and merged; complete native config discovery is not implemented.

## lazydocker

- **Sources:** [lazydocker configuration](https://github.com/jesseduffield/lazydocker/blob/master/docs/Config.md)
  and [Dracula lazydocker](https://github.com/dracula/lazydocker).
- **Detect / acquire:** fetch the registered YAML on apply. Like lazygit,
  there is no installed-theme registry, but Dracula's file already contains
  `gui.theme`; do not assume the two tools' fragment shapes are identical.
- **Placement / activate:** merge only `gui.theme` into the actual `config.yml`.
  Native docs name `~/.config/lazydocker/` on Linux and
  `~/Library/Application Support/jesseduffield/lazydocker/` on macOS. Respect XDG
  and actual app config selection instead of guessing from lazygit's defaults.
- **Effect / preserve:** close/reopen lazydocker. Retain Compose templates, stats,
  log options, custom commands and unrelated GUI state. Its terminal background
  is separate from the GUI border/selection colors.
- **Current / target gap:** current code merges embedded Dracula. It does not
  download assets; a fresh macOS path can differ from the app's documented
  vendor-qualified default and must be resolved explicitly.
  Catppuccin lazydocker is not a verified registered official port; do not reuse
  lazygit YAML or invent a repository for it.

## btop

- **Sources:** [btop theme documentation](https://github.com/aristocratos/btop#themes)
  and [actual shipped theme files](https://github.com/aristocratos/btop/tree/main/themes).
  Dracula's verified file is upstream `themes/dracula.theme`, not an assumed
  `dracula/btop` repository.
- **Detect / acquire:** locate the exact `.theme` in effective user/system theme
  paths. Dracula, Gruvbox Dark and Nord are verified shipped examples. For a
  registered missing file, retrieve that artifact only; builtins `Default`/`TTY`
  need no acquisition.
- **Placement:** normally `$XDG_CONFIG_HOME/btop/themes` or
  `~/.config/btop/themes`, including on macOS. Honor supported `--themes-dir`
  and `--config` overrides for the installed version; system themes can reside
  under the installation prefix's `share/btop/themes`.
- **Activate / effect:** change `color_theme = "<native-name>"` in the effective
  `btop.conf`, then relaunch (or select the theme in the app's options UI). Do not
  assume a universal external hot-reload command.
- **Preserve:** truecolor/background options, boxes/layout, graphs, process sort,
  intervals and filters. Avoid competing with a running UI saving its settings.
- **Current / target gap:** installed complete themes take priority. Otherwise
  Catppuccin Mocha is fetched from `catppuccin/btop`, while Dracula, Gruvbox
  Dark and Nord come from btop's own current upstream theme files. Each file is
  format-validated before the user theme is written.
  Applying a theme still does not reload an already-running btop UI.

## eza

- **Sources:** [eza-maintained YAML themes and precedence](https://github.com/eza-community/eza-themes),
  [native color schema](https://github.com/eza-community/eza/blob/main/man/eza_colors-explanation.5.md),
  and [Catppuccin eza](https://github.com/catppuccin/eza).
- **Detect / acquire:** retrieve a registered native YAML theme when absent.
  The application-maintained collection has `themes/dracula.yml` and
  `themes/gruvbox-dark.yml`; these are not Dracula-/Gruvbox-author provenance.
- **Placement / activate:** the effective `theme.yml` is itself the selection;
  copy, merge, or point a managed symlink to the chosen asset. Use
  `$EZA_CONFIG_DIR`, otherwise the platform config root plus `eza` (XDG-style
  on Linux, `~/Library/Application Support/eza` on macOS). Some versions also
  accept `theme.yaml`; do not write an unused second file over an existing setup.
- **Effect / preserve:** next eza invocation; no daemon/reload or named-theme
  command is needed. Retain custom file/extension/icon rules and unspecified styles.
- **Precedence:** `LS_COLORS` / `EZA_COLORS` can override YAML colors;
  `EXA_COLORS` is a fallback when `EZA_COLORS` is unset. `NO_COLOR` also affects
  rendering. Report conflicts rather than silently editing shell definitions.
- **Current / target gap:** Dracula, Gruvbox Dark and Catppuccin Mocha fetch
  current native YAML, validate its structure, then merge active config.
  Nord/Moonfly lack a verified eza-maintained or theme-author YAML
  in the inspected sources. Alternate `theme.yaml` discovery and
  environment-precedence reporting are not implemented.

## Implementation anchors and verification scope

- [Shared native list discovery and selectors](internal/adapter/shared_native_registry.go),
  [Neovim](internal/adapter/neovim.go),
  [tmux](internal/adapter/tmux.go),
  [delta](internal/adapter/delta.go),
  [Harlequin](internal/adapter/harlequin.go).
- [Shared file adapter registration](internal/adapter/shared_file_registry.go),
  [official source registry](internal/adapter/shared_official_sources.go),
  [Yazi flavor installers](internal/adapter/yazi_flavors.go),
  [installed Yazi family](internal/adapter/yazi_family.go),
  [btop source recipes](internal/adapter/btop_upstream.go),
  [eza Gruvbox](internal/adapter/eza_family.go),
  [Starship palettes](internal/adapter/starship_palette.go),
  [shared path resolution](internal/adapter/shared_file_helpers.go),
  [custom-first selection and skip reasons](internal/app/engine.go).

Isolated runtime checks verified fresh Catppuccin btop/Starship and Gruvbox
eza/Starship downloads, btop-maintained Dracula/Gruvbox Dark/Nord downloads
without packaged themes, and offline reapplication. Actual btop rendered an
acquired Nord `.theme`, eza emitted the selected Gruvbox RGB directory color,
and Starship reported the selected Gruvbox palette. A failed Catppuccin btop
download preserved the prior config. Earlier isolated checks also exercised
Yazi Dracula colors, syntax preview and Catppuccin-to-Dracula switching.
Dry-run remains read-only; no live theme settings or plugin-manager commands
were changed or invoked by these checks.
