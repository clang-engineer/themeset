package btop

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// Installed copies take precedence over the bundled upstream declaration.

func filesAcquireBtopUpstream(port model.Port, env model.Environment) ([]byte, error) {
	if port.Format != "btop-upstream" || port.Source != shared.BtopUpstreamBase+"themes/"+port.Name+".theme" {
		return nil, fmt.Errorf("unexpected btop official source %q", port.Source)
	}
	if !shared.Name(port.Name) {
		return nil, fmt.Errorf("invalid btop official name %q", port.Name)
	}
	data, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire btop %s: %w", port.Name, err)
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid btop %s: %w", port.Name, err)
	}
	if port.LicenseURL != "" {
		if _, err := shared.AcquireAsset(env, port.LicenseURL); err != nil {
			return nil, fmt.Errorf("acquire btop license: %w", err)
		}
	}
	return data, nil
}
