package adapter

import (
	_ "embed"
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

// Exact existing upstream assets; the sibling LICENSE files record revisions.
//
//go:embed assets/starship-dracula.toml
var filesStarshipDracula []byte

//go:embed assets/starship-moonfly.toml
var filesStarshipMoonfly []byte

const filesStarshipSource = "https://github.com/dracula/starship/blob/920e9f46ccc25beee15ed7fe0baddabdfeaaf92a/starship.toml"

func filesResolveStarship(name string, env model.Environment) (model.Port, bool, error) {
	switch name {
	case "dracula":
		return model.Port{Kind: "official", Name: name, Source: filesStarshipSource}, true, nil
	case "moonfly":
		return model.Port{Kind: "official", Name: name, Source: filesMoonflySource + "moonfly-starship.toml"}, true, nil
	}
	if port, found, err := filesResolveRosePinePort("starship", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolvePaletteFamily("starship", name, env); found || err != nil {
		return port, found, err
	}
	data, err := filesRead(filesPath("starship", env), true)
	if err != nil {
		return model.Port{}, false, err
	}
	config, err := filesTOML(data)
	if err != nil {
		return model.Port{}, false, err
	}
	palettes, _ := config["palettes"].(map[string]any)
	if palette, ok := palettes[name].(map[string]any); ok && len(palette) > 0 {
		return model.Port{Kind: "native", Name: name, Source: "https://starship.rs/config/#color-palette"}, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyStarship(port model.Port, env model.Environment) error {
	var incoming map[string]any
	if port.File != "" || port.Kind == "official" {
		var data []byte
		if port.File != "" {
			var err error
			data, err = filesCustom(port, env)
			if err != nil {
				return err
			}
		} else {
			switch port.Name {
			case "rose-pine":
				var err error
				data, err = filesAcquireRosePinePort("starship", port, env)
				if err != nil {
					return err
				}
			case "dracula":
				data = filesStarshipDracula
			case "moonfly":
				data = filesStarshipMoonfly
			default:
				var err error
				data, err = filesAcquirePaletteFamily("starship", port.Name, env)
				if err != nil {
					return err
				}
			}
		}
		var err error
		incoming, err = filesTOML(data)
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
	return filesEditTOML(filesPath("starship", env), func(config map[string]any) error {
		palettes, err := filesTable(config, "palettes")
		if err != nil {
			return err
		}
		if incoming != nil {
			filesMerge(palettes, incoming)
		}
		if palette, ok := palettes[port.Name].(map[string]any); !ok || len(palette) == 0 {
			return fmt.Errorf("Starship palette %q is not defined", port.Name)
		}
		config["palette"] = port.Name
		return nil
	})
}
