package starship

import (
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/pelletier/go-toml/v2"
)

func filesPaletteFamilyTOML(data []byte, native string) ([]byte, error) {
	config, err := shared.TOML(data)
	if err != nil {
		return nil, err
	}
	if selected, ok := config["palette"]; ok && selected != native {
		return nil, fmt.Errorf("unexpected Starship palette selector %v", selected)
	}
	palettes, ok := config["palettes"].(map[string]any)
	if !ok {
		return nil, errors.New("Starship palette tables missing")
	}
	palette, ok := palettes[native].(map[string]any)
	if !ok || len(palette) == 0 {
		return nil, fmt.Errorf("Starship palette %q is empty or missing", native)
	}
	return toml.Marshal(map[string]any{
		"palette":  native,
		"palettes": map[string]any{native: palette},
	})
}
