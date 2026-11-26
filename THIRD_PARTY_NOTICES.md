# Third-party notices

## Official theme sources

Theme files are not bundled or redistributed. On apply, themeset fetches
current official sources over HTTPS and imports only the relevant native theme
settings. The upstream repositories retain their own licensing terms.

- Dracula: [fzf](https://github.com/dracula/fzf),
  [Starship](https://github.com/dracula/starship),
  [Ghostty](https://github.com/dracula/ghostty),
  [lazygit](https://github.com/dracula/lazygit), and
  [lazydocker](https://github.com/dracula/lazydocker).
- Moonfly: [official extras](https://github.com/bluz71/vim-moonfly-colors/tree/master/extras)
  for fzf, Starship, and tmux. Shell commands, prompt layouts, and tmux status
  layouts are not imported.
- Cyberdream: [theme-author extras](https://github.com/scottmckendry/cyberdream.nvim/tree/main/extras)
  for btop, Ghostty, lazydocker, lazygit, and Yazi, with Yazi syntax from
  the upstream TextMate extra. Only theme-specific settings are imported;
  the upstream Yazi example path and unrelated application settings are ignored.
- Kanagawa Wave: [theme-author Ghostty colors](https://github.com/rebelot/kanagawa.nvim/tree/master/extras/ghostty),
  [application-maintained btop](https://github.com/aristocratos/btop/tree/main/themes),
  [eza themes](https://github.com/eza-community/eza-themes/tree/main/themes),
  and [Yazi flavor](https://github.com/dangooddd/kanagawa.yazi). Yazi
  retrieves the flavor and its TextMate syntax companion; installed Bat/Delta
  syntax themes are selected only if those applications list them.
- Everforest Dark Medium: [application-maintained btop](https://github.com/aristocratos/btop/tree/main/themes)
  and the [community Yazi flavor](https://github.com/Chromium-3-Oxide/everforest-medium.yazi)
  linked from Yazi's flavor directory. Yazi installs the flavor, syntax theme,
  and both upstream MIT license files.
- One Dark: [application-maintained btop](https://github.com/aristocratos/btop/tree/main/themes)
  and [eza native theme](https://github.com/eza-community/eza-themes/tree/main/themes).
- Monokai and Solarized Light:
  [application-maintained btop themes](https://github.com/aristocratos/btop/tree/main/themes).
- Modus Vivendi: [theme-author Ghostty colors](https://github.com/miikanissi/modus-themes.nvim/tree/master/extras/ghostty).
- Catppuccin Latte, Frappé, Macchiato, and Mocha:
  [fzf](https://github.com/catppuccin/fzf),
  [lazygit](https://github.com/catppuccin/lazygit),
  [Atuin](https://github.com/catppuccin/atuin),
  [eza](https://github.com/catppuccin/eza),
  [btop](https://github.com/catppuccin/btop),
  [delta](https://github.com/catppuccin/delta),
  [Starship](https://github.com/catppuccin/starship),
  [Ghostty](https://github.com/catppuccin/ghostty),
  [Yazi](https://github.com/catppuccin/yazi), and
  [Yazi syntax from bat](https://github.com/catppuccin/bat).
- Rosé Pine Main, Dawn, and Moon:
  [Atuin](https://github.com/rose-pine/atuin),
  [btop](https://github.com/rose-pine/btop),
  [delta](https://github.com/rose-pine/delta),
  [eza](https://github.com/rose-pine/eza),
  [fzf](https://github.com/rose-pine/fzf),
  [Ghostty](https://github.com/rose-pine/ghostty),
  [lazygit](https://github.com/rose-pine/lazygit),
  [Starship](https://github.com/rose-pine/starship), and
  [Yazi](https://github.com/rose-pine/yazi).
- fzf-author Gruvbox Dark, Nord, Everforest Dark, GitHub Dark, One Dark,
  Modus Vivendi, and Solarized Light presets:
  [fzf color themes](https://junegunn.github.io/fzf/color-themes/).
  Only literal color values are parsed from the author's site source; JavaScript
  is never executed. The site declares no license for its presets, so this is
  not represented as covered by the fzf software's MIT license.

Other native themes come from the user's installed applications/providers.

## Go dependencies

- `github.com/pelletier/go-toml/v2`: MIT,
  https://github.com/pelletier/go-toml/blob/v2.2.4/LICENSE.
- `gopkg.in/yaml.v3`: MIT and Apache-2.0; see
  https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE and NOTICE.
- `golang.org/x/term` v0.29.0 and `golang.org/x/sys` v0.30.0: BSD-3-Clause;
  identical redistribution terms retained in `licenses/golang-x.LICENSE`.

Custom theme files remain the user's responsibility. Their provenance and
licenses do not become official merely by being registered in configuration.

## Registered downloaded ports

These files are fetched from their registered upstream branches during apply, not
embedded in the executable. Tool adapters validate formats before changing
settings. Current upstream content can change between applications; no checksum
cache is used.

- Application-maintained Yazi Dracula flavor:
  https://github.com/yazi-rs/flavors/tree/main/dracula.yazi
  Runtime assets `flavor.toml` and `tmtheme.xml` are installed with their
  `LICENSE` and `LICENSE-tmtheme`. Independently installed flavors remain native.
- Application-maintained eza Dracula and Gruvbox Dark YAML:
  https://github.com/eza-community/eza-themes/tree/main/themes
  The theme is merged into active configuration, retaining unrelated user rules.
- Theme-author Catppuccin btop flavors:
  https://github.com/catppuccin/btop/tree/main/themes
  The selected `.theme` is installed under the effective user theme directory
  when absent.
- Application-maintained btop Dracula, Gruvbox Dark, and Nord:
  https://github.com/aristocratos/btop/tree/main/themes
  Already-installed system/user themes are reused instead of downloaded.
- Theme-author Catppuccin Starship flavors:
  https://github.com/catppuccin/starship/tree/main/themes
  Only the selected `[palettes.catppuccin_*]` table is merged, not an example
  prompt layout.
- Application-maintained Starship Gruvbox Rainbow:
  https://github.com/starship/starship/blob/master/docs/public/presets/toml/gruvbox-rainbow.toml
  Only `[palettes.gruvbox_dark]` is imported, without module or prompt settings.
