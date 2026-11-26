package tmux

import (
	"strings"
	"testing"
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
	colors, err := filesTokyoNightTmuxColors([]byte(script))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(colors), "status-left") || strings.Contains(string(colors), "status-right-length") || strings.Contains(string(colors), "if-shell") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("author layout or shell code leaked into tmux theme: %s", colors)
	}
	if !strings.Contains(string(colors), "pane-active-border-style") || !strings.Contains(string(colors), "window-status-style") {
		t.Fatalf("author color styles missing: %s", colors)
	}
	if _, err := filesTokyoNightTmuxColors([]byte("set -g status-left \"layout only\"")); err == nil {
		t.Fatal("accepted a layout-only tmux script")
	}
}
