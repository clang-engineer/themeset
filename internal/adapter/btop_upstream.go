package adapter

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

const (
	filesBtopUpstreamBase          = "https://raw.githubusercontent.com/aristocratos/btop/5d03929a45e0720a4c8e2d189360ff1eb9ce1731/"
	filesBtopUpstreamLicense       = filesBtopUpstreamBase + "LICENSE"
	filesBtopUpstreamLicenseDigest = "cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30"
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
	return model.Port{Kind: "official", Name: asset, Source: filesBtopUpstreamBase + "themes/" + asset + ".theme"}, true, nil
}

func filesAcquireBtopUpstream(port model.Port, env model.Environment) ([]byte, error) {
	var digest string
	switch port.Name {
	case "dracula":
		digest = "ca5f7a7a8aab828e2b3b464393797df1edc109e1aca11c716ff7f66947a05292"
	case "gruvbox_dark":
		digest = "5a96a4398118ae1ffd3ab5c2cf613318537845304b91b3a97e61737c24f2997b"
	case "nord":
		digest = "d4b43128675e091c5e191df246552e06dcaa738f38d80b8246e971e929241cbb"
	default:
		return nil, fmt.Errorf("unknown btop official theme %q", port.Name)
	}
	if port.Source != filesBtopUpstreamBase+"themes/"+port.Name+".theme" {
		return nil, fmt.Errorf("unexpected btop official source %q", port.Source)
	}
	data, err := filesAcquireAsset(env, port.Source, digest)
	if err != nil {
		return nil, fmt.Errorf("acquire btop %s: %w", port.Name, err)
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid btop %s: %w", port.Name, err)
	}
	if _, err := filesAcquireAsset(env, filesBtopUpstreamLicense, filesBtopUpstreamLicenseDigest); err != nil {
		return nil, fmt.Errorf("acquire btop license: %w", err)
	}
	return data, nil
}
