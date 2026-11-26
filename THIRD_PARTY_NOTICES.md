# Third-party notices

## Official theme sources

Theme files are not bundled or redistributed. On apply, themeset fetches
current official sources over HTTPS and imports only the relevant native theme
settings. The upstream repositories retain their own licensing terms.

- Dracula: [fzf](https://github.com/dracula/fzf),
  [Starship](https://github.com/dracula/starship),
  [lazygit](https://github.com/dracula/lazygit), and
  [lazydocker](https://github.com/dracula/lazydocker).
- Moonfly: [official extras](https://github.com/bluz71/vim-moonfly-colors/tree/master/extras)
  for fzf, Starship, and tmux. Shell commands, prompt layouts, and tmux status
  layouts are not imported.
- Catppuccin Mocha/Mauve: [fzf](https://github.com/catppuccin/fzf),
  [lazygit](https://github.com/catppuccin/lazygit),
  [Atuin](https://github.com/catppuccin/atuin),
  [eza](https://github.com/catppuccin/eza),
  [Yazi](https://github.com/catppuccin/yazi), and
  [Yazi syntax from bat](https://github.com/catppuccin/bat).
- fzf-author Gruvbox Dark and Nord preset data:
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

These files are fetched from current official branches during apply, not embedded
in the executable. Tool adapters validate their formats before changing settings.
Current upstream content can change between applications; no checksum cache is used.

- Application-maintained Yazi Dracula flavor:
  https://github.com/yazi-rs/flavors/tree/main/dracula.yazi
  Runtime assets `flavor.toml` and `tmtheme.xml` are installed with their
  `LICENSE` and `LICENSE-tmtheme`. Independently installed flavors remain native.
- Application-maintained eza Dracula and Gruvbox Dark YAML:
  https://github.com/eza-community/eza-themes/tree/main/themes
  The theme is merged into active configuration, retaining unrelated user rules.
- Theme-author Catppuccin Mocha btop:
  https://github.com/catppuccin/btop/blob/main/themes/catppuccin_mocha.theme
  The `.theme` is installed under the effective user theme directory when absent.
- Application-maintained btop Dracula, Gruvbox Dark, and Nord:
  https://github.com/aristocratos/btop/tree/main/themes
  Already-installed system/user themes are reused instead of downloaded.
- Theme-author Catppuccin Mocha Starship:
  https://github.com/catppuccin/starship/blob/main/themes/mocha.toml
  Only `[palettes.catppuccin_mocha]` is merged, not an example prompt layout.
- Application-maintained Starship Gruvbox Rainbow:
  https://github.com/starship/starship/blob/master/docs/public/presets/toml/gruvbox-rainbow.toml
  Only `[palettes.gruvbox_dark]` is imported, without module or prompt settings.
