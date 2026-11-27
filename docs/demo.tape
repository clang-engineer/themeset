# From the repository root: vhs docs/demo.tape
# --dry-run and --home keep this recording from changing application settings.
Output docs/demo.gif
Set FontFamily "Hack Nerd Font Mono"
Set FontSize 17
Set Width 1440
Set Height 720
Set Theme "Dracula"
Set TypingSpeed 30ms

Type "mkdir -p /tmp/themeset-demo"
Enter
Sleep 500ms

Type "go run ./cmd/themeset apply --dry-run --home /tmp/themeset-demo"
Enter
Sleep 2s
Enter
Sleep 2s
Type "dracula"
Sleep 2s
Enter
Sleep 2s
