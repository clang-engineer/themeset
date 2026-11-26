package harlequin

import "testing"

func TestNativeHarlequinListReadsOnlyThemeOption(t *testing.T) {
	help := []byte("--theme [dracula|catppuccin-mocha|nord]\n  Theme to use\n--adapter TEXT\n gruvbox adapter\n")
	if !nativeHarlequinHasTheme(help, "catppuccin-mocha") {
		t.Fatal("theme was not detected")
	}
	if nativeHarlequinHasTheme(help, "gruvbox") {
		t.Fatal("unrelated option text was interpreted as a theme")
	}
	if nativeHarlequinHasTheme(help, "catppuccin") {
		t.Fatal("partial theme name matched")
	}
	if nativeHarlequinHasTheme([]byte("--theme [rose-pine-moon|dracula]\n"), "rose-pine") {
		t.Fatal("substituted a different Harlequin variant")
	}
	if !nativeHarlequinHasTheme([]byte("--theme [rose-pine|dracula]\n"), "rose-pine") {
		t.Fatal("did not detect listed native theme")
	}
}
