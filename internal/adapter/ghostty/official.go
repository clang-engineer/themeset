package ghostty

import (
	"fmt"
	"strconv"
	"strings"
)

// Ghostty treats a theme as an unrestricted configuration file. Only accept
// plain hex colors so a changed upstream asset cannot set executable options.
func validateOfficialTheme(data []byte) error {
	var palette [256]bool
	colors := map[string]bool{}
	for number, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d is not a color setting", number+1)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "palette" {
			index, color, ok := strings.Cut(value, "=")
			if !ok {
				return fmt.Errorf("line %d has an invalid palette", number+1)
			}
			i, err := strconv.Atoi(index)
			if err != nil || i < 0 || i >= len(palette) || palette[i] || !ghosttyHexColor(color) {
				return fmt.Errorf("line %d has an invalid palette entry", number+1)
			}
			palette[i] = true
			continue
		}
		switch key {
		case "background", "foreground", "cursor-color", "cursor-text", "selection-background", "selection-foreground", "split-divider-color":
		default:
			return fmt.Errorf("line %d sets unsupported Ghostty option %q", number+1, key)
		}
		if colors[key] || !ghosttyHexColor(value) {
			return fmt.Errorf("line %d has an invalid %s color", number+1, key)
		}
		colors[key] = true
	}
	if !colors["background"] || !colors["foreground"] {
		return fmt.Errorf("theme must set background and foreground")
	}
	for i := range 16 {
		if !palette[i] {
			return fmt.Errorf("theme is missing palette color %d", i)
		}
	}
	return nil
}

func ghosttyHexColor(value string) bool {
	if strings.HasPrefix(value, "#") {
		value = value[1:]
	}
	if len(value) != 6 {
		return false
	}
	for i := range value {
		c := value[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
