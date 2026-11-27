package tmux

import (
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/catalog"
)

func TestTokyoNightTmuxKeepsOnlyAuthorColors(t *testing.T) {
	const script = `#!/usr/bin/env bash
set -g mode-style "fg=#7aa2f7,bg=#3b4261"
set -g pane-active-border-style "fg=#7aa2f7"
set -g status-style "fg=#7aa2f7,bg=#16161e"
set -g status-left "#[bg=#7aa2f7] unrelated user layout"
set -g status-right-length "100"
setw -g window-status-style "NONE,fg=#a9b1d6,bg=#16161e"
if-shell 'echo should-not-run' { run-shell 'false' }
`
	colors, err := filesTmuxStyles([]byte(script), *catalog.Port("tokyo-night", "tmux").Official)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(colors), "status-left") || strings.Contains(string(colors), "status-right-length") || strings.Contains(string(colors), "if-shell") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("author layout or shell code leaked into tmux theme: %s", colors)
	}
	if !strings.Contains(string(colors), "pane-active-border-style") || !strings.Contains(string(colors), "window-status-style") {
		t.Fatalf("author color styles missing: %s", colors)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-left \"layout only\""), *catalog.Port("tokyo-night", "tmux").Official); err == nil {
		t.Fatal("accepted a layout-only tmux script")
	}
}

func TestMoonflyTmuxUsesDeclaredStyleKeys(t *testing.T) {
	port := *catalog.Port("moonfly", "tmux").Official
	const script = "set -g pane-active-border-style \"fg=#111111\"\nset -g status-style \"bg=#222222\"\nset -g window-status-current-style none\nset -g mode-style \"fg=#333333\"\nrun-shell 'false'\n"
	colors, err := filesTmuxStyles([]byte(script), port)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(colors), "window-status-current-style none") || strings.Contains(string(colors), "mode-style") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("styles escaped declared allowlist: %s", colors)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-style \"bg=#222222\"\n"), port); err == nil {
		t.Fatal("accepted a source missing the required pane border color")
	}
}

func TestNightfoxTmuxImportsStylesWithoutReplacingStatusLayout(t *testing.T) {
	port := *catalog.Port("nightfox", "tmux").Official
	const source = `#!/usr/bin/env bash
set -g pane-active-border-style "fg=#719cd6"
set -g status-style "fg=#aeafb0,bg=#131a24"
setw -g window-status-style "NONE,fg=#71839b,bg=#131a24"
set -g status-left "#[fg=#131a24] #S"
setw -g window-status-current-format "#[fg=#aeafb0] #I"
set -g status-right-length "100"
`
	styles, err := filesTmuxStyles([]byte(source), port)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(styles), "pane-active-border-style") ||
		!strings.Contains(string(styles), "window-status-style") ||
		strings.Contains(string(styles), "status-left") ||
		strings.Contains(string(styles), "window-status-current-format") ||
		strings.Contains(string(styles), "status-right-length") {
		t.Fatalf("nightfox style selection changed status layout: %s", styles)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-left \"layout only\"\n"), port); err == nil {
		t.Fatal("accepted Nightfox source without required style colors")
	}
}
