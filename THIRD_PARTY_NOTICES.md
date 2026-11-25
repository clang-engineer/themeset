# Third-party notices

## Bundled official ports

- Dracula fzf integration: https://github.com/dracula/fzf at
  `f4c7758a813ca73b69e9c954c001586c33858c24`. Existing color options only;
  license retained in `internal/adapter/assets/fzf-dracula.LICENSE`.
- Dracula Starship integration: https://github.com/dracula/starship at
  `920e9f46ccc25beee15ed7fe0baddabdfeaaf92a`. Existing palette only; module
  configuration is not imported. License retained in
  `internal/adapter/assets/starship-dracula.LICENSE`.
- Moonfly fzf, Starship, and tmux integrations:
  https://github.com/bluz71/vim-moonfly-colors at
  `2c883835f5b7b380a0a9e80695c420f7efebdfe4`.
  `extras/moonfly-fzf.sh` supplies exact color options, with shell syntax removed
  and `--color VALUE` rewritten as `--color=VALUE`.
  `extras/moonfly-starship.toml` is retained unchanged; only its selected palette
  is imported, never module configuration.
  `extras/moonfly.tmux` supplies only its six exact style commands; status
  layouts, window formats, and the shell conditional are omitted.
  Full MIT license, source hashes, and adaptation details are retained in
  `internal/adapter/assets/moonfly.LICENSE`.
- Catppuccin fzf Mocha:
  https://github.com/catppuccin/fzf at
  `7508f8141286fb95249100a2b5325960320dcf32`.
  Exact color options from `themes/catppuccin-fzf-mocha.rc`, with shell export
  syntax removed. MIT license and source details retained in
  `internal/adapter/assets/fzf-catppuccin-mocha.LICENSE`.
- fzf-author Gruvbox Dark and Nord preset color data:
  https://github.com/junegunn/junegunn.github.io at
  `8112383a1781605e7b181c5b717327439a0fb981`,
  `static/js/fzf-color-themes.js`. Only literal color data is included; the
  generator code is not copied. `current-fg/bg/hl` use equivalent stable fzf
  aliases `fg+/bg+/hl+`. No website license was declared; this is explicitly
  recorded in `internal/adapter/assets/fzf-upstream-presets.NOTICE`.
  Neither the fzf repository's MIT license nor a theme project's license is
  asserted to license the website data.
- Dracula lazygit:
  https://github.com/dracula/lazygit at
  `f60ffa9e53d011739d86f0dcb681efacf3bc3cfb`, `config/dracula.yml`.
  The upstream theme mapping is imported into `gui.theme`, without importing
  unrelated preferences. MIT license retained in
  `internal/adapter/assets/lazygit-dracula.LICENSE`.
- Dracula lazydocker:
  https://github.com/dracula/lazydocker at
  `a699a726fab2f2c6925211606f0a28151d0562db`, `config/dracula.yml`.
  Only `gui.theme` is imported. MIT license retained in
  `internal/adapter/assets/lazydocker-dracula.LICENSE`.
- Catppuccin Mocha ports select the upstream Mauve accent:
  - lazygit: https://github.com/catppuccin/lazygit at
    `1a4b26cbc2b5be05cc5494d0d7f764120a2cf86d`;
    `themes/mocha/mauve.yml`.
  - Atuin: https://github.com/catppuccin/atuin at
    `68aa64b77573c235044b614e752a781701af4eec`;
    `themes/mocha/catppuccin-mocha-mauve.toml`.
  - eza: https://github.com/catppuccin/eza at
    `70f805f6cc27fa5b91750b75afb4296a0ec7fec9`;
    `themes/mocha/catppuccin-mocha-mauve.yml`.
  - Yazi: https://github.com/catppuccin/yazi at
    `d62802be39210ea10e54b3e3b09735c6cb9e57c1`;
    `themes/mocha/catppuccin-mocha-mauve.toml`. Only native UI style
    tables are imported; separators, icons, file rules and layout remain local.
  - Yazi syntax: https://github.com/catppuccin/bat at
    `6810349b28055dce54076712fc05fc68da4b8ec0`;
    `themes/Catppuccin Mocha.tmTheme`. Installed unchanged with a local path
    replacing the Yazi upstream example syntax path.
  Full MIT licenses and source details are retained in the corresponding
  `internal/adapter/assets/<app>-catppuccin-mocha.LICENSE` files and
  `yazi-catppuccin-mocha-syntax.LICENSE`.

These are official port assets, not colors synthesized from another application.
Other native themes are selected from the user's installed application/providers
and are not redistributed by this project.

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

These files are acquired during apply, not embedded in the executable.
Immutable URLs and SHA-256 hashes are defined in `internal/adapter/yazi_dracula.go`
and `internal/adapter/eza_dracula.go`; the acquisition helper verifies cache and
downloaded bytes before use.

- Application-maintained Yazi Dracula flavor:
  https://github.com/yazi-rs/flavors/tree/38c08d8eb9ea0d28c8c911a249a08f27ba2151ac/dracula.yazi
  Runtime assets `flavor.toml` and `tmtheme.xml` are installed with their exact
  `LICENSE` and `LICENSE-tmtheme`. Compatible independently installed flavors
  are reused without changing their provenance or ownership.
- Application-maintained eza Dracula native YAML:
  https://github.com/eza-community/eza-themes/blob/21184b55bfe34e8473c8160f9de0c03e4f76857f/themes/dracula.yml
  The theme and upstream MIT `LICENSE` are retained in the verified cache.
  Exact source values are preserved; active configuration is merged, not copied
  over unrelated user rules.
