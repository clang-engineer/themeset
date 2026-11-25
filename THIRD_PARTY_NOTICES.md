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
