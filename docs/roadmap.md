# themeset Development Roadmap

Status: Proposed | Updated: 2026-11-27

## 1. Vision

themeset is a native-first theme manager for CLI development environments: discover, resolve and safely apply existing application-native and upstream theme ports with one command. It is not primarily a palette generator.

## 2. Competitive positioning

| Project | Reference area |
| --- | --- |
| [Pult](https://github.com/subculture-collective/pult) | Declarative registry, configuration ownership, diff and rollback |
| [Scene](https://github.com/InsaneArts/scene) | Preview, recovery and user experience |
| [recol](https://github.com/nlkli/recol) | Theme discovery and live reload |
| [Tinty](https://github.com/tinted-theming/tinty) | Template ecosystem and extensibility |
| [thcon](https://thcon.app/) | Existing-theme orchestration |

These are research leads based on public documentation, not independently tested feature claims. Revisit their implementation and licenses before adopting ideas. Preserve themeset's native-first approach rather than duplicating palette-generation ecosystems.

## 3. Engineering principles

- Preserve existing CLI commands, JSON contracts and configuration compatibility.
- Prefer user-defined ports, then verified native and upstream ports; make fallback explicit.
- Keep the engine independent of tool IDs and tool-specific paths.
- Prefer declarative data for simple adapters and Go code for complex behavior.
- Preserve user configuration, comments, permissions and symlinks; respect dotfile ownership.
- Avoid unnecessary abstractions, dependencies and hardcoded tool-specific branches in core code.
- Keep fzf integration; a new TUI is not a prerequisite.
- Preserve provenance, upstream licensing and security boundaries.

## 4. Current baseline

Initial review of engine.go, model.go, shared_registry.go, shared/files.go and shared/acquisition.go found:
- 14 registered adapters and existing adapter tests.
- Atomic file replacement, mode preservation, symlink handling, HTTPS validation, download limits and timeouts already implemented.
- Tool-specific warnings and unsupported reasons in the engine.
- Tool-specific configuration paths centralized in shared.Path.
- TOML/YAML map reserialization may discard comments and formatting.
- No engine-wide transactional recovery in the reviewed apply flow.
- Fixed default fallback and per-apply asset acquisition.

These observations are from static inspection, not a complete execution audit.

## 5. Implementation phases

### Phase 1 — Core boundaries (P0)
- [ ] Move tool-specific warnings and unsupported reasons into adapters.
- [ ] Move tool-specific path policies into adapters; retain shared path helpers.
- [ ] Audit duplicated resolve/apply logic and registry validation.
- [ ] Preserve existing behavior and add regression tests.

### Phase 2 — Safe application (P0)
- [ ] Introduce a plan describing intended file changes and side effects.
- [ ] Provide dry-run diff without configuration mutations.
- [ ] Preserve comments and formatting where feasible; prefer managed include files when supported.
- [ ] Detect managed dotfiles (e.g. chezmoi, Stow) and avoid silently overwriting source-managed settings.
- [ ] Snapshot original content, modes and file existence before changes.
- [ ] Add rollback and durable change history; detect edits made after apply before restoring.
- [ ] Define partial failure and rollback behavior, including non-reversible plugin installs and reloads.
- [ ] Test interrupted writes, symlinks, malformed configs and partial failures.

Target flow: Discover → Resolve → Plan → Validate → Snapshot → Apply → Verify → Reload → Commit or Rollback.

### Phase 3 — Registry and resolution (P1)
- [ ] Classify adapters: native selector, config patch, theme file, include file, environment, plugin and composite.
- [ ] Prototype declarative registry for simple tools; retain explicit Go adapters for complex tools.
- [ ] Validate registry schema, unique IDs, paths and port capabilities.
- [ ] Add configurable fallback policies (existing default, skip, strict, appearance-compatible).
- [ ] Expose per-theme support matrix, provenance and actual application outcomes.
- [ ] Evaluate bounded asset caching, source pinning and integrity checks; preserve explicit refresh behavior.

### Phase 4 — Tool coverage (P1)
Target approximately 30 supported tools, subject to verified upstream ports and maintenance cost.

| Area | Candidates |
| --- | --- |
| Terminal | Kitty, Alacritty, WezTerm, iTerm2, foot |
| Shell | Fish, Nushell |
| Multiplexer | Zellij |
| Editor | Helix, Vim |
| Search | Television, skim |
| Files | lf, lsd |
| Monitoring | bottom |
| AI coding | OpenCode |

For each adapter, verify native configuration semantics, official theme sources, license, installation detection, safe apply, fallback, platform support and tests. Do not count a tool as supported until verified. Use Radar for discovery without making it a runtime dependency.

### Phase 5 — Verification and experience (P2)
- [ ] Distinguish last recorded state from actual current configuration.
- [ ] Add supported live reload and report restart requirements.
- [ ] Improve preview and source visibility.
- [ ] Document custom ports and adapter contribution workflow.
- [ ] Evaluate OS light/dark integration only after core stability.

### Phase 6 — Release and adoption
- [ ] English README with clear native-first positioning, quick start and support matrix.
- [ ] Record a real multi-tool theme-switch demo.
- [ ] Review GitHub description, topics and release artifacts.
- [ ] Publish relevant community introductions without spam.
- [ ] Track stars, forks, issues and user feedback; do not treat stars as the sole success metric.

## 6. Proposed package boundaries

Keep the existing app, adapter, catalog and model packages initially. Extract engine, resolver, configedit, transaction or acquisition packages only when responsibilities and tests justify the split. A declarative registry is a proposal, not a required wholesale rewrite.

## 7. Acceptance criteria

- Existing CLI and configuration behavior remains compatible unless explicitly documented.
- Adding a normal adapter does not require tool-specific changes to the engine.
- Dry-run does not mutate user configuration.
- Failed or interrupted applications have documented recovery behavior.
- Existing user edits are not silently lost during apply or rollback.
- Every supported tool has verified source provenance and regression tests.
- Run gofmt, go vet ./... and go test ./...; report platform-specific verification limits.
- Changes are reviewed in focused commits and pull requests before merging.

## 8. Release milestones

Indicative only: v0.2 core boundaries; v0.3 safe application; v0.4 registry and resolution; v0.5 tool expansion; v0.6 verification and reload; v1.0 stabilization and public launch. Confirm actual version numbering against existing releases.

## 9. Next action

Start with the existing refactor/adapter-boundaries branch. Implement phase 1 in small changes, run regression tests and open a PR. Do not expand tool coverage before the relevant shared contracts are stable. No implementation is claimed complete by this document.
