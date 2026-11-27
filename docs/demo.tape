# From the repository root: vhs docs/demo.tape
# Build a local binary and isolate HOME; require bat for the native syntax preview.
Output docs/demo.gif
Set FontFamily "Hack Nerd Font Mono"
Set FontSize 17
Set Width 1440
Set Height 720
Set Theme "Dracula"
Set Shell "bash"
Set TypingSpeed 30ms

# CI/recording shells may set NO_COLOR or TERM=dumb, suppressing fzf's
# ANSI preview even though themeset supplies the native bat colors.

Hide
Type 'unset NO_COLOR && export TERM=xterm-256color COLORTERM=truecolor && demo_home=$(mktemp -d) && go build -o "$demo_home/themeset" ./cmd/themeset && alias themeset="$demo_home/themeset" && export HOME="$demo_home" XDG_CONFIG_HOME="$demo_home/.config" XDG_DATA_HOME="$demo_home/.local/share" XDG_CACHE_HOME="$demo_home/.cache" && clear'
Enter
Sleep 2s
Show

Type "themeset apply --dry-run"
Enter
Sleep 2s
Enter
Sleep 2s
Type "dracula"
Sleep 3s
Enter
Sleep 2s

Hide
Type 'test -n "$demo_home" && rm -rf "$demo_home"'
Enter
