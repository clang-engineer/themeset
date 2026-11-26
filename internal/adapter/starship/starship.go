package starship

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "starship", Binary: "starship", Remote: true, Resolve: filesResolveStarship, Apply: filesApplyStarship}
}

func filesResolveStarship(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	if len(spec.Native) > 0 {
		data, err := shared.Read(shared.Path("starship", env), true)
		if err != nil {
			return model.Port{}, false, err
		}
		config, err := shared.TOML(data)
		if err != nil {
			return model.Port{}, false, err
		}
		palettes, _ := config["palettes"].(map[string]any)
		for _, name := range spec.Native {
			if palette, ok := palettes[name].(map[string]any); ok && len(palette) > 0 {
				return model.Port{Kind: "native", Name: name, Source: shared.StarshipPalettesSource}, true, nil
			}
		}
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyStarship(port model.Port, env model.Environment) error {
	var incoming map[string]any
	if port.File != "" || port.URL != "" || port.Kind == "official" {
		var data []byte
		if port.File != "" || port.URL != "" {
			var err error
			data, err = shared.Custom(port, env)
			if err != nil {
				return err
			}
		} else {
			if !catalog.Verified("starship", port) {
				return errors.New("unknown official Starship port")
			}
			var err error
			data, err = shared.AcquireAsset(env, port.Source)
			if err != nil {
				return fmt.Errorf("acquire %s Starship palette: %w", port.Name, err)
			}
			switch port.Format {
			case "starship-palette":
				variant := port.Variant
				if variant == "" {
					variant = port.Name
				}
				data, err = filesPaletteFamilyTOML(data, variant)
			case "starship-preset":
				data, err = filesPrepareOfficialStarshipPalette(data, port.Name)
			default:
				return fmt.Errorf("unsupported official Starship format %q", port.Format)
			}
			if err != nil {
				return fmt.Errorf("invalid %s Starship palette: %w", port.Name, err)
			}
		}
		var err error
		incoming, err = shared.TOML(data)
		if err != nil {
			return err
		}
		name, _ := incoming["palette"].(string)
		if name == "" {
			name = port.Name
		}
		palettes, _ := incoming["palettes"].(map[string]any)
		palette, ok := palettes[name].(map[string]any)
		if !ok || len(palette) == 0 {
			return errors.New("Starship custom file must define a nonempty selected [palettes.NAME] and palette selector (or port name)")
		}
		port.Name = name
		// Only the selected existing palette is imported, never module settings.
		incoming = map[string]any{name: palette}
	}
	return shared.EditTOML(shared.Path("starship", env), func(config map[string]any) error {
		palettes, err := shared.Table(config, "palettes")
		if err != nil {
			return err
		}
		if incoming != nil {
			shared.Merge(palettes, incoming)
		}
		if palette, ok := palettes[port.Name].(map[string]any); !ok || len(palette) == 0 {
			return fmt.Errorf("Starship palette %q is not defined", port.Name)
		}
		config["palette"] = port.Name
		return nil
	})
}

var filesStarshipOfficialColor = regexp.MustCompile(`^#[[:xdigit:]]{6}$`)

// Only the named palette is carried forward; prompt/module settings from
// upstream must never replace the user's Starship configuration.
func filesPrepareOfficialStarshipPalette(data []byte, name string) ([]byte, error) {
	config, err := shared.TOML(data)
	if err != nil {
		return nil, err
	}
	if config["palette"] != name {
		return nil, fmt.Errorf("Starship source does not select %q", name)
	}
	palettes, _ := config["palettes"].(map[string]any)
	palette, ok := palettes[name].(map[string]any)
	if !ok || len(palette) == 0 {
		return nil, fmt.Errorf("Starship source has no %q colors", name)
	}
	for key, value := range palette {
		color, ok := value.(string)
		if !ok || !filesStarshipOfficialColor.MatchString(color) {
			return nil, fmt.Errorf("invalid Starship color %q", key)
		}
	}
	return data, nil
}
