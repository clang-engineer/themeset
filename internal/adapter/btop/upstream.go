package btop

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// btop itself ships these native theme files. Installed copies always win;
// this registry only supplies a file when the installed package omits it.
func filesResolveBtopUpstream(name string) (model.Port, bool, error) {
	asset := ""
	switch name {
	case "dracula", "nord":
		asset = name
	case "gruvbox":
		asset = "gruvbox_dark"
	}
	if asset == "" {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: asset, Source: shared.BtopUpstreamBase + "themes/" + asset + ".theme"}, true, nil
}

func filesAcquireBtopUpstream(port model.Port, env model.Environment) ([]byte, error) {
	switch port.Name {
	case "dracula", "gruvbox_dark", "nord":
	default:
		return nil, fmt.Errorf("unknown btop official theme %q", port.Name)
	}
	if port.Source != shared.BtopUpstreamBase+"themes/"+port.Name+".theme" {
		return nil, fmt.Errorf("unexpected btop official source %q", port.Source)
	}
	data, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire btop %s: %w", port.Name, err)
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid btop %s: %w", port.Name, err)
	}
	if _, err := shared.AcquireAsset(env, shared.BtopUpstreamLicense); err != nil {
		return nil, fmt.Errorf("acquire btop license: %w", err)
	}
	return data, nil
}
